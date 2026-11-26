// 校验实时订阅并转发业务负载。
package notify

import (
	"context"
	"encoding/json"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"
)

// HandleSubscribe 处理交互式 WebSocket 订阅。
func (s *Service) HandleSubscribe(ctx context.Context, conn *ws.Conn) error {
	id, err := currentIdentity(ctx)
	if err != nil {
		return err
	}
	if err := conn.BindSession(ctx, ws.SessionKey{TenantID: id.TenantID, AccountID: id.AccountID}); err != nil {
		return apperr.ErrNotifyChannelUnavailable.WithCause(err)
	}
	if err := s.hub.Subscribe(conn, personalNotifyTopic(id.TenantID, id.AccountID)); err != nil {
		return apperr.ErrNotifyChannelUnavailable.WithCause(err)
	}
	for {
		var msg SubscribeMessage
		if err := conn.ReadJSON(&msg); err != nil {
			return err
		}
		if strings.TrimSpace(msg.Action) != "subscribe" || len(msg.Topics) == 0 {
			return apperr.ErrNotifySubscribeInvalid
		}
		for _, topic := range msg.Topics {
			if err := s.authorizeTopic(ctx, id, topic); err != nil {
				return err
			}
			if err := s.hub.Subscribe(conn, strings.TrimSpace(topic)); err != nil {
				return apperr.ErrNotifyChannelUnavailable.WithCause(err)
			}
		}
		if err := conn.SendJSON(map[string]any{"type": "subscribed", "topics": msg.Topics}); err != nil {
			return apperr.ErrNotifyChannelUnavailable.WithCause(err)
		}
	}
}

// authorizeTopic 对赛事榜单复核 M8 权限，其余主题保持 M10 的租户边界。
func (s *Service) authorizeTopic(ctx context.Context, id tenant.Identity, topic string) error {
	topic = strings.TrimSpace(topic)
	if matches := businessTopicPattern.FindStringSubmatch(topic); len(matches) == 5 && matches[2] == "contest" && matches[4] == "leaderboard" {
		organizerID, validOrganizer := ids.Parse(matches[1])
		contestID, validContest := ids.Parse(matches[3])
		if !validOrganizer || !validContest {
			return apperr.ErrNotifySubscribeInvalid
		}
		if err := s.contests.AuthorizeLeaderboardSubscription(ctx, organizerID, contestID); err != nil {
			return apperr.ErrNotifyTopicForbidden.WithCause(err)
		}
		return nil
	}
	return authorizeTenantTopic(id.TenantID, id.AccountID, topic)
}

// CloseSession 关闭指定用户实时连接。
func (s *Service) CloseSession(ctx context.Context, tenantID, accountID int64) error {
	return s.hub.CloseSession(ws.SessionKey{TenantID: tenantID, AccountID: accountID})
}

// Push 向统一 WebSocket topic 推送业务实时消息。
func (s *Service) Push(ctx context.Context, req contracts.NotifyPushRequest) error {
	if req.TenantID <= 0 || strings.TrimSpace(req.Topic) == "" || !json.Valid(req.Payload) {
		return apperr.ErrNotifyPushFailed
	}
	if err := ValidatePushTopic(req.TenantID, req.Topic); err != nil {
		return err
	}
	data, err := encodePushEnvelope(req.Topic, req.Payload)
	if err != nil {
		return apperr.ErrNotifyPushFailed.WithCause(err)
	}
	s.hub.Broadcast(req.Topic, data)
	return nil
}

// encodePushEnvelope 保持业务模块已经序列化的负载字节不变,只补充 M10 统一 topic 信封。
func encodePushEnvelope(topic string, payload json.RawMessage) ([]byte, error) {
	return jsonx.AnyBytes(struct {
		Topic   string          `json:"topic"`
		Payload json.RawMessage `json:"payload"`
	}{Topic: topic, Payload: payload}, apperr.ErrNotifyPushFailed)
}
