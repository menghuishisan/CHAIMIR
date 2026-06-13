// admin repo_convert 文件负责 M9 repo 查询写入与 sqlc 行到模块 DTO 的转换。
package admin

import (
	"context"
	"time"

	"chaimir/internal/modules/admin/internal/sqlcgen"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"

	"github.com/jackc/pgx/v5/pgtype"
)

// ListSystemConfigs 查询配置列表。
func (t *txStore) ListSystemConfigs(ctx context.Context, scope int16, tenantID int64) ([]ConfigDTO, error) {
	rows, err := t.q.ListSystemConfigs(ctx, sqlcgen.ListSystemConfigsParams{Scope: scope, TenantID: pgtypex.Int8(tenantID)})
	if err != nil {
		return nil, err
	}
	out := make([]ConfigDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, configDTO(row))
	}
	return out, nil
}

// GetSystemConfig 查询单条配置。
func (t *txStore) GetSystemConfig(ctx context.Context, scope int16, tenantID int64, key string) (ConfigDTO, error) {
	row, err := t.q.GetSystemConfig(ctx, sqlcgen.GetSystemConfigParams{Scope: scope, TenantID: pgtypex.Int8When(tenantID, tenantID > 0), Key: key})
	if err != nil {
		return ConfigDTO{}, err
	}
	return configDTO(row), nil
}

// CreateSystemConfig 创建配置项。
func (t *txStore) CreateSystemConfig(ctx context.Context, id int64, scope int16, tenantID int64, key string, value map[string]any, operatorID int64) (ConfigDTO, error) {
	data, err := jsonx.ObjectBytes(value, apperr.ErrAdminConfigInvalid)
	if err != nil {
		return ConfigDTO{}, err
	}
	row, err := t.q.CreateSystemConfig(ctx, sqlcgen.CreateSystemConfigParams{ID: id, Scope: scope, TenantID: pgtypex.Int8When(tenantID, tenantID > 0), Key: key, Value: data, UpdatedBy: operatorID})
	if err != nil {
		return ConfigDTO{}, err
	}
	return configDTO(row), nil
}

// UpdateSystemConfig 按乐观锁更新配置项。
func (t *txStore) UpdateSystemConfig(ctx context.Context, scope int16, tenantID int64, key string, value map[string]any, operatorID int64, version int32) (ConfigDTO, error) {
	data, err := jsonx.ObjectBytes(value, apperr.ErrAdminConfigInvalid)
	if err != nil {
		return ConfigDTO{}, err
	}
	row, err := t.q.UpdateSystemConfig(ctx, sqlcgen.UpdateSystemConfigParams{Value: data, UpdatedBy: operatorID, Scope: scope, TenantID: pgtypex.Int8When(tenantID, tenantID > 0), Key: key, Version: version})
	if err != nil {
		return ConfigDTO{}, err
	}
	return configDTO(row), nil
}

// CreateConfigChangeLog 写入配置变更历史。
func (t *txStore) CreateConfigChangeLog(ctx context.Context, id, configID, tenantID int64, oldValue, newValue map[string]any, operatorID int64) (ConfigChangeLogDTO, error) {
	oldData, err := jsonx.ObjectBytes(oldValue, apperr.ErrAdminConfigInvalid)
	if err != nil {
		return ConfigChangeLogDTO{}, err
	}
	newData, err := jsonx.ObjectBytes(newValue, apperr.ErrAdminConfigInvalid)
	if err != nil {
		return ConfigChangeLogDTO{}, err
	}
	row, err := t.q.CreateConfigChangeLog(ctx, sqlcgen.CreateConfigChangeLogParams{ID: id, ConfigID: configID, TenantID: pgtypex.Int8When(tenantID, tenantID > 0), OldValue: oldData, NewValue: newData, OperatorID: operatorID})
	if err != nil {
		return ConfigChangeLogDTO{}, err
	}
	return configLogDTO(row), nil
}

// GetConfigChangeLog 查询单条配置变更历史。
func (t *txStore) GetConfigChangeLog(ctx context.Context, id, configID int64) (ConfigChangeLogDTO, error) {
	row, err := t.q.GetConfigChangeLog(ctx, sqlcgen.GetConfigChangeLogParams{ID: id, ConfigID: configID})
	if err != nil {
		return ConfigChangeLogDTO{}, err
	}
	return configLogDTO(row), nil
}

// ListConfigChangeLogs 查询配置变更历史。
func (t *txStore) ListConfigChangeLogs(ctx context.Context, configID int64, page, size int) ([]ConfigChangeLogDTO, error) {
	rows, err := t.q.ListConfigChangeLogs(ctx, sqlcgen.ListConfigChangeLogsParams{ConfigID: configID, Limit: int32(size), Offset: int32((page - 1) * size)})
	if err != nil {
		return nil, err
	}
	out := make([]ConfigChangeLogDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, configLogDTO(row))
	}
	return out, nil
}

