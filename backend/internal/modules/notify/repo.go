// notify repo 文件定义 M10 持久化边界,只访问通知模块自有表。
package notify

import (
	"context"
	"errors"
	"fmt"
	"time"

	"chaimir/internal/modules/notify/internal/sqlcgen"
	"chaimir/internal/platform/db"

	"github.com/jackc/pgx/v5"
)

// Store 定义 M10 service 所需的事务入口。
type Store interface {
	// PlatformTx 在平台事务中访问通知模块全局表。
	PlatformTx(ctx context.Context, fn func(context.Context, TxStore) error) error
	// TenantTx 在租户 RLS 事务中执行通知表访问。
	TenantTx(ctx context.Context, tenantID int64, fn func(context.Context, TxStore) error) error
	// PrivilegedTx 在受控后台任务中跨租户清理 M10 自有表。
	PrivilegedTx(ctx context.Context, fn func(context.Context, TxStore) error) error
}

// TxStore 定义通知模块单事务数据访问能力。
type TxStore interface {
	GetNotificationTemplate(context.Context, string) (notificationTemplate, error)
	ListNotificationTemplates(context.Context) ([]notificationTemplate, error)
	CreateNotifications(context.Context, []notificationRecord) error
	ListNotifications(context.Context, int64, *bool, string, int, int) ([]NotificationDTO, int64, error)
	CountUnread(context.Context, int64) (int64, error)
	MarkNotificationRead(context.Context, int64, int64) (NotificationDTO, error)
	MarkAllNotificationsRead(context.Context, int64) error
	DeleteNotification(context.Context, int64, int64) (NotificationDTO, error)
	DeleteExpiredNotifications(context.Context, time.Time) error
	ListPreferences(context.Context, int64) ([]PreferenceDTO, error)
	PreferenceEnabled(context.Context, int64, int64, string) (bool, error)
	UpsertPreference(context.Context, int64, int64, int64, string, bool) (PreferenceDTO, error)
	CreateAnnouncement(context.Context, int64, int64, int64, AnnouncementRequest) (AnnouncementDTO, error)
	ListAnnouncements(context.Context, int64, int64, []int16, int, int) ([]AnnouncementDTO, int64, error)
	GetVisibleAnnouncement(context.Context, int64, int64, []int16, int64) (AnnouncementDTO, error)
	MarkAnnouncementRead(context.Context, int64, int64, int64, int64) error
}

type store struct{ database *db.DB }
type txStore struct{ q *sqlcgen.Queries }

// NewStore 创建 M10 持久化入口。
func NewStore(database *db.DB) Store { return &store{database: database} }

// PlatformTx 在普通平台事务中执行通知模块全局表访问。
func (s *store) PlatformTx(ctx context.Context, fn func(context.Context, TxStore) error) error {
	if s == nil || s.database == nil {
		return fmt.Errorf("notify store 未初始化")
	}
	return s.database.WithAppTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return fn(ctx, &txStore{q: sqlcgen.New(tx)})
	})
}

// TenantTx 在租户事务中执行通知模块读写。
func (s *store) TenantTx(ctx context.Context, tenantID int64, fn func(context.Context, TxStore) error) error {
	if s == nil || s.database == nil {
		return fmt.Errorf("notify store 未初始化")
	}
	return s.database.WithTenantTxID(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return fn(ctx, &txStore{q: sqlcgen.New(tx)})
	})
}

// PrivilegedTx 在 M10 模块自有表内执行后台跨租户清理,不得用于普通业务路径。
func (s *store) PrivilegedTx(ctx context.Context, fn func(context.Context, TxStore) error) error {
	if s == nil || s.database == nil {
		return fmt.Errorf("notify store 未初始化")
	}
	return s.database.WithPrivilegedModuleTx(ctx, "notify", func(ctx context.Context, tx pgx.Tx) error {
		return fn(ctx, &txStore{q: sqlcgen.New(tx)})
	})
}

// isNoRows 统一识别未命中错误。
func isNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
