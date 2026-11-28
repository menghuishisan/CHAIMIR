// experiment service_instance 文件实现实验实例创建、工作台、状态控制和资源回收。
package experiment

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"slices"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/intx"
	"chaimir/internal/platform/pagex"
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
	var created bool
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
			group, err := tx.GetGroupForUpdate(ctx, id.TenantID, req.GroupID.Int64())
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
		sourceRef := sourceRefForInstance(instanceID, timex.Now())
		inst, err = tx.CreateInstance(ctx, ExperimentInstance{ID: instanceID, TenantID: id.TenantID, ExperimentID: experimentID, OwnerAccountID: id.AccountID, GroupID: req.GroupID.Int64(), SourceRef: sourceRef, ScopeRef: scopeRefForInstanceGeneration(sourceRef, instanceID)})
		if err != nil {
			return err
		}
		created = true
		return s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumStudent, "experiment.instance.create", auditTargetInstance, inst.ID, map[string]any{"experiment_id": experimentID, "source_ref": inst.SourceRef})
	}); err != nil {
		return InstanceDTO{}, err
	}
	// 只有本次新建实例的请求负责资源编排;已有 creating 也必须复用,避免重复创建引擎资源。
	if !created {
		if shouldReconcileReleasedInstance(inst.Status) {
			released, err := s.reconcileReleasedInstance(ctx, inst)
			if err != nil {
				return InstanceDTO{}, err
			}
			inst = released
		}
		return s.GetInstance(ctx, inst.ID)
	}
	sandboxes, sims, createErr := s.createInitialEngineResources(ctx, exp, inst)
	targetStatus := InstanceStatusRunning
	if createErr != nil {
		targetStatus = InstanceStatusError
		cleanupInst := inst
		cleanupInst.SandboxRefs = sandboxes
		cleanupInst.SimSessionRefs = sims
		if cleanupErr := s.recycleEngines(ctx, cleanupInst, "create_failed"); cleanupErr != nil {
			createErr = errors.Join(createErr, cleanupErr)
		} else {
			sandboxes, sims = nil, nil
		}
	}
	var bound ExperimentInstance
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetInstanceForUpdate(ctx, id.TenantID, inst.ID)
		if err != nil {
			return err
		}
		if current.ScopeRef != inst.ScopeRef || current.Status != InstanceStatusCreating {
			return apperr.ErrExperimentInstanceStateInvalid
		}
		if err := s.reconcileCreatedResourceAuthorizations(ctx, tx, inst, sandboxes, sims); err != nil {
			return err
		}
		bound, err = tx.UpdateInstanceResources(ctx, id.TenantID, inst.ID, inst.ScopeRef, sandboxes, sims, targetStatus)
		return err
	}); err != nil {
		if createErr == nil {
			if cleanupErr := s.destroyCreatedStageResources(ctx, inst, sandboxes, sims); cleanupErr != nil {
				return InstanceDTO{}, apperr.ErrExperimentRecycleFailed.WithCause(errors.Join(err, cleanupErr))
			}
		}
		return InstanceDTO{}, errors.Join(createErr, err)
	}
	inst = bound
	s.publishInstanceChanged(ctx, inst.TenantID, inst.ID)
	if createErr != nil {
		return InstanceDTO{}, createErr
	}
	inst, err = s.reconcileSandboxFailure(ctx, inst)
	if err != nil {
		return InstanceDTO{}, err
	}
	return instanceDTOFromModel(inst, nil, stageDTOs(exp, inst, nil)), nil
}

// shouldReconcileReleasedInstance 判断已有实例是否需要根据沙箱实际生命周期重新对账。
// 资源回收事件可能在控制面重启期间丢失,进入入口时必须把已销毁的活跃实例恢复为可重建态。
func shouldReconcileReleasedInstance(status int16) bool {
	return status == InstanceStatusRunning || status == InstanceStatusPaused
}

