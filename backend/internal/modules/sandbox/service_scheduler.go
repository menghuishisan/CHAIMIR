// M2 沙箱后台调度器:扫描生命周期到期沙箱并复用 service 回收闭环。
package sandbox

import (
	"context"
	"time"

	"chaimir/internal/platform/background"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
)

// StartRecycleScheduler 启动 M2 生命周期回收调度器,由 platform/background 统一管理后台循环。
func (s *Service) StartRecycleScheduler(ctx context.Context) {
	background.Run(ctx, background.Task{
		Name:     "sandbox.recycle_scheduler",
		Interval: time.Duration(s.cfg.RecyclePollIntervalSeconds) * time.Second,
		Run:      s.RecycleDueSandboxesOnce,
	})
}

// RecycleDueSandboxesOnce 扫描并处理一轮 due 沙箱回收与快照过期清理。
func (s *Service) RecycleDueSandboxesOnce(ctx context.Context) error {
	limit, readyIdleTimeout, err := s.recycleSchedulerParams()
	if err != nil {
		return err
	}
	rows, err := s.listDueSandboxRecycles(ctx, limit, readyIdleTimeout)
	if err != nil {
		return err
	}
	for _, row := range rows {
		reason := sandboxRecycleReason(row)
		if err := s.recordSchedulerRecycleEvent(ctx, row, reason); err != nil {
			return apperr.ErrSandboxRecycleFinalizeFail.WithCause(err)
		}
		if err := s.finalizeSandboxRecycle(ctx, row.TenantID, row, reason); err != nil {
			return apperr.ErrSandboxRecycleFinalizeFail.WithCause(err)
		}
	}
	return s.cleanupExpiredSandboxSnapshots(ctx, limit)
}

// recycleSchedulerParams 校验后台调度运行边界,避免配置缺失导致全表扫描或空跑。
func (s *Service) recycleSchedulerParams() (int32, int32, error) {
	limit := int32(s.cfg.RecycleBatchSize)
	readyIdleTimeout := int32(s.cfg.ReadyIdleTimeoutSeconds)
	if limit <= 0 || readyIdleTimeout <= 0 {
		return 0, 0, apperr.ErrSandboxSchedulerConfigInvalid
	}
	return limit, readyIdleTimeout, nil
}

// listDueSandboxRecycles 在受控特权事务中锁定本模块自有表的回收候选行。
func (s *Service) listDueSandboxRecycles(ctx context.Context, limit, readyIdleTimeout int32) ([]SandboxLifecycleSnapshot, error) {
	return s.repo.listDueSandboxRecycles(ctx, limit, readyIdleTimeout)
}

// recordSchedulerRecycleEvent 在沙箱所属租户下记录调度器触发的回收事件。
func (s *Service) recordSchedulerRecycleEvent(ctx context.Context, row SandboxLifecycleSnapshot, reason string) error {
	detail, err := jsonx.ObjectBytes(map[string]any{"reason": reason}, apperr.ErrSandboxInvalidState)
	if err != nil {
		return err
	}
	return s.repo.createSandboxEvent(ctx, row.TenantID, row.ID, s.idgen.Generate(), SandboxEventRecycle, detail)
}

// cleanupExpiredSandboxSnapshots 删除到期保留的快照 Namespace,释放 PVC 与 VolumeSnapshot。
func (s *Service) cleanupExpiredSandboxSnapshots(ctx context.Context, limit int32) error {
	rows, err := s.repo.listExpiredSandboxSnapshots(ctx, limit)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err := s.orchestrator.Recycle(ctx, row.Namespace); err != nil {
			return apperr.ErrSandboxSnapshotCleanupFail.WithCause(err)
		}
		detail, err := jsonx.ObjectBytes(map[string]any{
			"reason": "snapshot-expired",
			"status": "snapshot_cleaned",
		}, apperr.ErrSandboxInvalidState)
		if err != nil {
			return err
		}
		if err := s.repo.createSandboxEvent(ctx, row.TenantID, row.ID, s.idgen.Generate(), SandboxEventRecycle, detail); err != nil {
			return apperr.ErrSandboxSnapshotCleanupFail.WithCause(err)
		}
	}
	return nil
}

// sandboxRecycleReason 根据已锁定候选行生成稳定回收原因,供事件和审计消费。
func sandboxRecycleReason(row SandboxLifecycleSnapshot) string {
	now := timex.Now()
	if row.Status == SandboxStatusRecycling {
		return "retry"
	}
	if expireAt := row.ExpireAt; !expireAt.IsZero() && !expireAt.After(now) {
		return "lifetime-expired"
	}
	if keepAliveUntil := row.KeepAliveUntil; !keepAliveUntil.IsZero() && !keepAliveUntil.After(now) {
		return "keepalive-expired"
	}
	if row.Status == SandboxStatusCreating || row.Status == SandboxStatusReady {
		return "ready-idle-timeout"
	}
	return "idle-timeout"
}
