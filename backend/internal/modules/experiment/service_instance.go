// experiment service_instance 文件实现实验实例创建、工作台、状态控制和资源回收。
package experiment

import (
	"context"
	"encoding/binary"
	"hash/fnv"
	"log/slog"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/intx"
	"chaimir/internal/platform/response"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
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
		if err := validateInstanceStart(exp, req.GroupID.Int64()); err != nil {
			return err
		}
		if err := tx.LockInstanceCreation(ctx, instanceCreationLockKey(id.TenantID, experimentID, id.AccountID, req.GroupID.Int64())); err != nil {
			return err
		}
		if req.GroupID > 0 {
			group, err := tx.GetGroup(ctx, id.TenantID, req.GroupID.Int64())
			if err != nil {
				return err
			}
			if group.ExperimentID != experimentID {
				return apperr.ErrExperimentGroupInvalid
			}
			if _, err := tx.GetGroupMember(ctx, id.TenantID, req.GroupID.Int64(), id.AccountID); err != nil {
				return err
			}
			existing, err := tx.GetActiveGroupInstance(ctx, id.TenantID, experimentID, req.GroupID.Int64())
			if err == nil {
				inst = existing
				return nil
			}
			if !isNoRows(err) {
				return err
			}
		} else {
			existing, err := tx.GetActiveOwnerInstance(ctx, id.TenantID, experimentID, id.AccountID)
			if err == nil {
				inst = existing
				return nil
			}
			if !isNoRows(err) {
				return err
			}
		}
		instanceID := s.ids.Generate()
		inst, err = tx.CreateInstance(ctx, ExperimentInstance{ID: instanceID, TenantID: id.TenantID, ExperimentID: experimentID, OwnerAccountID: id.AccountID, GroupID: req.GroupID.Int64(), SourceRef: sourceRefForInstance(instanceID, timex.Now())})
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
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumStudent, "experiment.instance.create", auditTargetInstance, inst.ID, map[string]any{"experiment_id": experimentID, "source_ref": inst.SourceRef}); err != nil {
		return InstanceDTO{}, err
	}
	return instanceDTOFromModel(inst, checkpointDefaults(exp, nil), stageDTOs(exp, inst, nil)), nil
}

// instanceCreationLockKey 为同一租户、实验和协作主体生成事务级互斥键,避免并发请求同时看不到实例而重复创建资源。
func instanceCreationLockKey(tenantID, experimentID, ownerID, groupID int64) int64 {
	h := fnv.New64a()
	var data [32]byte
	for i, value := range [...]int64{tenantID, experimentID, ownerID, groupID} {
		if value < 0 {
			return 0
		}
		binary.BigEndian.PutUint64(data[i*8:], uint64(value))
	}
	if _, err := h.Write(data[:]); err != nil {
		return 0
	}
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], h.Sum64())
	high := binary.BigEndian.Uint32(encoded[:4])
	low := binary.BigEndian.Uint32(encoded[4:])
	return int64(high)<<32 | int64(low)
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
	return ProgressDTO{Topic: "tenant:" + ids.Format(inst.TenantID) + ":experiment:" + ids.Format(inst.ID) + ":" + progressChannelName, Channel: progressChannelName}, nil
}

// PauseInstance 暂停实验实例并通知 M2 暂停已有沙箱。
func (s *Service) PauseInstance(ctx context.Context, instanceID int64) (InstanceDTO, error) {
	return s.controlInstance(ctx, instanceID, InstanceStatusPaused, func(ctx context.Context, inst ExperimentInstance) error {
		if s.sandbox == nil && len(inst.SandboxRefs) > 0 {
			return apperr.ErrExperimentSandboxUnavailable
		}
		for _, ref := range inst.SandboxRefs {
			if err := s.sandbox.PauseSandbox(ctx, contracts.SandboxControlRequest{TenantID: inst.TenantID, SandboxID: ref.SandboxID.Int64(), SourceRef: inst.SourceRef}); err != nil {
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
			if err := s.sandbox.ResumeSandbox(ctx, contracts.SandboxControlRequest{TenantID: inst.TenantID, SandboxID: ref.SandboxID.Int64(), SourceRef: inst.SourceRef}); err != nil {
				return apperr.ErrExperimentResumeFailed.WithCause(err)
			}
		}
		return nil
	})
}

// FinishInstance 完成实验实例,首次固化得分并在后续重试中幂等续跑资源回收。
func (s *Service) FinishInstance(ctx context.Context, instanceID int64) (InstanceDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return InstanceDTO{}, err
	}
	var inst ExperimentInstance
	action := instanceFinishPrepare
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetInstanceForUpdate(ctx, id.TenantID, instanceID)
		if err != nil {
			return err
		}
		if err := ensureInstanceAccess(ctx, tx, id.AccountID, current); err != nil {
			return err
		}
		action, err = resolveInstanceFinishAction(current.Status)
		if err != nil {
			return err
		}
		if action != instanceFinishPrepare {
			inst = current
			return nil
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
		if err := s.ensureTriggeredCheckpointsTerminal(ctx, tx, current); err != nil {
			return err
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
	if action == instanceFinishCompleted {
		return instanceDTOFromModel(inst, nil), nil
	}
	if action == instanceFinishPrepare {
		s.drainExperimentScoreOutboxBestEffort(ctx)
	}
	if err := s.recycleEngines(ctx, inst, "finished"); err != nil {
		return InstanceDTO{}, err
	}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		inst, err = tx.SetInstanceStatus(ctx, id.TenantID, instanceID, InstanceStatusRecycled)
		return err
	}); err != nil {
		return InstanceDTO{}, err
	}
	return instanceDTOFromModel(inst, nil), s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumStudent, "experiment.instance.finish", auditTargetInstance, inst.ID, map[string]any{"score": inst.Score})
}

