// experiment service_stage 文件实现 M7 阶段编排、解锁判定和参数绑定。
package experiment

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"

	"golang.org/x/sync/errgroup"
)

const (
	stageStatusLocked    = "locked"
	stageStatusAvailable = "available"
	stageStatusActive    = "active"
)

// ActivateStage 显式激活已满足条件的阶段,阶段资源只允许从该写接口创建。
func (s *Service) ActivateStage(ctx context.Context, instanceID int64, stageNo int32) (InstanceDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return InstanceDTO{}, err
	}
	var inst ExperimentInstance
	var exp Experiment
	var checkpoints []CheckpointResult
	var stage StageConfig
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		inst, err = tx.GetInstance(ctx, id.TenantID, instanceID)
		if err != nil {
			return err
		}
		if err := ensureInstanceAccess(ctx, tx, id.AccountID, inst); err != nil {
			return err
		}
		if inst.Status != InstanceStatusRunning && inst.Status != InstanceStatusPaused {
			return apperr.ErrExperimentInstanceStateInvalid
		}
		exp, err = tx.GetExperiment(ctx, id.TenantID, inst.ExperimentID)
		if err != nil {
			return err
		}
		checkpoints, err = tx.ListCheckpoints(ctx, id.TenantID, inst.ID)
		if err != nil {
			return err
		}
		var ok bool
		stage, ok = stageByNumber(exp.Components.Stages, stageNo)
		if !ok {
			return apperr.ErrExperimentInstanceStateInvalid
		}
		return nil
	}); err != nil {
		return InstanceDTO{}, err
	}
	checkpoints, err = s.currentCheckpointResults(ctx, inst, exp, checkpoints)
	if err != nil {
		return InstanceDTO{}, err
	}
	if !stageUnlocked(stage, checkpoints) {
		return InstanceDTO{}, apperr.ErrExperimentInstanceStateInvalid
	}
	if stageResourcesCreated(inst, stage.Stage) {
		return instanceDTOFromModel(inst, checkpoints, stageDTOs(exp, inst, checkpoints)), nil
	}
	if inst.Status == InstanceStatusPaused {
		return InstanceDTO{}, apperr.ErrExperimentInstanceStateInvalid
	}
	envs, sims, err := componentsForStage(exp.Components, stage, checkpoints)
	if err != nil {
		return InstanceDTO{}, err
	}
	sandboxes, simRefs, err := s.createEngineResourcesForStage(ctx, inst, stage.Stage, envs, sims)
	if err != nil {
		if cleanupErr := s.destroyCreatedStageResources(ctx, inst, sandboxes, simRefs); cleanupErr != nil {
			return InstanceDTO{}, apperr.ErrExperimentRecycleFailed.WithCause(errors.Join(err, cleanupErr))
		}
		return InstanceDTO{}, err
	}
	duplicateActivation := false
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		latest, err := tx.GetInstanceForUpdate(ctx, id.TenantID, instanceID)
		if err != nil {
			return err
		}
		if err := ensureInstanceAccess(ctx, tx, id.AccountID, latest); err != nil {
			return err
		}
		if latest.Status != InstanceStatusRunning || latest.SourceRef != inst.SourceRef || latest.ScopeRef != inst.ScopeRef {
			return apperr.ErrExperimentInstanceStateInvalid
		}
		if stageResourcesCreated(latest, stage.Stage) {
			inst = latest
			duplicateActivation = true
			return nil
		}
		current, err := tx.ListCheckpoints(ctx, id.TenantID, instanceID)
		if err != nil {
			return err
		}
		current, err = s.currentCheckpointResults(ctx, latest, exp, current)
		if err != nil {
			return err
		}
		_, currentSims, err := componentsForStage(exp.Components, stage, current)
		if err != nil {
			return err
		}
		if !stageUnlocked(stage, current) || !reflect.DeepEqual(sims, currentSims) {
			return apperr.ErrExperimentInstanceStateInvalid
		}
		if err := s.reconcileCreatedResourceAuthorizations(ctx, tx, latest, sandboxes, simRefs); err != nil {
			return err
		}
		latest.SandboxRefs = append(latest.SandboxRefs, sandboxes...)
		latest.SimSessionRefs = append(latest.SimSessionRefs, simRefs...)
		inst, err = tx.UpdateInstanceResources(ctx, id.TenantID, latest.ID, latest.ScopeRef, latest.SandboxRefs, latest.SimSessionRefs, InstanceStatusRunning)
		return err
	}); err != nil {
		if cleanupErr := s.destroyCreatedStageResources(ctx, inst, sandboxes, simRefs); cleanupErr != nil {
			return InstanceDTO{}, apperr.ErrExperimentRecycleFailed.WithCause(errors.Join(err, cleanupErr))
		}
		return InstanceDTO{}, err
	}
	if duplicateActivation {
		if cleanupErr := s.destroyCreatedStageResources(ctx, inst, sandboxes, simRefs); cleanupErr != nil {
			return InstanceDTO{}, cleanupErr
		}
	}
	if err := s.store.TenantTx(ctx, inst.TenantID, func(ctx context.Context, tx TxStore) error {
		checkpoints, err = tx.ListCheckpoints(ctx, inst.TenantID, inst.ID)
		return err
	}); err != nil {
		return InstanceDTO{}, err
	}
	return s.instanceWithCurrentJudgements(ctx, inst, exp, checkpoints)
}

