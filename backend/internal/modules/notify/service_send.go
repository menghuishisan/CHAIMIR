// 按模板、接收偏好和幂等标识投递站内信。
package notify

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
)

// Send 渲染模板并按接收人偏好写入站内信。
func (s *Service) Send(ctx context.Context, req contracts.NotifySendRequest) error {
	input, err := validateSendRequest(SendRequest{RequestID: req.RequestID, TenantID: ids.ID(req.TenantID), Type: req.Type, Receivers: req.Receivers, Params: req.Params, Link: req.Link})
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
			rows = append(rows, notificationRecord{ID: s.ids.Generate(), TenantID: input.TenantID.Int64(), ReceiverID: receiverID, Type: input.Type, Title: title, Content: content, Link: input.Link, SourceRef: input.RequestID})
			delivered = append(delivered, receiverID)
		}
		if len(rows) == 0 {
			return nil
		}
		if input.RequestID != "" {
			persisted, err := tx.CountNotificationsBySource(ctx, input.TenantID.Int64(), input.RequestID, delivered)
			if err != nil {
				return apperr.ErrNotifySendFailed.WithCause(err)
			}
			if persisted == int64(len(rows)) {
				return nil
			}
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

// checkRateLimit 使用 Redis 窗口计数限制通知发送频率。
func (s *Service) checkRateLimit(ctx context.Context, tenantID int64, typ string) error {
	key := fmt.Sprintf("tenant:%s:notify:send:%s", ids.Format(tenantID), strings.ToLower(strings.TrimSpace(typ)))
	count, err := s.redis.IncrWithTTL(ctx, key, time.Duration(s.cfg.SendRateWindowSeconds)*time.Second)
	if err != nil {
		return apperr.ErrNotifySendFailed.WithCause(err)
	}
	if count > int64(s.cfg.SendRateMax) {
		return apperr.ErrNotifyRateLimited
	}
	return nil
}
