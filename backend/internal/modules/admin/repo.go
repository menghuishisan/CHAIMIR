// M9 数据访问层:封装 admin 自有表的 sqlc 查询与 RLS 注入。
package admin

import (
	"context"
	"time"

	"chaimir/internal/modules/admin/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/pgtypex"
	"chaimir/pkg/apperr"

	"github.com/jackc/pgx/v5"
)

// repo 是 M9 模块数据库访问封装。
type repo struct {
	db *db.DB
}

// newRepo 构造 M9 repo。
func newRepo(database *db.DB) *repo { return &repo{db: database} }

// queryFunc 是 M9 sqlc 查询闭包。
type queryFunc func(q *sqlcgen.Queries) error

// inTenant 用显式租户 ID 执行可见混合表查询。
func (r *repo) inTenant(ctx context.Context, tenantID int64, fn queryFunc) error {
	return r.db.WithTenantTxID(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// inApp 执行全局表或平台级混合表查询。
func (r *repo) inApp(ctx context.Context, fn queryFunc) error {
	return r.db.WithAppTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// ListStatistics 查询统计快照。
func (r *repo) ListStatistics(ctx context.Context, scope int16, tenantID int64, from, to time.Time) ([]StatisticDTO, error) {
	var rows []sqlcgen.PlatformStatistic
	err := r.execByScope(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var e error
		rows, e = q.ListStatistics(ctx, sqlcgen.ListStatisticsParams{
			Scope: scope, TenantID: pgtypex.Int8(tenantID), FromDate: pgtypex.Date(from), ToDate: pgtypex.Date(to),
		})
		return e
	})
	if err != nil {
		return nil, apperr.ErrAdminDashboard.WithCause(err)
	}
	out := make([]StatisticDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, statisticDTOFromRow(row))
	}
	return out, nil
}

// ListConfigs 查询配置列表。
func (r *repo) ListConfigs(ctx context.Context, scope int16, tenantID int64) ([]ConfigDTO, error) {
	var rows []sqlcgen.SystemConfig
	err := r.execByScope(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var e error
		rows, e = q.ListConfigs(ctx, sqlcgen.ListConfigsParams{Scope: scope, TenantID: pgtypex.Int8(tenantID)})
		return e
	})
	if err != nil {
		return nil, apperr.ErrAdminConfigInvalid.WithCause(err)
	}
	return configDTOsFromRows(rows), nil
}

// GetConfig 按 scope 和 key 读取配置。
func (r *repo) GetConfig(ctx context.Context, scope int16, tenantID int64, key string) (ConfigDTO, error) {
	var row sqlcgen.SystemConfig
	err := r.execByScope(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var e error
		row, e = q.GetConfigByKey(ctx, sqlcgen.GetConfigByKeyParams{Scope: scope, TenantID: pgtypex.Int8(tenantID), Key: key})
		return e
	})
	if err != nil {
		if db.IsNoRows(err) {
			return ConfigDTO{}, apperr.ErrAdminConfigNotFound
		}
		return ConfigDTO{}, apperr.ErrAdminConfigInvalid.WithCause(err)
	}
	return configDTOFromRow(row), nil
}

// UpdateConfig 更新配置并写入结构化变更历史。
func (r *repo) UpdateConfig(ctx context.Context, configID, changeLogID, operatorID int64, current ConfigDTO, value map[string]any) (ConfigDTO, error) {
	newValue, err := jsonx.ObjectBytes(value, apperr.ErrAdminConfigInvalid)
	if err != nil {
		return ConfigDTO{}, err
	}
	oldValue, err := jsonx.ObjectBytes(current.Value, apperr.ErrAdminConfigInvalid)
	if err != nil {
		return ConfigDTO{}, err
	}
	tenantID := ids.ParseOrZero(current.TenantID)
	var row sqlcgen.SystemConfig
	err = r.execByScope(ctx, tenantID, func(q *sqlcgen.Queries) error {
		updated, e := q.UpdateConfigWithVersion(ctx, sqlcgen.UpdateConfigWithVersionParams{
			ID: configID, Version: current.Version, UpdatedBy: operatorID, Value: newValue,
		})
		if e != nil {
			return e
		}
		row = updated
		_, e = q.CreateConfigChangeLog(ctx, sqlcgen.CreateConfigChangeLogParams{
			ID: changeLogID, ConfigID: configID, TenantID: pgtypex.Int8(tenantID),
			OldValue: oldValue, NewValue: newValue, OperatorID: operatorID,
		})
		return e
	})
	if err != nil {
		if db.IsNoRows(err) {
			return ConfigDTO{}, apperr.ErrAdminConfigConflict
		}
		return ConfigDTO{}, apperr.ErrAdminConfigInvalid.WithCause(err)
	}
	return configDTOFromRow(row), nil
}