// reconcileCreatedResourceAuthorizations 在实例引用落库前锁住小组并读取最新成员。
// 创建阶段先并发建资源,这里是最终一致性的闸门:成员变更与资源引用写入按同一小组行串行,
// 从而不会把一个已创建但仍带旧成员快照的沙箱或仿真会话写进共享实例。
func (s *Service) reconcileCreatedResourceAuthorizations(ctx context.Context, tx TxStore, inst ExperimentInstance, sandboxes []SandboxRef, sims []SimSessionRef) error {
	if inst.GroupID <= 0 {
		return nil
	}
	group, err := tx.GetGroupForUpdate(ctx, inst.TenantID, inst.GroupID)
	if err != nil {
		return err
	}
	if group.ExperimentID != inst.ExperimentID {
		return apperr.ErrExperimentGroupInvalid
	}
	members, err := tx.ListGroupMembers(ctx, inst.TenantID, inst.GroupID)
	if err != nil {
		return err
	}
	accountIDs := groupMemberAccountIDs(members)
	for _, ref := range sandboxes {
		if s.sandbox == nil {
			return apperr.ErrExperimentSandboxUnavailable
		}
		if err := s.sandbox.UpdateSandboxAuthorizedAccounts(ctx, contracts.SandboxAuthorizedAccountsRequest{TenantID: inst.TenantID, SandboxID: ref.SandboxID.Int64(), SourceRef: inst.SourceRef, AuthorizedAccountIDs: accountIDs}); err != nil {
			return apperr.ErrExperimentSandboxUnavailable.WithCause(err)
		}
	}
	for _, ref := range sims {
		if s.sim == nil {
			return apperr.ErrExperimentSimUnavailable
		}
		if err := s.sim.UpdateSessionAuthorizedAccounts(ctx, contracts.SimAuthorizedAccountsRequest{TenantID: inst.TenantID, SessionID: ref.SessionID.Int64(), SourceRef: inst.SourceRef, AuthorizedAccountIDs: accountIDs}); err != nil {
			return apperr.ErrExperimentSimUnavailable.WithCause(err)
		}
	}
	return nil
}

