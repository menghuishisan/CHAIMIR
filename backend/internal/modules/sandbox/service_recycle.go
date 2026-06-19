// sandbox service_recycle 文件实现空闲、最长生命周期、来源级联和快照到期回收流程。
package sandbox

import (
	"context"
	"log/slog"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/response"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
)

// RunRecycleOnce 执行一轮回收扫描,供后台调度器和测试复用。
func (s *Service) RunRecycleOnce(ctx context.Context) error {
	if s.cfg.RecycleBatchSize <= 0 || s.cfg.ReadyIdleTimeoutSeconds <= 0 || s.cfg.ReadyTimeoutSeconds <= 0 {
		return apperr.ErrSandboxRecycleConfigInvalid
	}
	readyDeadline := timex.Now().Add(-time.Duration(s.cfg.ReadyIdleTimeoutSeconds) * time.Second)
	var candidates []Sandbox
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		if _, err := tx.MarkIdleSandboxes(ctx); err != nil {
			return apperr.ErrSandboxRecycleScanFailed.WithCause(err)
		}
		var err error
		candidates, err = tx.ListRecycleCandidates(ctx, readyDeadline, int32(s.cfg.RecycleBatchSize))
		if err != nil {
			return apperr.ErrSandboxRecycleScanFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return err
	}
	var firstErr error
	for _, candidate := range candidates {
		if err := s.recycleOne(ctx, candidate, "scheduled"); err != nil {
			logging.ErrorContext(ctx, "sandbox scheduled recycle failed", err.Error(), slog.Int64("tenant_id", candidate.TenantID), slog.Int64("sandbox_id", candidate.ID))
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	var snapshots []Sandbox
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		snapshots, err = tx.ListSnapshotCleanupCandidates(ctx, int32(s.cfg.RecycleBatchSize))
		if err != nil {
			return apperr.ErrSandboxRecycleScanFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return err
	}
	for _, snapshot := range snapshots {
		if err := s.orchestrator.CleanupSnapshotResources(ctx, snapshot); err != nil {
			wrapped := apperr.ErrSandboxSnapshotCleanupFailed.WithCause(err)
			logging.ErrorContext(ctx, "sandbox snapshot cleanup failed", err.Error(), slog.Int64("tenant_id", snapshot.TenantID), slog.Int64("sandbox_id", snapshot.ID), slog.String("namespace", snapshot.Namespace))
			if firstErr == nil {
				firstErr = wrapped
			}
			continue
		}
		if err := s.store.TenantTx(ctx, snapshot.TenantID, func(ctx context.Context, tx TxStore) error {
			if _, err := tx.UpdateSandboxSnapshot(ctx, snapshot.TenantID, snapshot.ID, "", nil, time.Time{}, time.Time{}); err != nil {
				return apperr.ErrSandboxStatePersistFailed.WithCause(err)
			}
			return nil
		}); err != nil {
			logging.ErrorContext(ctx, "sandbox snapshot cleanup state update failed", err.Error(), slog.Int64("tenant_id", snapshot.TenantID), slog.Int64("sandbox_id", snapshot.ID))
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// recycleOne 按保存代码、快照、删除资源、写终态、审计、发布事件的顺序回收单个沙箱。
func (s *Service) recycleOne(ctx context.Context, sb Sandbox, reason string) error {
	locked, err := s.lockRecycle(ctx, sb)
	if err != nil {
		return err
	}
	sb = locked
	if shouldPersistBeforeRecycle(sb) {
		if _, _, err := s.saveSandboxFiles(ctx, sb.TenantID, sb.ID); err != nil {
			s.markRecycleFailed(ctx, sb, err)
			return err
		}
	}
	if sb.SnapshotEnabled {
		retention := time.Until(sb.SnapshotExpireAt)
		if retention <= 0 {
			retention = time.Minute
		}
		snapshotCtx, cancel := context.WithTimeout(ctx, timeDurationSeconds(s.cfg.ReadyTimeoutSeconds))
		plan, err := s.planForExistingSandbox(ctx, sb)
		if err != nil {
			cancel()
			s.markRecycleFailed(ctx, sb, err)
			return err
		}
		result, err := s.orchestrator.CreateSnapshot(snapshotCtx, plan, retention)
		cancel()
		if err != nil {
			s.markRecycleFailed(ctx, sb, err)
			return apperr.ErrSandboxRecycleFailed.WithCause(err)
		}
		if err := s.store.TenantTx(ctx, sb.TenantID, func(ctx context.Context, tx TxStore) error {
			_, err := tx.UpdateSandboxSnapshot(ctx, sb.TenantID, sb.ID, result.Ref, result.Domains, timex.Now(), sb.SnapshotExpireAt)
			return err
		}); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		if err := s.orchestrator.StopComputeKeepSnapshot(ctx, sb); err != nil {
			s.markRecycleFailed(ctx, sb, err)
			return apperr.ErrSandboxRecycleFailed.WithCause(err)
		}
	} else if err := s.orchestrator.DestroySandboxResources(ctx, sb); err != nil {
		s.markRecycleFailed(ctx, sb, err)
		return apperr.ErrSandboxRecycleFailed.WithCause(err)
	}
	if err := s.store.TenantTx(ctx, sb.TenantID, func(ctx context.Context, tx TxStore) error {
		if _, err := tx.UpdateSandboxPhaseStatus(ctx, sb.TenantID, sb.ID, sb.Phase, SandboxStatusDestroyed); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		detail, err := jsonBytes(map[string]any{"reason": reason})
		if err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		if err := tx.CreateSandboxEvent(ctx, s.ids.Generate(), sb.TenantID, sb.ID, EventTypeRecycle, detail); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		_, err = tx.CreateSandboxRecycleOutbox(ctx, s.ids.Generate(), sb, reason, response.TraceFromContext(ctx), timex.Now())
		if err != nil {
			return apperr.ErrSandboxRecycleEventPublishFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return err
	}
	s.broadcastProgress(ctx, sb.TenantID, sb.ID, sb.Phase, SandboxStatusDestroyed, response.TraceFromContext(ctx))
	if err := s.writeSystemAudit(ctx, sb.TenantID, "sandbox.recycle", "sandbox", sb.ID, map[string]any{"reason": reason, "source_ref": sb.SourceRef}); err != nil {
		return err
	}
	s.drainSandboxRecycleOutboxBestEffort(ctx)
	return nil
}

// RunSandboxRecycleOutboxOnce 领取并发布沙箱回收事件,供后台任务和事务后补偿调用。
func (s *Service) RunSandboxRecycleOutboxOnce(ctx context.Context) error {
	limit := int32(s.cfg.RecycleOutboxBatchSize)
	if limit <= 0 || s.cfg.RecycleOutboxStaleMs <= 0 {
		return apperr.ErrSandboxRecycleEventPublishFailed
	}
	staleBefore := timex.Now().Add(-time.Duration(s.cfg.RecycleOutboxStaleMs) * time.Millisecond)
	var items []SandboxRecycleOutbox
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		items, err = tx.ClaimPendingSandboxRecycleOutbox(ctx, limit, staleBefore)
		if err != nil {
			return apperr.ErrSandboxRecycleEventPublishFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return err
	}
	for _, item := range items {
		if err := s.publishSandboxRecycleOutboxItem(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// publishSandboxRecycleOutboxItem 发布单条回收事件并按结果回写 outbox 状态。
func (s *Service) publishSandboxRecycleOutboxItem(ctx context.Context, item SandboxRecycleOutbox) error {
	eventCtx := response.WithTrace(ctx, item.TraceID)
	payload := contracts.SandboxRecycledEvent{TenantID: item.TenantID, TraceID: item.TraceID, SandboxID: item.SandboxID, SourceRef: item.SourceRef, Reason: item.Reason, RecycledAt: item.RecycledAt}
	if err := s.bus.Publish(eventCtx, contracts.SubjectSandboxRecycled, payload); err != nil {
		s.recordSandboxRecycleOutboxFailure(eventCtx, item, err)
		return apperr.ErrSandboxRecycleEventPublishFailed.WithCause(err)
	}
	return s.markSandboxRecycleOutboxPublished(eventCtx, item)
}

// markSandboxRecycleOutboxPublished 用特权事务标记回收事件投递成功。
func (s *Service) markSandboxRecycleOutboxPublished(ctx context.Context, item SandboxRecycleOutbox) error {
	return s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		_, err := tx.MarkSandboxRecycleOutboxPublished(ctx, item.TenantID, item.ID)
		if err != nil {
			return apperr.ErrSandboxRecycleEventPublishFailed.WithCause(err)
		}
		return nil
	})
}

// recordSandboxRecycleOutboxFailure 记录回收事件发布失败并等待后台重试。
func (s *Service) recordSandboxRecycleOutboxFailure(ctx context.Context, item SandboxRecycleOutbox, cause error) {
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		_, err := tx.MarkSandboxRecycleOutboxFailed(ctx, item.TenantID, item.ID, logging.SanitizeError(cause.Error()))
		return err
	}); err != nil {
		logging.ErrorContext(ctx, "sandbox recycle outbox failure mark failed", err.Error(), slog.Int64("tenant_id", item.TenantID), slog.Int64("sandbox_id", item.SandboxID), slog.Int64("outbox_id", item.ID))
	}
}

// drainSandboxRecycleOutboxBestEffort 在请求提交后尽快投递,失败只记录日志并交给后台任务补偿。
func (s *Service) drainSandboxRecycleOutboxBestEffort(ctx context.Context) {
	if err := s.RunSandboxRecycleOutboxOnce(ctx); err != nil {
		logging.ErrorContext(ctx, "sandbox recycle outbox drain failed", err.Error())
	}
}

// shouldPersistBeforeRecycle 判断回收前是否必须保存工作区代码。
func shouldPersistBeforeRecycle(sb Sandbox) bool {
	return !(sb.Phase == SandboxPhaseAllocating && sb.CodeHash == "")
}

// lockRecycle 在任何资源清理前先把沙箱锁定为 recycling,保证失败后调度器可继续重试。
func (s *Service) lockRecycle(ctx context.Context, sb Sandbox) (Sandbox, error) {
	if sb.Status == SandboxStatusRecycling {
		return sb, nil
	}
	if err := validateStateTransition(sb.Status, SandboxStatusRecycling); err != nil {
		return Sandbox{}, err
	}
	var locked Sandbox
	if err := s.store.TenantTx(ctx, sb.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		locked, err = tx.UpdateSandboxPhaseStatus(ctx, sb.TenantID, sb.ID, sb.Phase, SandboxStatusRecycling)
		if err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return Sandbox{}, err
	}
	locked.Namespace = sb.Namespace
	locked.RuntimeID = sb.RuntimeID
	locked.ImageID = sb.ImageID
	locked.SourceRef = sb.SourceRef
	locked.OwnerAccountID = sb.OwnerAccountID
	locked.SnapshotEnabled = sb.SnapshotEnabled
	locked.CodeStorageKey = sb.CodeStorageKey
	locked.CodeHash = sb.CodeHash
	locked.InitCodeRef = sb.InitCodeRef
	locked.InitScriptRef = sb.InitScriptRef
	locked.SnapshotRef = sb.SnapshotRef
	locked.SnapshotDomains = sb.SnapshotDomains
	locked.SnapshotExpireAt = sb.SnapshotExpireAt
	return locked, nil
}

// markRecycleFailed 标记回收失败并保留资源,等待下一轮重试或人工处理。
func (s *Service) markRecycleFailed(ctx context.Context, sb Sandbox, cause error) {
	if err := s.store.TenantTx(ctx, sb.TenantID, func(ctx context.Context, tx TxStore) error {
		_, err := tx.UpdateSandboxPhaseStatus(ctx, sb.TenantID, sb.ID, sb.Phase, SandboxStatusRecycling)
		if err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		detail, err := jsonBytes(map[string]any{"stage": "recycle", "error": logging.SanitizeError(cause.Error())})
		if err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		if err := tx.CreateSandboxEvent(ctx, s.ids.Generate(), sb.TenantID, sb.ID, EventTypeError, detail); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		logging.ErrorContext(ctx, "sandbox recycle failure mark failed", apperr.AsAppError(err).LogString(), slog.Int64("tenant_id", sb.TenantID), slog.Int64("sandbox_id", sb.ID))
	}
	s.broadcastProgress(ctx, sb.TenantID, sb.ID, sb.Phase, SandboxStatusRecycling, response.TraceFromContext(ctx))
}