// GetConfigHistory 读取指定配置历史记录,用于配置回退。
func (r *repo) GetConfigHistory(ctx context.Context, configID, historyID int64) (ConfigChangeLogDTO, error) {
	var row sqlcgen.ConfigChangeLog
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		var e error
		row, e = q.GetConfigHistoryByID(ctx, sqlcgen.GetConfigHistoryByIDParams{ID: historyID, ConfigID: configID})
		return e
	}); err != nil {
		if db.IsNoRows(err) {
			return ConfigChangeLogDTO{}, apperr.ErrAdminConfigNotFound
		}
		return ConfigChangeLogDTO{}, apperr.ErrAdminConfigInvalid.WithCause(err)
	}
	return configChangeDTOFromRow(row), nil
}

// ListConfigHistory 查询配置变更历史。
func (r *repo) ListConfigHistory(ctx context.Context, configID int64, page, size int) ([]ConfigChangeLogDTO, int64, error) {
	page, size = pagex.Normalize(page, size)
	var rows []sqlcgen.ConfigChangeLog
	var total int64
	err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		var e error
		total, e = q.CountConfigHistory(ctx, configID)
		if e != nil {
			return e
		}
		rows, e = q.ListConfigHistory(ctx, sqlcgen.ListConfigHistoryParams{ConfigID: configID, OffsetCount: int32((page - 1) * size), LimitCount: int32(size)})
		return e
	})
	if err != nil {
		return nil, 0, apperr.ErrAdminConfigInvalid.WithCause(err)
	}
	return configChangeDTOsFromRows(rows), total, nil
}

// ListAlertRules 查询告警规则。
func (r *repo) ListAlertRules(ctx context.Context, scope int16, tenantID int64, page, size int) ([]AlertRuleDTO, int64, error) {
	page, size = pagex.Normalize(page, size)
	var rows []sqlcgen.AlertRule
	var total int64
	err := r.execByScope(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var e error
		total, e = q.CountAlertRules(ctx, sqlcgen.CountAlertRulesParams{Scope: scope, TenantID: pgtypex.Int8(tenantID)})
		if e != nil {
			return e
		}
		rows, e = q.ListAlertRules(ctx, sqlcgen.ListAlertRulesParams{Scope: scope, TenantID: pgtypex.Int8(tenantID), OffsetCount: int32((page - 1) * size), LimitCount: int32(size)})
		return e
	})
	if err != nil {
		return nil, 0, apperr.ErrAdminAlertInvalid.WithCause(err)
	}
	return alertRuleDTOsFromRows(rows), total, nil
}

// CreateAlertRule 创建告警规则。
func (r *repo) CreateAlertRule(ctx context.Context, ruleID, tenantID int64, req AlertRuleRequest) (AlertRuleDTO, error) {
	condition, err := jsonx.ObjectBytes(req.Condition, apperr.ErrAdminAlertInvalid)
	if err != nil {
		return AlertRuleDTO{}, err
	}
	var row sqlcgen.AlertRule
	err = r.execByScope(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var e error
		row, e = q.CreateAlertRule(ctx, sqlcgen.CreateAlertRuleParams{
			ID: ruleID, Scope: req.Scope, TenantID: pgtypex.Int8(tenantID), Name: req.Name, Metric: req.Metric,
			Condition: condition, Level: req.Level, Enabled: req.Enabled,
		})
		return e
	})
	if err != nil {
		return AlertRuleDTO{}, apperr.ErrAdminAlertInvalid.WithCause(err)
	}
	return alertRuleDTOFromRow(row), nil
}

// UpdateAlertRule 更新告警规则。
func (r *repo) UpdateAlertRule(ctx context.Context, tenantID, ruleID int64, req AlertRulePatchRequest) (AlertRuleDTO, error) {
	condition, err := jsonx.ObjectBytes(req.Condition, apperr.ErrAdminAlertInvalid)
	if err != nil {
		return AlertRuleDTO{}, err
	}
	var row sqlcgen.AlertRule
	err = r.inApp(ctx, func(q *sqlcgen.Queries) error {
		var e error
		row, e = q.UpdateAlertRule(ctx, sqlcgen.UpdateAlertRuleParams{ID: ruleID, TenantID: pgtypex.Int8(tenantID), Name: req.Name, Metric: req.Metric, Condition: condition, Level: req.Level, Enabled: req.Enabled})
		return e
	})
	if err != nil {
		return AlertRuleDTO{}, apperr.ErrAdminAlertNotFound.WithCause(err)
	}
	return alertRuleDTOFromRow(row), nil
}

