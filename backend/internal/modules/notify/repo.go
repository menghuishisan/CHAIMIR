// M10 数据访问层:封装 notify 自有表的 sqlc 查询与 RLS 注入。
package notify

import (
	"context"

	"chaimir/internal/modules/notify/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/snowflake"

	"github.com/jackc/pgx/v5"
)

// repo 是 M10 模块数据库访问封装。
type repo struct {
	db    *db.DB
	idgen snowflake.Generator
}

// newRepo 构造 M10 repo。
func newRepo(database *db.DB, idgen snowflake.Generator) *repo {
	return &repo{db: database, idgen: idgen}
}

// queryFunc 是 M10 sqlc 查询闭包。
type queryFunc func(q *sqlcgen.Queries) error

// inTenant 用显式租户 ID 执行租户表查询。
func (r *repo) inTenant(ctx context.Context, tenantID int64, fn queryFunc) error {
	return r.db.WithTenantTxID(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// inApp 执行全局模板查询。
func (r *repo) inApp(ctx context.Context, fn queryFunc) error {
	return r.db.WithAppTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// GetTemplate 读取全局通知模板。
func (r *repo) GetTemplate(ctx context.Context, typ string) (TemplateDTO, error) {
	var row sqlcgen.NotificationTemplate
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		found, e := q.GetNotificationTemplate(ctx, typ)
		row = found
		return e
	}); err != nil {
		if db.IsNoRows(err) {
			return TemplateDTO{}, apperr.ErrNotifyTemplateMissing
		}
		return TemplateDTO{}, apperr.ErrNotifyTemplateMissing.WithCause(err)
	}
	return templateDTOFromRow(row), nil
}

// GetPreference 读取用户接收偏好,未配置时返回 found=false。
func (r *repo) GetPreference(ctx context.Context, tenantID, accountID int64, typ string) (bool, bool, error) {
	var row sqlcgen.NotificationPreference
	err := r.inTenant(ctx, tenantID, func(q *sqlcgen.Queries) error {
		found, e := q.GetNotificationPreference(ctx, sqlcgen.GetNotificationPreferenceParams{AccountID: accountID, Type: typ})
		row = found
		return e
	})
	if err != nil {
		if db.IsNoRows(err) {
			return true, false, nil
		}
		return false, false, apperr.ErrNotifySendFailed.WithCause(err)
	}
	return row.Enabled, true, nil
}

// CreateNotifications 在单个租户事务内批量写入站内信,避免一次发送留下部分投递结果。
func (r *repo) CreateNotifications(ctx context.Context, tenantID int64, rows []NotificationCreate) error {
	err := r.inTenant(ctx, tenantID, func(q *sqlcgen.Queries) error {
		for _, row := range rows {
			if row.TenantID != tenantID {
				return apperr.ErrNotifyInvalid
			}
			if _, e := q.CreateNotification(ctx, sqlcgen.CreateNotificationParams{
				ID: row.ID, TenantID: row.TenantID, ReceiverID: row.ReceiverID, Type: row.Type,
				Title: row.Title, Content: row.Content, Link: pgtypex.Text(row.Link),
			}); e != nil {
				return e
			}
		}
		return nil
	})
	if err != nil {
		return apperr.ErrNotifySendFailed.WithCause(err)
	}
	return nil
}

// ListInbox 查询指定账号收件箱。
func (r *repo) ListInbox(ctx context.Context, tenantID, accountID int64, query InboxQuery) ([]NotificationDTO, int64, error) {
	page, size := pagex.Normalize(query.Page, query.Size)
	var rows []sqlcgen.Notification
	var total int64
	err := r.inTenant(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var e error
		total, e = q.CountInbox(ctx, sqlcgen.CountInboxParams{ReceiverID: accountID, Type: pgtypex.Text(query.Type), IsRead: pgtypex.BoolPtr(query.IsRead)})
		if e != nil {
			return e
		}
		rows, e = q.ListInbox(ctx, sqlcgen.ListInboxParams{
			ReceiverID: accountID, Type: pgtypex.Text(query.Type), IsRead: pgtypex.BoolPtr(query.IsRead),
			OffsetCount: int32((page - 1) * size), LimitCount: int32(size),
		})
		return e
	})
	if err != nil {
		return nil, 0, apperr.ErrNotifySendFailed.WithCause(err)
	}
	return notificationDTOsFromRows(rows), total, nil
}

