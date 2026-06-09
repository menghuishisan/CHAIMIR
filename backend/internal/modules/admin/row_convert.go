// M9 行转换:把 admin 自有表的 sqlc 行模型转换为 HTTP/服务层 DTO。
package admin

import (
	"time"

	"chaimir/internal/modules/admin/internal/sqlcgen"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
)

// statisticDTOFromRow 转换统计快照行。
func statisticDTOFromRow(row sqlcgen.PlatformStatistic) StatisticDTO {
	statDate := ""
	if row.StatDate.Valid {
		statDate = row.StatDate.Time.Format(time.DateOnly)
	}
	return StatisticDTO{
		ID:        ids.Format(row.ID),
		Scope:     row.Scope,
		TenantID:  pgtypex.IDString(row.TenantID),
		StatDate:  statDate,
		Metrics:   jsonx.ObjectMap(row.Metrics),
		CreatedAt: timex.FromTimestamptz(row.CreatedAt),
	}
}

// configDTOFromRow 转换系统配置行。
func configDTOFromRow(row sqlcgen.SystemConfig) ConfigDTO {
	return ConfigDTO{
		ID:        ids.Format(row.ID),
		Scope:     row.Scope,
		TenantID:  pgtypex.IDString(row.TenantID),
		Key:       row.Key,
		Value:     jsonx.ObjectMap(row.Value),
		Version:   row.Version,
		UpdatedBy: ids.Format(row.UpdatedBy),
		UpdatedAt: timex.FromTimestamptz(row.UpdatedAt),
	}
}

// configDTOsFromRows 批量转换系统配置行。
func configDTOsFromRows(rows []sqlcgen.SystemConfig) []ConfigDTO {
	out := make([]ConfigDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, configDTOFromRow(row))
	}
	return out
}

// configChangeDTOFromRow 转换配置变更历史行。
func configChangeDTOFromRow(row sqlcgen.ConfigChangeLog) ConfigChangeLogDTO {
	return ConfigChangeLogDTO{
		ID:         ids.Format(row.ID),
		ConfigID:   ids.Format(row.ConfigID),
		TenantID:   pgtypex.IDString(row.TenantID),
		OldValue:   jsonx.ObjectMap(row.OldValue),
		NewValue:   jsonx.ObjectMap(row.NewValue),
		OperatorID: ids.Format(row.OperatorID),
		CreatedAt:  timex.FromTimestamptz(row.CreatedAt),
	}
}

// configChangeDTOsFromRows 批量转换配置变更历史行。
func configChangeDTOsFromRows(rows []sqlcgen.ConfigChangeLog) []ConfigChangeLogDTO {
	out := make([]ConfigChangeLogDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, configChangeDTOFromRow(row))
	}
	return out
}

// alertRuleDTOFromRow 转换告警规则行。
func alertRuleDTOFromRow(row sqlcgen.AlertRule) AlertRuleDTO {
	return AlertRuleDTO{
		ID:        ids.Format(row.ID),
		Scope:     row.Scope,
		TenantID:  pgtypex.IDString(row.TenantID),
		Name:      row.Name,
		Metric:    row.Metric,
		Condition: jsonx.ObjectMap(row.Condition),
		Level:     row.Level,
		Enabled:   row.Enabled,
		CreatedAt: timex.FromTimestamptz(row.CreatedAt),
		UpdatedAt: timex.FromTimestamptz(row.UpdatedAt),
	}
}

// alertRuleDTOsFromRows 批量转换告警规则行。
func alertRuleDTOsFromRows(rows []sqlcgen.AlertRule) []AlertRuleDTO {
	out := make([]AlertRuleDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, alertRuleDTOFromRow(row))
	}
	return out
}

// alertEventDTOFromRow 转换告警事件行。
func alertEventDTOFromRow(row sqlcgen.AlertEvent) AlertEventDTO {
	return AlertEventDTO{
		ID:          ids.Format(row.ID),
		RuleID:      ids.Format(row.RuleID),
		TenantID:    pgtypex.IDString(row.TenantID),
		Level:       row.Level,
		Message:     row.Message,
		Status:      row.Status,
		HandlerID:   pgtypex.IDString(row.HandlerID),
		TriggeredAt: timex.FromTimestamptz(row.TriggeredAt),
		HandledAt:   timex.FromTimestamptz(row.HandledAt),
	}
}

// alertEventDTOsFromRows 批量转换告警事件行。
func alertEventDTOsFromRows(rows []sqlcgen.AlertEvent) []AlertEventDTO {
	out := make([]AlertEventDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, alertEventDTOFromRow(row))
	}
	return out
}

// backupDTOFromRow 转换备份记录行。
func backupDTOFromRow(row sqlcgen.BackupRecord) BackupRecordDTO {
	return BackupRecordDTO{
		ID:         ids.Format(row.ID),
		Type:       row.Type,
		StorageRef: row.StorageRef,
		SizeBytes:  row.SizeBytes,
		Status:     row.Status,
		StartedAt:  timex.FromTimestamptz(row.StartedAt),
		FinishedAt: timex.FromTimestamptz(row.FinishedAt),
	}
}

// backupDTOsFromRows 批量转换备份记录行。
func backupDTOsFromRows(rows []sqlcgen.BackupRecord) []BackupRecordDTO {
	out := make([]BackupRecordDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, backupDTOFromRow(row))
	}
	return out
}
