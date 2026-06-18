// admin repo 文件定义 M9 持久化边界,仅操作管理后台自有表。
package admin

import (
	"context"
	"errors"
	"fmt"

	"chaimir/internal/modules/admin/internal/sqlcgen"
	"chaimir/internal/platform/db"

	"github.com/jackc/pgx/v5"
)

// roleReader 定义 M9 service 层复用的角色查询契约。
type roleReader interface {
	// HasRole 判断账号是否具备指定角色。
	HasRole(ctx context.Context, accountID int64, role string) (bool, error)
}

// Store 定义 M9 service 使用的持久化入口。
type Store interface {
	// PlatformTx 在平台事务中访问 M9 平台级表。
	PlatformTx(ctx context.Context, fn func(context.Context, TxStore) error) error
	// TenantTx 在租户事务中访问 M9 租户级表。
	TenantTx(ctx context.Context, tenantID int64, fn func(context.Context, TxStore) error) error
}

// TxStore 定义单事务内的 M9 数据访问能力。
type TxStore interface {
	ListSystemConfigs(context.Context, int16, int64) ([]ConfigDTO, error)
	GetSystemConfig(context.Context, int16, int64, string) (ConfigDTO, error)
	CreateSystemConfig(context.Context, int64, int16, int64, string, map[string]any, int64) (ConfigDTO, error)
	UpdateSystemConfig(context.Context, int16, int64, string, map[string]any, int64, int32) (ConfigDTO, error)
	CreateConfigChangeLog(context.Context, int64, int64, int64, map[string]any, map[string]any, int64) (ConfigChangeLogDTO, error)
	GetConfigChangeLog(context.Context, int64, int64) (ConfigChangeLogDTO, error)
	ListConfigChangeLogs(context.Context, int64, int, int) ([]ConfigChangeLogDTO, int64, error)
	CreateAlertRule(context.Context, int64, AlertRuleRequest) (AlertRuleDTO, error)
	ListAlertRules(context.Context, int16, int64) ([]AlertRuleDTO, error)
	UpdateAlertRule(context.Context, int64, AlertRuleRequest) (AlertRuleDTO, error)
	CreateAlertEvent(context.Context, int64, int64, int64, int16, string) (AlertEventDTO, error)
	ListAlertEvents(context.Context, int16, int64, int, int) ([]AlertEventDTO, int64, error)
	HandleAlertEvent(context.Context, int64, int64, int16, int64) (AlertEventDTO, error)
	ListPlatformStatistics(context.Context, int16, int64, string, string) ([]StatisticsDTO, error)
	UpsertPlatformStatistics(context.Context, int64, int16, int64, string, map[string]any) (StatisticsDTO, error)
	CreateBackupRecord(context.Context, int64, BackupRecordCreate) (BackupRecordDTO, error)
	ListBackupRecords(context.Context, int, int) ([]BackupRecordDTO, int64, error)
}

// RecordBackupResult 写入受控运维备份任务结果,仅供组合根 cron 命令使用。
func RecordBackupResult(ctx context.Context, store Store, id int64, req BackupRecordCreate) (BackupRecordDTO, error) {
	if store == nil || id <= 0 {
		return BackupRecordDTO{}, fmt.Errorf("备份记录写入依赖不完整")
	}
	var out BackupRecordDTO
	err := store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.CreateBackupRecord(ctx, id, req)
		return err
	})
	return out, err
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
	return s.database.WithAppTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
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

// isNoRows 统一识别未命中错误。
func isNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
