// Package ws 提供 WebSocket Hub 基础设施(M10 统一 Hub 的基础设施部分)。
// 依据 docs/总-技术选型.md §6:M10 统一 Hub 推送业务实时数据。
// 本包只管理连接生命周期、topic 订阅和广播;业务订阅语义由 notify 模块实现。
package ws

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

// Hub 维护活跃连接,支持按 topic 定向广播。
type Hub struct {
	mu           sync.RWMutex
	topics       map[string]map[*Conn]struct{} // topic -> 连接集合。
	originPolicy OriginPolicy
}

// Conn 是一条客户端连接(含其订阅的 topic 集合)。
type Conn struct {
	socket *websocket.Conn
	send   chan []byte
	topics map[string]struct{}
}

// SendChan 暴露只写发送通道给业务层,便于在订阅成功后补发当前快照。
func (c *Conn) SendChan() chan<- []byte { return c.send }

// OriginPolicy 统一校验 WebSocket Origin,避免各模块各自放行跨站请求。
type OriginPolicy struct {
	allowed map[string]struct{}
}

// NewOriginPolicy 构造 Origin 白名单策略;同源请求会被默认允许。
func NewOriginPolicy(origins []string) OriginPolicy {
	allowed := make(map[string]struct{}, len(origins))
	for _, raw := range origins {
		if origin, present, valid := normalizeOrigin(raw); present && valid {
			allowed[origin] = struct{}{}
		}
	}
	return OriginPolicy{allowed: allowed}
}

// Check 判断请求 Origin 是否为同源或配置白名单。
func (p OriginPolicy) Check(r *http.Request) bool {
	origin, present, valid := normalizeOrigin(r.Header.Get("Origin"))
	if !present {
		return true
	}
	if !valid {
		return false
	}
	if origin == requestOrigin(r) {
		return true
	}
	_, ok := p.allowed[origin]
	return ok
}

// normalizeOrigin 提取 scheme+host,同时区分缺失和非法 Origin,避免非法头被当作无 Origin。
func normalizeOrigin(raw string) (origin string, present bool, valid bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false, true
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", true, false
	}
	return parsed.Scheme + "://" + parsed.Host, true, true
}

// requestOrigin 根据反向代理头、URL 和 TLS 信息推导当前请求同源值。
func requestOrigin(r *http.Request) string {
	scheme := "http"
	if forwarded := r.Header.Get("X-Forwarded-Proto"); forwarded == "http" || forwarded == "https" {
		scheme = forwarded
	} else if r.URL != nil && (r.URL.Scheme == "http" || r.URL.Scheme == "https") {
		scheme = r.URL.Scheme
	} else if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

// NewHub 创建带统一 Origin 策略的 Hub,供 notify 和进度流复用同一连接基础设施。
func NewHub(policy OriginPolicy) *Hub {
	return &Hub{topics: make(map[string]map[*Conn]struct{}), originPolicy: policy}
}

// Serve 建立固定订阅型 WebSocket 连接,订阅失败时关闭 socket 并向上返回原因。
func (h *Hub) Serve(w http.ResponseWriter, r *http.Request, subscribe func(c *Conn) error) error {
	upgrader := h.upgrader()
	socket, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	// 第一步:为已升级连接建立发送队列和 topic 集合,避免并发写 socket。
	conn := &Conn{
		socket: socket,
		send:   make(chan []byte, 32),
		topics: make(map[string]struct{}),
	}
	// 第二步:交给业务回调完成鉴权后的 topic 订阅;失败必须关闭连接并保留错误链。
	if err := subscribe(conn); err != nil {
		if closeErr := socket.Close(); closeErr != nil {
			return errors.Join(err, closeErr)
		}
		return err
	}
	// 第三步:启动写循环并用读循环感知客户端断开,退出后统一清理订阅状态。
	go conn.writeLoop()
	conn.readLoop()
	h.Unsubscribe(conn)
	close(conn.send)
	return socket.Close()
}

// ServeInteractive 建立 WebSocket 连接并把读循环交给业务层处理。
// M10 使用该入口接收 subscribe/unsubscribe 消息;M2/M3 的固定 topic 进度流继续使用 Serve。
func (h *Hub) ServeInteractive(w http.ResponseWriter, r *http.Request, handle func(c *Conn) error) error {
	upgrader := h.upgrader()
	socket, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	conn := &Conn{
		socket: socket,
		send:   make(chan []byte, 32),
		topics: make(map[string]struct{}),
	}
	// 业务层负责读取客户端指令,Hub 只提供发送队列与最终清理。
	go conn.writeLoop()
	err = handle(conn)
	h.Unsubscribe(conn)
	close(conn.send)
	if closeErr := socket.Close(); closeErr != nil && err == nil {
		return closeErr
	}
	return err
}

// ReadJSON 从客户端读取一条 JSON 消息。
func (c *Conn) ReadJSON(v any) error { return c.socket.ReadJSON(v) }

// SendJSON 向客户端发送一条 JSON 消息,复用发送队列以避免并发写同一 WebSocket。
func (c *Conn) SendJSON(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.send <- data
	return nil
}

// Subscribe 把连接加入某 topic,同时维护连接侧反向索引用于关闭时批量清理。
func (h *Hub) Subscribe(c *Conn, topic string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.topics[topic] == nil {
		h.topics[topic] = make(map[*Conn]struct{})
	}
	h.topics[topic][c] = struct{}{}
	c.topics[topic] = struct{}{}
}

// Unsubscribe 把连接移出所有 topic(连接关闭时调用)。
func (h *Hub) Unsubscribe(c *Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for topic := range c.topics {
		if set := h.topics[topic]; set != nil {
			delete(set, c)
			if len(set) == 0 {
				delete(h.topics, topic)
			}
		}
	}
}

// Broadcast 向某 topic 所有连接推送;发送缓冲满则跳过(不阻塞 Hub,实时数据容忍丢帧)。
func (h *Hub) Broadcast(topic string, payload []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.topics[topic] {
		select {
		case c.send <- payload:
		default:
		}
	}
}

// writeLoop 把服务端广播写入客户端连接。
func (c *Conn) writeLoop() {
	for msg := range c.send {
		if err := c.socket.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}

// readLoop 持续读取直到客户端断开;当前仅用于保持连接活跃。
func (c *Conn) readLoop() {
	for {
		if _, _, err := c.socket.ReadMessage(); err != nil {
			return
		}
	}
}

// upgrader 构造带统一 Origin 策略的 WebSocket upgrader。
func (h *Hub) upgrader() websocket.Upgrader {
	return websocket.Upgrader{CheckOrigin: h.originPolicy.Check}
}
