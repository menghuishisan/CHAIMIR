// 管理本人消息、偏好和未读缓存。
package notify

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
)

// Inbox 查询当前用户站内信。
func (s *Service) Inbox(ctx context.Context, isRead *bool, typ string, page, size int) ([]NotificationDTO, int64, pagex.Facets, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, 0, nil, err
	}
	page, size = pagex.Normalize(page, size)
	var out []NotificationDTO
	var total int64
	var byRead pagex.FacetGroup
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, total, err = tx.ListNotifications(ctx, id.AccountID, isRead, typ, page, size)
		if err != nil {
			return err
		}
		// 「全部 / 未读」分段计数:不套用已读筛选
		byRead, err = tx.CountNotificationsByRead(ctx, id.AccountID, typ)
		return err
	})
	if err != nil {
		return nil, 0, nil, apperr.ErrNotifyInboxQueryInvalid.WithCause(err)
	}
	return out, total, pagex.Facets{"is_read": byRead}, nil
}

// Unread 查询当前用户未读数。
func (s *Service) Unread(ctx context.Context) (UnreadDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return UnreadDTO{}, err
	}
	n, err := s.countUnread(ctx, id.TenantID, id.AccountID)
	if err != nil {
		return UnreadDTO{}, apperr.ErrNotifyInboxQueryInvalid.WithCause(err)
	}
	return UnreadDTO{Unread: n}, nil
}

// MarkRead 标记单条站内信已读。
func (s *Service) MarkRead(ctx context.Context, notificationID int64) (NotificationDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return NotificationDTO{}, err
	}
	var out NotificationDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.MarkNotificationRead(ctx, notificationID, id.AccountID)
		return err
	})
	if err != nil {
		return NotificationDTO{}, apperr.ErrNotifyInboxNotFound.WithCause(err)
	}
	if err := s.refreshUnread(ctx, id.TenantID, id.AccountID); err != nil {
		logging.ErrorContext(ctx, "刷新通知未读数失败", err.Error(), slog.Int64("tenant_id", id.TenantID), slog.Int64("account_id", id.AccountID))
	}
	return out, nil
}

// MarkAllRead 标记当前用户全部站内信已读。
func (s *Service) MarkAllRead(ctx context.Context) error {
	id, err := currentIdentity(ctx)
	if err != nil {
		return err
	}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		return tx.MarkAllNotificationsRead(ctx, id.AccountID)
	}); err != nil {
		return apperr.ErrNotifyInboxQueryInvalid.WithCause(err)
	}
	if err := s.refreshUnread(ctx, id.TenantID, id.AccountID); err != nil {
		logging.ErrorContext(ctx, "刷新通知未读数失败", err.Error(), slog.Int64("tenant_id", id.TenantID), slog.Int64("account_id", id.AccountID))
	}
	return nil
}

// DeleteNotification 删除当前用户站内信。
func (s *Service) DeleteNotification(ctx context.Context, notificationID int64) (NotificationDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return NotificationDTO{}, err
	}
	var out NotificationDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.DeleteNotification(ctx, notificationID, id.AccountID)
		return err
	})
	if err != nil {
		return NotificationDTO{}, apperr.ErrNotifyInboxNotFound.WithCause(err)
	}
	if err := s.refreshUnread(ctx, id.TenantID, id.AccountID); err != nil {
		logging.ErrorContext(ctx, "刷新通知未读数失败", err.Error(), slog.Int64("tenant_id", id.TenantID), slog.Int64("account_id", id.AccountID))
	}
	return out, nil
}

// ListPreferences 查询当前用户通知偏好。
func (s *Service) ListPreferences(ctx context.Context) ([]PreferenceDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var out []PreferenceDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.ListPreferences(ctx, id.AccountID)
		return err
	})
	return out, err
}

// UpsertPreference 设置当前用户通知偏好。
func (s *Service) UpsertPreference(ctx context.Context, req PreferenceRequest) (PreferenceDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return PreferenceDTO{}, err
	}
	req.Type = strings.TrimSpace(req.Type)
	if req.Type == "" {
		return PreferenceDTO{}, apperr.ErrNotifyRequestInvalid
	}
	var out PreferenceDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		tpl, err := tx.GetNotificationTemplate(ctx, req.Type)
		if err != nil {
			return apperr.ErrNotifyTemplateUnavailable.WithCause(err)
		}
		if tpl.Force && !req.Enabled {
			return apperr.ErrNotifyPreferenceLocked
		}
		out, err = tx.UpsertPreference(ctx, s.ids.Generate(), id.TenantID, id.AccountID, req.Type, req.Enabled)
		return err
	})
	return out, err
}

// refreshUnread 重算未读数并通过统一实时通道推送给用户。
func (s *Service) refreshUnread(ctx context.Context, tenantID, accountID int64) error {
	if err := s.redis.Delete(ctx, unreadKey(tenantID, accountID)); err != nil {
		return err
	}
	unread, err := s.countUnread(ctx, tenantID, accountID)
	if err != nil {
		return err
	}
	data, err := jsonx.AnyBytes(map[string]any{"type": "unread", "unread": unread}, apperr.ErrNotifyInboxQueryInvalid)
	if err != nil {
		return err
	}
	s.hub.Broadcast(personalNotifyTopic(tenantID, accountID), data)
	return nil
}

// countUnread 在租户事务中读取用户未读站内信数量。
func (s *Service) countUnread(ctx context.Context, tenantID, accountID int64) (int64, error) {
	key := unreadKey(tenantID, accountID)
	if unread, ok, err := s.redis.GetInt64(ctx, key); err != nil {
		return 0, err
	} else if ok {
		return unread, nil
	}
	var unread int64
	err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		unread, err = tx.CountUnread(ctx, accountID)
		return err
	})
	if err != nil {
		return 0, err
	}
	if err := s.redis.SetInt64(ctx, key, unread, time.Duration(s.cfg.UnreadTTLHours)*time.Hour); err != nil {
		return 0, err
	}
	return unread, nil
}

// unreadKey 生成 M10 未读缓存键。
func unreadKey(tenantID, accountID int64) string {
	return fmt.Sprintf("tenant:%s:unread:%s", ids.Format(tenantID), ids.Format(accountID))
}

// RunCleanupOnce 执行一次站内信过期清理任务。
func (s *Service) RunCleanupOnce(ctx context.Context) error {
	cutoff := timex.Now().Add(-time.Duration(s.cfg.RetentionDays) * 24 * time.Hour)
	return s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		return tx.DeleteExpiredNotifications(ctx, cutoff)
	})
}
