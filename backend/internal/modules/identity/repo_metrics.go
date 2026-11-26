// 在学校 RLS 事务内读取本模块拥有的告警指标。
package identity

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/identity/internal/sqlcgen"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
)

// ReadAlertMetrics 只读取本模块自有表。
func (t *txStore) ReadAlertMetrics(ctx context.Context, q contracts.AlertMetricQuery) (map[string]float64, error) {
	row, err := t.q.CountLoginFailures(ctx, sqlcgen.CountLoginFailuresParams{TenantID: pgtypex.Int8(q.TenantID), CreatedAt: timex.RequiredTimestamptz(q.From), CreatedAt_2: timex.RequiredTimestamptz(q.To)})
	if err != nil {
		return nil, err
	}
	return map[string]float64{"account.login_failed_count": float64(row)}, nil
}

// AlertRecipients 仅从当前租户账号与角色读取接收人编号。
func (t *txStore) AlertRecipients(ctx context.Context, tenantID int64) ([]int64, error) {
	return t.q.ListAlertRecipients(ctx, tenantID)
}
