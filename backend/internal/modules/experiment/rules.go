// experiment rules 文件集中放置 M7 输入校验、状态机和组件编排规则。
package experiment

import (
	"fmt"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
)

const maxCheckpointBindingOutputBytes = 16 * 1024

// validateExperimentRequest 校验教师向导草稿的边界字段和组件结构。
func validateExperimentRequest(req ExperimentRequest) (ExperimentRequest, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	req.TemplateRef = strings.TrimSpace(req.TemplateRef)
	req.TemplateVersion = strings.TrimSpace(req.TemplateVersion)
	if req.Name == "" || len(req.Name) > 255 {
		return ExperimentRequest{}, apperr.ErrExperimentInvalid
	}
	if req.CourseID < 0 || req.WizardStep < 1 || req.WizardStep > 6 {
		return ExperimentRequest{}, apperr.ErrExperimentInvalid
	}
	if req.CollabMode == 0 {
		req.CollabMode = CollabModeSolo
	}
	if req.CollabMode != CollabModeSolo && req.CollabMode != CollabModeGroup {
		return ExperimentRequest{}, apperr.ErrExperimentInvalid
	}
	if req.Components.Envs == nil {
		req.Components.Envs = []EnvComponent{}
	}
	if req.Components.Sims == nil {
		req.Components.Sims = []SimComponent{}
	}
	if req.Components.Checkpoints == nil {
		req.Components.Checkpoints = []CheckpointComponent{}
	}
	if req.Components.Stages == nil {
		req.Components.Stages = []StageConfig{}
	}
	if err := validateComponentConfig(req.Components, req.CollabMode, req.GroupConfig); err != nil {
		return ExperimentRequest{}, err
	}
	return req, nil
}

// validateComponentConfig 校验自由组合组件的引用完整性和分值边界。
func validateComponentConfig(cfg ComponentConfig, collabMode int16, group GroupConfig) error {
	ids := map[string]bool{}
	envIDs := map[string]bool{}
	simIDs := map[string]bool{}
	for idx, env := range cfg.Envs {
		id := componentID(env.ID, "env", idx)
		if ids[id] || strings.TrimSpace(env.RuntimeCode) == "" {
			return apperr.ErrExperimentInvalid
		}
		if err := validateEnvComponentSandboxContract(env); err != nil {
			return err
		}
		ids[id] = true
		envIDs[id] = true
	}
	for idx, sim := range cfg.Sims {
		id := componentID(sim.ID, "sim", idx)
		if ids[id] || strings.TrimSpace(sim.PackageCode) == "" || strings.TrimSpace(sim.Version) == "" {
			return apperr.ErrExperimentInvalid
		}
		ids[id] = true
		simIDs[id] = true
	}
	checkpointIDs := map[string]bool{}
	for _, cp := range cfg.Checkpoints {
		if strings.TrimSpace(cp.ID) == "" || checkpointIDs[cp.ID] || strings.TrimSpace(cp.ItemCode) == "" || strings.TrimSpace(cp.ItemVersion) == "" || strings.TrimSpace(cp.JudgerCode) == "" || cp.Score <= 0 {
			return apperr.ErrExperimentCheckpointInvalid
		}
		if cp.Mode != "" && cp.Mode != contracts.JudgeSandboxModeFresh && cp.Mode != contracts.JudgeSandboxModeReuse {
			return apperr.ErrExperimentCheckpointInvalid
		}
		if cp.EnvID != "" && !envIDs[cp.EnvID] {
			return apperr.ErrExperimentCheckpointInvalid
		}
		if cp.SimID != "" && !simIDs[cp.SimID] {
			return apperr.ErrExperimentCheckpointInvalid
		}
		checkpointIDs[cp.ID] = true
	}
	if err := validateStageConfig(cfg, envIDs, simIDs, checkpointIDs); err != nil {
		return err
	}
	if collabMode == CollabModeGroup {
		if group.Size < 2 || len(group.Roles) == 0 {
			return apperr.ErrExperimentGroupInvalid
		}
	}
	return nil
}