// reconcileReleasedInstance 将实例记录与 M2 沙箱状态对齐,让已销毁资源进入正式 released 状态。
// 仅在全部引用沙箱都已销毁时变更状态,避免单个阶段资源回收误伤仍在运行的实例。
func (s *Service) reconcileReleasedInstance(ctx context.Context, inst ExperimentInstance) (ExperimentInstance, error) {
	if !shouldReconcileReleasedInstance(inst.Status) || len(inst.SandboxRefs) == 0 {
		return inst, nil
	}
	var updated ExperimentInstance
	changed := false
	if err := s.store.TenantTx(ctx, inst.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetInstanceForUpdate(ctx, inst.TenantID, inst.ID)
		if err != nil {
			return err
		}
		updated = current
		// 外部读取可能已过期,不能把新运行代次或已完成的实例改回释放态。
		if current.ScopeRef != inst.ScopeRef || !shouldReconcileReleasedInstance(current.Status) || len(current.SandboxRefs) == 0 {
			return nil
		}
		allDestroyed, err := s.allInstanceSandboxesDestroyed(ctx, current)
		if err != nil || !allDestroyed {
			return err
		}
		updated, err = tx.SetInstanceStatus(ctx, current.TenantID, current.ID, InstanceStatusReleased)
		changed = err == nil
		return err
	}); err != nil {
		return ExperimentInstance{}, err
	}
	if changed {
		s.publishInstanceChanged(ctx, updated.TenantID, updated.ID)
	}
	return updated, nil
}

// instanceCreationLockKey 为同一租户、实验和协作主体生成事务级互斥键,避免并发请求同时看不到实例而重复创建资源。
func instanceCreationLockKey(tenantID, experimentID, ownerID, groupID int64) int64 {
	// 小组实例按协作主体共享锁,不同组员不能因账号不同绕过同一实例的创建检查。
	if groupID > 0 {
		ownerID = 0
	}
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
		if err := ensureInstanceAccess(ctx, tx, id.AccountID, inst); err != nil {
			return err
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
	inst, err = s.reconcileSandboxFailure(ctx, inst)
	if err != nil {
		return InstanceDTO{}, err
	}
	return s.instanceWithCurrentJudgements(ctx, inst, exp, checkpoints)
}

// reconcileSandboxFailure 按 M2 权威状态对账实例启动或个性化初始化失败。
// 事件用于及时推进状态,读取路径负责覆盖事件投递与业务引用落库之间的短暂时间差。
func (s *Service) reconcileSandboxFailure(ctx context.Context, inst ExperimentInstance) (ExperimentInstance, error) {
	if !instanceCanReconcileSandboxFailure(inst.Status) || len(inst.SandboxRefs) == 0 || s.sandbox == nil {
		return inst, nil
	}
	for _, ref := range inst.SandboxRefs {
		info, err := s.sandbox.GetSandbox(ctx, inst.TenantID, ref.SandboxID.Int64())
		if err != nil {
			return ExperimentInstance{}, apperr.ErrExperimentSandboxUnavailable.WithCause(err)
		}
		if info.Status != contracts.SandboxStatusFailed {
			continue
		}
		if err := s.recycleEngines(ctx, inst, "sandbox_startup_failed"); err != nil {
			return ExperimentInstance{}, err
		}
		return s.markSandboxFailure(ctx, inst)
	}
	return inst, nil
}

// markSandboxFailure 只把补偿回收的同代实例推进错误态，保留期间重建或完成的状态。
func (s *Service) markSandboxFailure(ctx context.Context, inst ExperimentInstance) (ExperimentInstance, error) {
	var updated ExperimentInstance
	changed := false
	if err := s.store.TenantTx(ctx, inst.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetInstanceForUpdate(ctx, inst.TenantID, inst.ID)
		if err != nil {
			return err
		}
		updated = current
		if current.ScopeRef != inst.ScopeRef || !instanceCanReconcileSandboxFailure(current.Status) {
			return nil
		}
		updated, err = tx.SetInstanceStatus(ctx, inst.TenantID, inst.ID, InstanceStatusError)
		changed = err == nil && current.Status != updated.Status
		return err
	}); err != nil {
		return ExperimentInstance{}, err
	}
	if changed {
		s.publishInstanceChanged(ctx, updated.TenantID, updated.ID)
	}
	return updated, nil
}

// instanceCanReconcileSandboxFailure 列出仍可能持有启动资源的实例状态。
func instanceCanReconcileSandboxFailure(status int16) bool {
	return status == InstanceStatusCreating || status == InstanceStatusRunning || status == InstanceStatusPaused || status == InstanceStatusError
}

