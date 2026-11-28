package sandbox

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/response"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
)

func (s *Service) scheduleSandboxStart(ctx context.Context, tenantID, sandboxID int64) {
	s.asyncMu.Lock()
	defer s.asyncMu.Unlock()
	if s.closing {
		return
	}
	s.asyncTasks.Add(1)
	traceCtx := context.WithoutCancel(ctx)
	go func() {
		var err error
		defer func() { s.finishAsync(err) }()
		if err = s.startSandbox(traceCtx, tenantID, sandboxID); err != nil {
			logging.ErrorContext(traceCtx, "sandbox startup failed", apperr.AsAppError(err).LogString(), slog.Int64("tenant_id", tenantID), slog.Int64("sandbox_id", sandboxID))
		}
	}()
}

func (s *Service) startSandbox(ctx context.Context, tenantID, sandboxID int64) (failure error) {
	if s.resourceLeaseDuration() <= 0 {
		return apperr.ErrSandboxRecycleConfigInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, timeDurationSeconds(s.cfg.ReadyTimeoutSeconds))
	defer cancel()
	var current Sandbox
	var acquired bool
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		current, acquired, err = tx.ClaimSandboxResources(ctx, tenantID, sandboxID, SandboxStatusCreating, s.resourceLeaseDuration())
		return err
	}); err != nil {
		return apperr.ErrSandboxStatePersistFailed.WithCause(err)
	}
	if !acquired {
		// 重复后台任务只退出；同步恢复入口另外核对当前可执行状态。
		return nil
	}
	plan := CreateSandboxPlan{Sandbox: current}
	stage := "start"
	defer func() {
		if failure != nil {
			failure = errors.Join(failure, s.markStartupFailed(ctx, plan.Sandbox, stage, failure))
		}
	}()
	prepared, err := s.planForExistingSandbox(ctx, current)
	if err != nil {
		return err
	}
	plan = prepared
	needsInitialization := sandboxNeedsInitialization(plan)
	var archives []RuntimeInitArchive
	if needsInitialization {
		var err error
		archives, err = s.prepareRuntimeInitArchives(ctx, plan.Sandbox, plan.Runtimes)
		if err != nil {
			return err
		}
	}
	bootstrapPlan := plan
	bootstrapPlan.DeferTools = needsInitialization
	if err := s.orchestrator.CreateSandboxResources(ctx, bootstrapPlan, archives); err != nil {
		return apperr.ErrSandboxCreateFailed.WithCause(err)
	}
	if err := s.cleanWorkspaceBeforeAccess(ctx, plan.Sandbox, workspaceRuntimeForPlan(plan)); err != nil {
		return err
	}
	bootstrapPhase := SandboxPhaseReady
	if needsInitialization {
		bootstrapPhase = SandboxPhaseInitializing
	} else if current.Phase == SandboxPhaseFullyReady {
		bootstrapPhase = SandboxPhaseFullyReady
	}
	advanced, err := s.advanceStartupState(ctx, plan.Sandbox, bootstrapPhase, SandboxStatusReady, map[string]any{"phase": bootstrapPhase, "status": SandboxStatusReady}, SandboxStatusCreating)
	if err != nil {
		return err
	}
	if !advanced {
		return apperr.ErrSandboxStateInvalid
	}
	plan.Sandbox.Phase, plan.Sandbox.Status = bootstrapPhase, SandboxStatusReady
	if needsInitialization {
		s.broadcastProgress(ctx, current.TenantID, current.ID, bootstrapPhase, SandboxStatusReady, response.TraceFromContext(ctx))
		stage = "init"
		advanced, err = s.advanceStartupState(ctx, plan.Sandbox, SandboxPhaseInitializing, SandboxStatusRunning, map[string]any{"phase": SandboxPhaseInitializing}, SandboxStatusReady)
		if err != nil {
			return err
		}
		if !advanced {
			return apperr.ErrSandboxStateInvalid
		}
		plan.Sandbox.Status = SandboxStatusRunning
		s.broadcastProgress(ctx, current.TenantID, current.ID, SandboxPhaseInitializing, SandboxStatusRunning, response.TraceFromContext(ctx))
		if strings.TrimSpace(current.InitCodeRef) != "" {
			if err := s.restoreArchiveToWorkspace(ctx, plan.Sandbox, workspaceRuntimeForPlan(plan), current.InitCodeRef, "", "."); err != nil {
				return err
			}
		}
		if strings.TrimSpace(current.InitScriptRef) != "" {
			if err := s.runInitScriptIfNeeded(ctx, plan.Sandbox, workspaceRuntimeForPlan(plan), current.InitScriptRef); err != nil {
				return err
			}
		}
		bootstrapPlan.Sandbox = plan.Sandbox
		if err := s.initializeChains(ctx, bootstrapPlan); err != nil {
			return err
		}
		if err := s.orchestrator.CreateSandboxResources(ctx, plan, nil); err != nil {
			return apperr.ErrSandboxCreateFailed.WithCause(err)
		}
		advanced, err = s.advanceStartupState(ctx, plan.Sandbox, SandboxPhaseFullyReady, SandboxStatusRunning, map[string]any{"phase": SandboxPhaseFullyReady}, SandboxStatusRunning)
		if err != nil {
			return err
		}
		if !advanced {
			return apperr.ErrSandboxStateInvalid
		}
		plan.Sandbox.Phase = SandboxPhaseFullyReady
	}
	persistCtx, cancelPersist := asyncPersistenceContext(ctx, timeDurationSeconds(s.cfg.ReadyTimeoutSeconds))
	defer cancelPersist()
	if err := s.store.TenantTx(persistCtx, current.TenantID, func(ctx context.Context, tx TxStore) error {
		released, err := tx.ReleaseSandboxResourceLease(ctx, current.TenantID, current.ID, current.resourceLeaseToken)
		if err != nil {
			return err
		}
		if !released {
			return errSandboxResourceLeaseLost
		}
		return nil
	}); err != nil {
		return apperr.ErrSandboxStatePersistFailed.WithCause(err)
	}
	s.broadcastProgress(persistCtx, current.TenantID, current.ID, plan.Sandbox.Phase, plan.Sandbox.Status, response.TraceFromContext(ctx))
	return nil
}

