// notify service_contract 文件实现 M10 对其他模块开放的站内信与实时推送契约。
package notify

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
)

// Send 渲染模板并按接收人偏好写入站内信。
func (s *Service) Send(ctx context.Context, req contracts.NotifySendRequest) error {
	input, err := validateSendRequest(SendRequest{TenantID: ids.ID(req.TenantID), Type: req.Type, Receivers: req.Receivers, Params: req.Params, Link: req.Link})
	if err != nil {
		return err
	}
	delivered := make([]int64, 0, len(input.Receivers))
	err = s.store.TenantTx(ctx, input.TenantID.Int64(), func(ctx context.Context, tx TxStore) error {
		tpl, err := tx.GetNotificationTemplate(ctx, input.Type)
		if err != nil {
			return apperr.ErrNotifyTemplateUnavailable.WithCause(err)
		}
		title, content, err := renderNotificationTemplate(tpl, input.Params)
		if err != nil {
			return err
		}
		rows := make([]notificationRecord, 0, len(input.Receivers))
		for _, receiverID := range input.Receivers {
			enabled := true
			if !tpl.Force {
				enabled, err = tx.PreferenceEnabled(ctx, input.TenantID.Int64(), receiverID, input.Type)
				if err != nil {
					return apperr.ErrNotifySendFailed.WithCause(err)
				}
			}
			if !enabled {
				continue
			}
			rows = append(rows, notificationRecord{ID: s.ids.Generate(), TenantID: input.TenantID.Int64(), ReceiverID: receiverID, Type: input.Type, Title: title, Content: content, Link: input.Link})
			delivered = append(delivered, receiverID)
		}
		if len(rows) == 0 {
			return nil
		}
		if err := s.checkRateLimit(ctx, input.TenantID.Int64(), input.Type); err != nil {
			return err
		}
		return tx.CreateNotifications(ctx, rows)
	})
	if err != nil {
		return err
	}
	for _, receiverID := range delivered {
		if err := s.refreshUnread(ctx, input.TenantID.Int64(), receiverID); err != nil {
			logging.ErrorContext(ctx, "刷新通知未读数失败", err.Error(), slog.Int64("tenant_id", input.TenantID.Int64()), slog.Int64("receiver_id", receiverID))
		}
	}
	return nil
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