// ListTeacherInstances 查询教师可见的活跃实验实例。
func (s *Service) ListTeacherInstances(ctx context.Context, status int16, page, size int) ([]InstanceDTO, int64, int, int, pagex.Facets, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, 0, page, size, nil, err
	}
	isSchoolAdmin, err := s.isSchoolAdmin(ctx, id.AccountID)
	if err != nil {
		return nil, 0, page, size, nil, err
	}
	teacherID := id.AccountID
	if isSchoolAdmin {
		teacherID = 0
	}
	page, size = pagex.Normalize(page, size)
	var items []ExperimentInstance
	var total int64
	var byStatus []statusCount
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		items, total, err = tx.ListTeacherInstances(ctx, id.TenantID, teacherID, status, page, size)
		if err != nil {
			return err
		}
		byStatus, err = tx.CountTeacherInstancesByStatus(ctx, id.TenantID, teacherID)
		return err
	})
	if err != nil {
		return nil, 0, page, size, nil, err
	}
	out := make([]InstanceDTO, 0, len(items))
	for _, item := range items {
		item, err = s.reconcileSandboxFailure(ctx, item)
		if err != nil {
			return nil, 0, page, size, nil, err
		}
		out = append(out, instanceDTOFromModel(item, nil))
	}
	return out, total, page, size, statusFacets(byStatus), nil
}

// GetProgress 返回统一 M10 进度 topic 元信息。
func (s *Service) GetProgress(ctx context.Context, instanceID int64) (ProgressDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ProgressDTO{}, err
	}
	if err := s.AuthorizeProgressSubscription(ctx, id.TenantID, instanceID); err != nil {
		return ProgressDTO{}, err
	}
	return ProgressDTO{Topic: instanceProgressTopic(id.TenantID, instanceID), Channel: progressChannelName}, nil
}

