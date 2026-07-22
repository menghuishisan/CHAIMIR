// contest service_contract 文件实现 M8 对聚合层开放的只读契约。
package contest

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
)

// Stats 读取租户级竞赛统计,供 M9 学校看板聚合。
func (s *Service) Stats(ctx context.Context, tenantID int64) (contracts.ContestStats, error) {
	if tenantID <= 0 {
		var err error
		tenantID, err = currentServiceTenant(ctx)
		if err != nil {
			return contracts.ContestStats{}, err
		}
	}
	var stats ContestStatsSnapshot
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		stats, err = tx.Stats(ctx, tenantID)
		return err
	}); err != nil {
		return contracts.ContestStats{}, err
	}
	return contracts.ContestStats{TenantID: tenantID, ContestCount: stats.ContestCount, ActiveContestCount: stats.ActiveContestCount, ParticipantCount: stats.ParticipantCount}, nil
}

// ListStudentAchievements 读取学生竞赛成就,供 M11 与 GPA 分离展示。
func (s *Service) ListStudentAchievements(ctx context.Context, tenantID, studentID int64) ([]contracts.ContestAchievement, error) {
	if tenantID <= 0 {
		var err error
		tenantID, err = currentServiceTenant(ctx)
		if err != nil {
			return nil, err
		}
	}
	var records []StudentContestRecord
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		records, err = tx.ListStudentContestRecords(ctx, tenantID, studentID)
		return err
	}); err != nil {
		return nil, err
	}
	out := make([]contracts.ContestAchievement, 0, len(records))
	for _, item := range records {
		out = append(out, contestAchievementFromRecord(tenantID, studentID, item))
	}
	return out, nil
}

// ListMyContestRecords 读取当前学生自己的竞赛战绩。
func (s *Service) ListMyContestRecords(ctx context.Context) ([]ContestRecordDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var records []StudentContestRecord
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		records, err = tx.ListStudentContestRecords(ctx, id.TenantID, id.AccountID)
		return err
	}); err != nil {
		return nil, err
	}
	out := make([]ContestRecordDTO, 0, len(records))
	for _, item := range records {
		out = append(out, ContestRecordDTO{ContestID: ids.ID(item.ContestID), TeamID: ids.ID(item.TeamID), Score: item.Score, Rank: item.Rank, ContestName: item.ContestName, ContestStatus: item.ContestStatus})
	}
	return out, nil
}
