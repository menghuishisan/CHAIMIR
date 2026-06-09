// ws_test 校验 WebSocket Origin 白名单策略,防止默认放行任意跨站连接。
package ws

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestOriginPolicyRejectsUnlistedOrigin 确认不在白名单中的跨站 Origin 会被拒绝。
func TestOriginPolicyRejectsUnlistedOrigin(t *testing.T) {
	policy := NewOriginPolicy([]string{"https://chaimir.example.edu"})
	req, err := http.NewRequest(http.MethodGet, "https://api.example.edu/ws", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Origin", "https://evil.example.net")

	if policy.Check(req) {
		t.Fatalf("unexpectedly allowed unlisted origin")
	}
}

// TestOriginPolicyAllowsSameHostAndConfiguredOrigin 确认同源与配置白名单可建立 WebSocket。
func TestOriginPolicyAllowsSameHostAndConfiguredOrigin(t *testing.T) {
	policy := NewOriginPolicy([]string{"https://chaimir.example.edu"})
	sameHost, err := http.NewRequest(http.MethodGet, "https://api.example.edu/ws", nil)
	if err != nil {
		t.Fatalf("create same host request: %v", err)
	}
	sameHost.Header.Set("Origin", "https://api.example.edu")
	if !policy.Check(sameHost) {
		t.Fatalf("same host origin should be allowed")
	}

	listed, err := http.NewRequest(http.MethodGet, "https://api.example.edu/ws", nil)
	if err != nil {
		t.Fatalf("create listed origin request: %v", err)
	}
	listed.Header.Set("Origin", "https://chaimir.example.edu")
	if !policy.Check(listed) {
		t.Fatalf("configured origin should be allowed")
	}
}

// TestOriginPolicyRejectsMalformedOrigin 确认格式非法的 Origin 不会被当作空值放行。
func TestOriginPolicyRejectsMalformedOrigin(t *testing.T) {
	policy := NewOriginPolicy([]string{"https://chaimir.example.edu"})
	req, err := http.NewRequest(http.MethodGet, "https://api.example.edu/ws", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Origin", "://bad-origin")
	if policy.Check(req) {
		t.Fatalf("malformed origin must be rejected")
	}
}

// TestHubRegisterSessionClosesOldConnection 确认同一主体建立新连接时会主动关闭旧连接,满足单端登录联动要求。
func TestHubRegisterSessionClosesOldConnection(t *testing.T) {
	hub := NewHub(NewOriginPolicy(nil), HubOptions{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = hub.ServeInteractive(w, r, func(c *Conn) error {
			if err := c.BindSession(SessionKey{TenantID: 10, AccountID: 1001}); err != nil {
				return err
			}
			<-r.Context().Done()
			return nil
		})
	}))
	defer server.Close()

	first := dialWebSocket(t, server.URL)
	defer func() { _ = first.Close() }()
	waitForWSReady(t, first)

	second := dialWebSocket(t, server.URL)
	defer func() { _ = second.Close() }()
	waitForWSReady(t, second)

	_ = first.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := first.ReadMessage(); err == nil {
		t.Fatalf("old connection should be closed after session rebind")
	}
}

// TestHubHeartbeatRespondsToPingAndRefreshesDeadline 确认平台层心跳会回 pong 并刷新读超时。
func TestHubHeartbeatRespondsToPingAndRefreshesDeadline(t *testing.T) {
	hub := NewHub(NewOriginPolicy(nil), HubOptions{
		ReadTimeout:  200 * time.Millisecond,
		WriteTimeout: time.Second,
		PingInterval: 50 * time.Millisecond,
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = hub.ServeInteractive(w, r, func(c *Conn) error {
			wait := make(chan struct{})
			go func() {
				defer close(wait)
				c.readLoop()
			}()
			<-wait
			return nil
		})
	}))
	defer server.Close()

	conn := dialWebSocket(t, server.URL)
	defer func() { _ = conn.Close() }()
	waitForWSReady(t, conn)

	done := make(chan error, 1)
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				done <- err
				return
			}
		}
	}()

	// 等待时长故意超过读超时;若没有服务端 ping/pong 续期,连接会被提前关闭。
	time.Sleep(350 * time.Millisecond)
	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"still-alive"}`)); err != nil {
		t.Fatalf("write after heartbeat window: %v", err)
	}

	select {
	case err := <-done:
		t.Fatalf("connection should stay alive under heartbeat, got %v", err)
	default:
	}
}

// dialWebSocket 建立测试用 WebSocket 连接并复用统一 URL 转换逻辑。
func dialWebSocket(t *testing.T, rawURL string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(rawURL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	return conn
}

// waitForWSReady 发送一条 JSON 数据,确保连接端写循环和读循环已经就绪。
func waitForWSReady(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"type": "ready"})
	if err != nil {
		t.Fatalf("marshal ready payload: %v", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
		t.Fatalf("write ready payload: %v", err)
	}
}
