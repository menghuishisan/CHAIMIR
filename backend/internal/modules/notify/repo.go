// notify repo 文件定义 M10 持久化边界,只访问通知模块自有表。
package notify

import (
	"context"
	"errors"
	"fmt"

	"chaimir/internal/modules/notify/internal/sqlcgen"
	"chaimir/internal/platform/db"

	"github.com/jackc/pgx/v5"
)

// Store 定义 M10 service 所需的事务入口。
type Store interface {
	// TenantTx 在租户 RLS 事务中执行通知表访问。
	TenantTx(ctx context.Context, tenantID int64, fn func(context.Context, TxStore) error) error
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
	ListPreferences(context.Context, int64) ([]PreferenceDTO, error)
	PreferenceEnabled(context.Context, int64, int64, string) (bool, error)
	UpsertPreference(context.Context, int64, int64, int64, string, bool) (PreferenceDTO, error)
	CreateAnnouncement(context.Context, int64, int64, int64, AnnouncementRequest) (AnnouncementDTO, error)
	ListAnnouncements(context.Context, int64, int64, int, int) ([]AnnouncementDTO, error)
	MarkAnnouncementRead(context.Context, int64, int64, int64, int64) error
}

type store struct{ database *db.DB }
type txStore struct{ q *sqlcgen.Queries }

// NewStore 创建 M10 持久化入口。
func NewStore(database *db.DB) Store { return &store{database: database} }

// TenantTx 在租户事务中执行通知模块读写。
func (s *store) TenantTx(ctx context.Context, tenantID int64, fn func(context.Context, TxStore) error) error {
	if s == nil || s.database == nil {
		return fmt.Errorf("notify store 未初始化")
	}
	return s.database.WithTenantTxID(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return fn(ctx, &txStore{q: sqlcgen.New(tx)})
	})
}

// isNoRows 统一识别未命中错误。
func isNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
