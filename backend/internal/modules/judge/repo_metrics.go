// 在学校 RLS 事务内读取本模块拥有的告警指标。
package judge

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/judge/internal/sqlcgen"
	"chaimir/internal/platform/timex"
)

// ReadAlertMetrics 只读取本模块自有表。
func (t *txStore) ReadAlertMetrics(ctx context.Context, q contracts.AlertMetricQuery) (map[string]float64, error) {
	row, err := t.q.ReadJudgeAlertMetrics(ctx, sqlcgen.ReadJudgeAlertMetricsParams{WindowFrom: timex.RequiredTimestamptz(q.From), ObservedAt: timex.RequiredTimestamptz(q.To)})
	if err != nil {
		return nil, err
	}
	return map[string]float64{"judge.queue_length": float64(row.QueueLength), "judge.failed_rate": row.FailedRate}, nil
}
