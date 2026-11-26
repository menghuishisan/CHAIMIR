// 在学校 RLS 事务内读取本模块拥有的告警指标。
package sandbox

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/sandbox/internal/sqlcgen"
	"chaimir/internal/platform/timex"
)

// ReadAlertMetrics 只读取本模块自有表。
func (t *txStore) ReadAlertMetrics(ctx context.Context, q contracts.AlertMetricQuery) (map[string]float64, error) {
	row, err := t.q.ReadSandboxAlertMetrics(ctx, sqlcgen.ReadSandboxAlertMetricsParams{ObservedAt: timex.RequiredTimestamptz(q.To), WindowFrom: timex.RequiredTimestamptz(q.From)})
	if err != nil {
		return nil, err
	}
	return map[string]float64{"sandbox.active_count": float64(row.ActiveCount), "sandbox.pending_seconds": row.PendingSeconds, "sandbox.failed_rate": row.FailedRate}, nil
}
