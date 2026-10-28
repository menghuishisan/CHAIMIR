// ws 提供 WebSocket Hub 基础设施,统一管理连接、Origin 校验、订阅与广播。
package ws

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"

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
	socket    *websocket.Conn
	send      chan outboundMessage
	topics    map[string]struct{}
	done      <-chan struct{}
	ctx       context.Context
	cancel    context.CancelCauseFunc
	closeOnce sync.Once
	closeErr  error
	hub       *Hub
	session   SessionKey
}

// outboundMessage 在同一发送队列保留负载类型,区分业务文本和原始终端字节。
type outboundMessage struct {
	messageType int
	payload     []byte
	written     chan error
}

// SessionKey 标识一条需要单端互斥的连接主体；Scope 允许同一账号在不同受控资源上独立连接和精确撤销。
type SessionKey struct {
	TenantID  int64
	AccountID int64
	Scope     string
}

// HubOptions 描述 WebSocket 连接的统一生命周期边界。
type HubOptions struct {
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	PingInterval time.Duration
	ReadLimit    int64
}

// OriginPolicy 是统一的 WebSocket Origin 白名单策略。
type OriginPolicy struct {
	allowed        map[string]struct{}
	trustedProxies []*net.IPNet
}

// NewOriginPolicy 根据配置白名单构造 Origin 校验策略。
func NewOriginPolicy(origins []string, trustedProxies []string) OriginPolicy {
	allowed := make(map[string]struct{}, len(origins))
	for _, raw := range origins {
		if origin, present, valid := normalizeOrigin(raw); present && valid {
			allowed[origin] = struct{}{}
		}
	}
	trusted := make([]*net.IPNet, 0, len(trustedProxies))
	for _, raw := range trustedProxies {
		if network := parseTrustedProxy(raw); network != nil {
			trusted = append(trusted, network)
		}
	}
	return OriginPolicy{allowed: allowed, trustedProxies: trusted}
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
	if origin == p.requestOrigin(r) {
		return true
	}
	_, ok := p.allowed[origin]
	return ok
}