// validateEnvComponentSandboxContract 在 M7 输入边界落实 M2 沙箱配置合同,避免保存无法启动的实验定义。
func validateEnvComponentSandboxContract(env EnvComponent) error {
	if env.KeepAlive {
		if env.KeepAliveMinutes <= 0 {
			return apperr.ErrExperimentInvalid
		}
	} else if env.KeepAliveMinutes != 0 {
		return apperr.ErrExperimentInvalid
	}
	if env.SnapshotEnabled {
		if env.SnapshotRetentionMinutes <= 0 {
			return apperr.ErrExperimentInvalid
		}
	} else if env.SnapshotRetentionMinutes != 0 {
		return apperr.ErrExperimentInvalid
	}
	return nil
}

// validateStageConfig 校验阶段编排引用完整性、解锁条件和参数绑定来源。
func validateStageConfig(cfg ComponentConfig, envIDs, simIDs, checkpointIDs map[string]bool) error {
	if len(cfg.Stages) == 0 {
		return nil
	}
	stageIDs := map[int32]bool{}
	stagedEnvs := map[string]bool{}
	stagedSims := map[string]bool{}
	for _, stage := range cfg.Stages {
		if stage.Stage <= 0 || stageIDs[stage.Stage] || strings.TrimSpace(stage.Title) == "" || len(stage.Title) > 128 || (len(stage.Components.Envs) == 0 && len(stage.Components.Sims) == 0) {
			return apperr.ErrExperimentInvalid
		}
		stageIDs[stage.Stage] = true
		for _, envID := range stage.Components.Envs {
			id := strings.TrimSpace(envID)
			if !envIDs[id] || stagedEnvs[id] {
				return apperr.ErrExperimentInvalid
			}
			stagedEnvs[id] = true
		}
		for _, simID := range stage.Components.Sims {
			id := strings.TrimSpace(simID)
			if !simIDs[id] || stagedSims[id] {
				return apperr.ErrExperimentInvalid
			}
			stagedSims[id] = true
		}
		if err := validateUnlockCondition(stage.UnlockCondition, checkpointIDs); err != nil {
			return err
		}
		for _, binding := range stage.ParamBindings {
			if err := validateParamBinding(binding, simIDs, checkpointIDs); err != nil {
				return err
			}
		}
	}
	if len(stagedEnvs) != len(envIDs) || len(stagedSims) != len(simIDs) {
		return apperr.ErrExperimentInvalid
	}
	return nil
}

// validateUnlockCondition 校验阶段解锁来源。
func validateUnlockCondition(cond *UnlockCondition, checkpointIDs map[string]bool) error {
	if cond == nil {
		return nil
	}
	switch strings.TrimSpace(cond.Type) {
	case "manual":
		return nil
	case "checkpoint":
		if !checkpointIDs[strings.TrimSpace(cond.CheckpointID)] || cond.MinScore < 0 {
			return apperr.ErrExperimentCheckpointInvalid
		}
		return nil
	default:
		return apperr.ErrExperimentInvalid
	}
}

// validateParamBinding 校验参数绑定只写入后续仿真 init_params。
func validateParamBinding(binding ParamBinding, simIDs, checkpointIDs map[string]bool) error {
	if !simIDs[strings.TrimSpace(binding.TargetComponent)] || strings.TrimSpace(binding.TargetParam) == "" {
		return apperr.ErrExperimentInvalid
	}
	switch strings.TrimSpace(binding.SourceType) {
	case "constant":
		return nil
	case "checkpoint":
		if !checkpointIDs[strings.TrimSpace(binding.SourceRef)] {
			return apperr.ErrExperimentCheckpointInvalid
		}
		return nil
	default:
		return apperr.ErrExperimentInvalid
	}
}

// normalizeBindingOutput 校验检查点绑定输出只能以小型 JSON 对象参与后续阶段参数注入。
func normalizeBindingOutput(input map[string]any) (map[string]any, error) {
	if input == nil {
		return map[string]any{}, nil
	}
	raw, err := jsonx.ObjectBytes(input, apperr.ErrExperimentCheckpointInvalid)
	if err != nil {
		return nil, err
	}
	if len(raw) > maxCheckpointBindingOutputBytes {
		return nil, apperr.ErrExperimentCheckpointInvalid
	}
	out, err := jsonx.ObjectMapStrict(raw)
	if err != nil {
		return nil, apperr.ErrExperimentCheckpointInvalid.WithCause(err)
	}
	return out, nil
}

