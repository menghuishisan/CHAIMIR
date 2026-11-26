// 提供本模块的租户级告警汇总，不开放业务明细。
package judge

import (
	"context"

	"chaimir/internal/contracts"
)

// AlertMetrics 供 M9 只读聚合，不暴露业务明细。
func (s *Service) AlertMetrics(ctx context.Context, q contracts.AlertMetricQuery) (map[string]float64, error) {
	var out map[string]float64
	err := s.store.TenantTx(ctx, q.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.ReadAlertMetrics(ctx, q)
		return err
	})
	return out, err
}