// CountUnreadNotifications 统计账号当前未读站内信,供未读缓存 miss 时重建权威值。
func (r *repo) CountUnreadNotifications(ctx context.Context, tenantID, accountID int64) (int64, error) {
	var total int64
	isRead := false
	err := r.inTenant(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var e error
		total, e = q.CountInbox(ctx, sqlcgen.CountInboxParams{
			ReceiverID: accountID,
			Type:       pgtypex.Text(""),
			IsRead:     pgtypex.BoolPtr(&isRead),
		})
		return e
	})
	if err != nil {
		return 0, apperr.ErrNotifySendFailed.WithCause(err)
	}
	return total, nil
}

// MarkNotificationRead 标记账号自己的站内信已读。
func (r *repo) MarkNotificationRead(ctx context.Context, accountID, notificationID int64) error {
	err := r.inTenantFromContext(ctx, func(q *sqlcgen.Queries) error {
		_, e := q.MarkNotificationRead(ctx, sqlcgen.MarkNotificationReadParams{ID: notificationID, ReceiverID: accountID})
		return e
	})
	return mapNotificationMutationError(err)
}

// MarkAllNotificationsRead 标记账号全部站内信已读。
func (r *repo) MarkAllNotificationsRead(ctx context.Context, accountID int64) error {
	err := r.inTenantFromContext(ctx, func(q *sqlcgen.Queries) error {
		return q.MarkAllNotificationsRead(ctx, accountID)
	})
	if err != nil {
		return apperr.ErrNotifySendFailed.WithCause(err)
	}
	return nil
}

// SoftDeleteNotification 软删账号自己的站内信。
func (r *repo) SoftDeleteNotification(ctx context.Context, accountID, notificationID int64) error {
	err := r.inTenantFromContext(ctx, func(q *sqlcgen.Queries) error {
		_, e := q.SoftDeleteNotification(ctx, sqlcgen.SoftDeleteNotificationParams{ID: notificationID, ReceiverID: accountID})
		return e
	})
	return mapNotificationMutationError(err)
}

// ListPreferences 查询用户偏好,未配置模板默认 enabled=true。
func (r *repo) ListPreferences(ctx context.Context, tenantID, accountID int64) ([]PreferenceDTO, error) {
	var rows []sqlcgen.ListPreferencesRow
	if err := r.inTenant(ctx, tenantID, func(q *sqlcgen.Queries) error {
		found, e := q.ListPreferences(ctx, accountID)
		rows = found
		return e
	}); err != nil {
		return nil, apperr.ErrNotifySendFailed.WithCause(err)
	}
	return preferenceDTOsFromRows(rows), nil
}

// UpsertPreferences 保存用户偏好。
func (r *repo) UpsertPreferences(ctx context.Context, tenantID, accountID int64, preferences []PreferenceRequest) error {
	err := r.inTenant(ctx, tenantID, func(q *sqlcgen.Queries) error {
		for _, pref := range preferences {
			if _, e := q.UpsertNotificationPreference(ctx, sqlcgen.UpsertNotificationPreferenceParams{
				ID: r.nextID(), TenantID: tenantID, AccountID: accountID, Type: pref.Type, Enabled: pref.Enabled,
			}); e != nil {
				return e
			}
		}
		return nil
	})
	if err != nil {
		return apperr.ErrNotifySendFailed.WithCause(err)
	}
	return nil
}