// PauseInstance 暂停实验实例并通知 M2 暂停已有沙箱。
func (s *Service) PauseInstance(ctx context.Context, instanceID int64) (InstanceDTO, error) {
	return s.controlInstance(ctx, instanceID, InstanceStatusPaused, func(ctx context.Context, inst *ExperimentInstance) error {
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
	return s.controlInstance(ctx, instanceID, InstanceStatusRunning, func(ctx context.Context, inst *ExperimentInstance) error {
		if inst.Status == InstanceStatusReleased || inst.Status == InstanceStatusError {
			previous := *inst
			if err := s.recycleEngines(ctx, *inst, "resume_recovery"); err != nil {
				return err
			}
			// 资源释放后恢复属于新的运行代次:source_ref 继续代表同一实验实例,
			// scope_ref 改为新的唯一值,供 M2/M4 精确回收本次重建资源。
			inst.ScopeRef = scopeRefForInstanceGeneration(inst.SourceRef, s.ids.Generate())
			var exp Experiment
			if err := s.store.TenantTx(ctx, inst.TenantID, func(ctx context.Context, tx TxStore) error {
				var err error
				exp, err = tx.GetExperiment(ctx, inst.TenantID, inst.ExperimentID)
				return err
			}); err != nil {
				return err
			}
			if err := s.applySavedWorkspaceArchives(ctx, &exp, *inst); err != nil {
				return err
			}
			var err error
			inst.SandboxRefs, inst.SimSessionRefs, err = s.createInitialEngineResources(ctx, exp, *inst)
			if err != nil {
				if recycleErr := s.recycleEngines(ctx, *inst, "resume_failed"); recycleErr != nil {
					_, statusErr := s.saveInstanceControl(ctx, previous, *inst, InstanceStatusError)
					return errors.Join(err, recycleErr, statusErr)
				}
				inst.SandboxRefs, inst.SimSessionRefs = nil, nil
				return err
			}
			return nil
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

// applySavedWorkspaceArchives 把已回收沙箱最后一次保存的工作区作为重建初始化归档。
// 归档引用只在服务端模块契约内流转,浏览器仍只看到新沙箱的能力摘要。
func (s *Service) applySavedWorkspaceArchives(ctx context.Context, exp *Experiment, inst ExperimentInstance) error {
	if len(inst.SandboxRefs) == 0 {
		return nil
	}
	if s.sandbox == nil {
		return apperr.ErrExperimentSandboxUnavailable
	}
	archives := make(map[string]string, len(inst.SandboxRefs))
	for _, ref := range inst.SandboxRefs {
		archive, err := s.sandbox.GetSandboxWorkspaceArchive(ctx, inst.TenantID, ref.SandboxID.Int64())
		if err != nil {
			return apperr.ErrExperimentSandboxUnavailable.WithCause(err)
		}
		if strings.TrimSpace(archive) != "" {
			archives[strings.TrimSpace(ref.ComponentID)] = archive
		}
	}
	for i := range exp.Components.Envs {
		if archive := archives[strings.TrimSpace(exp.Components.Envs[i].ID)]; archive != "" {
			exp.Components.Envs[i].InitCodeRef = archive
		}
	}
	return nil
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
				if errors.Is(err, apperr.ErrExperimentReportNotFound) {
					return apperr.ErrExperimentReportRequired.WithCause(err)
				}
				return err
			}
		}
		if err := s.syncTerminalCheckpointResults(ctx, tx, current, exp); err != nil {
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
		if err := s.enqueueExperimentEventOutbox(ctx, tx, inst); err != nil {
			return err
		}
		return s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumStudent, "experiment.instance.finish", auditTargetInstance, inst.ID, map[string]any{"score": inst.Score})
	}); err != nil {
		return InstanceDTO{}, err
	}
	if action == instanceFinishCompleted {
		return instanceDTOFromModel(inst, nil), nil
	}
	if action == instanceFinishPrepare {
		s.publishInstanceChanged(ctx, inst.TenantID, inst.ID)
		s.drainExperimentEventOutboxBestEffort(ctx)
	}
	inst, err = s.recycleInstance(ctx, inst, "finished")
	if err != nil {
		return InstanceDTO{}, err
	}
	return instanceDTOFromModel(inst, nil), nil
}

// syncTerminalCheckpointResults 在实例锁内同步终态结果，消除事件回写延迟对首次结算的影响。
func (s *Service) syncTerminalCheckpointResults(ctx context.Context, tx TxStore, inst ExperimentInstance, exp Experiment) error {
	checkpoints, err := tx.ListCheckpoints(ctx, inst.TenantID, inst.ID)
	if err != nil {
		return err
	}
	current, err := s.currentCheckpointResults(ctx, inst, exp, checkpoints)
	if err != nil {
		return err
	}
	for _, checkpoint := range current {
		switch checkpoint.JudgeStatus {
		case contracts.JudgeTaskStatusQueued, contracts.JudgeTaskStatusRunning:
			return apperr.ErrExperimentJudgePending
		case contracts.JudgeTaskStatusDone, contracts.JudgeTaskStatusFailed, contracts.JudgeTaskStatusCanceled:
			if _, err := tx.UpsertCheckpoint(ctx, checkpoint); err != nil {
				return err
			}
		default:
			return apperr.ErrExperimentCheckpointInvalid
		}
	}
	return nil
}

// RunRecycleOnce 执行一次 M7 后台回收扫描,供统一 background runner 调用。
func (s *Service) RunRecycleOnce(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
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
		return errors.Join(err, ctx.Err())
	}
	return s.recycleInstances(ctx, items, "lifecycle_recycle")
}

func (s *Service) controlInstance(ctx context.Context, instanceID int64, next int16, beforeSave func(context.Context, *ExperimentInstance) error) (InstanceDTO, error) {
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
	previous := inst
	if err := beforeSave(ctx, &inst); err != nil {
		return InstanceDTO{}, err
	}
	updated, err := s.saveInstanceControl(ctx, previous, inst, next)
	if err != nil {
		if inst.ScopeRef != previous.ScopeRef {
			err = errors.Join(err, s.recycleEngines(ctx, inst, "resume_persist_failed"))
		}
		return InstanceDTO{}, err
	}
	s.publishInstanceChanged(ctx, updated.TenantID, updated.ID)
	return instanceDTOFromModel(updated, nil), nil
}

// saveInstanceControl 拒绝过期操作；新作用域及其资源、状态必须一次提交。
func (s *Service) saveInstanceControl(ctx context.Context, previous, pending ExperimentInstance, next int16) (ExperimentInstance, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ExperimentInstance{}, err
	}
	var updated ExperimentInstance
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetInstanceForUpdate(ctx, id.TenantID, previous.ID)
		if err != nil {
			return err
		}
		if current.Status != previous.Status || !sameInstanceResources(current, previous) {
			return apperr.ErrExperimentInstanceStateInvalid
		}
		if err := ensureInstanceAccess(ctx, tx, id.AccountID, current); err != nil {
			return err
		}
		if pending.ScopeRef != previous.ScopeRef {
			if err := s.reconcileCreatedResourceAuthorizations(ctx, tx, current, pending.SandboxRefs, pending.SimSessionRefs); err != nil {
				return err
			}
			updated, err = tx.UpdateInstanceResources(ctx, id.TenantID, current.ID, pending.ScopeRef, pending.SandboxRefs, pending.SimSessionRefs, next)
		} else {
			updated, err = tx.SetInstanceStatus(ctx, id.TenantID, current.ID, next)
		}
		return err
	})
	return updated, err
}

