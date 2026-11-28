package sandbox

import (
	"context"
	"errors"
	"strings"
	"time"

	"chaimir/internal/contracts"

	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
)

// PauseSandbox 暂停单个沙箱。
func (s *Service) PauseSandbox(ctx context.Context, req contracts.SandboxControlRequest) (resultErr error) {
	if s.resourceLeaseDuration() <= 0 {
		return apperr.ErrSandboxRecycleConfigInvalid
	}
	if err := validateSandboxControlRequest(req); err != nil {
		return err
	}
	tenantID, sandboxID := req.TenantID, req.SandboxID
	var sb Sandbox
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		sb, err = tx.GetSandbox(ctx, tenantID, sandboxID)
		if err != nil {
			return apperr.ErrSandboxNotFound.WithCause(err)
		}
		if sb.SourceRef != strings.TrimSpace(req.SourceRef) {
			return apperr.ErrSandboxOwnershipInvalid
		}
		if err := validateStateTransition(sb.Status, SandboxStatusPaused); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	if sb.Status == SandboxStatusPaused {
		return nil
	}
	var acquired bool
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		sb, acquired, err = tx.ClaimSandboxResources(ctx, tenantID, sandboxID, sb.Status, s.resourceLeaseDuration())
		return err
	}); err != nil {
		return apperr.ErrSandboxStatePersistFailed.WithCause(err)
	}
	if !acquired {
		return apperr.ErrSandboxStateInvalid
	}
	defer func() {
		if resultErr == nil {
			return
		}
		resultErr = errors.Join(resultErr, s.queueFailedPause(ctx, sb, resultErr, false))
	}()
	if err := s.withResourceLease(ctx, sb, func(ctx context.Context) error {
		plan, err := cleanupPlanForSandbox(sb)
		if err != nil {
			return err
		}
		archiveCtx, cancelArchive := context.WithTimeout(ctx, timeDurationSeconds(s.cfg.ReadyTimeoutSeconds))
		err = s.orchestrator.PrepareWorkspaceArchive(archiveCtx, plan)
		cancelArchive()
		if err != nil {
			return apperr.ErrSandboxFilePersistFailed.WithCause(err)
		}
		if _, err := s.saveSandboxFiles(ctx, sb); err != nil {
			return err
		}
		if err := s.orchestrator.StopSandboxCompute(ctx, sb); err != nil {
			return apperr.ErrSandboxRecycleFailed.WithCause(err)
		}
		if sb.SnapshotEnabled && sb.SnapshotExpireAt.After(timex.Now()) {
			retention := time.Until(sb.SnapshotExpireAt)
			snapshotCtx, cancel := context.WithTimeout(ctx, timeDurationSeconds(s.cfg.ReadyTimeoutSeconds))
			result, err := s.orchestrator.CreateSnapshot(snapshotCtx, plan, retention)
			cancel()
			if err != nil {
				return apperr.ErrSandboxRecycleFailed.WithCause(err)
			}
			if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
				if _, err := tx.UpdateSandboxSnapshot(ctx, tenantID, sandboxID, sb.resourceLeaseToken, result.Ref, result.Domains, timex.Now(), sb.SnapshotExpireAt); err != nil {
					return apperr.ErrSandboxStatePersistFailed.WithCause(err)
				}
				detail, err := jsonBytes(map[string]any{"snapshot_ref": result.Ref, "snapshot_domains": result.Domains})
				if err != nil {
					return apperr.ErrSandboxStatePersistFailed.WithCause(err)
				}
				if err := tx.CreateSandboxEvent(ctx, s.ids.Generate(), tenantID, sandboxID, EventTypePhaseChange, detail); err != nil {
					return apperr.ErrSandboxStatePersistFailed.WithCause(err)
				}
				return nil
			}); err != nil {
				return err
			}
			sb.SnapshotRef = result.Ref
			sb.SnapshotDomains = result.Domains
		}
		if err := s.orchestrator.StopComputeKeepSnapshot(ctx, sb); err != nil {
			return apperr.ErrSandboxRecycleFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return err
	}
	ctx, cancelPersist := asyncPersistenceContext(ctx, timeDurationSeconds(s.cfg.ReadyTimeoutSeconds))
	defer cancelPersist()
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetSandboxForUpdate(ctx, tenantID, sandboxID)
		if err != nil {
			return err
		}
		if current.resourceLeaseToken != sb.resourceLeaseToken || !current.resourceLeaseUntil.After(timex.Now()) || current.Phase != sb.Phase || (current.Status != SandboxStatusReady && current.Status != SandboxStatusRunning && current.Status != SandboxStatusIdle) {
			return apperr.ErrSandboxStateInvalid
		}
		if _, err := tx.ReleaseSandboxResourceLease(ctx, tenantID, sandboxID, sb.resourceLeaseToken); err != nil {
			return err
		}
		if _, err := tx.UpdateSandboxPhaseStatus(ctx, tenantID, sandboxID, sb.Phase, SandboxStatusPaused); err != nil {
			return err
		}
		detail, err := jsonBytes(map[string]any{"phase": sb.Phase, "status": SandboxStatusPaused})
		if err != nil {
			return err
		}
		if err := tx.CreateSandboxEvent(ctx, s.ids.Generate(), tenantID, sandboxID, EventTypePhaseChange, detail); err != nil {
			return err
		}
		return s.writeSystemAudit(ctx, tenantID, "sandbox.pause", "sandbox", sandboxID, nil)
	}); err != nil {
		return apperr.ErrSandboxStatePersistFailed.WithCause(err)
	}
	s.broadcastProgress(ctx, tenantID, sandboxID, sb.Phase, SandboxStatusPaused, logging.TraceFromContext(ctx))
	return nil
}