// createEngineResourcesForStage 并发创建单个阶段内的 M2/M4 资源。
func (s *Service) createEngineResourcesForStage(ctx context.Context, inst ExperimentInstance, stageNo int32, envs []EnvComponent, sims []SimComponent) ([]SandboxRef, []SimSessionRef, error) {
	authorizedAccountIDs, err := s.instanceAuthorizedAccountIDs(ctx, inst)
	if err != nil {
		return nil, nil, err
	}
	var mu sync.Mutex
	var group errgroup.Group
	sandboxes := []SandboxRef{}
	simRefs := []SimSessionRef{}
	for _, env := range envs {
		componentID := strings.TrimSpace(env.ID)
		group.Go(func() error {
			if s.sandbox == nil {
				return apperr.ErrExperimentSandboxUnavailable
			}
			info, err := s.sandbox.CreateSandbox(ctx, contracts.SandboxCreateRequest{TenantID: inst.TenantID, CompositionSnapshot: env.CompositionSnapshot, InitCodeRef: env.InitCodeRef, InitScriptRef: env.InitScriptRef, OwnerAccountID: inst.OwnerAccountID, AuthorizedAccountIDs: authorizedAccountIDs, SourceRef: inst.SourceRef, ScopeRef: inst.ScopeRef, KeepAlive: env.KeepAlive, SnapshotEnabled: env.SnapshotEnabled, KeepAliveMinutes: env.KeepAliveMinutes, SnapshotRetentionMinutes: env.SnapshotRetentionMinutes})
			if err != nil {
				return apperr.ErrExperimentSandboxUnavailable.WithCause(err)
			}
			ref := sandboxRefFromContract(componentID, info)
			ref.Stage = stageNo
			mu.Lock()
			sandboxes = append(sandboxes, ref)
			mu.Unlock()
			return nil
		})
	}
	for _, sim := range sims {
		componentID := strings.TrimSpace(sim.ID)
		group.Go(func() error {
			if s.sim == nil {
				return apperr.ErrExperimentSimUnavailable
			}
			info, err := s.sim.CreateSession(ctx, contracts.SimCreateSessionRequest{TenantID: inst.TenantID, PackageCode: sim.PackageCode, Version: sim.Version, Seed: sim.Seed, InitParams: sim.Params, OwnerAccountID: inst.OwnerAccountID, AuthorizedAccountIDs: authorizedAccountIDs, SourceRef: inst.SourceRef, ScopeRef: inst.ScopeRef})
			if err != nil {
				return apperr.ErrExperimentSimUnavailable.WithCause(err)
			}
			ref := simRefFromContract(componentID, info)
			ref.Stage = stageNo
			mu.Lock()
			simRefs = append(simRefs, ref)
			mu.Unlock()
			return nil
		})
	}
	// 等待全部创建结束后再读取结果，失败时也保留成功资源供补偿。
	err = group.Wait()
	return sandboxes, simRefs, err
}

// instanceAuthorizedAccountIDs 在创建阶段资源前固定小组账号集合,避免各环境并发读取出不同授权快照。
func (s *Service) instanceAuthorizedAccountIDs(ctx context.Context, inst ExperimentInstance) ([]int64, error) {
	if inst.GroupID <= 0 {
		return nil, nil
	}
	var members []GroupMember
	if err := s.store.TenantTx(ctx, inst.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		members, err = tx.ListGroupMembers(ctx, inst.TenantID, inst.GroupID)
		return err
	}); err != nil {
		return nil, err
	}
	return groupMemberAccountIDs(members), nil
}

// groupMemberAccountIDs 把 M7 成员模型投影为 M2 稳定契约需要的账号列表。
func groupMemberAccountIDs(members []GroupMember) []int64 {
	out := make([]int64, 0, len(members))
	for _, member := range members {
		out = append(out, member.StudentID)
	}
	return out
}

// createInitialEngineResources 创建实例启动时应立即可用的资源。
func (s *Service) createInitialEngineResources(ctx context.Context, exp Experiment, inst ExperimentInstance) ([]SandboxRef, []SimSessionRef, error) {
	if len(exp.Components.Stages) == 0 {
		return s.createEngineResourcesForStage(ctx, inst, 0, exp.Components.Envs, exp.Components.Sims)
	}
	sandboxes := []SandboxRef{}
	sims := []SimSessionRef{}
	for _, stage := range sortedStages(exp.Components.Stages) {
		if stage.UnlockCondition != nil {
			continue
		}
		envs, stageSims, err := componentsForStage(exp.Components, stage, nil)
		if err != nil {
			return sandboxes, sims, err
		}
		stageSandboxes, stageSimRefs, err := s.createEngineResourcesForStage(ctx, inst, stage.Stage, envs, stageSims)
		sandboxes = append(sandboxes, stageSandboxes...)
		sims = append(sims, stageSimRefs...)
		if err != nil {
			// 调用者按本次作用域统一补偿，失败时须保留残余引用供重试。
			return sandboxes, sims, err
		}
	}
	return sandboxes, sims, nil
}

