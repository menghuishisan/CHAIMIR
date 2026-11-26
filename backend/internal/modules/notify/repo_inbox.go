// 访问通知模板、个人消息和接收偏好。
package notify

import (
	"context"
	"strings"
	"time"

	"chaimir/internal/modules/notify/internal/sqlcgen"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
)

// GetNotificationTemplate 查询通知模板。
func (t *txStore) GetNotificationTemplate(ctx context.Context, typ string) (notificationTemplate, error) {
	row, err := t.q.GetNotificationTemplate(ctx, typ)
	if err != nil {
		return notificationTemplate{}, err
	}
	return templateFromRow(row), nil
}

// CountNotificationsBySource 统计同一业务事件已写入的收件人,供重投幂等判断使用。
func (t *txStore) CountNotificationsBySource(ctx context.Context, tenantID int64, sourceRef string, receiverIDs []int64) (int64, error) {
	return t.q.CountNotificationsBySource(ctx, sqlcgen.CountNotificationsBySourceParams{
		TenantID:    tenantID,
		SourceRef:   pgtypex.Text(sourceRef),
		ReceiverIds: receiverIDs,
	})
}

// CreateNotifications 批量写入站内信。
func (t *txStore) CreateNotifications(ctx context.Context, records []notificationRecord) error {
	now := timex.Now()
	for _, item := range records {
		if err := t.q.CreateNotification(ctx, sqlcgen.CreateNotificationParams{
			ID:         item.ID,
			TenantID:   item.TenantID,
			ReceiverID: item.ReceiverID,
			Type:       item.Type,
			Title:      item.Title,
			Content:    item.Content,
			Link:       pgtypex.Text(item.Link),
			SourceRef:  pgtypex.Text(item.SourceRef),
			IsRead:     false,
			CreatedAt:  timex.RequiredTimestamptz(now),
		}); err != nil {
			return err
		}
	}
	return nil
}

// CountNotificationsByRead 按已读与否聚合本人收件箱(套用类型筛选,不套用已读筛选)。
func (t *txStore) CountNotificationsByRead(ctx context.Context, accountID int64, typ string) (pagex.FacetGroup, error) {
	rows, err := t.q.CountNotificationsByRead(ctx, sqlcgen.CountNotificationsByReadParams{ReceiverID: accountID, Type: strings.TrimSpace(typ)})
	if err != nil {
		return nil, err
	}
	group := pagex.FacetGroup{}
	for _, row := range rows {
		group.Add(row.IsRead, row.Count)
	}
	return group, nil
}

// ListNotifications 查询站内信分页。
func (t *txStore) ListNotifications(ctx context.Context, accountID int64, isRead *bool, typ string, page, size int) ([]NotificationDTO, int64, error) {
	limit, offset := pagex.LimitOffset(page, size)
	arg := sqlcgen.ListNotificationsParams{ReceiverID: accountID, IsRead: pgtypex.BoolPtr(isRead), Type: strings.TrimSpace(typ), PageOffset: offset, PageLimit: limit}
	rows, err := t.q.ListNotifications(ctx, arg)
	if err != nil {
		return nil, 0, err
	}
	total, err := t.q.CountNotifications(ctx, sqlcgen.CountNotificationsParams{ReceiverID: accountID, IsRead: pgtypex.BoolPtr(isRead), Type: strings.TrimSpace(typ)})
	if err != nil {
		return nil, 0, err
	}
	out := make([]NotificationDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, notificationDTO(row))
	}
	return out, total, nil
}

// CountUnread 查询未读数量。
func (t *txStore) CountUnread(ctx context.Context, accountID int64) (int64, error) {
	return t.q.CountUnreadNotifications(ctx, accountID)
}

// MarkNotificationRead 标记单条站内信已读。
func (t *txStore) MarkNotificationRead(ctx context.Context, id, accountID int64) (NotificationDTO, error) {
	row, err := t.q.MarkNotificationRead(ctx, sqlcgen.MarkNotificationReadParams{ID: id, ReceiverID: accountID})
	if err != nil {
		return NotificationDTO{}, err
	}
	return notificationDTO(row), nil
}

// MarkAllNotificationsRead 标记全部站内信已读。
func (t *txStore) MarkAllNotificationsRead(ctx context.Context, accountID int64) error {
	return t.q.MarkAllNotificationsRead(ctx, accountID)
}

// DeleteNotification 删除站内信。
func (t *txStore) DeleteNotification(ctx context.Context, id, accountID int64) (NotificationDTO, error) {
	row, err := t.q.DeleteNotification(ctx, sqlcgen.DeleteNotificationParams{ID: id, ReceiverID: accountID})
	if err != nil {
		return NotificationDTO{}, err
	}
	return notificationDTO(row), nil
}

// DeleteExpiredNotifications 软删除超过保留期的站内信。
func (t *txStore) DeleteExpiredNotifications(ctx context.Context, before time.Time) error {
	return t.q.DeleteExpiredNotifications(ctx, timex.RequiredTimestamptz(before))
}

// ListPreferences 查询本人通知偏好(以模板类型目录为基准,含未设置过的类型)。
func (t *txStore) ListPreferences(ctx context.Context, accountID int64) ([]PreferenceDTO, error) {
	rows, err := t.q.ListPreferences(ctx, accountID)
	if err != nil {
		return nil, err
	}
	out := make([]PreferenceDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, PreferenceDTO{Type: row.Type, Enabled: row.Enabled, Force: row.Force})
	}
	return out, nil
}

// PreferenceEnabled 查询某类通知是否允许投递。
func (t *txStore) PreferenceEnabled(ctx context.Context, tenantID, accountID int64, typ string) (bool, error) {
	return t.q.PreferenceEnabled(ctx, sqlcgen.PreferenceEnabledParams{TenantID: tenantID, AccountID: accountID, Type: typ})
}

// UpsertPreference 保存通知偏好。
func (t *txStore) UpsertPreference(ctx context.Context, id, tenantID, accountID int64, typ string, enabled bool) (PreferenceDTO, error) {
	row, err := t.q.UpsertPreference(ctx, sqlcgen.UpsertPreferenceParams{ID: id, TenantID: tenantID, AccountID: accountID, Type: typ, Enabled: enabled})
	if err != nil {
		return PreferenceDTO{}, err
	}
	return PreferenceDTO{Type: row.Type, Enabled: row.Enabled, Force: row.Force}, nil
}
