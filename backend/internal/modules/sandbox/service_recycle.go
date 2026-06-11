// sandbox service_recycle 文件实现空闲、最长生命周期、来源级联和快照到期回收流程。
package sandbox

import (
	"context"
	"log/slog"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
	"chaimir/pkg/response"
)

// RunRecycleOnce 执行一轮回收扫描,供后台调度器和测试复用。
func (s *Service) RunRecycleOnce(ctx context.Context) error {
	if s.cfg.RecycleBatchSize <= 0 || s.cfg.ReadyIdleTimeoutSeconds <= 0 || s.cfg.ReadyTimeoutSeconds <= 0 {
		return apperr.ErrSandboxRecycleConfigInvalid
	}
	readyDeadline := timex.Now().Add(-time.Duration(s.cfg.ReadyIdleTimeoutSeconds) * time.Second)
	var candidates []Sandbox
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		candidates, err = tx.ListRecycleCandidates(ctx, readyDeadline, int32(s.cfg.RecycleBatchSize))
		if err != nil {
			return apperr.ErrSandboxRecycleScanFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return err
	}
	for _, candidate := range candidates {
		if err := s.recycleOne(ctx, candidate, "scheduled"); err != nil {
			return err
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
			return apperr.ErrSandboxSnapshotCleanupFailed.WithCause(err)
		}
		if err := s.store.TenantTx(ctx, snapshot.TenantID, func(ctx context.Context, tx TxStore) error {
			if _, err := tx.UpdateSandboxSnapshot(ctx, snapshot.TenantID, snapshot.ID, "", nil, time.Time{}, time.Time{}); err != nil {
				return apperr.ErrSandboxStatePersistFailed.WithCause(err)
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
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
		snapshotCtx, cancel := context.WithTimeout(ctx, time.Duration(s.cfg.ReadyTimeoutSeconds)*time.Second)
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
		return tx.CreateSandboxEvent(ctx, s.ids.Generate(), sb.TenantID, sb.ID, EventTypeRecycle, detail)
	}); err != nil {
		return err
	}
	s.broadcastProgress(sb.TenantID, sb.ID, sb.Phase, SandboxStatusDestroyed, response.TraceFromContext(ctx))
	if err := s.writeAudit(ctx, sb.TenantID, sb.OwnerAccountID, 5, "sandbox.recycle", "sandbox", sb.ID, map[string]any{"reason": reason, "source_ref": sb.SourceRef}); err != nil {
		return err
	}
	return s.bus.Publish(ctx, contracts.SubjectSandboxRecycled, contracts.SandboxRecycledEvent{
		TenantID:   sb.TenantID,
		SandboxID:  sb.ID,
		SourceRef:  sb.SourceRef,
		Reason:     reason,
		RecycledAt: timex.Now(),
	})
}

// shouldPersistBeforeRecycle 判断回收前是否必须保存工作区代码。
func shouldPersistBeforeRecycle(sb Sandbox) bool {
	return !(sb.Status == SandboxStatusFailed && sb.Phase == SandboxPhaseAllocating && sb.CodeHash == "")
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
			return err
		}
		detail, err := jsonBytes(map[string]any{"stage": "recycle", "error": logging.SanitizeError(cause.Error())})
		if err != nil {
			return err
		}
		return tx.CreateSandboxEvent(ctx, s.ids.Generate(), sb.TenantID, sb.ID, EventTypeError, detail)
	}); err != nil {
		logging.ErrorContext(ctx, "sandbox recycle failure mark failed", err.Error(), slog.Int64("tenant_id", sb.TenantID), slog.Int64("sandbox_id", sb.ID))
	}
	s.broadcastProgress(sb.TenantID, sb.ID, sb.Phase, SandboxStatusRecycling, response.TraceFromContext(ctx))
}
