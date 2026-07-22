// experiment service_stage 文件实现 M7 阶段编排、解锁判定和参数绑定。
package experiment

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"

	"chaimir/internal/contracts"
	"chaimir/pkg/apperr"
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
		if !ok || !stageUnlocked(stage, checkpoints) {
			return apperr.ErrExperimentInstanceStateInvalid
		}
		return nil
	}); err != nil {
		return InstanceDTO{}, err
	}
	if stageResourcesCreated(inst, stage.Stage) {
		return instanceDTOFromModel(inst, checkpointDefaults(exp, checkpoints), stageDTOs(exp, inst, checkpoints)), nil
	}
	envs, sims, err := componentsForStage(exp.Components, stage, checkpoints)
	if err != nil {
		return InstanceDTO{}, err
	}
	sandboxes, simRefs, err := s.createEngineResourcesForStage(ctx, inst, stage.Stage, envs, sims)
	if err != nil {
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
		if latest.Status != InstanceStatusRunning && latest.Status != InstanceStatusPaused {
			return apperr.ErrExperimentInstanceStateInvalid
		}
		if stageResourcesCreated(latest, stage.Stage) {
			inst = latest
			duplicateActivation = true
			return nil
		}
		latest.SandboxRefs = append(latest.SandboxRefs, sandboxes...)
		latest.SimSessionRefs = append(latest.SimSessionRefs, simRefs...)
		inst, err = tx.UpdateInstanceResources(ctx, id.TenantID, latest.ID, latest.SandboxRefs, latest.SimSessionRefs, InstanceStatusRunning)
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
	checkpoints, err = s.loadInstanceCheckpoints(ctx, inst.TenantID, inst.ID)
	if err != nil {
		return InstanceDTO{}, err
	}
	return instanceDTOFromModel(inst, checkpointDefaults(exp, checkpoints), stageDTOs(exp, inst, checkpoints)), nil
}

// createEngineResourcesForStage 并发创建单个阶段内的 M2/M4 资源。
func (s *Service) createEngineResourcesForStage(ctx context.Context, inst ExperimentInstance, stageNo int32, envs []EnvComponent, sims []SimComponent) ([]SandboxRef, []SimSessionRef, error) {
	var mu sync.Mutex
	var wg sync.WaitGroup
	var firstErr error
	sandboxes := []SandboxRef{}
	simRefs := []SimSessionRef{}
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
	for _, env := range envs {
		componentID := strings.TrimSpace(env.ID)
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
			ref := sandboxRefFromContract(componentID, info)
			ref.Stage = stageNo
			mu.Lock()
			sandboxes = append(sandboxes, ref)
			mu.Unlock()
		}(env, componentID)
	}
	for _, sim := range sims {
		componentID := strings.TrimSpace(sim.ID)
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
			ref := simRefFromContract(componentID, info)
			ref.Stage = stageNo
			mu.Lock()
			simRefs = append(simRefs, ref)
			mu.Unlock()
		}(sim, componentID)
	}
	wg.Wait()
	if firstErr != nil {
		if cleanupErr := s.destroyCreatedStageResources(ctx, inst, sandboxes, simRefs); cleanupErr != nil {
			return nil, nil, apperr.ErrExperimentRecycleFailed.WithCause(errors.Join(firstErr, cleanupErr))
		}
		return nil, nil, firstErr
	}
	return sandboxes, simRefs, nil
}

// createInitialEngineResources 创建实例启动时应立即可用的资源。
func (s *Service) createInitialEngineResources(ctx context.Context, exp Experiment, inst ExperimentInstance) ([]SandboxRef, []SimSessionRef, error) {
	if len(exp.Components.Stages) == 0 {
		return s.createEngineResourcesForStage(ctx, inst, 0, exp.Components.Envs, exp.Components.Sims)
	}
	sandboxes := []SandboxRef{}
	sims := []SimSessionRef{}
	current := inst
	for _, stage := range sortedStages(exp.Components.Stages) {
		if stage.UnlockCondition != nil {
			continue
		}
		envs, stageSims, err := componentsForStage(exp.Components, stage, nil)
		if err != nil {
			return nil, nil, err
		}
		stageSandboxes, stageSimRefs, err := s.createEngineResourcesForStage(ctx, inst, stage.Stage, envs, stageSims)
		if err != nil {
			if cleanupErr := s.destroyCreatedStageResources(ctx, current, sandboxes, sims); cleanupErr != nil {
				return nil, nil, apperr.ErrExperimentRecycleFailed.WithCause(errors.Join(err, cleanupErr))
			}
			return nil, nil, err
		}
		sandboxes = append(sandboxes, stageSandboxes...)
		sims = append(sims, stageSimRefs...)
		current.SandboxRefs = sandboxes
		current.SimSessionRefs = sims
	}
	return sandboxes, sims, nil
}

