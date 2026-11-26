// 保存审计导出筛选快照及下次扫描时间。
package admin

import (
	"context"
	"time"

	"chaimir/internal/modules/admin/internal/sqlcgen"
	"chaimir/internal/platform/timex"
)

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