func (s *Service) advanceStartupState(ctx context.Context, expected Sandbox, phase, status int16, detail map[string]any, expectedStatuses ...int16) (bool, error) {
	advanced := false
	tenantID, sandboxID := expected.TenantID, expected.ID
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		sb, err := tx.GetSandboxForUpdate(ctx, tenantID, sandboxID)
		if err != nil {
			return apperr.ErrSandboxNotFound.WithCause(err)
		}
		if startupStateOwnedByRecycle(sb.Status) || sb.resourceLeaseToken != expected.resourceLeaseToken || !sb.resourceLeaseUntil.After(timex.Now()) {
			return nil
		}
		if !slices.Contains(expectedStatuses, sb.Status) {
			return nil
		}
		if err := validateStateTransition(sb.Status, status); err != nil {
			return err
		}
		if _, err := tx.UpdateSandboxPhaseStatus(ctx, tenantID, sandboxID, phase, status); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		detail["status"] = status
		rawDetail, err := jsonBytes(detail)
		if err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		if err := tx.CreateSandboxEvent(ctx, s.ids.Generate(), tenantID, sandboxID, EventTypePhaseChange, rawDetail); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		advanced = true
		return nil
	}); err != nil {
		return false, err
	}
	return advanced, nil
}

func startupStateOwnedByRecycle(status int16) bool {
	return status == SandboxStatusRecycling || status == SandboxStatusDestroyed
}