func componentsForStage(cfg ComponentConfig, stage StageConfig, checkpoints []CheckpointResult) ([]EnvComponent, []SimComponent, error) {
	envByID := envComponentsByID(cfg.Envs)
	simByID := simComponentsByID(cfg.Sims)
	envs := []EnvComponent{}
	for _, id := range stage.Components.Envs {
		env, ok := envByID[strings.TrimSpace(id)]
		if !ok {
			return nil, nil, apperr.ErrExperimentInvalid
		}
		envs = append(envs, env)
	}
	sims := []SimComponent{}
	for _, id := range stage.Components.Sims {
		sim, ok := simByID[strings.TrimSpace(id)]
		if !ok {
			return nil, nil, apperr.ErrExperimentInvalid
		}
		bound, err := applyStageBindings(sim, stage.ParamBindings, checkpoints)
		if err != nil {
			return nil, nil, err
		}
		sims = append(sims, bound)
	}
	return envs, sims, nil
}

func applyStageBindings(sim SimComponent, bindings []ParamBinding, checkpoints []CheckpointResult) (SimComponent, error) {
	out := sim
	params, err := jsonx.CloneObjectStrict(sim.Params)
	if err != nil {
		return SimComponent{}, apperr.ErrExperimentInvalid.WithCause(err)
	}
	out.Params = params
	for _, binding := range bindings {
		if strings.TrimSpace(binding.TargetComponent) != strings.TrimSpace(sim.ID) {
			continue
		}
		value, err := resolveBindingValue(binding, checkpoints)
		if err != nil {
			return SimComponent{}, err
		}
		setParamValue(out.Params, strings.TrimSpace(binding.TargetParam), value)
	}
	return out, nil
}

func resolveBindingValue(binding ParamBinding, checkpoints []CheckpointResult) (any, error) {
	switch strings.TrimSpace(binding.SourceType) {
	case "constant":
		return binding.ConstantValue, nil
	case "checkpoint":
		source, ok := checkpointResultByID(checkpoints, strings.TrimSpace(binding.SourceRef))
		if !ok {
			return nil, apperr.ErrExperimentCheckpointInvalid
		}
		payload := map[string]any{
			"passed":         source.Passed,
			"score":          source.Score,
			"detail_ref":     source.DetailRef,
			"judge_task_ref": source.JudgeTaskRef,
			"binding_output": source.BindingOutput,
		}
		if len(source.BindingOutput) > 0 {
			for key, value := range source.BindingOutput {
				payload[key] = value
			}
		}
		if strings.TrimSpace(binding.SourcePath) == "" {
			return payload, nil
		}
		value, ok := valueAtPath(payload, binding.SourcePath)
		if !ok {
			return nil, apperr.ErrExperimentCheckpointInvalid
		}
		return value, nil
	default:
		return nil, apperr.ErrExperimentInvalid
	}
}

func stageDTOs(exp Experiment, inst ExperimentInstance, checkpoints []CheckpointResult) []StageDTO {
	if len(exp.Components.Stages) == 0 {
		return nil
	}
	out := make([]StageDTO, 0, len(exp.Components.Stages))
	for _, stage := range sortedStages(exp.Components.Stages) {
		status := stageStatusLocked
		if stageResourcesCreated(inst, stage.Stage) {
			status = stageStatusActive
		} else if stageUnlocked(stage, checkpoints) {
			status = stageStatusAvailable
		}
		out = append(out, StageDTO{Stage: stage.Stage, Title: stage.Title, Description: stage.Description, Status: status, Components: stage.Components, UnlockCondition: stage.UnlockCondition})
	}
	return out
}

func stageUnlocked(stage StageConfig, checkpoints []CheckpointResult) bool {
	cond := stage.UnlockCondition
	if cond == nil || strings.TrimSpace(cond.Type) == "manual" {
		return true
	}
	if strings.TrimSpace(cond.Type) != "checkpoint" {
		return false
	}
	cp, ok := checkpointResultByID(checkpoints, strings.TrimSpace(cond.CheckpointID))
	return ok && cp.JudgeStatus == contracts.JudgeTaskStatusDone && cp.Score >= cond.MinScore
}