// requestOrigin 根据受信代理头、URL 和 TLS 推导当前请求的同源 origin。
func (p OriginPolicy) requestOrigin(r *http.Request) string {
	scheme := "http"
	if p.isTrustedProxy(r) {
		if forwarded := r.Header.Get("X-Forwarded-Proto"); forwarded == "http" || forwarded == "https" {
			scheme = forwarded
		}
	} else if r.URL != nil && (r.URL.Scheme == "http" || r.URL.Scheme == "https") {
		scheme = r.URL.Scheme
	} else if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

// isTrustedProxy 只允许来自配置代理网段的请求使用转发协议头。
func (p OriginPolicy) isTrustedProxy(r *http.Request) bool {
	if len(p.trustedProxies) == 0 || r == nil {
		return false
	}
	host := r.RemoteAddr
	if parsedHost, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		host = parsedHost
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	if ip == nil {
		return false
	}
	for _, network := range p.trustedProxies {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// parseTrustedProxy 解析单个代理 IP 或 CIDR,非法值按不可信处理。
func parseTrustedProxy(raw string) *net.IPNet {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	if ip := net.ParseIP(value); ip != nil {
		bits := 128
		if ip.To4() != nil {
			ip = ip.To4()
			bits = 32
		}
		return &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)}
	}
	_, network, err := net.ParseCIDR(value)
	if err != nil {
		return nil
	}
	return network
}

// NewHub 创建带统一 Origin 策略和连接生命周期约束的 Hub。
func NewHub(policy OriginPolicy, options HubOptions) (*Hub, error) {
	if err := validateHubOptions(options); err != nil {
		return nil, err
	}
	return &Hub{
		topics:       make(map[string]map[*Conn]struct{}),
		sessions:     make(map[SessionKey]*Conn),
		originPolicy: policy,
		options:      options,
	}, nil
}

// ReadJSON 从客户端读取一条 JSON 消息。
func (c *Conn) ReadJSON(v any) error {
	if err := c.socket.ReadJSON(v); err != nil {
		c.close(err)
		return err
	}
	if err := c.refreshReadDeadline(); err != nil {
		c.close(err)
		return err
	}
	return nil
}

// Context 保留请求身份与trace,并在连接结束时取消关联执行。
func (c *Conn) Context() context.Context {
	return c.ctx
}

// SendJSON 向客户端发送一条 JSON 消息,复用发送队列避免并发写同一连接。
func (c *Conn) SendJSON(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if !c.enqueue(websocket.TextMessage, data) {
		return io.ErrClosedPipe
	}
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

// BindSession 转移主体的唯一连接索引;旧连接关闭异常只记录,不否定已完成的新连接登记。
func (c *Conn) BindSession(ctx context.Context, session SessionKey) error {
	if c == nil || c.hub == nil {
		return fmt.Errorf("WebSocket 连接未挂接 Hub")
	}
	if session.TenantID <= 0 || session.AccountID <= 0 {
		return fmt.Errorf("WebSocket 会话主体非法")
	}
	return c.hub.bindSession(ctx, c, session)
}

// Serve 建立固定订阅型连接,由业务回调完成鉴权和初始订阅。
func (h *Hub) Serve(w http.ResponseWriter, r *http.Request, subscribe func(c *Conn) error) error {
	if h == nil {
		return fmt.Errorf("WebSocket Hub 未初始化")
	}
	if subscribe == nil {
		return fmt.Errorf("WebSocket 订阅回调不能为空")
	}
	if !websocket.IsWebSocketUpgrade(r) {
		return fmt.Errorf("WebSocket 协议升级请求不完整")
	}
	upgrader := h.upgrader()
	socket, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	conn := h.newConn(r.Context(), socket)
	if err := conn.initializeReadLifecycle(); err != nil {
		conn.close(err)
		logging.ErrorContext(r.Context(), "WebSocket 读生命周期初始化失败", apperr.ErrInternal.WithCause(errors.Join(err, conn.closeErr)).LogString())
		return nil
	}
	// 第一步:升级成功后立即建立发送队列和订阅容器,避免业务回调期间并发写 socket。
	if err := subscribe(conn); err != nil {
		// 回调可能已经登记会话或订阅,初始化失败不能留下指向已关闭连接的索引。
		conn.close(err)
		logging.ErrorContext(r.Context(), "WebSocket 订阅初始化失败", apperr.ErrInternal.WithCause(errors.Join(err, conn.closeErr)).LogString())
		return nil
	}
	// 第二步:写循环负责服务端推送;读循环只用于感知客户端断开并触发清理。
	go conn.writeLoop()
	go conn.pingLoop()
	readErr := conn.readLoop()
	conn.close(readErr)
	logConnectionError(r, "WebSocket 连接异常结束", readErr)
	return nil
}

// ServeInteractive 建立由业务层主动处理读循环的交互式连接。
func (h *Hub) ServeInteractive(w http.ResponseWriter, r *http.Request, handle func(c *Conn) error) error {
	if h == nil {
		return fmt.Errorf("WebSocket Hub 未初始化")
	}
	if handle == nil {
		return fmt.Errorf("WebSocket 处理回调不能为空")
	}
	if !websocket.IsWebSocketUpgrade(r) {
		return fmt.Errorf("WebSocket 协议升级请求不完整")
	}
	upgrader := h.upgrader()
	socket, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	conn := h.newConn(r.Context(), socket)
	if err := conn.initializeReadLifecycle(); err != nil {
		conn.close(err)
		logging.ErrorContext(r.Context(), "WebSocket 交互读生命周期初始化失败", apperr.ErrInternal.WithCause(errors.Join(err, conn.closeErr)).LogString())
		return nil
	}
	go conn.writeLoop()
	go conn.pingLoop()
	handleErr := handle(conn)
	if handleErr == nil && conn.ctx.Err() == nil {
		// 正常执行结束时先发关闭控制帧,避免直接断开底层连接被浏览器视为传输失败。
		handleErr = conn.closeWithControl(websocket.CloseNormalClosure, "")
	} else {
		conn.close(handleErr)
	}
	// 协议升级成功后连接已被 hijack,业务流结束或断开不能再返回给 HTTP handler 写 JSON 响应。
	// 升级前的错误仍由调用方按统一错误结构返回给客户端;升级后错误在这里进入结构化日志。
	logConnectionError(r, "WebSocket 交互连接异常结束", errors.Join(handleErr, context.Cause(conn.ctx)))
	return nil
}

// newConn 为两类入口建立同一队列与连接context,父请求结束时终止底层I/O。
func (h *Hub) newConn(parent context.Context, socket *websocket.Conn) *Conn {
	ctx, cancel := context.WithCancelCause(parent)
	conn := &Conn{
		socket: socket,
		send:   make(chan outboundMessage, 32),
		topics: make(map[string]struct{}),
		done:   ctx.Done(),
		ctx:    ctx,
		cancel: cancel,
		hub:    h,
	}
	context.AfterFunc(ctx, func() { conn.close(context.Cause(ctx)) })
	return conn
}

// close 幂等结束连接并保留首个原因,关闭错误仅记内部日志,不包含任何负载。
func (c *Conn) close(cause error) {
	c.closeOnce.Do(func() {
		if cause == nil {
			cause = io.ErrClosedPipe
		}
		c.cancel(cause)
		c.closeErr = c.socket.Close()
		c.hub.Unsubscribe(c)
		if c.closeErr != nil {
			logging.ErrorContext(c.ctx, "WebSocket 连接关闭失败", apperr.ErrInternal.WithCause(c.closeErr).LogString())
		}
	})
}

// connectionError 把结束原因交给仍在等待的流式读写者,保留实际网络错误。
func (c *Conn) connectionError() error {
	if cause := context.Cause(c.ctx); cause != nil {
		return cause
	}
	return io.ErrClosedPipe
}

// logConnectionError 记录升级后的非预期连接错误,正常关闭和客户端断连不作为服务故障上报。
func logConnectionError(r *http.Request, message string, err error) {
	if err == nil {
		return
	}
	// 应用失败必须保留,不能因同一原因链里包含正常断连而整组忽略。
	if expectedConnectionEnd(err) {
		return
	}
	logging.ErrorContext(r.Context(), message, apperr.ErrInternal.WithCause(err).LogString())
}

// expectedConnectionEnd 只在全部原因均为正常断开时忽略日志,不掩盖同组协议或执行失败。
func expectedConnectionEnd(err error) bool {
	if err == nil {
		return true
	}
	if _, isAppError := apperr.As(err); isAppError {
		return false
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		for _, cause := range joined.Unwrap() {
			if !expectedConnectionEnd(cause) {
				return false
			}
		}
		return true
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return expectedConnectionEnd(wrapped.Unwrap())
	}
	return errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) || errors.Is(err, net.ErrClosed) ||
		websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseNoStatusReceived)
}

