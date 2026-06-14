// experiment service_instance 文件实现实验实例创建、工作台、状态控制和资源回收。
package experiment

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
	"chaimir/pkg/response"
)

// CreateInstance 发起实验实例并并发编排 M2 沙箱与 M4 仿真资源。
func (s *Service) CreateInstance(ctx context.Context, experimentID int64, req CreateInstanceRequest) (InstanceDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return InstanceDTO{}, err
	}
	var exp Experiment
	var inst ExperimentInstance
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		exp, err = tx.GetExperiment(ctx, id.TenantID, experimentID)
		if err != nil {
			return err
		}
		if err := validateInstanceStart(exp, req.GroupID); err != nil {
			return err
		}
		if req.GroupID > 0 {
			group, err := tx.GetGroup(ctx, id.TenantID, req.GroupID)
			if err != nil {
				return err
			}
			if group.ExperimentID != experimentID {
				return apperr.ErrExperimentGroupInvalid
			}
			if _, err := tx.GetGroupMember(ctx, id.TenantID, req.GroupID, id.AccountID); err != nil {
				return err
			}
			existing, err := tx.GetActiveGroupInstance(ctx, id.TenantID, experimentID, req.GroupID)
			if err == nil {
				inst = existing
				return nil
			}
			if !isNoRows(err) {
				return err
			}
		}
		instanceID := s.ids.Generate()
		inst, err = tx.CreateInstance(ctx, ExperimentInstance{ID: instanceID, TenantID: id.TenantID, ExperimentID: experimentID, OwnerAccountID: id.AccountID, GroupID: req.GroupID, SourceRef: sourceRefForInstance(instanceID, timex.Now())})
		return err
	}); err != nil {
		return InstanceDTO{}, err
	}
	if inst.Status != InstanceStatusCreating {
		return s.GetInstance(ctx, inst.ID)
	}
	sandboxes, sims, createErr := s.createEngineResources(ctx, exp, inst)
	targetStatus := InstanceStatusRunning
	if createErr != nil {
		targetStatus = InstanceStatusError
		if err := s.compensateRecycle(ctx, inst, "create_failed"); err != nil {
			return InstanceDTO{}, err
		}
	}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		inst, err = tx.UpdateInstanceResources(ctx, id.TenantID, inst.ID, sandboxes, sims, targetStatus)
		return err
	}); err != nil {
		return InstanceDTO{}, err
	}
	if createErr != nil {
		return InstanceDTO{}, createErr
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, audit.ActorRoleStudent, "experiment.instance.create", auditTargetInstance, inst.ID, map[string]any{"experiment_id": experimentID, "source_ref": inst.SourceRef}); err != nil {
		return InstanceDTO{}, err
	}
	return instanceDTOFromModel(inst, checkpointDefaults(exp, nil), stageDTOs(exp, inst, nil)), nil
}

// GetInstance 读取实验工作台,包含引擎入口和检查点状态。
func (s *Service) GetInstance(ctx context.Context, instanceID int64) (InstanceDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return InstanceDTO{}, err
	}
	var inst ExperimentInstance
	var exp Experiment
	var checkpoints []CheckpointResult
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		inst, err = tx.GetInstance(ctx, id.TenantID, instanceID)
		if err != nil {
			return err
		}
		members := []GroupMember{}
		if inst.GroupID > 0 {
			members, err = tx.ListGroupMembers(ctx, id.TenantID, inst.GroupID)
			if err != nil {
				return err
			}
		}
		if !canAccessInstance(id.AccountID, inst, members) {
			return apperr.ErrExperimentInstanceAccessDenied
		}
		exp, err = tx.GetExperiment(ctx, id.TenantID, inst.ExperimentID)
		if err != nil {
			return err
		}
		checkpoints, err = tx.ListCheckpoints(ctx, id.TenantID, inst.ID)
		if err != nil {
			return err
		}
		_, err = tx.TouchInstance(ctx, id.TenantID, inst.ID)
		return err
	}); err != nil {
		return InstanceDTO{}, err
	}
	return instanceDTOFromModel(inst, checkpointDefaults(exp, checkpoints), stageDTOs(exp, inst, checkpoints)), nil
}

