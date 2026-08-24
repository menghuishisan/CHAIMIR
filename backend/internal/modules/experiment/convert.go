// experiment convert 文件负责领域模型与 HTTP/contract DTO 之间的纯转换。
package experiment

import (
	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
)

// experimentDTOFromModel 转换实验定义为 HTTP 输出。
func experimentDTOFromModel(item Experiment) ExperimentDTO {
	return ExperimentDTO{ID: ids.ID(item.ID), CourseID: ids.ID(item.CourseID), AuthorID: ids.ID(item.AuthorID), TemplateRef: item.TemplateRef, TemplateVersion: item.TemplateVersion, Name: item.Name, Description: item.Description, Components: teacherComponentConfigDTOFromModel(item.Components), CollabMode: item.CollabMode, GroupConfig: item.GroupConfig, RequireReport: item.RequireReport, WizardStep: item.WizardStep, Status: item.Status, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

// teacherComponentConfigDTOFromModel 只回显声明和摘要,不把完整执行快照暴露给教师页面。
func teacherComponentConfigDTOFromModel(cfg ComponentConfig) TeacherComponentConfigDTO {
	out := TeacherComponentConfigDTO{
		Envs:        make([]TeacherEnvComponentDTO, 0, len(cfg.Envs)),
		Sims:        append([]SimComponent(nil), cfg.Sims...),
		Checkpoints: append([]CheckpointComponent(nil), cfg.Checkpoints...),
		Stages:      append([]StageConfig(nil), cfg.Stages...),
	}
	for _, env := range cfg.Envs {
		spec := env.CompositionSnapshot.Spec
		out.Envs = append(out.Envs, TeacherEnvComponentDTO{
			ID:                       env.ID,
			Runtimes:                 append([]contracts.CompositionRuntimeRef(nil), spec.Runtimes...),
			WorkspaceRuntimeInstance: spec.WorkspaceRuntimeInstance,
			Infra:                    append([]contracts.CompositionComponentRef(nil), spec.Infra...),
			Tools:                    append([]contracts.CompositionComponentRef(nil), spec.Tools...),
			Links:                    append([]contracts.CompositionLink(nil), spec.Links...),
			AccessProfile:            spec.AccessProfile,
			CompositionDigest:        env.CompositionSnapshot.Digest,
			InitCodeRef:              env.InitCodeRef,
			InitScriptRef:            env.InitScriptRef,
			KeepAlive:                env.KeepAlive,
			SnapshotEnabled:          env.SnapshotEnabled,
			KeepAliveMinutes:         env.KeepAliveMinutes,
			SnapshotRetentionMinutes: env.SnapshotRetentionMinutes,
		})
	}
	return out
}

// studentExperimentDTOFromModel 生成不含环境初始化与判题答案的学生实验视图。
// myGroupID 为该学生在本实验所属小组(0 表示未分组或单人实验),供前端发起实例时携带。
func studentExperimentDTOFromModel(item Experiment, myGroupID int64) StudentExperimentDTO {
	components := StudentComponentConfig{
		Envs:        make([]StudentEnvComponent, 0, len(item.Components.Envs)),
		Sims:        make([]StudentSimComponent, 0, len(item.Components.Sims)),
		Checkpoints: make([]StudentCheckpointComponent, 0, len(item.Components.Checkpoints)),
		Stages:      make([]StudentStageConfig, 0, len(item.Components.Stages)),
	}
	for _, env := range item.Components.Envs {
		spec := env.CompositionSnapshot.Spec
		components.Envs = append(components.Envs, StudentEnvComponent{ID: env.ID, Runtimes: append([]contracts.CompositionRuntimeRef(nil), spec.Runtimes...), WorkspaceRuntimeInstance: spec.WorkspaceRuntimeInstance, Infra: append([]contracts.CompositionComponentRef(nil), spec.Infra...), Tools: append([]contracts.CompositionComponentRef(nil), spec.Tools...), Links: append([]contracts.CompositionLink(nil), spec.Links...), AccessProfile: spec.AccessProfile, CompositionDigest: env.CompositionSnapshot.Digest})
	}
	for _, sim := range item.Components.Sims {
		components.Sims = append(components.Sims, StudentSimComponent{ID: sim.ID, PackageCode: sim.PackageCode, Version: sim.Version})
	}
	for _, checkpoint := range item.Components.Checkpoints {
		components.Checkpoints = append(components.Checkpoints, StudentCheckpointComponent{ID: checkpoint.ID, Score: checkpoint.Score, Mode: checkpoint.Mode})
	}
	for _, stage := range item.Components.Stages {
		components.Stages = append(components.Stages, StudentStageConfig{Stage: stage.Stage, Title: stage.Title, Description: stage.Description, Components: stage.Components, UnlockCondition: stage.UnlockCondition})
	}
	return StudentExperimentDTO{ID: ids.ID(item.ID), CourseID: ids.ID(item.CourseID), Name: item.Name, Description: item.Description, Components: components, CollabMode: item.CollabMode, GroupConfig: item.GroupConfig, MyGroupID: ids.ID(myGroupID), RequireReport: item.RequireReport, Status: item.Status, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

// instanceDTOFromModel 转换实验实例为工作台输出。
func instanceDTOFromModel(item ExperimentInstance, checkpoints []CheckpointResult, stages ...[]StageDTO) InstanceDTO {
	out := InstanceDTO{ID: ids.ID(item.ID), ExperimentID: ids.ID(item.ExperimentID), OwnerAccountID: ids.ID(item.OwnerAccountID), GroupID: ids.ID(item.GroupID), SourceRef: item.SourceRef, Sandboxes: item.SandboxRefs, Sims: item.SimSessionRefs, Status: item.Status, Score: item.Score, StartedAt: item.StartedAt, FinishedAt: item.FinishedAt, LastActiveAt: item.LastActiveAt}
	out.Checkpoints = make([]CheckpointDTO, 0, len(checkpoints))
	for _, cp := range checkpoints {
		out.Checkpoints = append(out.Checkpoints, CheckpointDTO{ID: cp.CheckpointID, JudgeTaskRef: cp.JudgeTaskRef, Passed: cp.Passed, Score: cp.Score, DetailRef: cp.DetailRef, BindingOutput: cp.BindingOutput})
	}
	if len(stages) > 0 {
		out.Stages = stages[0]
	}
	return out
}

// groupDTOFromModel 转换小组和成员为 HTTP 输出。
// profiles 是经 M1 契约批量解析的账号档案;取不到时给确定性占位而不是留空,
// 让界面能解释「这个学生已离校」而不是显示一片空白(§8 文案面向用户)。
func groupDTOFromModel(item ExperimentGroup, profiles map[int64]contracts.AccountInfo) GroupDTO {
	out := GroupDTO{ID: ids.ID(item.ID), ExperimentID: ids.ID(item.ExperimentID), Name: item.Name, CreatedAt: item.CreatedAt}
	out.Members = make([]GroupMemberDTO, 0, len(item.Members))
	for _, member := range item.Members {
		profile, ok := profiles[member.StudentID]
		if !ok {
			profile = contracts.AccountInfo{Name: "已离校学生"}
		}
		out.Members = append(out.Members, GroupMemberDTO{ID: ids.ID(member.ID), GroupID: ids.ID(member.GroupID), StudentID: ids.ID(member.StudentID), StudentName: profile.Name, StudentNo: profile.No, Role: member.Role, CreatedAt: member.CreatedAt})
	}
	return out
}

// groupDTOWithSharedInstance 转换小组详情并附带当前共享实例。
func groupDTOWithSharedInstance(group ExperimentGroup, profiles map[int64]contracts.AccountInfo, inst *ExperimentInstance) GroupDTO {
	out := groupDTOFromModel(group, profiles)
	if inst != nil {
		dto := instanceDTOFromModel(*inst, nil)
		out.SharedInstance = &dto
	}
	return out
}

// reportDTOFromModel 转换报告为 HTTP 输出。
// 提交者姓名与学号经 M1 账号契约解析后随报告下发:报告表只存 student_id,
// 而批改界面必须显示是谁交的;取不到时给确定性占位而非空白。
func reportDTOFromModel(item ExperimentReport, profiles map[int64]contracts.AccountInfo) ReportDTO {
	profile, ok := profiles[item.StudentID]
	if !ok {
		profile = contracts.AccountInfo{Name: "已离校学生"}
	}
	return ReportDTO{ID: ids.ID(item.ID), InstanceID: ids.ID(item.InstanceID), StudentID: ids.ID(item.StudentID), StudentName: profile.Name, StudentNo: profile.No, ContentRef: item.ContentRef, ManualScore: item.ManualScore, Comment: item.Comment, Status: item.Status, SubmittedAt: item.SubmittedAt}
}

// sandboxRefFromContract 提取 M2 沙箱摘要中工作台需要的稳定字段。
func sandboxRefFromContract(componentID string, info contracts.SandboxInfo) SandboxRef {
	tools := make([]SandboxToolDTO, 0, len(info.ToolAccess))
	for _, tool := range info.ToolAccess {
		tools = append(tools, SandboxToolDTO{Code: tool.ToolCode, Kind: tool.Kind, Endpoint: tool.Endpoint, Status: tool.Status})
	}
	return SandboxRef{ComponentID: componentID, SandboxID: ids.ID(info.SandboxID), RuntimeCode: info.RuntimeCode, Tools: tools}
}

// simRefFromContract 提取 M4 仿真摘要中工作台需要的稳定字段。
func simRefFromContract(componentID string, info contracts.SimSessionInfo) SimSessionRef {
	return SimSessionRef{ComponentID: componentID, SessionID: ids.ID(info.SessionID), PackageCode: info.PackageCode, Version: info.Version, Compute: info.Compute}
}

// scoreSnapshotFromInstance 转换 M7 得分快照为跨模块只读契约。
func scoreSnapshotFromInstance(item ExperimentInstance) contracts.ExperimentScoreSnapshot {
	return contracts.ExperimentScoreSnapshot{TenantID: item.TenantID, ExperimentID: item.ExperimentID, InstanceID: item.ID, StudentID: item.OwnerAccountID, Score: item.Score}
}