// validatePublishResult 将发布前校验结果转换为是否允许发布。
func validatePublishResult(result ValidationResultDTO) error {
	for _, issue := range result.Issues {
		if issue.Level == ValidationLevelError {
			return apperr.ErrExperimentDependencyInvalid
		}
	}
	return nil
}

// validateInstanceStart 校验发起实例的定义状态与小组参数。
func validateInstanceStart(exp Experiment, groupID int64) error {
	if exp.Status != ExperimentStatusPublished {
		return apperr.ErrExperimentStateInvalid
	}
	if exp.CollabMode == CollabModeSolo && groupID != 0 {
		return apperr.ErrExperimentGroupInvalid
	}
	if exp.CollabMode == CollabModeGroup && groupID <= 0 {
		return apperr.ErrExperimentGroupInvalid
	}
	return nil
}

// canAccessInstance 判断当前账号是否可访问单人或小组实例。
func canAccessInstance(accountID int64, item ExperimentInstance, members []GroupMember) bool {
	if item.GroupID == 0 {
		return item.OwnerAccountID == accountID
	}
	for _, member := range members {
		if member.StudentID == accountID {
			return true
		}
	}
	return false
}

// ensureTeacherCanManage 校验教师作者或学校管理员对实验定义的管理边界。
func ensureTeacherCanManage(accountID int64, isSchoolAdmin bool, item Experiment) error {
	if isSchoolAdmin || item.AuthorID == accountID {
		return nil
	}
	return apperr.ErrForbidden
}

// validateInstanceTransition 校验实例状态机的单步操作。
func validateInstanceTransition(current, next int16) error {
	switch next {
	case InstanceStatusPaused:
		if current == InstanceStatusRunning {
			return nil
		}
	case InstanceStatusRunning:
		if current == InstanceStatusPaused || current == InstanceStatusReleased || current == InstanceStatusCreating {
			return nil
		}
	case InstanceStatusFinished:
		if current == InstanceStatusRunning || current == InstanceStatusPaused || current == InstanceStatusReleased {
			return nil
		}
	case InstanceStatusRecycled:
		if current != InstanceStatusRecycled {
			return nil
		}
	case InstanceStatusError:
		return nil
	case InstanceStatusReleased:
		if current == InstanceStatusRunning || current == InstanceStatusPaused || current == InstanceStatusCreating {
			return nil
		}
	}
	return apperr.ErrExperimentInstanceStateInvalid
}

// checkpointByID 在组件配置中查找检查点定义。
func checkpointByID(exp Experiment, checkpointID string) (CheckpointComponent, bool) {
	for _, cp := range exp.Components.Checkpoints {
		if cp.ID == strings.TrimSpace(checkpointID) {
			return cp, true
		}
	}
	return CheckpointComponent{}, false
}

// validateManualScore 校验教师批改分处在平台统一百分制范围内。
func validateManualScore(score float64) error {
	if score < 0 || score > 100 {
		return apperr.ErrExperimentScoreInvalid
	}
	return nil
}

// sourceRefForInstance 按全局 source_ref 规范生成实验实例来源引用。
func sourceRefForInstance(id int64, now time.Time) string {
	return fmt.Sprintf("experiment:%04d:instance:%d", now.Year(), id)
}

// componentID 返回显式组件 ID 或稳定派生 ID,避免存储空组件键。
func componentID(raw, prefix string, idx int) string {
	if strings.TrimSpace(raw) != "" {
		return strings.TrimSpace(raw)
	}
	return fmt.Sprintf("%s%d", prefix, idx+1)
}

// validExperimentSourceRef 校验事件来源确属 M7 实例。
func validExperimentSourceRef(sourceRef string) bool {
	return auth.ValidSourceRef(sourceRef) && strings.HasPrefix(strings.TrimSpace(sourceRef), "experiment:")
}
