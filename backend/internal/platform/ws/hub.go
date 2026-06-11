// ws 提供 WebSocket Hub 基础设施,统一管理连接、Origin 校验、订阅与广播。
package ws

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Hub 维护活跃连接并支持按 topic 定向广播。
type Hub struct {
	mu           sync.RWMutex
	topics       map[string]map[*Conn]struct{}
	sessions     map[SessionKey]*Conn
	originPolicy OriginPolicy
	options      HubOptions
}

// Conn 表示一条 WebSocket 连接及其订阅集合。
type Conn struct {
	socket  *websocket.Conn
	send    chan []byte
	topics  map[string]struct{}
	done    chan struct{}
	hub     *Hub
	session SessionKey
}

// SessionKey 标识一条需要单端互斥的连接主体。
type SessionKey struct {
	TenantID  int64
	AccountID int64
}

// HubOptions 描述 WebSocket 连接的统一生命周期边界。
type HubOptions struct {
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	PingInterval time.Duration
}

// OriginPolicy 是统一的 WebSocket Origin 白名单策略。
type OriginPolicy struct {
	allowed map[string]struct{}
}

// NewOriginPolicy 根据配置白名单构造 Origin 校验策略。
func NewOriginPolicy(origins []string) OriginPolicy {
	allowed := make(map[string]struct{}, len(origins))
	for _, raw := range origins {
		if origin, present, valid := normalizeOrigin(raw); present && valid {
			allowed[origin] = struct{}{}
		}
	}
	return OriginPolicy{allowed: allowed}
}

// Check 判断请求 Origin 是否为同源或白名单内来源。
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

// NewHub 创建带统一 Origin 策略和连接生命周期约束的 Hub。
func NewHub(policy OriginPolicy, options HubOptions) *Hub {
	options = normalizeHubOptions(options)
	return &Hub{
		topics:       make(map[string]map[*Conn]struct{}),
		sessions:     make(map[SessionKey]*Conn),
		originPolicy: policy,
		options:      options,
	}
}

// SendChan 暴露只写发送通道,便于业务层在订阅成功后补发快照。
func (c *Conn) SendChan() chan<- []byte {
	return c.send
}

// ReadJSON 从客户端读取一条 JSON 消息。
func (c *Conn) ReadJSON(v any) error {
	return c.socket.ReadJSON(v)
}

// SendJSON 向客户端发送一条 JSON 消息,复用发送队列避免并发写同一连接。
func (c *Conn) SendJSON(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.send <- data
	return nil
}

// Reader 返回 WebSocket 文本/二进制消息的连续读取流,供终端等交互场景透传输入。
func (c *Conn) Reader() io.Reader {
	return &connReader{conn: c}
}

// Writer 返回写入 WebSocket 二进制消息的流式 writer,供终端等交互场景透传输出。
func (c *Conn) Writer() io.Writer {
	return connWriter{conn: c}
}

// BindSession 把连接绑定到单端互斥主体,若旧连接仍在线则主动关闭旧连接。
func (c *Conn) BindSession(session SessionKey) error {
	if c == nil || c.hub == nil {
		return fmt.Errorf("WebSocket 连接未挂接 Hub")
	}
	if session.TenantID <= 0 || session.AccountID <= 0 {
		return fmt.Errorf("WebSocket 会话主体非法")
	}
	c.hub.bindSession(c, session)
	return nil
}

// Serve 建立固定订阅型连接,由业务回调完成鉴权和初始订阅。
func (h *Hub) Serve(w http.ResponseWriter, r *http.Request, subscribe func(c *Conn) error) error {
	upgrader := h.upgrader()
	socket, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	conn := &Conn{
		socket: socket,
		send:   make(chan []byte, 32),
		topics: make(map[string]struct{}),
		done:   make(chan struct{}),
		hub:    h,
	}
	// 第一步:升级成功后立即建立发送队列和订阅容器,避免业务回调期间并发写 socket。
	if err := subscribe(conn); err != nil {
		if closeErr := socket.Close(); closeErr != nil {
			return errors.Join(err, closeErr)
		}
		return err
	}
	// 第二步:写循环负责服务端推送;读循环只用于感知客户端断开并触发清理。
	go conn.writeLoop()
	go conn.pingLoop()
	conn.readLoop()
	close(conn.done)
	h.Unsubscribe(conn)
	close(conn.send)
	return socket.Close()
}

// ServeInteractive 建立由业务层主动处理读循环的交互式连接。
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
		done:   make(chan struct{}),
		hub:    h,
	}
	go conn.writeLoop()
	go conn.pingLoop()
	err = handle(conn)
	close(conn.done)
	h.Unsubscribe(conn)
	close(conn.send)
	if closeErr := socket.Close(); closeErr != nil && err == nil {
		return closeErr
	}
	return err
}

// Subscribe 把连接加入指定 topic,并维护反向索引供断连时清理。
func (h *Hub) Subscribe(c *Conn, topic string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.topics[topic] == nil {
		h.topics[topic] = make(map[*Conn]struct{})
	}
	h.topics[topic][c] = struct{}{}
	c.topics[topic] = struct{}{}
}

// Unsubscribe 把连接从所有 topic 中移除。
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
	if c.session.TenantID > 0 && c.session.AccountID > 0 {
		if current, ok := h.sessions[c.session]; ok && current == c {
			delete(h.sessions, c.session)
		}
	}
}

