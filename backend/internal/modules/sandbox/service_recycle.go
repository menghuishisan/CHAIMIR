package sandbox

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/intx"
	"chaimir/internal/platform/response"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
)

var errSandboxResourceLeaseLost = errors.New("sandbox resource lease lost")

// RunRecycleOnce 执行一轮回收扫描,供后台调度器和测试复用。
func (s *Service) RunRecycleOnce(ctx context.Context) error {
	if s.cfg.RecycleBatchSize <= 0 || s.cfg.ReadyIdleTimeoutSeconds <= 0 || s.cfg.ReadyTimeoutSeconds <= 0 {
		return apperr.ErrSandboxRecycleConfigInvalid
	}
	batchSize, ok := intx.Int32(s.cfg.RecycleBatchSize)
	if !ok || batchSize <= 0 {
		return apperr.ErrSandboxRecycleConfigInvalid
	}
	var failures []error
	if err := s.recoverPendingSandboxStarts(ctx, batchSize); err != nil {
		failures = append(failures, err)
	}
	if err := s.cleanupExpiredRuntimeSelftests(ctx); err != nil {
		logging.ErrorContext(ctx, "expired runtime selftest cleanup failed", err.Error())
		failures = append(failures, err)
	}
	readyDeadline := timex.Now().Add(-time.Duration(s.cfg.ReadyIdleTimeoutSeconds) * time.Second)
	var candidates []Sandbox
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		if _, err := tx.MarkIdleSandboxes(ctx); err != nil {
			return apperr.ErrSandboxRecycleScanFailed.WithCause(err)
		}
		var err error
		candidates, err = tx.ListRecycleCandidates(ctx, readyDeadline, batchSize)
		if err != nil {
			return apperr.ErrSandboxRecycleScanFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return errors.Join(append(failures, err)...)
	}
	for _, candidate := range candidates {
		if err := s.recycleOne(ctx, candidate, "scheduled"); err != nil {
			logging.ErrorContext(ctx, "sandbox scheduled recycle failed", err.Error(), slog.Int64("tenant_id", candidate.TenantID), slog.Int64("sandbox_id", candidate.ID))
			failures = append(failures, err)
		}
	}
	var snapshots []Sandbox
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		snapshots, err = tx.ListSnapshotCleanupCandidates(ctx, batchSize)
		if err != nil {
			return apperr.ErrSandboxRecycleScanFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return errors.Join(append(failures, err)...)
	}
	for _, snapshot := range snapshots {
		if err := s.cleanupExpiredSnapshot(ctx, snapshot); err != nil {
			logging.ErrorContext(ctx, "sandbox snapshot cleanup failed", err.Error(), slog.Int64("tenant_id", snapshot.TenantID), slog.Int64("sandbox_id", snapshot.ID), slog.String("namespace", snapshot.Namespace))
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func (s *Service) recoverPendingSandboxStarts(ctx context.Context, limit int32) error {
	deadline := timex.Now().Add(-timeDurationSeconds(s.cfg.ReadyTimeoutSeconds))
	var pending []Sandbox
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		pending, err = tx.ListPendingSandboxStarts(ctx, deadline, limit)
		return err
	}); err != nil {
		return apperr.ErrSandboxRecycleScanFailed.WithCause(err)
	}
	var failures []error
	for _, sb := range pending {
		if sb.Phase != SandboxPhaseInitializing && sb.Status != SandboxStatusCreating {
			err := fmt.Errorf("暂停操作未在预算内确认完成: %w", context.DeadlineExceeded)
			failures = append(failures, errors.Join(err, s.queueFailedPause(ctx, sb, err, true)))
			continue
		}
		if sb.Phase == SandboxPhaseInitializing {
			err := fmt.Errorf("资源操作未在预算内确认完成，不能自动重放: %w", context.DeadlineExceeded)
			s.markStartupFailed(ctx, sb, "init", err, sb.Status)
			failures = append(failures, err)
			continue
		}
		plan, err := s.planForExistingSandbox(ctx, sb)
		if err != nil {
			s.markStartupFailed(ctx, sb, "start", err, SandboxStatusCreating)
			failures = append(failures, err)
			continue
		}
		s.scheduleSandboxStart(ctx, plan)
	}
	return errors.Join(failures...)
}

func (s *Service) cleanupExpiredSnapshot(ctx context.Context, snapshot Sandbox) error {
	leaseDuration := s.resourceLeaseDuration()
	if leaseDuration <= 0 {
		return apperr.ErrSandboxRecycleConfigInvalid
	}
	var claimed Sandbox
	var acquired bool
	if err := s.store.TenantTx(ctx, snapshot.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		claimed, acquired, err = tx.ClaimExpiredSandboxSnapshotCleanup(ctx, snapshot.TenantID, snapshot.ID, leaseDuration)
		return err
	}); err != nil {
		return apperr.ErrSandboxSnapshotCleanupFailed.WithCause(err)
	}
	if !acquired {
		return nil
	}

	leaseCtx, stopLease := s.maintainResourceLease(ctx, claimed)
	cleanupCtx, cancelCleanup := context.WithTimeout(leaseCtx, timeDurationSeconds(s.cfg.ReadyTimeoutSeconds))
	err := s.orchestrator.DestroySandboxResources(cleanupCtx, claimed)
	cancelCleanup()
	if err != nil {
		if leaseErr := stopLease(); leaseErr != nil {
			err = errors.Join(err, leaseErr)
		}
		s.markCleanupFailed(ctx, claimed, err)
		return apperr.ErrSandboxSnapshotCleanupFailed.WithCause(err)
	}
	if err := stopLease(); err != nil {
		s.markCleanupFailed(ctx, claimed, err)
		return apperr.ErrSandboxSnapshotCleanupFailed.WithCause(err)
	}

	persistCtx, cancelPersist := asyncPersistenceContext(ctx, timeDurationSeconds(s.cfg.ReadyTimeoutSeconds))
	defer cancelPersist()
	var completed bool
	err = s.store.TenantTx(persistCtx, claimed.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		completed, err = tx.CompleteExpiredSandboxSnapshotCleanup(ctx, claimed.TenantID, claimed.ID, claimed.resourceLeaseToken)
		if err != nil {
			return err
		}
		if !completed {
			return errSandboxResourceLeaseLost
		}
		return nil
	})
	if err != nil {
		s.markCleanupFailed(ctx, claimed, err)
		return apperr.ErrSandboxSnapshotCleanupFailed.WithCause(err)
	}
	return nil
}