// CreateAnnouncement 创建一条公告,不复制到 notification。
func (r *repo) CreateAnnouncement(ctx context.Context, id, publisherID int64, req AnnouncementRequest) (AnnouncementDTO, error) {
	current, err := currentTenant(ctx)
	if err != nil {
		return AnnouncementDTO{}, err
	}
	expireAt, ok := parseOptionalDateTime(req.ExpireAt)
	if !ok {
		return AnnouncementDTO{}, apperr.ErrNotifyAnnouncementInvalid
	}
	tenantID := current.TenantID
	if req.Scope == AnnouncementScopePlatform {
		tenantID = 0
	}
	var row sqlcgen.SystemAnnouncement
	err = r.execAnnouncementScope(ctx, tenantID, func(q *sqlcgen.Queries) error {
		found, e := q.CreateSystemAnnouncement(ctx, sqlcgen.CreateSystemAnnouncementParams{
			ID: id, TenantID: pgtypex.Int8(tenantID), Title: req.Title, Content: req.Content, Scope: req.Scope,
			TargetRoles: req.TargetRoles, PublisherID: publisherID, ExpireAt: timex.Timestamptz(expireAt),
		})
		row = found
		return e
	})
	if err != nil {
		return AnnouncementDTO{}, apperr.ErrNotifyAnnouncementInvalid.WithCause(err)
	}
	return announcementDTOFromRow(row), nil
}

// ListAnnouncements 查询当前租户可见公告及已读状态。
func (r *repo) ListAnnouncements(ctx context.Context, tenantID, accountID int64, roles []int16) ([]AnnouncementDTO, error) {
	var rows []sqlcgen.ListAnnouncementsRow
	if err := r.inTenant(ctx, tenantID, func(q *sqlcgen.Queries) error {
		found, e := q.ListAnnouncements(ctx, sqlcgen.ListAnnouncementsParams{TenantID: tenantID, AccountID: accountID, Roles: roles})
		rows = found
		return e
	}); err != nil {
		return nil, apperr.ErrNotifyAnnouncementNotFound.WithCause(err)
	}
	return announcementDTOsFromListRows(rows), nil
}

// GetAnnouncement 读取当前租户可见公告。
func (r *repo) GetAnnouncement(ctx context.Context, tenantID, announcementID int64) (AnnouncementDTO, error) {
	var row sqlcgen.SystemAnnouncement
	if err := r.inTenant(ctx, tenantID, func(q *sqlcgen.Queries) error {
		found, e := q.GetAnnouncement(ctx, sqlcgen.GetAnnouncementParams{ID: announcementID, TenantID: pgtypex.Int8(tenantID)})
		row = found
		return e
	}); err != nil {
		if db.IsNoRows(err) {
			return AnnouncementDTO{}, apperr.ErrNotifyAnnouncementNotFound
		}
		return AnnouncementDTO{}, apperr.ErrNotifyAnnouncementNotFound.WithCause(err)
	}
	return announcementDTOFromRow(row), nil
}

// MarkAnnouncementRead 写入公告已读状态,重复调用保持幂等。
func (r *repo) MarkAnnouncementRead(ctx context.Context, tenantID, accountID, announcementID, id int64) error {
	err := r.inTenant(ctx, tenantID, func(q *sqlcgen.Queries) error {
		_, e := q.MarkAnnouncementRead(ctx, sqlcgen.MarkAnnouncementReadParams{
			ID: id, TenantID: tenantID, AnnouncementID: announcementID, AccountID: accountID,
		})
		return e
	})
	if err != nil {
		return apperr.ErrNotifyAnnouncementNotFound.WithCause(err)
	}
	return nil
}

// inTenantFromContext 从请求上下文取租户后执行 RLS 查询。
func (r *repo) inTenantFromContext(ctx context.Context, fn queryFunc) error {
	id, err := currentTenant(ctx)
	if err != nil {
		return err
	}
	return r.inTenant(ctx, id.TenantID, fn)
}

// execAnnouncementScope 按公告 scope 选择平台级或租户级事务。
func (r *repo) execAnnouncementScope(ctx context.Context, tenantID int64, fn queryFunc) error {
	if tenantID > 0 {
		return r.inTenant(ctx, tenantID, fn)
	}
	return r.inApp(ctx, fn)
}

// mapNotificationMutationError 将站内信写操作错误映射为用户向错误。
func mapNotificationMutationError(err error) error {
	if err == nil {
		return nil
	}
	if db.IsNoRows(err) {
		return apperr.ErrNotifyNotFound
	}
	return apperr.ErrNotifySendFailed.WithCause(err)
}

// nextID 生成 M10 repo 内需要的行 ID。
func (r *repo) nextID() int64 {
	return r.idgen.Generate()
}
