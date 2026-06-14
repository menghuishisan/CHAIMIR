// notify repo_data 文件负责 M10 repo 查询写入与 sqlc 行到模块模型、DTO 的转换。
package notify

import (
	"context"
	"strings"
	"time"

	"chaimir/internal/modules/notify/internal/sqlcgen"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
)

type notificationTemplate struct {
	Type       string
	TitleTpl   string
	ContentTpl string
	Channels   []string
	Force      bool
}

type notificationRecord struct {
	ID         int64
	TenantID   int64
	ReceiverID int64
	Type       string
	Title      string
	Content    string
	Link       string
}

// GetNotificationTemplate 查询通知模板。
func (t *txStore) GetNotificationTemplate(ctx context.Context, typ string) (notificationTemplate, error) {
	row, err := t.q.GetNotificationTemplate(ctx, typ)
	if err != nil {
		return notificationTemplate{}, err
	}
	return templateFromRow(row), nil
}

// ListNotificationTemplates 查询模板列表。
func (t *txStore) ListNotificationTemplates(ctx context.Context) ([]notificationTemplate, error) {
	rows, err := t.q.ListNotificationTemplates(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]notificationTemplate, 0, len(rows))
	for _, row := range rows {
		out = append(out, templateFromRow(row))
	}
	return out, nil
}

// CreateNotifications 批量写入站内信。
func (t *txStore) CreateNotifications(ctx context.Context, records []notificationRecord) error {
	rows := make([]sqlcgen.CreateNotificationsParams, 0, len(records))
	now := timex.Now()
	for _, item := range records {
		rows = append(rows, sqlcgen.CreateNotificationsParams{
			ID:         item.ID,
			TenantID:   item.TenantID,
			ReceiverID: item.ReceiverID,
			Type:       item.Type,
			Title:      item.Title,
			Content:    item.Content,
			Link:       pgtypex.Text(item.Link),
			IsRead:     false,
			CreatedAt:  timex.RequiredTimestamptz(now),
		})
	}
	if len(rows) == 0 {
		return nil
	}
	_, err := t.q.CreateNotifications(ctx, rows)
	return err
}

// ListNotifications 查询站内信分页。
func (t *txStore) ListNotifications(ctx context.Context, accountID int64, isRead *bool, typ string, page, size int) ([]NotificationDTO, int64, error) {
	arg := sqlcgen.ListNotificationsParams{ReceiverID: accountID, IsRead: pgtypex.BoolPtr(isRead), Type: strings.TrimSpace(typ), PageOffset: int32((page - 1) * size), PageLimit: int32(size)}
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

// ListPreferences 查询通知偏好。
func (t *txStore) ListPreferences(ctx context.Context, accountID int64) ([]PreferenceDTO, error) {
	rows, err := t.q.ListPreferences(ctx, accountID)
	if err != nil {
		return nil, err
	}
	out := make([]PreferenceDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, PreferenceDTO{Type: row.Type, Enabled: row.Enabled})
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
	return PreferenceDTO{Type: row.Type, Enabled: row.Enabled}, nil
}

// CreateAnnouncement 创建系统公告。
func (t *txStore) CreateAnnouncement(ctx context.Context, id, tenantID, publisherID int64, req AnnouncementRequest) (AnnouncementDTO, error) {
	expireAt, err := parseOptionalTime(req.ExpireAt)
	if err != nil {
		return AnnouncementDTO{}, apperr.ErrNotifyAnnouncementInvalid.WithCause(err)
	}
	row, err := t.q.CreateAnnouncement(ctx, sqlcgen.CreateAnnouncementParams{ID: id, TenantID: pgtypex.Int8When(tenantID, req.Scope != AnnouncementScopePlatform), Title: strings.TrimSpace(req.Title), Content: strings.TrimSpace(req.Content), Scope: req.Scope, TargetRoles: req.TargetRoles, PublisherID: publisherID, ExpireAt: timex.Timestamptz(expireAt)})
	if err != nil {
		return AnnouncementDTO{}, err
	}
	return announcementDTO(row, false), nil
}

// ListAnnouncements 查询系统公告。
func (t *txStore) ListAnnouncements(ctx context.Context, tenantID, accountID int64, roleNumbers []int16, page, size int) ([]AnnouncementDTO, error) {
	rows, err := t.q.ListAnnouncements(ctx, sqlcgen.ListAnnouncementsParams{TenantID: tenantID, AccountID: accountID, RoleNumbers: roleNumbers, PageLimit: int32(size), PageOffset: int32((page - 1) * size)})
	if err != nil {
		return nil, err
	}
	out := make([]AnnouncementDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, announcementRowDTO(row))
	}
	return out, nil
}