// GetProgress 返回统一 M10 进度 topic 元信息。
func (s *Service) GetProgress(ctx context.Context, instanceID int64) (ProgressDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ProgressDTO{}, err
	}
	var inst ExperimentInstance
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		inst, err = tx.GetInstance(ctx, id.TenantID, instanceID)
		if err != nil {
			return err
		}
		return ensureInstanceAccess(ctx, tx, id.AccountID, inst)
	}); err != nil {
		return ProgressDTO{}, err
	}
	return ProgressDTO{Topic: fmt.Sprintf("experiment:%d:%s", inst.ID, progressChannelName), Channel: progressChannelName}, nil
}

// PauseInstance 暂停实验实例并通知 M2 暂停已有沙箱。
func (s *Service) PauseInstance(ctx context.Context, instanceID int64) (InstanceDTO, error) {
	return s.controlInstance(ctx, instanceID, InstanceStatusPaused, func(ctx context.Context, inst ExperimentInstance) error {
		if s.sandbox == nil && len(inst.SandboxRefs) > 0 {
			return apperr.ErrExperimentSandboxUnavailable
		}
		for _, ref := range inst.SandboxRefs {
			if err := s.sandbox.PauseSandbox(ctx, contracts.SandboxControlRequest{TenantID: inst.TenantID, SandboxID: ref.SandboxID, SourceRef: inst.SourceRef}); err != nil {
				return apperr.ErrExperimentSandboxUnavailable.WithCause(err)
			}
		}
		return nil
	})
}

// ResumeInstance 恢复暂停实例;环境已释放时按 source_ref 重建引擎资源。
func (s *Service) ResumeInstance(ctx context.Context, instanceID int64) (InstanceDTO, error) {
	return s.controlInstance(ctx, instanceID, InstanceStatusRunning, func(ctx context.Context, inst ExperimentInstance) error {
		if inst.Status == InstanceStatusReleased {
			exp, err := s.loadExperimentForInstance(ctx, inst)
			if err != nil {
				return err
			}
			sandboxes, sims, err := s.createEngineResources(ctx, exp, inst)
			if err != nil {
				if recycleErr := s.compensateRecycle(ctx, inst, "resume_failed"); recycleErr != nil {
					return recycleErr
				}
				if statusErr := s.store.TenantTx(ctx, inst.TenantID, func(ctx context.Context, tx TxStore) error {
					_, statusErr := tx.SetInstanceStatus(ctx, inst.TenantID, inst.ID, InstanceStatusError)
					return statusErr
				}); statusErr != nil {
					return statusErr
				}
				return err
			}
			return s.store.TenantTx(ctx, inst.TenantID, func(ctx context.Context, tx TxStore) error {
				_, err := tx.UpdateInstanceResources(ctx, inst.TenantID, inst.ID, sandboxes, sims, InstanceStatusRunning)
				return err
			})
		}
		if s.sandbox == nil && len(inst.SandboxRefs) > 0 {
			return apperr.ErrExperimentSandboxUnavailable
		}
		for _, ref := range inst.SandboxRefs {
			if err := s.sandbox.ResumeSandbox(ctx, contracts.SandboxControlRequest{TenantID: inst.TenantID, SandboxID: ref.SandboxID, SourceRef: inst.SourceRef}); err != nil {
				return apperr.ErrExperimentResumeFailed.WithCause(err)
			}
		}
		return nil
	})
}