func (s *Service) cleanupExpiredRuntimeSelftests(ctx context.Context) error {
	if s.cfg.SelftestRecycleTimeoutSeconds <= 0 {
		return apperr.ErrSandboxSelftestRecycleConfigInvalid
	}
	cleanupCtx, cancel := context.WithTimeout(ctx, timeDurationSeconds(s.cfg.SelftestRecycleTimeoutSeconds))
	defer cancel()
	return s.orchestrator.CleanupExpiredRuntimeSelftests(cleanupCtx)
}

func (s *Service) recycleOne(ctx context.Context, sb Sandbox, reason string) error {
	locked, claimed, err := s.lockRecycle(ctx, sb)
	if err != nil {
		return err
	}
	if !claimed {
		return nil
	}
	sb = locked
	leaseCtx, stopLease := s.maintainResourceLease(ctx, sb)
	if err := s.recycleResources(leaseCtx, sb, shouldPersistBeforeRecycle(sb)); err != nil {
		if leaseErr := stopLease(); leaseErr != nil {
			err = errors.Join(err, leaseErr)
		}
		s.markCleanupFailed(ctx, sb, err)
		return apperr.ErrSandboxRecycleFailed.WithCause(err)
	}
	if err := stopLease(); err != nil {
		s.markCleanupFailed(ctx, sb, err)
		return apperr.ErrSandboxRecycleFailed.WithCause(err)
	}
	if err := s.completeRecycle(ctx, sb, reason); err != nil {
		s.markCleanupFailed(ctx, sb, err)
		return err
	}
	s.broadcastProgress(ctx, sb.TenantID, sb.ID, sb.Phase, SandboxStatusDestroyed, response.TraceFromContext(ctx))
	s.drainSandboxRecycleOutboxBestEffort(ctx)
	return nil
}