// Subscribe 把连接加入指定 topic,并维护反向索引供断连时清理。
func (h *Hub) Subscribe(c *Conn, topic string) error {
	if h == nil {
		return fmt.Errorf("WebSocket Hub 未初始化")
	}
	if c == nil {
		return fmt.Errorf("WebSocket 连接为空")
	}
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return fmt.Errorf("WebSocket topic 不能为空")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if c.ctx.Err() != nil {
		return c.connectionError()
	}
	if h.topics[topic] == nil {
		h.topics[topic] = make(map[*Conn]struct{})
	}
	h.topics[topic][c] = struct{}{}
	c.topics[topic] = struct{}{}
	return nil
}

// Unsubscribe 把连接从所有 topic 中移除。
func (h *Hub) Unsubscribe(c *Conn) {
	if h == nil || c == nil {
		return
	}
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
func (h *Hub) Broadcast(topic string, payload []byte) int {
	if h == nil {
		return 0
	}
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return 0
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	delivered := 0
	for c := range h.topics[topic] {
		if c.enqueue(websocket.TextMessage, payload) {
			delivered++
		}
	}
	return delivered
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

// CloseAll 在服务进入 draining 状态时关闭全部 WebSocket 连接,让客户端重新连接到仍然就绪的副本。
func (h *Hub) CloseAll(reason string) error {
	if h == nil {
		return nil
	}
	h.mu.RLock()
	connections := make(map[*Conn]SessionKey)
	for _, conn := range h.sessions {
		if conn != nil {
			connections[conn] = conn.session
		}
	}
	for _, topicConnections := range h.topics {
		for conn := range topicConnections {
			if conn != nil {
				connections[conn] = conn.session
			}
		}
	}
	h.mu.RUnlock()
	var failures []error
	for conn, session := range connections {
		if err := conn.closeWithControl(websocket.CloseGoingAway, reason); err != nil {
			failures = append(failures, fmt.Errorf("关闭 WebSocket 连接失败: tenant_id=%d account_id=%d scope=%s: %w", session.TenantID, session.AccountID, session.Scope, err))
		}
	}
	return errors.Join(failures...)
}

// writeLoop 按入队类型顺序写入消息,不把原始终端字节改成UTF-8文本。
func (c *Conn) writeLoop() {
	for {
		var msg outboundMessage
		select {
		case <-c.done:
			return
		case msg = <-c.send:
		}
		if c.ctx.Err() != nil {
			return
		}
		// 每次写入前设置统一写超时,避免慢连接永久占住发送协程。
		err := c.socket.SetWriteDeadline(time.Now().Add(c.hub.options.WriteTimeout))
		if err == nil {
			err = c.socket.WriteMessage(msg.messageType, msg.payload)
		}
		if msg.written != nil {
			msg.written <- err
		}
		if err != nil {
			c.close(err)
			logging.ErrorContext(c.ctx, "WebSocket 消息写入失败", apperr.ErrInternal.WithCause(errors.Join(err, c.closeErr)).LogString())
			return
		}
	}
}

// readLoop 持续读取直到客户端断开;当前固定订阅场景不解析消息体。
func (c *Conn) readLoop() error {
	for {
		if _, _, err := c.socket.ReadMessage(); err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return nil
			}
			return err
		}
		if err := c.refreshReadDeadline(); err != nil {
			return err
		}
	}
}

