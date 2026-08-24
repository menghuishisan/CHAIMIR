// admin repo_data 文件负责 M9 repo 查询写入与 sqlc 行到模块 DTO 的转换。
package admin

import (
	"context"
	"time"

	"chaimir/internal/modules/admin/internal/sqlcgen"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pagex"
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
		item, err := configDTO(row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// GetSystemConfig 查询单条配置。
func (t *txStore) GetSystemConfig(ctx context.Context, scope int16, tenantID int64, key string) (ConfigDTO, error) {
	row, err := t.q.GetSystemConfig(ctx, sqlcgen.GetSystemConfigParams{Scope: scope, TenantID: pgtypex.Int8When(tenantID, tenantID > 0), Key: key})
	if err != nil {
		return ConfigDTO{}, err
	}
	return configDTO(row)
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
	return configDTO(row)
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
	return configDTO(row)
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
	return configLogDTO(row)
}

// GetConfigChangeLog 查询单条配置变更历史。
func (t *txStore) GetConfigChangeLog(ctx context.Context, id, configID int64) (ConfigChangeLogDTO, error) {
	row, err := t.q.GetConfigChangeLog(ctx, sqlcgen.GetConfigChangeLogParams{ID: id, ConfigID: configID})
	if err != nil {
		return ConfigChangeLogDTO{}, err
	}
	return configLogDTO(row)
}

// ListConfigChangeLogs 查询配置变更历史和总数。
func (t *txStore) ListConfigChangeLogs(ctx context.Context, configID int64, page, size int) ([]ConfigChangeLogDTO, int64, error) {
	limit, offset := pagex.LimitOffset(page, size)
	rows, err := t.q.ListConfigChangeLogs(ctx, sqlcgen.ListConfigChangeLogsParams{ConfigID: configID, Limit: limit, Offset: offset})
	if err != nil {
		return nil, 0, err
	}
	out := make([]ConfigChangeLogDTO, 0, len(rows))
	for _, row := range rows {
		item, err := configLogDTO(row)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, item)
	}
	total, err := t.q.CountConfigChangeLogs(ctx, configID)
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// CreateAlertRule 创建告警规则。
func (t *txStore) CreateAlertRule(ctx context.Context, id int64, req AlertRuleRequest) (AlertRuleDTO, error) {
	condition, err := jsonx.ObjectBytes(req.Condition, apperr.ErrAdminAlertInvalid)
	if err != nil {
		return AlertRuleDTO{}, err
	}
	row, err := t.q.CreateAlertRule(ctx, sqlcgen.CreateAlertRuleParams{ID: id, Scope: req.Scope, TenantID: pgtypex.Int8When(req.TenantID.Int64(), req.TenantID > 0), Name: req.Name, Metric: req.Metric, Condition: condition, Level: req.Level, Enabled: req.Enabled})
	if err != nil {
		return AlertRuleDTO{}, err
	}
	return alertRuleDTO(row)
}

// ListAlertRules 查询告警规则。
func (t *txStore) ListAlertRules(ctx context.Context, scope int16, tenantID int64) ([]AlertRuleDTO, error) {
	rows, err := t.q.ListAlertRules(ctx, sqlcgen.ListAlertRulesParams{Scope: scope, TenantID: pgtypex.Int8(tenantID)})
	if err != nil {
		return nil, err
	}
	out := make([]AlertRuleDTO, 0, len(rows))
	for _, row := range rows {
		item, err := alertRuleDTO(row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// UpdateAlertRule 更新告警规则。
func (t *txStore) UpdateAlertRule(ctx context.Context, id int64, req AlertRuleRequest) (AlertRuleDTO, error) {
	condition, err := jsonx.ObjectBytes(req.Condition, apperr.ErrAdminAlertInvalid)
	if err != nil {
		return AlertRuleDTO{}, err
	}
	row, err := t.q.UpdateAlertRule(ctx, sqlcgen.UpdateAlertRuleParams{ID: id, Name: req.Name, Metric: req.Metric, Condition: condition, Level: req.Level, Enabled: req.Enabled, Scope: req.Scope, TenantID: pgtypex.Int8When(req.TenantID.Int64(), req.TenantID > 0)})
	if err != nil {
		return AlertRuleDTO{}, err
	}
	return alertRuleDTO(row)
}

// CreateAlertEvent 创建告警事件。
func (t *txStore) CreateAlertEvent(ctx context.Context, id, ruleID, tenantID int64, level int16, message string) (AlertEventDTO, error) {
	row, err := t.q.CreateAlertEvent(ctx, sqlcgen.CreateAlertEventParams{ID: id, RuleID: ruleID, TenantID: pgtypex.Int8When(tenantID, tenantID > 0), Level: level, Message: message})
	if err != nil {
		return AlertEventDTO{}, err
	}
	return alertEventDTO(row), nil
}

// ListAlertEvents 查询告警事件和总数。
func (t *txStore) ListAlertEvents(ctx context.Context, status, level int16, tenantID int64, page, size int) ([]AlertEventDTO, int64, error) {
	limit, offset := pagex.LimitOffset(page, size)
	rows, err := t.q.ListAlertEvents(ctx, sqlcgen.ListAlertEventsParams{Status: status, Level: level, TenantID: pgtypex.Int8(tenantID), PageOffset: offset, PageLimit: limit})
	if err != nil {
		return nil, 0, err
	}
	out := make([]AlertEventDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, alertEventDTO(row))
	}
	total, err := t.q.CountAlertEvents(ctx, sqlcgen.CountAlertEventsParams{Status: status, Level: level, TenantID: pgtypex.Int8(tenantID)})
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// HandleAlertEvent 处理告警事件。
func (t *txStore) HandleAlertEvent(ctx context.Context, id, tenantID int64, status int16, handlerID int64) (AlertEventDTO, error) {
	row, err := t.q.HandleAlertEvent(ctx, sqlcgen.HandleAlertEventParams{ID: id, Status: status, HandlerID: pgtypex.Int8(handlerID), TenantID: pgtypex.Int8When(tenantID, tenantID > 0)})
	if err != nil {
		return AlertEventDTO{}, err
	}
	return alertEventDTO(row), nil
}

// ListPlatformStatistics 查询运营统计时间序列。
func (t *txStore) ListPlatformStatistics(ctx context.Context, scope int16, tenantID int64, fromDate, toDate string) ([]StatisticsDTO, error) {
	from, err := timex.ParseDate(fromDate)
	if err != nil {
		return nil, apperr.ErrAdminStatisticsInvalid.WithCause(err)
	}
	to, err := timex.ParseDate(toDate)
	if err != nil {
		return nil, apperr.ErrAdminStatisticsInvalid.WithCause(err)
	}
	rows, err := t.q.ListPlatformStatistics(ctx, sqlcgen.ListPlatformStatisticsParams{Scope: scope, TenantID: pgtypex.Int8When(tenantID, tenantID > 0), FromDate: pgtypex.Date(from), ToDate: pgtypex.Date(to)})
	if err != nil {
		return nil, err
	}
	out := make([]StatisticsDTO, 0, len(rows))
	for _, row := range rows {
		metrics, err := decodeStoredObject(row.Metrics, apperr.ErrAdminStatisticsInvalid)
		if err != nil {
			return nil, err
		}
		out = append(out, StatisticsDTO{Scope: row.Scope, TenantID: ids.ID(pgtypex.Int8Value(row.TenantID)), Date: timex.DateOrEmpty(pgtypex.DateValue(row.StatDate)), Metrics: metrics})
	}
	return out, nil
}

// UpsertPlatformStatistics 写入或更新运营统计快照。
func (t *txStore) UpsertPlatformStatistics(ctx context.Context, id int64, scope int16, tenantID int64, statDate string, metrics map[string]any) (StatisticsDTO, error) {
	date, err := timex.ParseDate(statDate)
	if err != nil {
		return StatisticsDTO{}, apperr.ErrAdminStatisticsInvalid.WithCause(err)
	}
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
	decodedMetrics, err := decodeStoredObject(row.Metrics, apperr.ErrAdminStatisticsInvalid)
	if err != nil {
		return StatisticsDTO{}, err
	}
	return StatisticsDTO{Scope: row.Scope, TenantID: ids.ID(pgtypex.Int8Value(row.TenantID)), Date: timex.DateOrEmpty(pgtypex.DateValue(row.StatDate)), Metrics: decodedMetrics}, nil
}

// CreateBackupRecord 写入真实运维备份执行结果。
func (t *txStore) CreateBackupRecord(ctx context.Context, id int64, req BackupRecordCreate) (BackupRecordDTO, error) {
	finishedAt := pgtype.Timestamptz{}
	if req.Status == BackupStatusSucceeded || req.Status == BackupStatusFailed {
		finishedAt = timex.RequiredTimestamptz(timex.Now())
	}
	row, err := t.q.CreateBackupRecord(ctx, sqlcgen.CreateBackupRecordParams{ID: id, Type: req.Type, StorageRef: req.StorageRef, SizeBytes: req.SizeBytes, Status: req.Status, FinishedAt: finishedAt})
	if err != nil {
		return BackupRecordDTO{}, err
	}
	return backupDTO(row), nil
}

// CreateAuditExportRequest 保存可由 M9 worker 重放的审计导出命令。
func (t *txStore) CreateAuditExportRequest(ctx context.Context, req AuditExportRequest) (AuditExportRequest, error) {
	row, err := t.q.CreateAuditExportRequest(ctx, sqlcgen.CreateAuditExportRequestParams{TransferTaskID: req.TransferTaskID, TenantID: req.TenantID, AccountID: req.AccountID, QuerySnapshot: req.QuerySnapshot})
	if err != nil {
		return AuditExportRequest{}, err
	}
	return auditExportRequestFromRow(row), nil
}

// ListDueAuditExportRequests 查询到期的 M9 导出请求。
func (t *txStore) ListDueAuditExportRequests(ctx context.Context, now time.Time, limit int32) ([]AuditExportRequest, error) {
	rows, err := t.q.ListDueAuditExportRequests(ctx, sqlcgen.ListDueAuditExportRequestsParams{NextCheckAt: timex.RequiredTimestamptz(now), Limit: limit})
	if err != nil {
		return nil, err
	}
	out := make([]AuditExportRequest, 0, len(rows))
	for _, row := range rows {
		out = append(out, auditExportRequestFromRow(row))
	}
	return out, nil
}

// SetAuditExportRequestNextCheck 延后尚未完成的 M9 导出请求。
func (t *txStore) SetAuditExportRequestNextCheck(ctx context.Context, tenantID, taskID int64, next time.Time) (AuditExportRequest, error) {
	row, err := t.q.SetAuditExportRequestNextCheck(ctx, sqlcgen.SetAuditExportRequestNextCheckParams{TransferTaskID: taskID, TenantID: tenantID, NextCheckAt: timex.RequiredTimestamptz(next)})
	if err != nil {
		return AuditExportRequest{}, err
	}
	return auditExportRequestFromRow(row), nil
}

// DeleteAuditExportRequest 删除已达到 transfer 终态的 M9 导出请求。
func (t *txStore) DeleteAuditExportRequest(ctx context.Context, tenantID, taskID int64) error {
	return t.q.DeleteAuditExportRequest(ctx, sqlcgen.DeleteAuditExportRequestParams{TransferTaskID: taskID, TenantID: tenantID})
}

// ListBackupRecords 查询备份记录和总数。
// status 传 0 表示不按结果过滤;总数按同一条件计,与列表同口径。
func (t *txStore) ListBackupRecords(ctx context.Context, status int16, page, size int) ([]BackupRecordDTO, int64, error) {
	limit, offset := pagex.LimitOffset(page, size)
	rows, err := t.q.ListBackupRecords(ctx, sqlcgen.ListBackupRecordsParams{Status: status, PageLimit: limit, PageOffset: offset})
	if err != nil {
		return nil, 0, err
	}
	out := make([]BackupRecordDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, backupDTO(row))
	}
	total, err := t.q.CountBackupRecords(ctx, status)
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// configDTO 转换配置行。
func configDTO(row sqlcgen.SystemConfig) (ConfigDTO, error) {
	value, err := decodeStoredObject(row.Value, apperr.ErrAdminConfigInvalid)
	if err != nil {
		return ConfigDTO{}, err
	}
	return ConfigDTO{ID: ids.ID(row.ID), Scope: row.Scope, TenantID: ids.ID(pgtypex.Int8Value(row.TenantID)), Key: row.Key, Value: value, Version: row.Version, UpdatedBy: ids.ID(row.UpdatedBy), UpdatedAt: row.UpdatedAt.Time}, nil
}

// configLogDTO 转换配置历史行。
func configLogDTO(row sqlcgen.ConfigChangeLog) (ConfigChangeLogDTO, error) {
	oldValue, err := decodeStoredObject(row.OldValue, apperr.ErrAdminConfigInvalid)
	if err != nil {
		return ConfigChangeLogDTO{}, err
	}
	newValue, err := decodeStoredObject(row.NewValue, apperr.ErrAdminConfigInvalid)
	if err != nil {
		return ConfigChangeLogDTO{}, err
	}
	return ConfigChangeLogDTO{ID: ids.ID(row.ID), ConfigID: ids.ID(row.ConfigID), TenantID: ids.ID(pgtypex.Int8Value(row.TenantID)), OldValue: oldValue, NewValue: newValue, OperatorID: ids.ID(row.OperatorID), CreatedAt: timex.RFC3339OrEmpty(timex.FromTimestamptz(row.CreatedAt))}, nil
}

// alertRuleDTO 转换告警规则行。
func alertRuleDTO(row sqlcgen.AlertRule) (AlertRuleDTO, error) {
	condition, err := decodeStoredObject(row.Condition, apperr.ErrAdminAlertInvalid)
	if err != nil {
		return AlertRuleDTO{}, err
	}
	return AlertRuleDTO{ID: ids.ID(row.ID), Scope: row.Scope, TenantID: ids.ID(pgtypex.Int8Value(row.TenantID)), Name: row.Name, Metric: row.Metric, Condition: condition, Level: row.Level, Enabled: row.Enabled, CreatedAt: timex.RFC3339OrEmpty(timex.FromTimestamptz(row.CreatedAt)), UpdatedAt: timex.RFC3339OrEmpty(timex.FromTimestamptz(row.UpdatedAt))}, nil
}

// decodeStoredObject 严格读取 JSONB 对象,损坏或形态错误时保留模块错误链而不是静默改为空对象。
func decodeStoredObject(raw []byte, invalid *apperr.Error) (map[string]any, error) {
	value, err := jsonx.ObjectMapStrict(raw)
	if err != nil {
		return nil, invalid.WithCause(err)
	}
	return value, nil
}

// auditExportRequestFromRow 转换 M9 审计导出请求行。
func auditExportRequestFromRow(row sqlcgen.AuditExportRequest) AuditExportRequest {
	return AuditExportRequest{TransferTaskID: row.TransferTaskID, TenantID: row.TenantID, AccountID: row.AccountID, QuerySnapshot: row.QuerySnapshot, NextCheckAt: timex.FromTimestamptz(row.NextCheckAt), CreatedAt: timex.FromTimestamptz(row.CreatedAt), UpdatedAt: timex.FromTimestamptz(row.UpdatedAt)}
}

// alertEventDTO 转换告警事件行。
func alertEventDTO(row sqlcgen.AlertEvent) AlertEventDTO {
	return AlertEventDTO{ID: ids.ID(row.ID), RuleID: ids.ID(row.RuleID), TenantID: ids.ID(pgtypex.Int8Value(row.TenantID)), Level: row.Level, Message: row.Message, Status: row.Status, HandlerID: ids.ID(pgtypex.Int8Value(row.HandlerID)), TriggeredAt: timex.RFC3339OrEmpty(timex.FromTimestamptz(row.TriggeredAt)), HandledAt: timex.RFC3339OrEmpty(timex.FromTimestamptz(row.HandledAt))}
}

// backupDTO 转换备份记录行。
func backupDTO(row sqlcgen.BackupRecord) BackupRecordDTO {
	return BackupRecordDTO{ID: ids.ID(row.ID), Type: row.Type, SizeBytes: row.SizeBytes, Status: row.Status, StartedAt: timex.RFC3339OrEmpty(timex.FromTimestamptz(row.StartedAt)), FinishedAt: timex.RFC3339OrEmpty(timex.FromTimestamptz(row.FinishedAt))}
}
