// 通过各模块只读契约汇总平台和学校看板。
package admin

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/timex"
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
func (o operationsStats) ResourceQuotaSnapshot() *ResourceQuotaSnapshotDTO {
	return &ResourceQuotaSnapshotDTO{MaxConcurrentSandbox: o.MaxConcurrentSandbox, MaxCPU: o.MaxCPU, MaxMemoryMB: o.MaxMemoryMB}
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

// PlatformDashboard 聚合平台级看板。
func (s *Service) PlatformDashboard(ctx context.Context) (DashboardDTO, error) {
	if _, err := requirePlatform(ctx); err != nil {
		return DashboardDTO{}, err
	}
	stats, err := s.stats.PlatformStats(ctx)
	if err != nil {
		return DashboardDTO{}, apperr.ErrAdminDashboardIdentityFailed.WithCause(err)
	}
	tenants, err := s.identity.ListTenants(ctx)
	if err != nil {
		return DashboardDTO{}, apperr.ErrAdminDashboardIdentityFailed.WithCause(err)
	}
	ops, err := s.aggregateTenantOperations(ctx, tenants)
	if err != nil {
		return DashboardDTO{}, err
	}
	return DashboardDTO{Scope: ScopeGlobal, TenantCount: stats.TenantCount, AccountCount: stats.AccountCount, TeacherCount: stats.TeacherCount, StudentCount: stats.StudentCount, ActiveAccountCount: stats.ActiveAccountCount, CourseCount: ops.CourseCount, ActiveCourseCount: ops.ActiveCourseCount, ExperimentCount: ops.ExperimentCount, ActiveInstanceCount: ops.ActiveInstanceCount, ContestCount: ops.ContestCount, ActiveContestCount: ops.ActiveContestCount, ActiveSandboxCount: ops.ActiveSandboxCount, PendingApplyCount: stats.PendingApplyCount, ResourceQuotaSnapshot: ops.ResourceQuotaSnapshot(), GeneratedAt: timex.Now()}, nil
}

// SchoolDashboard 聚合学校级看板。
func (s *Service) SchoolDashboard(ctx context.Context) (DashboardDTO, error) {
	id, err := s.requireTenantAdmin(ctx)
	if err != nil {
		return DashboardDTO{}, err
	}
	stats, err := s.stats.TenantStats(ctx, id.TenantID)
	if err != nil {
		return DashboardDTO{}, apperr.ErrAdminDashboardIdentityFailed.WithCause(err)
	}
	out := DashboardDTO{Scope: ScopeTenant, TenantID: ids.ID(id.TenantID), AccountCount: stats.AccountCount, TeacherCount: stats.TeacherCount, StudentCount: stats.StudentCount, ActiveAccountCount: stats.ActiveAccountCount, GeneratedAt: timex.Now()}
	t, err := s.teaching.Stats(ctx, id.TenantID)
	if err != nil {
		return DashboardDTO{}, apperr.ErrAdminDashboardTeachingFailed.WithCause(err)
	}
	out.CourseCount = t.CourseCount
	out.ActiveCourseCount = t.ActiveCourseCount
	e, err := s.experiment.Stats(ctx, contracts.ExperimentStatsQuery{TenantID: id.TenantID})
	if err != nil {
		return DashboardDTO{}, apperr.ErrAdminDashboardExperimentFailed.WithCause(err)
	}
	out.ExperimentCount = e.ExperimentCount
	out.ActiveInstanceCount = e.ActiveInstanceCount
	c, err := s.contest.Stats(ctx, id.TenantID)
	if err != nil {
		return DashboardDTO{}, apperr.ErrAdminDashboardContestFailed.WithCause(err)
	}
	out.ContestCount = c.ContestCount
	out.ActiveContestCount = c.ActiveContestCount
	q, err := s.sandbox.Stats(ctx, id.TenantID)
	if err != nil {
		return DashboardDTO{}, apperr.ErrAdminDashboardSandboxFailed.WithCause(err)
	}
	out.ActiveSandboxCount = q.ActiveSandboxCount
	out.ResourceQuotaSnapshot = &ResourceQuotaSnapshotDTO{
		MaxConcurrentSandbox: int64(q.MaxConcurrentSandbox),
		MaxCPU:               int64(q.MaxCPU),
		MaxMemoryMB:          int64(q.MaxMemoryMB),
	}
	return out, nil
}
