// 生成并读取 M9 周期统计快照。
package admin

import (
	"context"
	"errors"
	"fmt"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
)

// PlatformStatistics 读取平台级周期统计快照。
func (s *Service) PlatformStatistics(ctx context.Context, fromDate, toDate string) ([]StatisticsDTO, error) {
	if _, err := requirePlatform(ctx); err != nil {
		return nil, err
	}
	if err := validateDateRange(fromDate, toDate); err != nil {
		return nil, err
	}
	return runAdminRead(ctx, s.store, 0, func(ctx context.Context, tx TxStore) ([]StatisticsDTO, error) {
		rows, err := tx.ListPlatformStatistics(ctx, ScopeGlobal, 0, fromDate, toDate)
		if err != nil {
			return nil, apperr.ErrAdminStatisticsInvalid.WithCause(err)
		}
		return rows, nil
	})
}

// SchoolStatistics 读取当前学校的周期统计快照。
func (s *Service) SchoolStatistics(ctx context.Context, fromDate, toDate string) ([]StatisticsDTO, error) {
	id, err := s.requireTenantAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if err := validateDateRange(fromDate, toDate); err != nil {
		return nil, err
	}
	return runAdminRead(ctx, s.store, id.TenantID, func(ctx context.Context, tx TxStore) ([]StatisticsDTO, error) {
		rows, err := tx.ListPlatformStatistics(ctx, ScopeTenant, id.TenantID, fromDate, toDate)
		if err != nil {
			return nil, apperr.ErrAdminStatisticsInvalid.WithCause(err)
		}
		return rows, nil
	})
}

// RunStatisticsSnapshotOnce 生成当天平台与租户统计快照。
func (s *Service) RunStatisticsSnapshotOnce(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	statDate := timex.DateOrEmpty(timex.Now())
	tenants, err := s.identity.ListTenants(ctx)
	if err != nil {
		return errors.Join(apperr.ErrAdminStatisticsInvalid.WithCause(err), ctx.Err())
	}
	var failures error
	if err := s.snapshotGlobalStats(ctx, statDate, tenants); err != nil {
		failures = fmt.Errorf("生成平台统计快照失败: %w", err)
	}
	for _, item := range tenants {
		if err := ctx.Err(); err != nil {
			return errors.Join(failures, err)
		}
		if item.TenantID <= 0 {
			continue
		}
		if err := s.snapshotTenantStats(ctx, item.TenantID, statDate); err != nil {
			failures = errors.Join(failures, fmt.Errorf("生成学校统计快照失败 tenant_id=%d: %w", item.TenantID, err))
		}
	}
	return errors.Join(failures, ctx.Err())
}

// snapshotGlobalStats 聚合平台级只读指标并写入 M9 自有统计快照。
func (s *Service) snapshotGlobalStats(ctx context.Context, statDate string, tenants []contracts.TenantSummary) error {
	stats, err := s.stats.PlatformStats(ctx)
	if err != nil {
		return apperr.ErrAdminStatisticsInvalid.WithCause(err)
	}
	ops, err := s.aggregateTenantOperations(ctx, tenants)
	if err != nil {
		return apperr.ErrAdminStatisticsInvalid.WithCause(err)
	}
	metrics := map[string]any{
		"tenant_count":           stats.TenantCount,
		"account_count":          stats.AccountCount,
		"teacher_count":          stats.TeacherCount,
		"student_count":          stats.StudentCount,
		"active_account_count":   stats.ActiveAccountCount,
		"pending_apply_count":    stats.PendingApplyCount,
		"course_count":           ops.CourseCount,
		"active_course_count":    ops.ActiveCourseCount,
		"learning_duration_sec":  ops.LearningDurationSec,
		"experiment_count":       ops.ExperimentCount,
		"active_instance_count":  ops.ActiveInstanceCount,
		"contest_count":          ops.ContestCount,
		"active_contest_count":   ops.ActiveContestCount,
		"participant_count":      ops.ParticipantCount,
		"active_sandbox_count":   ops.ActiveSandboxCount,
		"max_concurrent_sandbox": ops.MaxConcurrentSandbox,
		"max_cpu":                ops.MaxCPU,
		"max_memory_mb":          ops.MaxMemoryMB,
	}
	return s.upsertStatistics(ctx, 0, ScopeGlobal, 0, statDate, metrics)
}

// snapshotTenantStats 聚合单租户只读指标并写入 M9 自有统计快照。
func (s *Service) snapshotTenantStats(ctx context.Context, tenantID int64, statDate string) error {
	stats, err := s.stats.TenantStats(ctx, tenantID)
	if err != nil {
		return apperr.ErrAdminStatisticsInvalid.WithCause(err)
	}
	metrics := map[string]any{
		"account_count":        stats.AccountCount,
		"teacher_count":        stats.TeacherCount,
		"student_count":        stats.StudentCount,
		"active_account_count": stats.ActiveAccountCount,
	}
	t, err := s.teaching.Stats(ctx, tenantID)
	if err != nil {
		return apperr.ErrAdminStatisticsInvalid.WithCause(err)
	}
	metrics["course_count"] = t.CourseCount
	metrics["active_course_count"] = t.ActiveCourseCount
	metrics["learning_duration_sec"] = t.LearningDurationSec
	e, err := s.experiment.Stats(ctx, contracts.ExperimentStatsQuery{TenantID: tenantID})
	if err != nil {
		return apperr.ErrAdminStatisticsInvalid.WithCause(err)
	}
	metrics["experiment_count"] = e.ExperimentCount
	metrics["active_instance_count"] = e.ActiveInstanceCount
	c, err := s.contest.Stats(ctx, tenantID)
	if err != nil {
		return apperr.ErrAdminStatisticsInvalid.WithCause(err)
	}
	metrics["contest_count"] = c.ContestCount
	metrics["active_contest_count"] = c.ActiveContestCount
	metrics["participant_count"] = c.ParticipantCount
	q, err := s.sandbox.Stats(ctx, tenantID)
	if err != nil {
		return apperr.ErrAdminStatisticsInvalid.WithCause(err)
	}
	metrics["active_sandbox_count"] = q.ActiveSandboxCount
	metrics["max_concurrent_sandbox"] = q.MaxConcurrentSandbox
	metrics["max_cpu"] = q.MaxCPU
	metrics["max_memory_mb"] = q.MaxMemoryMB
	return s.upsertStatistics(ctx, tenantID, ScopeTenant, tenantID, statDate, metrics)
}

// upsertStatistics 在正确的 RLS 边界内写入 M9 自有统计快照。
func (s *Service) upsertStatistics(ctx context.Context, txTenantID int64, scope int16, tenantID int64, statDate string, metrics map[string]any) error {
	run := func(ctx context.Context, tx TxStore) error {
		_, err := tx.UpsertPlatformStatistics(ctx, s.ids.Generate(), scope, tenantID, statDate, metrics)
		return err
	}
	var err error
	if txTenantID > 0 {
		err = s.store.TenantTx(ctx, txTenantID, run)
	} else {
		err = s.store.PlatformTx(ctx, run)
	}
	if err != nil {
		return apperr.ErrAdminStatisticsInvalid.WithCause(err)
	}
	return nil
}