// FinishInstance 完成实验实例,汇总得分并发布 experiment.scored。
func (s *Service) FinishInstance(ctx context.Context, instanceID int64) (InstanceDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return InstanceDTO{}, err
	}
	var inst ExperimentInstance
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetInstance(ctx, id.TenantID, instanceID)
		if err != nil {
			return err
		}
		if err := ensureInstanceAccess(ctx, tx, id.AccountID, current); err != nil {
			return err
		}
		if err := validateInstanceTransition(current.Status, InstanceStatusFinished); err != nil {
			return err
		}
		exp, err := tx.GetExperiment(ctx, id.TenantID, current.ExperimentID)
		if err != nil {
			return err
		}
		if exp.RequireReport {
			if _, err := tx.GetReportByInstanceStudent(ctx, id.TenantID, instanceID, id.AccountID); err != nil {
				return apperr.ErrExperimentReportRequired.WithCause(err)
			}
		}
		score, err := tx.SumScores(ctx, id.TenantID, instanceID)
		if err != nil {
			return err
		}
		inst, err = tx.FinishInstance(ctx, id.TenantID, instanceID, score)
		if err != nil {
			return err
		}
		return s.enqueueExperimentScoreOutbox(ctx, tx, inst)
	}); err != nil {
		return InstanceDTO{}, err
	}
	s.drainExperimentScoreOutboxBestEffort(ctx)
	if err := s.recycleEngines(ctx, inst, "finished"); err != nil {
		return InstanceDTO{}, err
	}
	return instanceDTOFromModel(inst, nil), s.writeAudit(ctx, id.TenantID, id.AccountID, audit.ActorRoleStudent, "experiment.instance.finish", auditTargetInstance, inst.ID, map[string]any{"score": inst.Score})
}

// RecycleInstance 手动释放实验实例的引擎资源并保留结果。
func (s *Service) RecycleInstance(ctx context.Context, instanceID int64) error {
	id, err := currentIdentity(ctx)
	if err != nil {
		return err
	}
	var inst ExperimentInstance
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		inst, err = tx.GetInstance(ctx, id.TenantID, instanceID)
		if err != nil {
			return err
		}
		if err := ensureInstanceAccess(ctx, tx, id.AccountID, inst); err != nil {
			return err
		}
		if err := validateInstanceTransition(inst.Status, InstanceStatusRecycled); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	if err := s.recycleEngines(ctx, inst, "manual_recycle"); err != nil {
		return err
	}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		_, err := tx.SetInstanceStatus(ctx, id.TenantID, instanceID, InstanceStatusRecycled)
		return err
	}); err != nil {
		return err
	}
	return s.writeAudit(ctx, id.TenantID, id.AccountID, audit.ActorRoleStudent, "experiment.instance.recycle", auditTargetInstance, inst.ID, nil)
}

// RunRecycleOnce 执行一次 M7 后台回收扫描,供统一 background runner 调用。
func (s *Service) RunRecycleOnce(ctx context.Context) error {
	var items []ExperimentInstance
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		items, err = tx.ClaimRecyclableInstances(ctx, s.cfg.PausedTimeoutSeconds, s.cfg.InstanceIdleTimeoutSeconds, int32(s.cfg.RecycleBatchSize))
		return err
	}); err != nil {
		return err
	}
	for _, item := range items {
		if err := s.recycleEngines(ctx, item, "lifecycle_recycle"); err != nil {
			return err
		}
		if err := s.store.TenantTx(ctx, item.TenantID, func(ctx context.Context, tx TxStore) error {
			_, err := tx.SetInstanceStatus(ctx, item.TenantID, item.ID, InstanceStatusRecycled)
			return err
		}); err != nil {
			return err
		}
	}
	return nil
}

// controlInstance 封装暂停/恢复这类状态控制的访问校验和落库。
func (s *Service) controlInstance(ctx context.Context, instanceID int64, next int16, beforeSave func(context.Context, ExperimentInstance) error) (InstanceDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return InstanceDTO{}, err
	}
	var inst ExperimentInstance
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		inst, err = tx.GetInstance(ctx, id.TenantID, instanceID)
		if err != nil {
			return err
		}
		if err := ensureInstanceAccess(ctx, tx, id.AccountID, inst); err != nil {
			return err
		}
		return validateInstanceTransition(inst.Status, next)
	}); err != nil {
		return InstanceDTO{}, err
	}
	if err := beforeSave(ctx, inst); err != nil {
		return InstanceDTO{}, err
	}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		inst, err = tx.SetInstanceStatus(ctx, id.TenantID, instanceID, next)
		return err
	}); err != nil {
		return InstanceDTO{}, err
	}
	return instanceDTOFromModel(inst, nil), nil
}