func (s *Service) recycleResources(ctx context.Context, sb Sandbox, persistWorkspace bool) error {
	if persistWorkspace {
		plan, err := s.planForExistingSandbox(ctx, sb)
		if err != nil {
			return err
		}
		recoveryCtx, cancel := context.WithTimeout(ctx, timeDurationSeconds(s.cfg.ReadyTimeoutSeconds))
		err = s.orchestrator.PrepareWorkspaceArchive(recoveryCtx, plan)
		cancel()
		if err != nil {
			return apperr.ErrSandboxFilePersistFailed.WithCause(err)
		}
		if err := s.renewSandboxResourceLease(ctx, sb, s.resourceLeaseDuration()); err != nil {
			return apperr.ErrSandboxRecycleFailed.WithCause(err)
		}
		if _, err := s.saveSandboxFiles(ctx, sb); err != nil {
			return err
		}
	}
	if sb.Status != SandboxStatusFailed {
		if err := s.orchestrator.StopSandboxCompute(ctx, sb); err != nil {
			return apperr.ErrSandboxRecycleFailed.WithCause(err)
		}
	}
	if sb.SnapshotEnabled && sb.Status != SandboxStatusFailed && sb.Phase != SandboxPhaseAllocating && sb.Phase != SandboxPhaseInitializing && sb.SnapshotExpireAt.After(timex.Now()) {
		retention := time.Until(sb.SnapshotExpireAt)
		snapshotCtx, cancel := context.WithTimeout(ctx, timeDurationSeconds(s.cfg.ReadyTimeoutSeconds))
		plan, err := s.planForExistingSandbox(ctx, sb)
		if err != nil {
			cancel()
			return err
		}
		result, err := s.orchestrator.CreateSnapshot(snapshotCtx, plan, retention)
		cancel()
		if err != nil {
			return apperr.ErrSandboxRecycleFailed.WithCause(err)
		}
		if err := s.store.TenantTx(ctx, sb.TenantID, func(ctx context.Context, tx TxStore) error {
			_, err := tx.UpdateSandboxSnapshot(ctx, sb.TenantID, sb.ID, sb.resourceLeaseToken, result.Ref, result.Domains, timex.Now(), sb.SnapshotExpireAt)
			return err
		}); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		sb.SnapshotRef = result.Ref
	}
	cleanupCtx, cancelCleanup := context.WithTimeout(ctx, timeDurationSeconds(s.cfg.ReadyTimeoutSeconds))
	defer cancelCleanup()
	var cleanupErr error
	if sb.SnapshotRef != "" && sb.SnapshotExpireAt.After(timex.Now()) {
		cleanupErr = s.orchestrator.StopComputeKeepSnapshot(cleanupCtx, sb)
	} else {
		cleanupErr = s.orchestrator.DestroySandboxResources(cleanupCtx, sb)
	}
	if cleanupErr != nil {
		return apperr.ErrSandboxRecycleFailed.WithCause(cleanupErr)
	}
	return nil
}

func (s *Service) completeRecycle(ctx context.Context, sb Sandbox, reason string) error {
	detail, err := jsonBytes(map[string]any{"reason": reason})
	if err != nil {
		return apperr.ErrSandboxStatePersistFailed.WithCause(err)
	}
	persistCtx, cancel := asyncPersistenceContext(ctx, timeDurationSeconds(s.cfg.ReadyTimeoutSeconds))
	defer cancel()
	return s.store.TenantTx(persistCtx, sb.TenantID, func(ctx context.Context, tx TxStore) error {
		completed, err := tx.CompleteSandboxRecycle(ctx, sb.TenantID, sb.ID, sb.resourceLeaseToken)
		if err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		if !completed {
			return apperr.ErrSandboxRecycleFailed.WithCause(errSandboxResourceLeaseLost)
		}
		if err := tx.CreateSandboxEvent(ctx, s.ids.Generate(), sb.TenantID, sb.ID, EventTypeRecycle, detail); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		if _, err := tx.CreateSandboxRecycleOutbox(ctx, s.ids.Generate(), sb, reason, response.TraceFromContext(ctx), timex.Now()); err != nil {
			return apperr.ErrSandboxRecycleEventPublishFailed.WithCause(err)
		}
		return s.writeSystemAudit(ctx, sb.TenantID, "sandbox.recycle", "sandbox", sb.ID, map[string]any{"reason": reason, "source_ref": sb.SourceRef})
	})
}

