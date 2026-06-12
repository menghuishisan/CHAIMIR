// experiment service_contract 文件实现 M7 对聚合层开放的只读契约。
package experiment

import (
	"context"

	"chaimir/internal/contracts"
)

// GetInstanceScore 读取单个实验实例的最终得分。
func (s *Service) GetInstanceScore(ctx context.Context, tenantID, instanceID int64) (contracts.ExperimentScoreSnapshot, error) {
	var inst ExperimentInstance
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		inst, err = tx.GetInstance(ctx, tenantID, instanceID)
		return err
	}); err != nil {
		return contracts.ExperimentScoreSnapshot{}, err
	}
	return scoreSnapshotFromInstance(inst), nil
}

// Stats 按过滤条件读取实验统计,供 M9 学校看板聚合。
func (s *Service) Stats(ctx context.Context, query contracts.ExperimentStatsQuery) (contracts.ExperimentStats, error) {
	tenantID := query.TenantID
	if tenantID <= 0 {
		var err error
		tenantID, err = currentServiceTenant(ctx)
		if err != nil {
			return contracts.ExperimentStats{}, err
		}
	}
	var stats ExperimentStatsSnapshot
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		stats, err = tx.Stats(ctx, tenantID, query.CourseID)
		return err
	}); err != nil {
		return contracts.ExperimentStats{}, err
	}
	return contracts.ExperimentStats{TenantID: tenantID, CourseID: query.CourseID, ExperimentCount: stats.ExperimentCount, ActiveInstanceCount: stats.ActiveInstanceCount}, nil
}