func (s *Service) recycleInstances(ctx context.Context, items []ExperimentInstance, reason string) error {
	var resultErr error
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return errors.Join(resultErr, err)
		}
		if _, err := s.recycleInstance(ctx, item, reason); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("回收实验实例失败 tenant_id=%d instance_id=%d: %w", item.TenantID, item.ID, err))
		}
	}
	return errors.Join(resultErr, ctx.Err())
}

// recycleInstance 只将已回收的同一组资源落为终态，避免期间重建或新增阶段被误报为已回收。
func (s *Service) recycleInstance(ctx context.Context, inst ExperimentInstance, reason string) (ExperimentInstance, error) {
	if err := s.recycleEngines(ctx, inst, reason); err != nil {
		return ExperimentInstance{}, err
	}
	var updated ExperimentInstance
	changed := false
	err := s.store.TenantTx(ctx, inst.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetInstanceForUpdate(ctx, inst.TenantID, inst.ID)
		if err != nil {
			return err
		}
		if (current.Status != inst.Status && current.Status != InstanceStatusRecycled) || !sameInstanceResources(current, inst) {
			return apperr.ErrExperimentInstanceStateInvalid
		}
		updated = current
		if current.Status != InstanceStatusRecycled {
			updated, err = tx.SetInstanceStatus(ctx, inst.TenantID, inst.ID, InstanceStatusRecycled)
			changed = err == nil
		}
		return err
	})
	if err == nil && changed {
		s.publishInstanceChanged(ctx, updated.TenantID, updated.ID)
	}
	return updated, err
}

func sameInstanceResources(a, b ExperimentInstance) bool {
	return a.ScopeRef == b.ScopeRef &&
		slices.EqualFunc(a.SandboxRefs, b.SandboxRefs, func(a, b SandboxRef) bool { return a.SandboxID == b.SandboxID }) &&
		slices.EqualFunc(a.SimSessionRefs, b.SimSessionRefs, func(a, b SimSessionRef) bool { return a.SessionID == b.SessionID })
}

// recycleEngines 按实例 scope_ref 回收 M2/M4 资源,并以 source_ref 做稳定归属校验。
func (s *Service) recycleEngines(ctx context.Context, inst ExperimentInstance, reason string) error {
	if !validExperimentSourceRef(inst.SourceRef) {
		return apperr.ErrExperimentSourceRefInvalid
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Duration(s.cfg.CleanupTimeoutSeconds)*time.Second)
	defer cancel()
	var sandboxErr, simErr error
	if len(inst.SandboxRefs) > 0 {
		if s.sandbox == nil {
			sandboxErr = apperr.ErrExperimentSandboxUnavailable
		} else {
			sandboxErr = s.sandbox.RecycleByScopeRef(ctx, contracts.SandboxRecycleRequest{TenantID: inst.TenantID, ScopeRef: inst.ScopeRef, SourceRef: inst.SourceRef, Reason: reason})
		}
	}
	if len(inst.SimSessionRefs) > 0 {
		if s.sim == nil {
			simErr = apperr.ErrExperimentSimUnavailable
		} else {
			simErr = s.sim.RecycleByScopeRef(ctx, contracts.SimRecycleRequest{TenantID: inst.TenantID, ScopeRef: inst.ScopeRef, SourceRef: inst.SourceRef, Reason: reason})
		}
	}
	if err := errors.Join(sandboxErr, simErr); err != nil {
		return apperr.ErrExperimentRecycleFailed.WithCause(err)
	}
	return nil
}