func (s *Service) queueFailedPause(ctx context.Context, expected Sandbox, cause error, expiredOnly bool) error {
	persistCtx, cancel := asyncPersistenceContext(ctx, timeDurationSeconds(s.cfg.ReadyTimeoutSeconds))
	defer cancel()
	queued := false
	err := s.store.TenantTx(persistCtx, expected.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetSandboxForUpdate(ctx, expected.TenantID, expected.ID)
		if err != nil {
			return err
		}
		if current.resourceLeaseToken != expected.resourceLeaseToken || current.resourceLeaseToken == "" || current.Phase != expected.Phase || current.Status != expected.Status || (expiredOnly && current.resourceLeaseUntil.After(timex.Now())) {
			return nil
		}
		queued, err = tx.QueueSandboxRecycle(ctx, current.TenantID, current.ID, current.Status)
		if err != nil {
			return err
		}
		if !queued {
			return apperr.ErrSandboxStateInvalid
		}
		released, err := tx.ReleaseSandboxResourceLease(ctx, current.TenantID, current.ID, current.resourceLeaseToken)
		if err != nil {
			return err
		}
		if !released {
			return errSandboxResourceLeaseLost
		}
		detail, err := jsonBytes(map[string]any{"stage": "pause", "error": logging.SanitizeError(cause.Error())})
		if err != nil {
			return err
		}
		return tx.CreateSandboxEvent(ctx, s.ids.Generate(), current.TenantID, current.ID, EventTypeError, detail)
	})
	if err != nil {
		return apperr.ErrSandboxStatePersistFailed.WithCause(err)
	}
	if queued {
		s.broadcastProgress(persistCtx, expected.TenantID, expected.ID, expected.Phase, SandboxStatusRecycling, logging.TraceFromContext(ctx))
	}
	return nil
}

// ResumeSandbox 恢复单个沙箱。
func (s *Service) ResumeSandbox(ctx context.Context, req contracts.SandboxControlRequest) error {
	if err := validateSandboxControlRequest(req); err != nil {
		return err
	}
	tenantID, sandboxID := req.TenantID, req.SandboxID
	var sb Sandbox
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		sb, err = tx.GetSandbox(ctx, tenantID, sandboxID)
		if err != nil {
			return apperr.ErrSandboxNotFound.WithCause(err)
		}
		if sb.SourceRef != strings.TrimSpace(req.SourceRef) {
			return apperr.ErrSandboxOwnershipInvalid
		}
		return nil
	}); err != nil {
		return err
	}
	if sb.Status == SandboxStatusPaused || (sb.Status == SandboxStatusDestroyed && strings.TrimSpace(sb.SnapshotRef) != "") {
		return s.resumeRetainedSandbox(ctx, sb)
	}
	if !sandboxExecAllowed(sb) {
		return apperr.ErrSandboxStateInvalid
	}
	return s.transition(ctx, sb, SandboxStatusRunning, "sandbox.resume")
}