// ensureTriggeredCheckpointsTerminal 防止学生在 M3 仍执行检查点时固化不完整总分。
func (s *Service) ensureTriggeredCheckpointsTerminal(ctx context.Context, tx TxStore, inst ExperimentInstance) error {
	checkpoints, err := tx.ListCheckpoints(ctx, inst.TenantID, inst.ID)
	if err != nil {
		return err
	}
	for _, checkpoint := range checkpoints {
		if checkpoint.JudgeTaskRef == "" {
			continue
		}
		if s.judge == nil {
			return apperr.ErrExperimentJudgeUnavailable
		}
		taskID, ok := ids.Parse(checkpoint.JudgeTaskRef)
		if !ok {
			return apperr.ErrExperimentCheckpointInvalid
		}
		info, err := s.judge.GetJudgeTask(ctx, inst.TenantID, taskID)
		if err != nil {
			return apperr.ErrExperimentJudgeUnavailable.WithCause(err)
		}
		if info.Status == contracts.JudgeTaskStatusQueued || info.Status == contracts.JudgeTaskStatusRunning {
			return apperr.ErrExperimentJudgePending
		}
	}
	return nil
}

// RunRecycleOnce 执行一次 M7 后台回收扫描,供统一 background runner 调用。
func (s *Service) RunRecycleOnce(ctx context.Context) error {
	var items []ExperimentInstance
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		limit, ok := intx.Int32(s.cfg.RecycleBatchSize)
		if !ok || limit <= 0 {
			return apperr.ErrExperimentInstanceInvalid
		}
		items, err = tx.ClaimRecyclableInstances(ctx, s.cfg.PausedTimeoutSeconds, s.cfg.InstanceIdleTimeoutSeconds, limit)
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
	limit, ok := intx.Int32(s.cfg.ScoreOutboxBatchSize)
	if !ok || limit <= 0 {
		return apperr.ErrExperimentEventFailed
	}
	now := timex.Now()
	staleBefore := now
	leaseUntil := now.Add(time.Duration(s.cfg.ScoreOutboxStaleMs) * time.Millisecond)
	var items []ExperimentScoreOutbox
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		maxAttempts, ok := intx.Int32(s.cfg.ScoreOutboxMaxAttempts)
		if !ok || maxAttempts <= 0 {
			return apperr.ErrExperimentEventFailed
		}
		items, err = tx.ClaimPendingExperimentScoreOutbox(ctx, limit, maxAttempts, staleBefore, leaseUntil)
		if err != nil {
			return apperr.ErrExperimentEventFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return err
	}
	for _, item := range items {
		if err := s.publishScoreOutboxItem(ctx, item); err != nil {
			logging.ErrorContext(ctx, "experiment score outbox publish failed", err.Error(), slog.Int64("tenant_id", item.TenantID), slog.Int64("instance_id", item.InstanceID), slog.Int64("outbox_id", item.ID))
		}
	}
	return nil
}

// publishScoreOutboxItem 发布单条得分事件并按结果回写 outbox 状态。
func (s *Service) publishScoreOutboxItem(ctx context.Context, item ExperimentScoreOutbox) error {
	eventCtx := response.WithTrace(ctx, item.TraceID)
	experimentName, err := s.experimentNameForScoreEvent(eventCtx, item)
	if err != nil {
		s.recordExperimentScoreOutboxFailure(eventCtx, item, err)
		return apperr.ErrExperimentEventFailed.WithCause(err)
	}
	payload := contracts.ExperimentScoredEvent{EventID: ids.Format(item.ID), TenantID: item.TenantID, TraceID: item.TraceID, ExperimentID: item.ExperimentID, ExperimentName: experimentName, InstanceID: item.InstanceID, StudentID: item.StudentID, Score: item.Score, ScoredAt: item.ScoredAt}
	if err := s.bus.Publish(eventCtx, contracts.SubjectExperimentScored, payload); err != nil {
		s.recordExperimentScoreOutboxFailure(eventCtx, item, err)
		return apperr.ErrExperimentEventFailed.WithCause(err)
	}
	return s.markExperimentScoreOutboxPublished(eventCtx, item)
}