// CreateAlertRule 创建告警规则。
func (t *txStore) CreateAlertRule(ctx context.Context, id int64, req AlertRuleRequest) (AlertRuleDTO, error) {
	condition, err := jsonx.ObjectBytes(req.Condition, apperr.ErrAdminAlertInvalid)
	if err != nil {
		return AlertRuleDTO{}, err
	}
	row, err := t.q.CreateAlertRule(ctx, sqlcgen.CreateAlertRuleParams{ID: id, Scope: req.Scope, TenantID: pgtypex.Int8When(req.TenantID, req.TenantID > 0), Name: req.Name, Metric: req.Metric, Condition: condition, Level: req.Level, Enabled: req.Enabled})
	if err != nil {
		return AlertRuleDTO{}, err
	}
	return alertRuleDTO(row), nil
}

// ListAlertRules 查询告警规则。
func (t *txStore) ListAlertRules(ctx context.Context, scope int16, tenantID int64) ([]AlertRuleDTO, error) {
	rows, err := t.q.ListAlertRules(ctx, sqlcgen.ListAlertRulesParams{Scope: scope, TenantID: pgtypex.Int8(tenantID)})
	if err != nil {
		return nil, err
	}
	out := make([]AlertRuleDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, alertRuleDTO(row))
	}
	return out, nil
}

// UpdateAlertRule 更新告警规则。
func (t *txStore) UpdateAlertRule(ctx context.Context, id int64, req AlertRuleRequest) (AlertRuleDTO, error) {
	condition, err := jsonx.ObjectBytes(req.Condition, apperr.ErrAdminAlertInvalid)
	if err != nil {
		return AlertRuleDTO{}, err
	}
	row, err := t.q.UpdateAlertRule(ctx, sqlcgen.UpdateAlertRuleParams{ID: id, Name: req.Name, Metric: req.Metric, Condition: condition, Level: req.Level, Enabled: req.Enabled})
	if err != nil {
		return AlertRuleDTO{}, err
	}
	return alertRuleDTO(row), nil
}

// CreateAlertEvent 创建告警事件。
func (t *txStore) CreateAlertEvent(ctx context.Context, id, ruleID, tenantID int64, level int16, message string) (AlertEventDTO, error) {
	row, err := t.q.CreateAlertEvent(ctx, sqlcgen.CreateAlertEventParams{ID: id, RuleID: ruleID, TenantID: pgtypex.Int8When(tenantID, tenantID > 0), Level: level, Message: message})
	if err != nil {
		return AlertEventDTO{}, err
	}
	return alertEventDTO(row), nil
}

// ListAlertEvents 查询告警事件。
func (t *txStore) ListAlertEvents(ctx context.Context, status int16, tenantID int64, page, size int) ([]AlertEventDTO, error) {
	rows, err := t.q.ListAlertEvents(ctx, sqlcgen.ListAlertEventsParams{Status: status, TenantID: pgtypex.Int8(tenantID), PageOffset: int32((page - 1) * size), PageLimit: int32(size)})
	if err != nil {
		return nil, err
	}
	out := make([]AlertEventDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, alertEventDTO(row))
	}
	return out, nil
}

// HandleAlertEvent 处理告警事件。
func (t *txStore) HandleAlertEvent(ctx context.Context, id int64, status int16, handlerID int64) (AlertEventDTO, error) {
	row, err := t.q.HandleAlertEvent(ctx, sqlcgen.HandleAlertEventParams{ID: id, Status: status, HandlerID: pgtypex.Int8(handlerID)})
	if err != nil {
		return AlertEventDTO{}, err
	}
	return alertEventDTO(row), nil
}

// ListPlatformStatistics 查询运营统计时间序列。
func (t *txStore) ListPlatformStatistics(ctx context.Context, scope int16, tenantID int64, fromDate, toDate string) ([]StatisticsDTO, error) {
	from, _ := time.Parse("2006-01-02", fromDate)
	to, _ := time.Parse("2006-01-02", toDate)
	rows, err := t.q.ListPlatformStatistics(ctx, sqlcgen.ListPlatformStatisticsParams{Scope: scope, TenantID: pgtypex.Int8When(tenantID, tenantID > 0), FromDate: pgtypex.Date(from), ToDate: pgtypex.Date(to)})
	if err != nil {
		return nil, err
	}
	out := make([]StatisticsDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, StatisticsDTO{Scope: row.Scope, TenantID: pgtypex.Int8Value(row.TenantID), Date: pgtypex.DateValue(row.StatDate).Format("2006-01-02"), Metrics: jsonx.ObjectMap(row.Metrics)})
	}
	return out, nil
}