func stageResourcesCreated(inst ExperimentInstance, stageNo int32) bool {
	for _, ref := range inst.SandboxRefs {
		if ref.Stage == stageNo {
			return true
		}
	}
	for _, ref := range inst.SimSessionRefs {
		if ref.Stage == stageNo {
			return true
		}
	}
	return false
}

func stageByNumber(stages []StageConfig, stageNo int32) (StageConfig, bool) {
	for _, stage := range stages {
		if stage.Stage == stageNo {
			return stage, true
		}
	}
	return StageConfig{}, false
}

func sortedStages(stages []StageConfig) []StageConfig {
	out := append([]StageConfig{}, stages...)
	sort.Slice(out, func(i, j int) bool { return out[i].Stage < out[j].Stage })
	return out
}

func checkpointResultByID(items []CheckpointResult, id string) (CheckpointResult, bool) {
	for _, item := range items {
		if item.CheckpointID == id {
			return item, true
		}
	}
	return CheckpointResult{}, false
}

func envComponentsByID(items []EnvComponent) map[string]EnvComponent {
	out := map[string]EnvComponent{}
	for _, item := range items {
		out[strings.TrimSpace(item.ID)] = item
	}
	return out
}

func simComponentsByID(items []SimComponent) map[string]SimComponent {
	out := map[string]SimComponent{}
	for _, item := range items {
		out[strings.TrimSpace(item.ID)] = item
	}
	return out
}

// valueAtPath 读取点分隔路径,支持对象字段和数组数字下标。
func valueAtPath(value any, path string) (any, bool) {
	current := value
	for _, part := range strings.Split(path, ".") {
		key := strings.TrimSpace(part)
		if key == "" {
			return nil, false
		}
		switch typed := current.(type) {
		case map[string]any:
			next, ok := typed[key]
			if !ok {
				return nil, false
			}
			current = next
		case []any:
			idx, err := strconv.Atoi(key)
			if err != nil || idx < 0 || idx >= len(typed) {
				return nil, false
			}
			current = typed[idx]
		default:
			return nil, false
		}
	}
	return current, true
}

// setParamValue 按点分隔路径写入仿真 init_params。
func setParamValue(params map[string]any, path string, value any) {
	parts := strings.Split(path, ".")
	current := params
	for i, part := range parts {
		key := strings.TrimSpace(part)
		if key == "" {
			continue
		}
		if i == len(parts)-1 {
			current[key] = value
			return
		}
		next, ok := current[key].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[key] = next
		}
		current = next
	}
}

// destroyCreatedStageResources 补偿释放阶段创建中已成功的资源。
func (s *Service) destroyCreatedStageResources(ctx context.Context, inst ExperimentInstance, sandboxes []SandboxRef, sims []SimSessionRef) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Duration(s.cfg.CleanupTimeoutSeconds)*time.Second)
	defer cancel()
	var failures []error
	for _, ref := range sandboxes {
		if s.sandbox == nil {
			failures = append(failures, apperr.ErrExperimentSandboxUnavailable)
			break
		}
		if err := s.sandbox.DestroySandbox(ctx, contracts.SandboxControlRequest{TenantID: inst.TenantID, SandboxID: ref.SandboxID.Int64(), SourceRef: inst.SourceRef}); err != nil {
			failures = append(failures, err)
		}
	}
	for _, ref := range sims {
		if s.sim == nil {
			failures = append(failures, apperr.ErrExperimentSimUnavailable)
			break
		}
		req := contracts.SimDestroySessionRequest{TenantID: inst.TenantID, SessionID: ref.SessionID.Int64(), SourceRef: inst.SourceRef}
		if err := s.sim.DestroySession(ctx, req); err != nil {
			failures = append(failures, err)
		}
	}
	if err := errors.Join(failures...); err != nil {
		return apperr.ErrExperimentRecycleFailed.WithCause(err)
	}
	return nil
}