// experimentNameForScoreEvent 读取通知展示所需的最小实验名称,不把 M7 内部组件配置带入事件。
func (s *Service) experimentNameForScoreEvent(ctx context.Context, item ExperimentScoreOutbox) (string, error) {
	var exp Experiment
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		exp, err = tx.GetExperiment(ctx, item.TenantID, item.ExperimentID)
		return err
	}); err != nil {
		return "", err
	}
	return exp.Name, nil
}

// markExperimentScoreOutboxPublished 标记实验得分事件发布成功。
func (s *Service) markExperimentScoreOutboxPublished(ctx context.Context, item ExperimentScoreOutbox) error {
	return s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		_, err := tx.MarkExperimentScoreOutboxPublished(ctx, item.TenantID, item.ID, item.LeaseToken)
		if err != nil {
			return apperr.ErrExperimentEventFailed.WithCause(err)
		}
		return nil
	})
}

// recordExperimentScoreOutboxFailure 记录得分事件发布失败并等待后台重试。
func (s *Service) recordExperimentScoreOutboxFailure(ctx context.Context, item ExperimentScoreOutbox, cause error) {
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		_, err := tx.MarkExperimentScoreOutboxFailed(ctx, item.TenantID, item.ID, logging.SanitizeError(cause.Error()), item.LeaseToken)
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

// HandleSandboxRecycled 消费 M2 回收事件,同步进行中实例或续跑已完成实例的整体回收。
func (s *Service) HandleSandboxRecycled(ctx context.Context, event contracts.SandboxRecycledEvent) error {
	if event.TenantID <= 0 || !validExperimentSourceRef(event.SourceRef) {
		return apperr.ErrExperimentSourceRefInvalid
	}
	var inst ExperimentInstance
	if err := s.store.TenantTx(ctx, event.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		inst, err = tx.GetInstanceBySourceRef(ctx, event.TenantID, event.SourceRef)
		if err != nil {
			return err
		}
		if inst.Status == InstanceStatusRunning || inst.Status == InstanceStatusPaused || inst.Status == InstanceStatusCreating {
			_, err = tx.SetInstanceStatus(ctx, event.TenantID, inst.ID, InstanceStatusReleased)
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	if inst.Status != InstanceStatusFinished {
		return nil
	}
	allDestroyed, err := s.allInstanceSandboxesDestroyed(ctx, inst)
	if err != nil {
		return err
	}
	if !allDestroyed {
		return nil
	}
	if err := s.recycleEngines(ctx, inst, "finished_recovery"); err != nil {
		return err
	}
	return s.store.TenantTx(ctx, event.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetInstanceForUpdate(ctx, event.TenantID, inst.ID)
		if err != nil {
			return err
		}
		if current.Status != InstanceStatusFinished {
			return nil
		}
		_, err = tx.SetInstanceStatus(ctx, event.TenantID, inst.ID, InstanceStatusRecycled)
		return err
	})
}

// allInstanceSandboxesDestroyed 确认实例引用的全部 M2 沙箱都已销毁,避免单个组件事件提前结束整实例。
func (s *Service) allInstanceSandboxesDestroyed(ctx context.Context, inst ExperimentInstance) (bool, error) {
	if len(inst.SandboxRefs) == 0 {
		return true, nil
	}
	if s.sandbox == nil {
		return false, apperr.ErrExperimentRecycleFailed
	}
	for _, ref := range inst.SandboxRefs {
		info, err := s.sandbox.GetSandbox(ctx, inst.TenantID, ref.SandboxID.Int64())
		if err != nil {
			return false, apperr.ErrExperimentRecycleFailed.WithCause(err)
		}
		if info.Status != contracts.SandboxStatusDestroyed {
			return false, nil
		}
	}
	return true, nil
}

// HandleCourseEnded 课程结束或归档后级联回收课内仍占用引擎资源的实验实例(M7 需求 D3)。
//
// 逐个实例回收而不是批量:回收要按 source_ref 通知 M2/M4,任一失败都必须显式返回让事件重投,
// 而已回收成功的实例已落 recycled 态,重投时不会被再次取出(查询只看仍占资源的四态),天然幂等。
func (s *Service) HandleCourseEnded(ctx context.Context, event contracts.TeachingCourseEndedEvent) error {
	if event.TenantID <= 0 || event.CourseID <= 0 {
		return apperr.ErrExperimentInstanceInvalid
	}
	var items []ExperimentInstance
	if err := s.store.TenantTx(ctx, event.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		items, err = tx.ListLiveInstancesByCourse(ctx, event.TenantID, event.CourseID)
		return err
	}); err != nil {
		return err
	}
	for _, item := range items {
		if err := s.recycleEngines(ctx, item, "course_ended"); err != nil {
			return err
		}
		if err := s.store.TenantTx(ctx, event.TenantID, func(ctx context.Context, tx TxStore) error {
			_, err := tx.SetInstanceStatus(ctx, event.TenantID, item.ID, InstanceStatusRecycled)
			return err
		}); err != nil {
			return err
		}
	}
	return nil
}