// createEngineResources 并发创建实验定义中的沙箱和仿真组件。
func (s *Service) createEngineResources(ctx context.Context, exp Experiment, inst ExperimentInstance) ([]SandboxRef, []SimSessionRef, error) {
	return s.createInitialEngineResources(ctx, exp, inst)
}

// recycleEngines 按实例 source_ref 回收 M2/M4 资源,契约缺失时显式失败。
func (s *Service) recycleEngines(ctx context.Context, inst ExperimentInstance, reason string) error {
	if len(inst.SandboxRefs) > 0 {
		if s.sandbox == nil {
			return apperr.ErrExperimentRecycleFailed
		}
		if err := s.sandbox.RecycleBySourceRef(ctx, contracts.SandboxRecycleRequest{TenantID: inst.TenantID, SourceRef: inst.SourceRef, Reason: reason}); err != nil {
			return apperr.ErrExperimentRecycleFailed.WithCause(err)
		}
	}
	if len(inst.SimSessionRefs) > 0 {
		if s.sim == nil {
			return apperr.ErrExperimentRecycleFailed
		}
		if err := s.sim.RecycleBySourceRef(ctx, contracts.SimRecycleRequest{TenantID: inst.TenantID, SourceRef: inst.SourceRef, Reason: reason}); err != nil {
			return apperr.ErrExperimentRecycleFailed.WithCause(err)
		}
	}
	return nil
}

// compensateRecycle 在引擎部分创建失败后释放已成功创建的组件。
func (s *Service) compensateRecycle(ctx context.Context, inst ExperimentInstance, reason string) error {
	if !validExperimentSourceRef(inst.SourceRef) {
		return apperr.ErrExperimentSourceRefInvalid
	}
	if s.sandbox != nil {
		if err := s.sandbox.RecycleBySourceRef(ctx, contracts.SandboxRecycleRequest{TenantID: inst.TenantID, SourceRef: inst.SourceRef, Reason: reason}); err != nil {
			return apperr.ErrExperimentRecycleFailed.WithCause(err)
		}
	}
	if s.sim != nil {
		if err := s.sim.RecycleBySourceRef(ctx, contracts.SimRecycleRequest{TenantID: inst.TenantID, SourceRef: inst.SourceRef, Reason: reason}); err != nil {
			return apperr.ErrExperimentRecycleFailed.WithCause(err)
		}
	}
	return nil
}

// ensureInstanceAccess 校验当前账号对实例拥有者或小组成员身份。
func ensureInstanceAccess(ctx context.Context, tx TxStore, accountID int64, inst ExperimentInstance) error {
	members := []GroupMember{}
	var err error
	if inst.GroupID > 0 {
		members, err = tx.ListGroupMembers(ctx, inst.TenantID, inst.GroupID)
		if err != nil {
			return err
		}
	}
	if !canAccessInstance(accountID, inst, members) {
		return apperr.ErrExperimentInstanceAccessDenied
	}
	return nil
}

// loadExperimentForInstance 读取实例绑定的实验定义。
func (s *Service) loadExperimentForInstance(ctx context.Context, inst ExperimentInstance) (Experiment, error) {
	var exp Experiment
	if err := s.store.TenantTx(ctx, inst.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		exp, err = tx.GetExperiment(ctx, inst.TenantID, inst.ExperimentID)
		return err
	}); err != nil {
		return Experiment{}, err
	}
	return exp, nil
}

// checkpointDefaults 合并组件定义和已有判分结果,保证工作台总能看到全部检查点。
func checkpointDefaults(exp Experiment, existing []CheckpointResult) []CheckpointResult {
	byID := map[string]CheckpointResult{}
	for _, item := range existing {
		byID[item.CheckpointID] = item
	}
	out := make([]CheckpointResult, 0, len(exp.Components.Checkpoints))
	for _, cp := range exp.Components.Checkpoints {
		if item, ok := byID[cp.ID]; ok {
			out = append(out, item)
			continue
		}
		out = append(out, CheckpointResult{CheckpointID: cp.ID})
	}
	return out
}

