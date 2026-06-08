// M10 WebSocket 协议:鉴权连接、订阅业务 topic 并按租户隔离映射到底层 Hub。
package notify

import (
	"net/http"
	"strconv"
	"strings"

	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"
)

// subscribeMessage 是客户端发送的订阅协议。
type subscribeMessage struct {
	Action string   `json:"action"`
	Topics []string `json:"topics"`
}

// wsAck 是订阅处理返回给客户端的确认消息。
type wsAck struct {
	Action  string   `json:"action"`
	Topics  []string `json:"topics,omitempty"`
	Code    string   `json:"code"`
	Message string   `json:"message"`
}

// ServeWS 建立 M10 统一 WebSocket 通道。
func ServeWS(hub *ws.Hub, authMgr *auth.Manager, w http.ResponseWriter, r *http.Request) error {
	if hub == nil || authMgr == nil {
		return apperr.ErrNotifyRealtimeUnavailable
	}
	claims, err := authMgr.VerifyAccess(strings.TrimSpace(r.URL.Query().Get("token")))
	if err != nil {
		return apperr.ErrUnauthorized.WithCause(err)
	}
	id := tenant.Identity{TenantID: claims.TenantID, AccountID: claims.AccountID, IsPlatform: claims.IsPlatform}
	ctx := tenant.WithContext(r.Context(), id)
	req := r.WithContext(ctx)
	return hub.ServeInteractive(w, req, func(conn *ws.Conn) error {
		return serveSubscriptionLoop(hub, conn, id)
	})
}

// serveSubscriptionLoop 持续读取客户端订阅消息直到断开。
func serveSubscriptionLoop(hub *ws.Hub, conn *ws.Conn, id tenant.Identity) error {
	for {
		var msg subscribeMessage
		if err := conn.ReadJSON(&msg); err != nil {
			return nil
		}
		if strings.ToLower(strings.TrimSpace(msg.Action)) != "subscribe" || len(msg.Topics) == 0 {
			if err := conn.SendJSON(subscriptionProtocolErrorAck(msg.Action)); err != nil {
				return err
			}
			continue
		}
		subscribed := make([]string, 0, len(msg.Topics))
		for _, topic := range msg.Topics {
			internalTopic, err := authorizeTopic(id, topic)
			if err != nil {
				if err := conn.SendJSON(wsAck{Action: "subscribe", Code: apperr.ErrNotifyTopicForbidden.Code, Message: apperr.ErrNotifyTopicForbidden.Message}); err != nil {
					return err
				}
				continue
			}
			hub.Subscribe(conn, internalTopic)
			subscribed = append(subscribed, topic)
		}
		if err := conn.SendJSON(wsAck{Action: "subscribe", Topics: subscribed, Code: "0", Message: "ok"}); err != nil {
			return err
		}
	}
}

// subscriptionProtocolErrorAck 统一生成 M10 WebSocket 协议错误响应,避免协议错误落到全局 BadRequest。
func subscriptionProtocolErrorAck(action string) wsAck {
	return wsAck{Action: action, Code: apperr.ErrNotifySubscriptionInvalid.Code, Message: apperr.ErrNotifySubscriptionInvalid.Message}
}

// authorizeTopic 校验客户端可订阅的业务 topic,并补租户隔离前缀。
func authorizeTopic(id tenant.Identity, topic string) (string, error) {
	topic = strings.TrimSpace(topic)
	if topic == "" || strings.HasPrefix(topic, "tenant:") {
		return "", apperr.ErrNotifyTopicForbidden
	}
	parts := strings.Split(topic, ":")
	if len(parts) < 2 {
		return "", apperr.ErrNotifyTopicForbidden
	}
	switch parts[0] {
	case "notify":
		accountID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || accountID != id.AccountID {
			return "", apperr.ErrNotifyTopicForbidden
		}
	case "alert":
		tenantID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || tenantID != id.TenantID {
			return "", apperr.ErrNotifyTopicForbidden
		}
	}
	if id.TenantID <= 0 {
		return "", apperr.ErrNotifyTopicForbidden
	}
	return tenantTopic(id.TenantID, topic), nil
}
