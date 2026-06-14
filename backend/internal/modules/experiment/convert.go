// experiment convert 文件负责领域模型与 HTTP/contract DTO 之间的纯转换。
package experiment

import "chaimir/internal/contracts"

// experimentDTOFromModel 转换实验定义为 HTTP 输出。
func experimentDTOFromModel(item Experiment) ExperimentDTO {
	return ExperimentDTO{ID: item.ID, CourseID: item.CourseID, AuthorID: item.AuthorID, TemplateRef: item.TemplateRef, TemplateVersion: item.TemplateVersion, Name: item.Name, Description: item.Description, Components: item.Components, CollabMode: item.CollabMode, GroupConfig: item.GroupConfig, RequireReport: item.RequireReport, WizardStep: item.WizardStep, Status: item.Status, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

// instanceDTOFromModel 转换实验实例为工作台输出。
func instanceDTOFromModel(item ExperimentInstance, checkpoints []CheckpointResult, stages ...[]StageDTO) InstanceDTO {
	out := InstanceDTO{ID: item.ID, ExperimentID: item.ExperimentID, OwnerAccountID: item.OwnerAccountID, GroupID: item.GroupID, SourceRef: item.SourceRef, Sandboxes: item.SandboxRefs, Sims: item.SimSessionRefs, Status: item.Status, Score: item.Score, StartedAt: item.StartedAt, FinishedAt: item.FinishedAt, LastActiveAt: item.LastActiveAt}
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
func groupDTOFromModel(item ExperimentGroup) GroupDTO {
	out := GroupDTO{ID: item.ID, ExperimentID: item.ExperimentID, Name: item.Name, CreatedAt: item.CreatedAt}
	out.Members = make([]GroupMemberDTO, 0, len(item.Members))
	for _, member := range item.Members {
		out.Members = append(out.Members, GroupMemberDTO{ID: member.ID, GroupID: member.GroupID, StudentID: member.StudentID, Role: member.Role, CreatedAt: member.CreatedAt})
	}
	return out
}

// groupDTOWithSharedInstance 转换小组详情并附带当前共享实例。
func groupDTOWithSharedInstance(group ExperimentGroup, inst *ExperimentInstance) GroupDTO {
	out := groupDTOFromModel(group)
	if inst != nil {
		dto := instanceDTOFromModel(*inst, nil)
		out.SharedInstance = &dto
	}
	return out
}

// reportDTOFromModel 转换报告为 HTTP 输出。
func reportDTOFromModel(item ExperimentReport) ReportDTO {
	return ReportDTO{ID: item.ID, InstanceID: item.InstanceID, StudentID: item.StudentID, ContentRef: item.ContentRef, ManualScore: item.ManualScore, Comment: item.Comment, Status: item.Status, SubmittedAt: item.SubmittedAt}
}

// sandboxRefFromContract 提取 M2 沙箱摘要中工作台需要的稳定字段。
func sandboxRefFromContract(componentID string, info contracts.SandboxInfo) SandboxRef {
	tools := make([]SandboxToolDTO, 0, len(info.ToolAccess))
	for _, tool := range info.ToolAccess {
		tools = append(tools, SandboxToolDTO{Code: tool.ToolCode, Kind: tool.Kind, Endpoint: tool.Endpoint, Status: tool.Status})
	}
	return SandboxRef{ComponentID: componentID, SandboxID: info.SandboxID, RuntimeCode: info.RuntimeCode, Tools: tools, Meta: map[string]string{"namespace": info.Namespace}}
}

// simRefFromContract 提取 M4 仿真摘要中工作台需要的稳定字段。
func simRefFromContract(componentID string, info contracts.SimSessionInfo) SimSessionRef {
	return SimSessionRef{ComponentID: componentID, SessionID: info.SessionID, PackageCode: info.PackageCode, Version: info.Version, BundleRef: info.BundleRef}
}

// scoreSnapshotFromInstance 转换 M7 得分快照为跨模块只读契约。
func scoreSnapshotFromInstance(item ExperimentInstance) contracts.ExperimentScoreSnapshot {
	return contracts.ExperimentScoreSnapshot{TenantID: item.TenantID, ExperimentID: item.ExperimentID, InstanceID: item.ID, StudentID: item.OwnerAccountID, Score: item.Score}
}