// Broadcast 向指定 topic 的所有连接广播;发送缓冲满时跳过以避免阻塞整个 Hub。
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

// CloseSession 主动关闭指定主体的在线连接,供上层在单端登录踢线等场景复用。
func (h *Hub) CloseSession(session SessionKey) error {
	h.mu.RLock()
	conn := h.sessions[session]
	h.mu.RUnlock()
	if conn == nil {
		return nil
	}
	return conn.closeWithControl(websocket.ClosePolicyViolation, "session_replaced")
}

// writeLoop 把服务端广播顺序写入客户端连接。
func (c *Conn) writeLoop() {
	for msg := range c.send {
		// 每次写入前设置统一写超时,避免慢连接永久占住发送协程。
		if err := c.socket.SetWriteDeadline(time.Now().Add(c.hub.options.WriteTimeout)); err != nil {
			return
		}
		if err := c.socket.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}

// readLoop 持续读取直到客户端断开;当前固定订阅场景不解析消息体。
func (c *Conn) readLoop() {
	// 统一设置读超时与 pong 续期,确保死连接能被及时回收而不是无限悬挂。
	c.socket.SetReadLimit(1 << 20)
	_ = c.socket.SetReadDeadline(time.Now().Add(c.hub.options.ReadTimeout))
	c.socket.SetPongHandler(func(appData string) error {
		_ = c.socket.SetReadDeadline(time.Now().Add(c.hub.options.ReadTimeout))
		return nil
	})
	c.socket.SetPingHandler(func(appData string) error {
		_ = c.socket.SetReadDeadline(time.Now().Add(c.hub.options.ReadTimeout))
		return c.writeControl(websocket.PongMessage, []byte(appData))
	})
	for {
		if _, _, err := c.socket.ReadMessage(); err != nil {
			return
		}
		_ = c.socket.SetReadDeadline(time.Now().Add(c.hub.options.ReadTimeout))
	}
}

// pingLoop 定期发送 ping,让对端回 pong 以持续刷新读超时并尽早识别失活连接。
func (c *Conn) pingLoop() {
	ticker := time.NewTicker(c.hub.options.PingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := c.writeControl(websocket.PingMessage, []byte("ping")); err != nil {
				return
			}
		case <-c.done:
			return
		}
	}
}

// upgrader 构造带统一 Origin 校验策略的 WebSocket upgrader。
func (h *Hub) upgrader() websocket.Upgrader {
	return websocket.Upgrader{CheckOrigin: h.originPolicy.Check}
}

// bindSession 建立主体到连接的唯一索引,并在同主体重连时主动淘汰旧连接。
func (h *Hub) bindSession(conn *Conn, session SessionKey) {
	h.mu.Lock()
	previous := h.sessions[session]
	h.sessions[session] = conn
	conn.session = session
	h.mu.Unlock()

	if previous != nil && previous != conn {
		_ = previous.closeWithControl(websocket.ClosePolicyViolation, "session_replaced")
	}
}

// writeControl 在统一写超时保护下发送控制帧。
func (c *Conn) writeControl(messageType int, payload []byte) error {
	return c.socket.WriteControl(messageType, payload, time.Now().Add(c.hub.options.WriteTimeout))
}

// closeWithControl 先发关闭控制帧,再关闭底层连接,避免客户端感知为无原因断链。
func (c *Conn) closeWithControl(code int, reason string) error {
	message := websocket.FormatCloseMessage(code, reason)
	writeErr := c.writeControl(websocket.CloseMessage, message)
	closeErr := c.socket.Close()
	return errors.Join(writeErr, closeErr)
}

type connReader struct {
	conn *Conn
	buf  *bytes.Reader
}

// Read 从下一条 WebSocket 消息读取字节,消息边界由底层连接维护。
func (r *connReader) Read(p []byte) (int, error) {
	for {
		if r.buf != nil && r.buf.Len() > 0 {
			return r.buf.Read(p)
		}
		messageType, data, err := r.conn.socket.ReadMessage()
		if err != nil {
			return 0, err
		}
		if messageType == websocket.TextMessage || messageType == websocket.BinaryMessage {
			if len(data) == 0 {
				continue
			}
			r.buf = bytes.NewReader(data)
		}
	}
}

type connWriter struct {
	conn *Conn
}

// Write 把字节作为一条 WebSocket 消息加入统一发送队列。
func (w connWriter) Write(p []byte) (int, error) {
	data := append([]byte(nil), p...)
	select {
	case w.conn.send <- data:
		return len(p), nil
	case <-w.conn.done:
		return 0, io.ErrClosedPipe
	}
}

// normalizeHubOptions 统一补默认超时,避免平台层各处出现不一致的连接回收口径。
func normalizeHubOptions(options HubOptions) HubOptions {
	if options.ReadTimeout <= 0 {
		options.ReadTimeout = 60 * time.Second
	}
	if options.WriteTimeout <= 0 {
		options.WriteTimeout = 10 * time.Second
	}
	if options.PingInterval <= 0 {
		options.PingInterval = 25 * time.Second
	}
	return options
}

// normalizeOrigin 解析 Origin 为 scheme://host,并区分缺失与格式非法。
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

// requestOrigin 根据代理头、URL 和 TLS 推导当前请求的同源 origin。
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