// ListAlertEvents 查询告警事件。
func (r *repo) ListAlertEvents(ctx context.Context, tenantID int64, status int16, page, size int) ([]AlertEventDTO, int64, error) {
	page, size = pagex.Normalize(page, size)
	var rows []sqlcgen.AlertEvent
	var total int64
	err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		var e error
		total, e = q.CountAlertEvents(ctx, sqlcgen.CountAlertEventsParams{Status: pgtypex.Int2(status), TenantID: pgtypex.Int8(tenantID)})
		if e != nil {
			return e
		}
		rows, e = q.ListAlertEvents(ctx, sqlcgen.ListAlertEventsParams{Status: pgtypex.Int2(status), TenantID: pgtypex.Int8(tenantID), OffsetCount: int32((page - 1) * size), LimitCount: int32(size)})
		return e
	})
	if err != nil {
		return nil, 0, apperr.ErrAdminAlertInvalid.WithCause(err)
	}
	return alertEventDTOsFromRows(rows), total, nil
}

// GetAlertEvent 读取告警事件。
func (r *repo) GetAlertEvent(ctx context.Context, tenantID, eventID int64) (AlertEventDTO, error) {
	var row sqlcgen.AlertEvent
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		var e error
		row, e = q.GetAlertEventByID(ctx, sqlcgen.GetAlertEventByIDParams{ID: eventID, TenantID: pgtypex.Int8(tenantID)})
		return e
	}); err != nil {
		return AlertEventDTO{}, apperr.ErrAdminAlertNotFound.WithCause(err)
	}
	return alertEventDTOFromRow(row), nil
}

// HandleAlertEvent 处理或忽略告警事件。
func (r *repo) HandleAlertEvent(ctx context.Context, tenantID, eventID, handlerID int64, status int16) (AlertEventDTO, error) {
	var row sqlcgen.AlertEvent
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		var e error
		row, e = q.HandleAlertEvent(ctx, sqlcgen.HandleAlertEventParams{ID: eventID, TenantID: pgtypex.Int8(tenantID), HandlerID: pgtypex.Int8(handlerID), Status: status})
		return e
	}); err != nil {
		return AlertEventDTO{}, apperr.ErrAdminAlertState.WithCause(err)
	}
	return alertEventDTOFromRow(row), nil
}

// RevertAlertEvent 回滚刚处理的告警事件状态,避免通知失败时留下半成功状态。
func (r *repo) RevertAlertEvent(ctx context.Context, tenantID, eventID int64) error {
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		_, e := q.RevertAlertEvent(ctx, sqlcgen.RevertAlertEventParams{ID: eventID, TenantID: pgtypex.Int8(tenantID)})
		return e
	}); err != nil {
		return apperr.ErrAdminAlertNotifyFailed.WithCause(err)
	}
	return nil
}

// ListBackups 查询备份记录。
func (r *repo) ListBackups(ctx context.Context, page, size int) ([]BackupRecordDTO, int64, error) {
	page, size = pagex.Normalize(page, size)
	var rows []sqlcgen.BackupRecord
	var total int64
	err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		var e error
		total, e = q.CountBackups(ctx)
		if e != nil {
			return e
		}
		rows, e = q.ListBackups(ctx, sqlcgen.ListBackupsParams{OffsetCount: int32((page - 1) * size), LimitCount: int32(size)})
		return e
	})
	if err != nil {
		return nil, 0, apperr.ErrAdminBackupInvalid.WithCause(err)
	}
	return backupDTOsFromRows(rows), total, nil
}

// CreateBackupRecord 创建进行中的备份记录。
func (r *repo) CreateBackupRecord(ctx context.Context, id int64, req BackupTriggerRequest) (BackupRecordDTO, error) {
	var row sqlcgen.BackupRecord
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		var e error
		row, e = q.CreateBackupRecord(ctx, sqlcgen.CreateBackupRecordParams{ID: id, Type: req.Type, StorageRef: req.StorageRef, Status: BackupStatusRunning})
		return e
	}); err != nil {
		return BackupRecordDTO{}, apperr.ErrAdminBackupInvalid.WithCause(err)
	}
	return backupDTOFromRow(row), nil
}

// execByScope 根据 tenantID 选择平台级或租户级事务。
func (r *repo) execByScope(ctx context.Context, tenantID int64, fn queryFunc) error {
	if tenantID > 0 {
		return r.inTenant(ctx, tenantID, fn)
	}
	return r.inApp(ctx, fn)
}