// GetVisibleAnnouncement 查询当前用户可见公告。
func (t *txStore) GetVisibleAnnouncement(ctx context.Context, tenantID, accountID int64, roleNumbers []int16, announcementID int64) (AnnouncementDTO, error) {
	row, err := t.q.GetVisibleAnnouncement(ctx, sqlcgen.GetVisibleAnnouncementParams{TenantID: tenantID, AccountID: accountID, ID: announcementID, RoleNumbers: roleNumbers})
	if err != nil {
		return AnnouncementDTO{}, err
	}
	return announcementVisibleRowDTO(row), nil
}

// MarkAnnouncementRead 标记公告已读。
func (t *txStore) MarkAnnouncementRead(ctx context.Context, id, tenantID, announcementID, accountID int64) error {
	_, err := t.q.MarkAnnouncementRead(ctx, sqlcgen.MarkAnnouncementReadParams{ID: id, TenantID: tenantID, AnnouncementID: announcementID, AccountID: accountID})
	return err
}

// templateFromRow 转换模板行。
func templateFromRow(row sqlcgen.NotificationTemplate) notificationTemplate {
	return notificationTemplate{Type: row.Type, TitleTpl: row.TitleTpl, ContentTpl: row.ContentTpl, Channels: row.Channels, Force: row.Force}
}

// notificationDTO 转换通知行。
func notificationDTO(row sqlcgen.Notification) NotificationDTO {
	return NotificationDTO{ID: row.ID, Type: row.Type, Title: row.Title, Content: row.Content, Link: pgtypex.TextValue(row.Link), IsRead: row.IsRead, ReadAt: timex.PtrFromTimestamptz(row.ReadAt), CreatedAt: timex.FromTimestamptz(row.CreatedAt)}
}

// announcementDTO 转换公告行。
func announcementDTO(row sqlcgen.SystemAnnouncement, isRead bool) AnnouncementDTO {
	return AnnouncementDTO{ID: row.ID, TenantID: pgtypex.Int8Value(row.TenantID), Title: row.Title, Content: row.Content, Scope: row.Scope, TargetRoles: row.TargetRoles, PublisherID: row.PublisherID, PublishedAt: timex.FromTimestamptz(row.PublishedAt), ExpireAt: timex.PtrFromTimestamptz(row.ExpireAt), IsRead: isRead}
}

// announcementRowDTO 转换公告列表行。
func announcementRowDTO(row sqlcgen.ListAnnouncementsRow) AnnouncementDTO {
	return AnnouncementDTO{ID: row.ID, TenantID: pgtypex.Int8Value(row.TenantID), Title: row.Title, Content: row.Content, Scope: row.Scope, TargetRoles: row.TargetRoles, PublisherID: row.PublisherID, PublishedAt: timex.FromTimestamptz(row.PublishedAt), ExpireAt: timex.PtrFromTimestamptz(row.ExpireAt), IsRead: row.IsRead}
}

// announcementVisibleRowDTO 转换可见公告查询行。
func announcementVisibleRowDTO(row sqlcgen.GetVisibleAnnouncementRow) AnnouncementDTO {
	return AnnouncementDTO{ID: row.ID, TenantID: pgtypex.Int8Value(row.TenantID), Title: row.Title, Content: row.Content, Scope: row.Scope, TargetRoles: row.TargetRoles, PublisherID: row.PublisherID, PublishedAt: timex.FromTimestamptz(row.PublishedAt), ExpireAt: timex.PtrFromTimestamptz(row.ExpireAt), IsRead: row.IsRead}
}

// parseOptionalTime 解析可选过期时间。
func parseOptionalTime(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02", raw)
}