func sandboxNeedsInitialization(plan CreateSandboxPlan) bool {
	if plan.Sandbox.Phase != SandboxPhaseAllocating || strings.TrimSpace(plan.Sandbox.SnapshotRef) != "" {
		return false
	}
	for _, runtime := range plan.Runtimes {
		if len(runtime.Runtime.AdapterSpec.InitAssets) > 0 {
			return true
		}
	}
	return len(plan.InitChainSteps) > 0 || strings.TrimSpace(plan.Sandbox.InitCodeRef) != "" || strings.TrimSpace(plan.Sandbox.InitScriptRef) != ""
}

func (s *Service) markStartupFailed(ctx context.Context, sb Sandbox, stage string, cause error, expectedStatuses ...int16) error {
	persistCtx, cancel := asyncPersistenceContext(ctx, timeDurationSeconds(s.cfg.ReadyTimeoutSeconds))
	defer cancel()
	shouldBroadcast := false
	broadcastPhase := sb.Phase
	if err := s.store.TenantTx(persistCtx, sb.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetSandboxForUpdate(ctx, sb.TenantID, sb.ID)
		if err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		if startupStateOwnedByRecycle(current.Status) || current.Status == SandboxStatusFailed {
			return nil
		}
		if len(expectedStatuses) == 0 && (sb.resourceLeaseToken == "" || current.resourceLeaseToken != sb.resourceLeaseToken || !current.resourceLeaseUntil.After(timex.Now()) || current.Phase != sb.Phase || current.Status != sb.Status) {
			return nil
		}
		// 后台扫描使用完整状态版本，避免将随后完成的同一生命周期误标为失败。
		if len(expectedStatuses) > 0 && (!slices.Contains(expectedStatuses, current.Status) || current.Phase != sb.Phase || !current.UpdatedAt.Equal(sb.UpdatedAt) || current.resourceLeaseUntil.After(timex.Now())) {
			return nil
		}
		if _, err := tx.ReleaseSandboxResourceLease(ctx, sb.TenantID, sb.ID, current.resourceLeaseToken); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		if _, err := tx.UpdateSandboxPhaseStatus(ctx, sb.TenantID, sb.ID, current.Phase, SandboxStatusFailed); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		broadcastPhase = current.Phase
		detail, err := jsonBytes(map[string]any{"status": SandboxStatusFailed, "stage": stage, "error": logging.SanitizeError(cause.Error())})
		if err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		if err := tx.CreateSandboxEvent(ctx, s.ids.Generate(), sb.TenantID, sb.ID, EventTypeError, detail); err != nil {
			return apperr.ErrSandboxStatePersistFailed.WithCause(err)
		}
		shouldBroadcast = true
		return nil
	}); err != nil {
		logging.ErrorContext(persistCtx, "sandbox startup failure mark failed", apperr.AsAppError(errors.Join(cause, err)).LogString(), slog.Int64("tenant_id", sb.TenantID), slog.Int64("sandbox_id", sb.ID), slog.String("stage", stage))
		return err
	}
	if !shouldBroadcast {
		return nil
	}
	s.broadcastProgress(persistCtx, sb.TenantID, sb.ID, broadcastPhase, SandboxStatusFailed, response.TraceFromContext(ctx))
	err := s.bus.Publish(persistCtx, contracts.SubjectSandboxFailed, contracts.SandboxFailedEvent{
		TenantID:  sb.TenantID,
		TraceID:   response.TraceFromContext(ctx),
		SandboxID: sb.ID,
		SourceRef: sb.SourceRef,
		ScopeRef:  sb.ScopeRef,
		FailedAt:  timex.Now(),
	})
	if err != nil {
		logging.ErrorContext(persistCtx, "sandbox failure event publish failed", apperr.AsAppError(err).LogString(), slog.Int64("tenant_id", sb.TenantID), slog.Int64("sandbox_id", sb.ID), slog.String("source_ref", sb.SourceRef), slog.String("scope_ref", sb.ScopeRef))
	}
	logging.ErrorContext(persistCtx, "sandbox startup failed", apperr.AsAppError(cause).LogString(), slog.Int64("tenant_id", sb.TenantID), slog.Int64("sandbox_id", sb.ID), slog.String("stage", stage))
	return err
}