// DestroySandbox 主动销毁单个沙箱。
func (s *Service) DestroySandbox(ctx context.Context, req contracts.SandboxControlRequest) error {
	if err := validateSandboxControlRequest(req); err != nil {
		return err
	}
	tenantID, sandboxID := req.TenantID, req.SandboxID
	var sb Sandbox
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		sb, err = tx.GetSandbox(ctx, tenantID, sandboxID)
		if err != nil {
			return apperr.ErrSandboxNotFound.WithCause(err)
		}
		if sb.SourceRef != strings.TrimSpace(req.SourceRef) {
			return apperr.ErrSandboxOwnershipInvalid
		}
		if err := validateStateTransition(sb.Status, SandboxStatusRecycling); err != nil {
			return err
		}
		queued, err := tx.QueueSandboxRecycle(ctx, tenantID, sandboxID, sb.Status)
		if err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		if !queued {
			return apperr.ErrSandboxStatePersistFailed
		}
		return nil
	}); err != nil {
		return err
	}
	return s.recycleOne(ctx, sb, "manual_destroy")
}

func (s *Service) resumeRetainedSandbox(ctx context.Context, sb Sandbox) error {
	plan, err := s.planForExistingSandbox(ctx, sb)
	if err != nil {
		return err
	}
	if err := validateSingleSandboxResourceLimit(plan, s.cfg); err != nil {
		return err
	}
	var started bool
	if err := s.store.TenantTx(ctx, sb.TenantID, func(ctx context.Context, tx TxStore) error {
		quota, err := tx.GetTenantQuotaForUpdate(ctx, sb.TenantID)
		if err != nil {
			return apperr.ErrSandboxQuotaInvalid.WithCause(err)
		}
		current, err := tx.GetSandboxForUpdate(ctx, sb.TenantID, sb.ID)
		if err != nil {
			return apperr.ErrSandboxNotFound.WithCause(err)
		}
		if current.Status != SandboxStatusPaused && current.Status != SandboxStatusDestroyed {
			return apperr.ErrSandboxStateInvalid
		}
		count, err := tx.CountActiveSandboxes(ctx, sb.TenantID)
		if err != nil {
			return apperr.ErrSandboxQuotaInvalid.WithCause(err)
		}
		if current.Status == SandboxStatusDestroyed {
			count++
		}
		if err := validateTenantSandboxCapacity(quota, count, s.cfg); err != nil {
			return err
		}
		started, err = tx.StartRetainedSandbox(ctx, sb.TenantID, sb.ID)
		if err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return err
	}
	if !started {
		return apperr.ErrSandboxStateInvalid
	}
	if err := s.startSandbox(ctx, sb.TenantID, sb.ID); err != nil {
		return err
	}
	if err := s.store.TenantTx(ctx, sb.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		sb, err = tx.GetSandbox(ctx, sb.TenantID, sb.ID)
		return err
	}); err != nil {
		return apperr.ErrSandboxNotFound.WithCause(err)
	}
	return s.transition(ctx, sb, SandboxStatusRunning, "sandbox.resume")
}

func (s *Service) transition(ctx context.Context, expected Sandbox, status int16, action string) error {
	tenantID, sandboxID := expected.TenantID, expected.ID
	phase := expected.Phase
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		sb, err := tx.GetSandboxForUpdate(ctx, tenantID, sandboxID)
		if err != nil {
			return apperr.ErrSandboxNotFound.WithCause(err)
		}
		if err := validateStateTransition(sb.Status, status); err != nil {
			return err
		}
		if !sandboxExecAllowed(sb) || sb.Phase != expected.Phase || sb.resourceLeaseUntil.After(timex.Now()) {
			return apperr.ErrSandboxStateInvalid
		}
		if _, err := tx.UpdateSandboxPhaseStatus(ctx, tenantID, sandboxID, phase, status); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		detail, err := jsonBytes(map[string]any{"status": status, "phase": phase})
		if err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		if err := tx.CreateSandboxEvent(ctx, s.ids.Generate(), tenantID, sandboxID, EventTypePhaseChange, detail); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		return s.writeSystemAudit(ctx, tenantID, action, "sandbox", sandboxID, nil)
	}); err != nil {
		return err
	}
	s.broadcastProgress(ctx, tenantID, sandboxID, phase, status, logging.TraceFromContext(ctx))
	return nil
}

func asyncPersistenceContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = time.Second
	}
	return context.WithTimeout(context.WithoutCancel(ctx), timeout)
}