// HandleSandboxFailed 消费 M2 启动或个性化初始化失败事件,先补偿作用域资源再写实例错误态。
func (s *Service) HandleSandboxFailed(ctx context.Context, event contracts.SandboxFailedEvent) error {
	if event.TenantID <= 0 || event.SandboxID <= 0 || !validExperimentSourceRef(event.SourceRef) || !auth.ValidScopeRef(event.ScopeRef) {
		return apperr.ErrExperimentSourceRefInvalid
	}
	var inst ExperimentInstance
	if err := s.store.TenantTx(ctx, event.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		inst, err = tx.GetInstanceBySourceRef(ctx, event.TenantID, event.SourceRef)
		if isNoRows(err) {
			return nil
		}
		return err
	}); err != nil {
		return err
	}
	if inst.ID == 0 || inst.ScopeRef != event.ScopeRef || !instanceCanReconcileSandboxFailure(inst.Status) {
		return nil
	}
	if err := s.recycleEngines(ctx, inst, "sandbox_startup_failed"); err != nil {
		return err
	}
	_, err := s.markSandboxFailure(ctx, inst)
	return err
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

// HandleSandboxRecycled 消费 M2 回收事件,同步进行中实例或续跑已完成实例的整体回收。
func (s *Service) HandleSandboxRecycled(ctx context.Context, event contracts.SandboxRecycledEvent) error {
	if event.TenantID <= 0 || !validExperimentSourceRef(event.SourceRef) {
		return apperr.ErrExperimentSourceRefInvalid
	}
	var inst ExperimentInstance
	var finishRecovery bool
	released := false
	if err := s.store.TenantTx(ctx, event.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		inst, err = tx.GetInstanceBySourceRef(ctx, event.TenantID, event.SourceRef)
		if err != nil {
			return err
		}
		inst, err = tx.GetInstanceForUpdate(ctx, event.TenantID, inst.ID)
		if err != nil {
			return err
		}
		// source_ref 跨重建保持稳定,只有当前引用的沙箱回收事件才能推进本代实例。
		belongs := false
		for _, ref := range inst.SandboxRefs {
			if ref.SandboxID.Int64() == event.SandboxID {
				belongs = true
				break
			}
		}
		if !belongs {
			return nil
		}
		if inst.Status == InstanceStatusRunning || inst.Status == InstanceStatusPaused || inst.Status == InstanceStatusCreating {
			allDestroyed, err := s.allInstanceSandboxesDestroyed(ctx, inst)
			if err != nil || !allDestroyed {
				return err
			}
			_, err = tx.SetInstanceStatus(ctx, event.TenantID, inst.ID, InstanceStatusReleased)
			released = err == nil
			return err
		}
		finishRecovery = inst.Status == InstanceStatusFinished
		return nil
	}); err != nil {
		return err
	}
	if released {
		s.publishInstanceChanged(ctx, inst.TenantID, inst.ID)
	}
	if !finishRecovery {
		return nil
	}
	allDestroyed, err := s.allInstanceSandboxesDestroyed(ctx, inst)
	if err != nil {
		return err
	}
	if !allDestroyed {
		return nil
	}
	_, err = s.recycleInstance(ctx, inst, "finished_recovery")
	return err
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

// HandleCourseEnded 回收课程内尚未完成清理的实例；失败返回供事件重投，成功实例不再扫描。
func (s *Service) HandleCourseEnded(ctx context.Context, event contracts.TeachingCourseEndedEvent) error {
	if event.TenantID <= 0 || event.CourseID <= 0 {
		return apperr.ErrExperimentInstanceInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	var items []ExperimentInstance
	if err := s.store.TenantTx(ctx, event.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		items, err = tx.ListLiveInstancesByCourse(ctx, event.TenantID, event.CourseID)
		return err
	}); err != nil {
		return errors.Join(err, ctx.Err())
	}
	return s.recycleInstances(ctx, items, "course_ended")
}
