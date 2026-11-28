// admin repo 文件定义 M9 持久化边界,仅操作管理后台自有表。
package admin

import (
	"context"
	"fmt"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/admin/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/pagex"

	"github.com/jackc/pgx/v5"
)

// roleReader 定义 M9 service 层复用的角色查询契约。
type roleReader interface {
	// HasRole 判断账号是否具备指定角色。
	HasRole(ctx context.Context, accountID int64, role string) (bool, error)
}

// accountReader 批量解析本校账号姓名(审计操作人)。
type accountReader interface {
	BatchGetAccounts(ctx context.Context, tenantID int64, accountIDs []int64) ([]contracts.AccountInfo, error)
}

// Store 定义 M9 service 使用的持久化入口。
type Store interface {
	// PlatformTx 在平台事务中访问 M9 平台级表。
	PlatformTx(ctx context.Context, fn func(context.Context, TxStore) error) error
	// TenantTx 在租户事务中访问 M9 租户级表。
	TenantTx(ctx context.Context, tenantID int64, fn func(context.Context, TxStore) error) error
	// PrivilegedTx 在受控后台任务中跨租户扫描 M9 自有请求表。
	PrivilegedTx(ctx context.Context, fn func(context.Context, TxStore) error) error
}

// TxStore 定义单事务内的 M9 数据访问能力。
type TxStore interface {
	ListEvaluationRules(context.Context) ([]AlertRuleDTO, error)
	EvaluateAlert(context.Context, AlertObservation) error
	ClaimAlertNotifications(context.Context, int32, int32, int64, time.Time) ([]AlertNotification, error)
	FinishAlertNotification(context.Context, int64, int64, bool, string) error
	CreateAlertRule(context.Context, int64, AlertRuleRequest) (AlertRuleDTO, error)
	ListAlertRules(context.Context, int16, int64) ([]AlertRuleDTO, error)
	UpdateAlertRule(context.Context, int64, AlertRuleRequest) (AlertRuleDTO, error)
	CreateAlertEvent(context.Context, int64, int64, int64, int16, string) (AlertEventDTO, error)
	ListAlertEvents(context.Context, int16, int16, int64, int, int) ([]AlertEventDTO, int64, error)
	CountAlertEventsByStatus(context.Context, int16, int64) (pagex.FacetGroup, error)
	HandleAlertEvent(context.Context, int64, int64, int16, int64) (AlertEventDTO, error)
	ListPlatformStatistics(context.Context, int16, int64, string, string) ([]StatisticsDTO, error)
	UpsertPlatformStatistics(context.Context, int64, int16, int64, string, map[string]any) (StatisticsDTO, error)
	CreateBackupRecord(context.Context, int64, BackupRecordCreate) (BackupRecordDTO, error)
	ListBackupRecords(context.Context, int16, int, int) ([]BackupRecordDTO, int64, error)
	CountBackupRecordsByStatus(context.Context) (pagex.FacetGroup, error)
	CreateAuditExportRequest(context.Context, AuditExportRequest) (AuditExportRequest, error)
	ListDueAuditExportRequests(context.Context, time.Time, int32) ([]AuditExportRequest, error)
	SetAuditExportRequestNextCheck(context.Context, int64, int64, time.Time) (AuditExportRequest, error)
	DeleteAuditExportRequest(context.Context, int64, int64) error
}

type store struct{ database *db.DB }
type txStore struct{ q *sqlcgen.Queries }

// NewStore 创建 M9 持久化入口。
func NewStore(database *db.DB) Store { return &store{database: database} }

// PlatformTx 在普通平台事务中执行 M9 表访问。
func (s *store) PlatformTx(ctx context.Context, fn func(context.Context, TxStore) error) error {
	if s == nil || s.database == nil {
		return fmt.Errorf("admin store 未初始化")
	}
	return s.database.WithPlatformTx(ctx, "admin", func(ctx context.Context, tx pgx.Tx) error {
		return fn(ctx, &txStore{q: sqlcgen.New(tx)})
	})
}

// TenantTx 在租户 RLS 事务中执行 M9 表访问。
func (s *store) TenantTx(ctx context.Context, tenantID int64, fn func(context.Context, TxStore) error) error {
	if s == nil || s.database == nil {
		return fmt.Errorf("admin store 未初始化")
	}
	return s.database.WithTenantTxID(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return fn(ctx, &txStore{q: sqlcgen.New(tx)})
	})
}

// PrivilegedTx 仅用于 M9 后台 worker 跨租户扫描本模块自有告警和导出请求。
func (s *store) PrivilegedTx(ctx context.Context, fn func(context.Context, TxStore) error) error {
	if s == nil || s.database == nil {
		return fmt.Errorf("admin store 未初始化")
	}
	return s.database.WithPrivilegedModuleTx(ctx, "admin", func(ctx context.Context, tx pgx.Tx) error {
		return fn(ctx, &txStore{q: sqlcgen.New(tx)})
	})
}