// initializeReadLifecycle 为所有连接统一设置消息上限、空闲超时和控制帧续期。
func (c *Conn) initializeReadLifecycle() error {
	c.socket.SetReadLimit(c.hub.options.ReadLimit)
	if err := c.refreshReadDeadline(); err != nil {
		return err
	}
	c.socket.SetPongHandler(func(string) error {
		return c.refreshReadDeadline()
	})
	c.socket.SetPingHandler(func(appData string) error {
		if err := c.refreshReadDeadline(); err != nil {
			return err
		}
		return c.writeControl(websocket.PongMessage, []byte(appData))
	})
	return nil
}

// refreshReadDeadline 在收到有效消息或控制帧后续期连接读截止时间。
func (c *Conn) refreshReadDeadline() error {
	return c.socket.SetReadDeadline(time.Now().Add(c.hub.options.ReadTimeout))
}

// pingLoop 定期发送 ping,让对端回 pong 以持续刷新读超时并尽早识别失活连接。
func (c *Conn) pingLoop() {
	ticker := time.NewTicker(c.hub.options.PingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := c.writeControl(websocket.PingMessage, []byte("ping")); err != nil {
				c.close(err)
				logging.ErrorContext(c.ctx, "WebSocket 心跳写入失败", apperr.ErrInternal.WithCause(errors.Join(err, c.closeErr)).LogString())
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

// bindSession 在锁内转移唯一索引,旧连接关闭失败不回滚也不传给新连接。
func (h *Hub) bindSession(ctx context.Context, conn *Conn, session SessionKey) error {
	h.mu.Lock()
	if conn.ctx.Err() != nil {
		h.mu.Unlock()
		return conn.connectionError()
	}
	previous := h.sessions[session]
	h.sessions[session] = conn
	conn.session = session
	h.mu.Unlock()

	if previous != nil && previous != conn {
		if err := previous.closeWithControl(websocket.ClosePolicyViolation, "session_replaced"); err != nil {
			logging.ErrorContext(ctx, "WebSocket 旧连接清理失败,新连接已登记", err.Error(),
				slog.Int64("tenant_id", session.TenantID), slog.Int64("account_id", session.AccountID), slog.String("scope", session.Scope))
		}
	}
	return nil
}

// writeControl 在统一写超时保护下发送控制帧。
func (c *Conn) writeControl(messageType int, payload []byte) error {
	return c.socket.WriteControl(messageType, payload, time.Now().Add(c.hub.options.WriteTimeout))
}

// closeWithControl 先发关闭控制帧,再关闭底层连接,避免客户端感知为无原因断链。
func (c *Conn) closeWithControl(code int, reason string) error {
	message := websocket.FormatCloseMessage(code, reason)
	writeErr := c.writeControl(websocket.CloseMessage, message)
	c.close(writeErr)
	return errors.Join(writeErr, c.closeErr)
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
			r.conn.close(err)
			return 0, err
		}
		if err := r.conn.refreshReadDeadline(); err != nil {
			r.conn.close(err)
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

// Write 经唯一队列等待原始二进制消息实际写入,背压到PTY且不提前丢弃尾块。
func (w connWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if w.conn.ctx.Err() != nil {
		return 0, w.conn.connectionError()
	}
	data := append([]byte(nil), p...)
	written := make(chan error, 1)
	select {
	case w.conn.send <- outboundMessage{messageType: websocket.BinaryMessage, payload: data, written: written}:
	case <-w.conn.done:
		return 0, w.conn.connectionError()
	}
	select {
	case err := <-written:
		if err != nil {
			return 0, err
		}
		return len(p), nil
	case <-w.conn.done:
		return 0, w.conn.connectionError()
	}
}

// enqueue 为业务文本非阻塞入队;消费者不关闭队列,结束信号消除发送与关闭竞争。
func (c *Conn) enqueue(messageType int, data []byte) bool {
	if c == nil || c.ctx.Err() != nil {
		return false
	}
	select {
	case c.send <- outboundMessage{messageType: messageType, payload: data}:
		return true
	case <-c.done:
		return false
	default:
		return false
	}
}

// validateHubOptions 校验组合根注入的连接生命周期阈值,避免基础层用硬编码默认值兜底。
func validateHubOptions(options HubOptions) error {
	if options.ReadTimeout <= 0 {
		return fmt.Errorf("WebSocket 读超时必须大于 0")
	}
	if options.WriteTimeout <= 0 {
		return fmt.Errorf("WebSocket 写超时必须大于 0")
	}
	if options.PingInterval <= 0 {
		return fmt.Errorf("WebSocket ping 间隔必须大于 0")
	}
	if options.ReadLimit <= 0 {
		return fmt.Errorf("WebSocket 单消息读取上限必须大于 0")
	}
	return nil
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