// UpsertPlatformStatistics 写入或更新运营统计快照。
func (t *txStore) UpsertPlatformStatistics(ctx context.Context, id int64, scope int16, tenantID int64, statDate string, metrics map[string]any) (StatisticsDTO, error) {
	date, _ := time.Parse("2006-01-02", statDate)
	data, err := jsonx.ObjectBytes(metrics, apperr.ErrAdminStatisticsInvalid)
	if err != nil {
		return StatisticsDTO{}, err
	}
	var row sqlcgen.PlatformStatistic
	if scope == ScopeGlobal {
		row, err = t.q.UpsertGlobalPlatformStatistics(ctx, sqlcgen.UpsertGlobalPlatformStatisticsParams{ID: id, StatDate: pgtypex.Date(date), Metrics: data})
	} else {
		row, err = t.q.UpsertTenantPlatformStatistics(ctx, sqlcgen.UpsertTenantPlatformStatisticsParams{ID: id, Scope: scope, TenantID: pgtypex.Int8When(tenantID, tenantID > 0), StatDate: pgtypex.Date(date), Metrics: data})
	}
	if err != nil {
		return StatisticsDTO{}, err
	}
	return StatisticsDTO{Scope: row.Scope, TenantID: pgtypex.Int8Value(row.TenantID), Date: pgtypex.DateValue(row.StatDate).Format("2006-01-02"), Metrics: jsonx.ObjectMap(row.Metrics)}, nil
}

// CreateBackupRecord 创建备份记录。
func (t *txStore) CreateBackupRecord(ctx context.Context, id int64, typ int16, ref string, sizeBytes int64, status int16) (BackupRecordDTO, error) {
	finishedAt := pgtype.Timestamptz{}
	if status == BackupStatusSucceeded || status == BackupStatusFailed {
		finishedAt = timex.RequiredTimestamptz(timex.Now())
	}
	row, err := t.q.CreateBackupRecord(ctx, sqlcgen.CreateBackupRecordParams{ID: id, Type: typ, StorageRef: ref, SizeBytes: sizeBytes, Status: status, FinishedAt: finishedAt})
	if err != nil {
		return BackupRecordDTO{}, err
	}
	return backupDTO(row), nil
}

// ListBackupRecords 查询备份记录。
func (t *txStore) ListBackupRecords(ctx context.Context, page, size int) ([]BackupRecordDTO, error) {
	rows, err := t.q.ListBackupRecords(ctx, sqlcgen.ListBackupRecordsParams{Limit: int32(size), Offset: int32((page - 1) * size)})
	if err != nil {
		return nil, err
	}
	out := make([]BackupRecordDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, backupDTO(row))
	}
	return out, nil
}

// configDTO 转换配置行。
func configDTO(row sqlcgen.SystemConfig) ConfigDTO {
	return ConfigDTO{ID: row.ID, Scope: row.Scope, TenantID: pgtypex.Int8Value(row.TenantID), Key: row.Key, Value: jsonx.ObjectMap(row.Value), Version: row.Version, UpdatedBy: row.UpdatedBy, UpdatedAt: row.UpdatedAt.Time}
}

// configLogDTO 转换配置历史行。
func configLogDTO(row sqlcgen.ConfigChangeLog) ConfigChangeLogDTO {
	return ConfigChangeLogDTO{ID: row.ID, ConfigID: row.ConfigID, TenantID: pgtypex.Int8Value(row.TenantID), OldValue: jsonx.ObjectMap(row.OldValue), NewValue: jsonx.ObjectMap(row.NewValue), OperatorID: row.OperatorID, CreatedAt: row.CreatedAt.Time.Format(time.RFC3339)}
}

// alertRuleDTO 转换告警规则行。
func alertRuleDTO(row sqlcgen.AlertRule) AlertRuleDTO {
	return AlertRuleDTO{ID: row.ID, Scope: row.Scope, TenantID: pgtypex.Int8Value(row.TenantID), Name: row.Name, Metric: row.Metric, Condition: jsonx.ObjectMap(row.Condition), Level: row.Level, Enabled: row.Enabled, CreatedAt: row.CreatedAt.Time.Format(time.RFC3339), UpdatedAt: row.UpdatedAt.Time.Format(time.RFC3339)}
}

// alertEventDTO 转换告警事件行。
func alertEventDTO(row sqlcgen.AlertEvent) AlertEventDTO {
	return AlertEventDTO{ID: row.ID, RuleID: row.RuleID, TenantID: pgtypex.Int8Value(row.TenantID), Level: row.Level, Message: row.Message, Status: row.Status, HandlerID: pgtypex.Int8Value(row.HandlerID), TriggeredAt: row.TriggeredAt.Time.Format(time.RFC3339), HandledAt: formatOptionalTime(row.HandledAt)}
}

// backupDTO 转换备份记录行。
func backupDTO(row sqlcgen.BackupRecord) BackupRecordDTO {
	return BackupRecordDTO{ID: row.ID, Type: row.Type, StorageRef: row.StorageRef, SizeBytes: row.SizeBytes, Status: row.Status, StartedAt: row.StartedAt.Time.Format(time.RFC3339), FinishedAt: formatOptionalTime(row.FinishedAt)}
}

// formatOptionalTime 把可空时间转换为 API 字符串。
func formatOptionalTime(v pgtype.Timestamptz) string {
	if !v.Valid {
		return ""
	}
	return v.Time.UTC().Format(time.RFC3339)
}