// componentsForStage 解析阶段组件并把参数绑定注入到对应仿真。
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

// applyStageBindings 复制仿真参数并注入阶段绑定值。
func applyStageBindings(sim SimComponent, bindings []ParamBinding, checkpoints []CheckpointResult) (SimComponent, error) {
	out := sim
	out.Params = cloneAnyMap(sim.Params)
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

// resolveBindingValue 从常量或检查点绑定输出中提取参数值。
func resolveBindingValue(binding ParamBinding, checkpoints []CheckpointResult) (any, error) {
	switch strings.TrimSpace(binding.SourceType) {
	case "constant":
		return binding.ConstantValue, nil
	case "checkpoint":
		source, ok := checkpointResultByID(checkpoints)[strings.TrimSpace(binding.SourceRef)]
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

// stageDTOs 计算工作台可展示的阶段状态。
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

// stageUnlocked 判断阶段解锁条件是否满足。
func stageUnlocked(stage StageConfig, checkpoints []CheckpointResult) bool {
	cond := stage.UnlockCondition
	if cond == nil || strings.TrimSpace(cond.Type) == "manual" {
		return true
	}
	if strings.TrimSpace(cond.Type) != "checkpoint" {
		return false
	}
	cp, ok := checkpointResultByID(checkpoints)[strings.TrimSpace(cond.CheckpointID)]
	return ok && cp.Score >= cond.MinScore
}

// stageResourcesCreated 判断阶段资源是否已经落库。
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

// loadInstanceCheckpoints 在阶段激活落库后重新读取检查点,保证响应展示最新状态。
func (s *Service) loadInstanceCheckpoints(ctx context.Context, tenantID, instanceID int64) ([]CheckpointResult, error) {
	var checkpoints []CheckpointResult
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		checkpoints, err = tx.ListCheckpoints(ctx, tenantID, instanceID)
		return err
	}); err != nil {
		return nil, err
	}
	return checkpoints, nil
}

// stageByNumber 按编号查找阶段配置。
func stageByNumber(stages []StageConfig, stageNo int32) (StageConfig, bool) {
	for _, stage := range stages {
		if stage.Stage == stageNo {
			return stage, true
		}
	}
	return StageConfig{}, false
}

// sortedStages 返回按阶段号升序排列的副本。
func sortedStages(stages []StageConfig) []StageConfig {
	out := append([]StageConfig{}, stages...)
	sort.Slice(out, func(i, j int) bool { return out[i].Stage < out[j].Stage })
	return out
}

// checkpointResultByID 生成检查点结果索引。
func checkpointResultByID(items []CheckpointResult) map[string]CheckpointResult {
	out := map[string]CheckpointResult{}
	for _, item := range items {
		out[item.CheckpointID] = item
	}
	return out
}

// envComponentsByID 按显式稳定 ID 生成环境组件索引。
func envComponentsByID(items []EnvComponent) map[string]EnvComponent {
	out := map[string]EnvComponent{}
	for _, item := range items {
		out[strings.TrimSpace(item.ID)] = item
	}
	return out
}

// simComponentsByID 按显式稳定 ID 生成仿真组件索引。
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
	for _, ref := range sandboxes {
		if s.sandbox == nil {
			return apperr.ErrExperimentRecycleFailed
		}
		if err := s.sandbox.DestroySandbox(ctx, contracts.SandboxControlRequest{TenantID: inst.TenantID, SandboxID: ref.SandboxID.Int64(), SourceRef: inst.SourceRef}); err != nil {
			return apperr.ErrExperimentRecycleFailed.WithCause(err)
		}
	}
	for _, ref := range sims {
		if s.sim == nil {
			return apperr.ErrExperimentRecycleFailed
		}
		req := contracts.SimDestroySessionRequest{TenantID: inst.TenantID, SessionID: ref.SessionID.Int64(), SourceRef: inst.SourceRef}
		if err := s.sim.DestroySession(ctx, req); err != nil {
			return apperr.ErrExperimentRecycleFailed.WithCause(err)
		}
	}
	return nil
}
