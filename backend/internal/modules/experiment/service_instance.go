// experiment service_instance 文件实现实验实例创建、工作台、状态控制和资源回收。
package experiment

import (
	"context"
	"fmt"
	"sync"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
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
	return instanceDTOFromModel(inst, checkpointDefaults(exp, nil)), nil
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
	return instanceDTOFromModel(inst, checkpointDefaults(exp, checkpoints)), nil
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
				return apperr.ErrExperimentReportNotFound.WithCause(err).WithMessage("请先提交实验报告")
			}
		}
		score, err := tx.SumScores(ctx, id.TenantID, instanceID)
		if err != nil {
			return err
		}
		inst, err = tx.FinishInstance(ctx, id.TenantID, instanceID, score)
		return err
	}); err != nil {
		return InstanceDTO{}, err
	}
	if err := s.publishScored(ctx, inst); err != nil {
		return InstanceDTO{}, err
	}
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
	var mu sync.Mutex
	var wg sync.WaitGroup
	var firstErr error
	sandboxes := []SandboxRef{}
	sims := []SimSessionRef{}
	setErr := func(err error) {
		if err == nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if firstErr == nil {
			firstErr = err
		}
	}
	for idx, env := range exp.Components.Envs {
		componentID := componentID(env.ID, "env", idx)
		wg.Add(1)
		go func(env EnvComponent, componentID string) {
			defer wg.Done()
			if s.sandbox == nil {
				setErr(apperr.ErrExperimentSandboxUnavailable)
				return
			}
			info, err := s.sandbox.CreateSandbox(ctx, contracts.SandboxCreateRequest{TenantID: inst.TenantID, RuntimeCode: env.RuntimeCode, RuntimeImageVersion: env.RuntimeImageVersion, ToolCodes: env.Tools, InitCodeRef: env.InitCodeRef, InitScriptRef: env.InitScriptRef, OwnerAccountID: inst.OwnerAccountID, SourceRef: inst.SourceRef, KeepAlive: env.KeepAlive, SnapshotEnabled: env.SnapshotEnabled, KeepAliveMinutes: env.KeepAliveMinutes, SnapshotRetentionMinutes: env.SnapshotRetentionMinutes})
			if err != nil {
				setErr(apperr.ErrExperimentSandboxUnavailable.WithCause(err))
				return
			}
			mu.Lock()
			sandboxes = append(sandboxes, sandboxRefFromContract(componentID, info))
			mu.Unlock()
		}(env, componentID)
	}
	for idx, sim := range exp.Components.Sims {
		componentID := componentID(sim.ID, "sim", idx)
		wg.Add(1)
		go func(sim SimComponent, componentID string) {
			defer wg.Done()
			if s.sim == nil {
				setErr(apperr.ErrExperimentSimUnavailable)
				return
			}
			info, err := s.sim.CreateSession(ctx, contracts.SimCreateSessionRequest{TenantID: inst.TenantID, PackageCode: sim.PackageCode, Version: sim.Version, Seed: sim.Seed, InitParams: sim.Params, OwnerAccountID: inst.OwnerAccountID, SourceRef: inst.SourceRef})
			if err != nil {
				setErr(apperr.ErrExperimentSimUnavailable.WithCause(err))
				return
			}
			mu.Lock()
			sims = append(sims, simRefFromContract(componentID, info))
			mu.Unlock()
		}(sim, componentID)
	}
	wg.Wait()
	return sandboxes, sims, firstErr
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

// publishScored 发布实验得分事件,供 M6/M11/M9 等上层流程按事件消费。
func (s *Service) publishScored(ctx context.Context, inst ExperimentInstance) error {
	if s.bus == nil {
		return apperr.ErrExperimentEventFailed
	}
	event := contracts.ExperimentScoredEvent{TenantID: inst.TenantID, ExperimentID: inst.ExperimentID, InstanceID: inst.ID, StudentID: inst.OwnerAccountID, Score: inst.Score, ScoredAt: time.Now().UTC()}
	if err := s.bus.Publish(ctx, contracts.SubjectExperimentScored, event); err != nil {
		return apperr.ErrExperimentEventFailed.WithCause(err)
	}
	return nil
}