// RunSandboxRecycleOutboxOnce 领取并发布沙箱回收事件,供后台任务和事务后补偿调用。
func (s *Service) RunSandboxRecycleOutboxOnce(ctx context.Context) error {
	limit, ok := intx.Int32(s.cfg.RecycleOutboxBatchSize)
	if !ok || limit <= 0 || s.cfg.RecycleOutboxStaleMs <= 0 {
		return apperr.ErrSandboxRecycleEventPublishFailed
	}
	now := timex.Now()
	staleBefore := now
	leaseUntil := now.Add(time.Duration(s.cfg.RecycleOutboxStaleMs) * time.Millisecond)
	var items []SandboxRecycleOutbox
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		maxAttempts, ok := intx.Int32(s.cfg.RecycleOutboxMaxAttempts)
		if !ok || maxAttempts <= 0 {
			return apperr.ErrSandboxRecycleEventPublishFailed
		}
		items, err = tx.ClaimPendingSandboxRecycleOutbox(ctx, limit, maxAttempts, staleBefore, leaseUntil)
		if err != nil {
			return apperr.ErrSandboxRecycleEventPublishFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return err
	}
	var firstErr error
	for _, item := range items {
		if err := s.publishSandboxRecycleOutboxItem(ctx, item); err != nil {
			logging.ErrorContext(ctx, "sandbox recycle outbox publish failed", err.Error(), slog.Int64("tenant_id", item.TenantID), slog.Int64("sandbox_id", item.SandboxID), slog.Int64("outbox_id", item.ID))
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (s *Service) publishSandboxRecycleOutboxItem(ctx context.Context, item SandboxRecycleOutbox) error {
	eventCtx := response.WithTrace(ctx, item.TraceID)
	payload := contracts.SandboxRecycledEvent{TenantID: item.TenantID, TraceID: item.TraceID, SandboxID: item.SandboxID, SourceRef: item.SourceRef, ScopeRef: item.ScopeRef, Reason: item.Reason, RecycledAt: item.RecycledAt}
	if err := s.bus.Publish(eventCtx, contracts.SubjectSandboxRecycled, payload); err != nil {
		s.recordSandboxRecycleOutboxFailure(eventCtx, item, err)
		return apperr.ErrSandboxRecycleEventPublishFailed.WithCause(err)
	}
	return s.markSandboxRecycleOutboxPublished(eventCtx, item)
}

func (s *Service) markSandboxRecycleOutboxPublished(ctx context.Context, item SandboxRecycleOutbox) error {
	return s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		_, err := tx.MarkSandboxRecycleOutboxPublished(ctx, item.TenantID, item.ID, item.LeaseToken)
		if err != nil {
			return apperr.ErrSandboxRecycleEventPublishFailed.WithCause(err)
		}
		return nil
	})
}

func (s *Service) recordSandboxRecycleOutboxFailure(ctx context.Context, item SandboxRecycleOutbox, cause error) {
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		_, err := tx.MarkSandboxRecycleOutboxFailed(ctx, item.TenantID, item.ID, logging.SanitizeError(cause.Error()), item.LeaseToken)
		return err
	}); err != nil {
		logging.ErrorContext(ctx, "sandbox recycle outbox failure mark failed", err.Error(), slog.Int64("tenant_id", item.TenantID), slog.Int64("sandbox_id", item.SandboxID), slog.Int64("outbox_id", item.ID))
	}
}

func (s *Service) drainSandboxRecycleOutboxBestEffort(ctx context.Context) {
	if err := s.RunSandboxRecycleOutboxOnce(ctx); err != nil {
		logging.ErrorContext(ctx, "sandbox recycle outbox drain failed", err.Error())
	}
}

func shouldPersistBeforeRecycle(sb Sandbox) bool {
	if sb.Status == SandboxStatusFailed {
		return false
	}
	if sb.Status == SandboxStatusRecycling && sb.recycleSaved {
		return false
	}
	return sb.Phase == SandboxPhaseReady || sb.Phase == SandboxPhaseFullyReady
}

func (s *Service) lockRecycle(ctx context.Context, sb Sandbox) (Sandbox, bool, error) {
	if sb.Status != SandboxStatusRecycling {
		if err := validateStateTransition(sb.Status, SandboxStatusRecycling); err != nil {
			return Sandbox{}, false, err
		}
	}
	leaseDuration := s.resourceLeaseDuration()
	if leaseDuration <= 0 {
		return Sandbox{}, false, apperr.ErrSandboxRecycleConfigInvalid
	}
	var claimed Sandbox
	var acquired bool
	err := s.store.TenantTx(ctx, sb.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		claimed, acquired, err = tx.ClaimSandboxRecycle(ctx, sb.TenantID, sb.ID, sb.Status, leaseDuration)
		if err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		return nil
	})
	if err != nil {
		return Sandbox{}, false, err
	}
	return claimed, acquired, nil
}