// enqueueExperimentScoreOutbox 在实例得分写入同一事务内保存实验得分事件。
func (s *Service) enqueueExperimentScoreOutbox(ctx context.Context, tx TxStore, inst ExperimentInstance) error {
	traceID := strings.TrimSpace(response.TraceFromContext(ctx))
	if inst.TenantID <= 0 || inst.ExperimentID <= 0 || inst.ID <= 0 || inst.OwnerAccountID <= 0 || traceID == "" {
		return apperr.ErrExperimentEventFailed
	}
	if _, err := tx.CreateExperimentScoreOutbox(ctx, s.ids.Generate(), inst, traceID, timex.Now()); err != nil {
		return apperr.ErrExperimentEventFailed.WithCause(err)
	}
	return nil
}

// RunExperimentScoreOutboxOnce 领取并发布 M7 实验得分事件。
func (s *Service) RunExperimentScoreOutboxOnce(ctx context.Context) error {
	limit := int32(s.cfg.ScoreOutboxBatchSize)
	if limit <= 0 {
		return apperr.ErrExperimentEventFailed
	}
	staleBefore := timex.Now().Add(-time.Duration(s.cfg.ScoreOutboxStaleMs) * time.Millisecond)
	var items []ExperimentScoreOutbox
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		items, err = tx.ClaimPendingExperimentScoreOutbox(ctx, limit, staleBefore)
		if err != nil {
			return apperr.ErrExperimentEventFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return err
	}
	for _, item := range items {
		if err := s.publishScoreOutboxItem(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// publishScoreOutboxItem 发布单条得分事件并按结果回写 outbox 状态。
func (s *Service) publishScoreOutboxItem(ctx context.Context, item ExperimentScoreOutbox) error {
	eventCtx := response.WithTrace(ctx, item.TraceID)
	payload := contracts.ExperimentScoredEvent{TenantID: item.TenantID, TraceID: item.TraceID, ExperimentID: item.ExperimentID, InstanceID: item.InstanceID, StudentID: item.StudentID, Score: item.Score, ScoredAt: item.ScoredAt}
	if err := s.bus.Publish(eventCtx, contracts.SubjectExperimentScored, payload); err != nil {
		s.recordExperimentScoreOutboxFailure(eventCtx, item, err)
		return apperr.ErrExperimentEventFailed.WithCause(err)
	}
	return s.markExperimentScoreOutboxPublished(eventCtx, item)
}

// markExperimentScoreOutboxPublished 标记实验得分事件发布成功。
func (s *Service) markExperimentScoreOutboxPublished(ctx context.Context, item ExperimentScoreOutbox) error {
	return s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		_, err := tx.MarkExperimentScoreOutboxPublished(ctx, item.TenantID, item.ID)
		if err != nil {
			return apperr.ErrExperimentEventFailed.WithCause(err)
		}
		return nil
	})
}

// recordExperimentScoreOutboxFailure 记录得分事件发布失败并等待后台重试。
func (s *Service) recordExperimentScoreOutboxFailure(ctx context.Context, item ExperimentScoreOutbox, cause error) {
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		_, err := tx.MarkExperimentScoreOutboxFailed(ctx, item.TenantID, item.ID, logging.SanitizeError(cause.Error()))
		return err
	}); err != nil {
		logging.ErrorContext(ctx, "experiment score outbox failure mark failed", err.Error(), slog.Int64("tenant_id", item.TenantID), slog.Int64("instance_id", item.InstanceID), slog.Int64("outbox_id", item.ID))
	}
}

// drainExperimentScoreOutboxBestEffort 在请求提交后尽快投递,失败交给后台任务补偿。
func (s *Service) drainExperimentScoreOutboxBestEffort(ctx context.Context) {
	if err := s.RunExperimentScoreOutboxOnce(ctx); err != nil {
		logging.ErrorContext(ctx, "experiment score outbox drain failed", err.Error())
	}
}
