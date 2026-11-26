// 在学校 RLS 事务内读取本模块拥有的告警指标。
package grade

import (
	"context"

	"chaimir/internal/contracts"
)

// ReadAlertMetrics 只读取本模块自有表。
func (t *txStore) ReadAlertMetrics(ctx context.Context, q contracts.AlertMetricQuery) (map[string]float64, error) {
	row, err := t.q.CountCurrentUnackedWarnings(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]float64{"grade.warning_count": float64(row)}, nil
}
