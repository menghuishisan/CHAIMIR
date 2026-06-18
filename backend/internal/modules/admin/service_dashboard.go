// admin service_dashboard 文件实现 M9 看板跨模块只读聚合辅助逻辑。
package admin

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/pkg/apperr"
)

// operationsStats 保存 M9 从下层只读契约汇总出的运营指标。
type operationsStats struct {
	CourseCount          int64
	ActiveCourseCount    int64
	LearningDurationSec  int64
	ExperimentCount      int64
	ActiveInstanceCount  int64
	ContestCount         int64
	ActiveContestCount   int64
	ParticipantCount     int64
	ActiveSandboxCount   int64
	MaxConcurrentSandbox int64
	MaxCPU               int64
	MaxMemoryMB          int64
}

// ResourceQuotaSnapshot 输出平台看板需要的资源配额摘要。
func (o operationsStats) ResourceQuotaSnapshot() map[string]any {
	return map[string]any{
		"max_concurrent_sandbox": o.MaxConcurrentSandbox,
		"max_cpu":                o.MaxCPU,
		"max_memory_mb":          o.MaxMemoryMB,
	}
}

// aggregateTenantOperations 通过下层只读 contracts 汇总全平台运营指标。
func (s *Service) aggregateTenantOperations(ctx context.Context, tenants []contracts.TenantSummary) (operationsStats, error) {
	var out operationsStats
	for _, item := range tenants {
		if item.TenantID <= 0 {
			continue
		}
		t, err := s.teaching.Stats(ctx, item.TenantID)
		if err != nil {
			return operationsStats{}, apperr.ErrAdminDashboardTeachingFailed.WithCause(err)
		}
		out.CourseCount += t.CourseCount
		out.ActiveCourseCount += t.ActiveCourseCount
		out.LearningDurationSec += t.LearningDurationSec

		e, err := s.experiment.Stats(ctx, contracts.ExperimentStatsQuery{TenantID: item.TenantID})
		if err != nil {
			return operationsStats{}, apperr.ErrAdminDashboardExperimentFailed.WithCause(err)
		}
		out.ExperimentCount += e.ExperimentCount
		out.ActiveInstanceCount += e.ActiveInstanceCount

		c, err := s.contest.Stats(ctx, item.TenantID)
		if err != nil {
			return operationsStats{}, apperr.ErrAdminDashboardContestFailed.WithCause(err)
		}
		out.ContestCount += c.ContestCount
		out.ActiveContestCount += c.ActiveContestCount
		out.ParticipantCount += c.ParticipantCount

		q, err := s.sandbox.Stats(ctx, item.TenantID)
		if err != nil {
			return operationsStats{}, apperr.ErrAdminDashboardSandboxFailed.WithCause(err)
		}
		out.ActiveSandboxCount += q.ActiveSandboxCount
		out.MaxConcurrentSandbox += int64(q.MaxConcurrentSandbox)
		out.MaxCPU += int64(q.MaxCPU)
		out.MaxMemoryMB += int64(q.MaxMemoryMB)
	}
	return out, nil
}