func (s *Service) maintainResourceLease(ctx context.Context, sb Sandbox) (context.Context, func() error) {
	leaseDuration := s.resourceLeaseDuration()
	interval := leaseDuration / 3
	if interval <= 0 {
		interval = leaseDuration
	}
	leaseCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	var leaseErr error
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-leaseCtx.Done():
				return
			case <-ticker.C:
				renewCtx, cancelRenew := context.WithTimeout(leaseCtx, leaseDuration)
				leaseErr = s.renewSandboxResourceLease(renewCtx, sb, leaseDuration)
				cancelRenew()
				// 停止循环会取消正在续租的调用，该取消不表示执行权丢失。
				if errors.Is(leaseErr, context.Canceled) && leaseCtx.Err() != nil {
					leaseErr = nil
				}
				if leaseErr != nil {
					cancel()
					return
				}
			}
		}
	}()
	return leaseCtx, func() error {
		cancel()
		<-done
		return leaseErr
	}
}

func (s *Service) resourceLeaseDuration() time.Duration {
	if s.cfg.ExecTimeoutSeconds <= 0 || s.cfg.ReadyTimeoutSeconds <= 0 {
		return 0
	}
	return timeDurationSeconds(s.cfg.ExecTimeoutSeconds) + timeDurationSeconds(s.cfg.ReadyTimeoutSeconds)
}

func (s *Service) renewSandboxResourceLease(ctx context.Context, sb Sandbox, leaseDuration time.Duration) error {
	var renewed bool
	if err := s.store.TenantTx(ctx, sb.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		renewed, err = tx.RenewSandboxResourceLease(ctx, sb.TenantID, sb.ID, sb.resourceLeaseToken, leaseDuration)
		return err
	}); err != nil {
		return err
	}
	if !renewed {
		return errSandboxResourceLeaseLost
	}
	return nil
}

func (s *Service) markCleanupFailed(ctx context.Context, sb Sandbox, cause error) {
	stage := "recycle"
	if sb.Status == SandboxStatusDestroyed {
		stage = "snapshot_cleanup"
	}
	detail, err := jsonBytes(map[string]any{"stage": stage, "error": logging.SanitizeError(cause.Error())})
	if err != nil {
		logging.ErrorContext(ctx, "sandbox recycle failure detail encode failed", err.Error(), slog.Int64("tenant_id", sb.TenantID), slog.Int64("sandbox_id", sb.ID))
		return
	}
	persistCtx, cancel := asyncPersistenceContext(ctx, timeDurationSeconds(s.cfg.ReadyTimeoutSeconds))
	defer cancel()
	var released bool
	err = s.store.TenantTx(persistCtx, sb.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		released, err = tx.ReleaseSandboxResourceLease(ctx, sb.TenantID, sb.ID, sb.resourceLeaseToken)
		if err != nil || !released {
			return err
		}
		return tx.CreateSandboxEvent(ctx, s.ids.Generate(), sb.TenantID, sb.ID, EventTypeError, detail)
	})
	if err != nil {
		logging.ErrorContext(persistCtx, "sandbox recycle failure mark failed", apperr.AsAppError(err).LogString(), slog.Int64("tenant_id", sb.TenantID), slog.Int64("sandbox_id", sb.ID))
		return
	}
	if !released {
		return
	}
	if sb.Status == SandboxStatusRecycling {
		s.broadcastProgress(ctx, sb.TenantID, sb.ID, sb.Phase, SandboxStatusRecycling, response.TraceFromContext(ctx))
	}
}

// RecycleByScopeRef 按生命周期作用域级联回收沙箱。
func (s *Service) RecycleByScopeRef(ctx context.Context, req contracts.SandboxRecycleRequest) error {
	if req.TenantID <= 0 || !validSourceRef(req.SourceRef) || !auth.ValidScopeRef(req.ScopeRef) {
		return apperr.ErrSandboxRecycleRequestInvalid
	}
	var items []Sandbox
	if err := s.store.TenantTx(ctx, req.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		items, err = tx.ListSandboxesByScopeRef(ctx, req.TenantID, req.ScopeRef)
		if err != nil {
			return apperr.ErrSandboxRecycleScanFailed.WithCause(err)
		}
		for i := range items {
			item := &items[i]
			if item.Status != SandboxStatusRecycling {
				if err := validateStateTransition(item.Status, SandboxStatusRecycling); err != nil {
					return err
				}
			}
			queued, err := tx.QueueSandboxRecycle(ctx, item.TenantID, item.ID, item.Status)
			if err != nil {
				return apperr.ErrSandboxStatePersistFailed.WithCause(err)
			}
			if !queued {
				return apperr.ErrSandboxStatePersistFailed
			}
		}
		return nil
	}); err != nil {
		return err
	}
	for _, item := range items {
		if err := s.recycleOne(ctx, item, req.Reason); err != nil {
			return err
		}
	}
	return nil
}
