// 将 M9 持久化行转换为公开响应和内部请求。
package admin

import (
	"chaimir/internal/modules/admin/internal/sqlcgen"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
)

// alertRuleDTO 转换告警规则行。
func alertRuleDTO(row sqlcgen.AlertRule) (AlertRuleDTO, error) {
	condition, err := decodeStoredObject(row.Condition, apperr.ErrAdminAlertInvalid)
	if err != nil {
		return AlertRuleDTO{}, err
	}
	return AlertRuleDTO{revision: row.Revision, ID: ids.ID(row.ID), Scope: row.Scope, TenantID: ids.ID(pgtypex.Int8Value(row.TenantID)), Name: row.Name, Metric: row.Metric, Condition: condition, Level: row.Level, Enabled: row.Enabled, CreatedAt: timex.RFC3339OrEmpty(timex.FromTimestamptz(row.CreatedAt)), UpdatedAt: timex.RFC3339OrEmpty(timex.FromTimestamptz(row.UpdatedAt))}, nil
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
