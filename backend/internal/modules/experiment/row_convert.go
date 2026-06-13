// experiment row_convert 文件负责 sqlc 行到 M7 领域模型的纯映射。
package experiment

import (
	"chaimir/internal/modules/experiment/internal/sqlcgen"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"

	"github.com/jackc/pgx/v5/pgtype"
)

// experimentFromRow 转换实验定义行并解析组件 JSON。
func experimentFromRow(row sqlcgen.Experiment) (Experiment, error) {
	components, err := decodeComponentConfig(row.Components)
	if err != nil {
		return Experiment{}, err
	}
	groupConfig, err := decodeGroupConfig(row.GroupConfig)
	if err != nil {
		return Experiment{}, err
	}
	return Experiment{ID: row.ID, TenantID: row.TenantID, CourseID: pgtypex.Int8Value(row.CourseID), AuthorID: row.AuthorID, TemplateRef: pgtypex.TextValue(row.TemplateRef), TemplateVersion: pgtypex.TextValue(row.TemplateVersion), Name: row.Name, Description: row.Description, Components: components, CollabMode: row.CollabMode, GroupConfig: groupConfig, RequireReport: row.RequireReport, WizardStep: row.WizardStep, Status: row.Status, CreatedAt: timex.FromTimestamptz(row.CreatedAt), UpdatedAt: timex.FromTimestamptz(row.UpdatedAt)}, nil
}

// instanceFromCreateRow 转换创建实例返回行。
func instanceFromCreateRow(row sqlcgen.CreateExperimentInstanceRow) (ExperimentInstance, error) {
	return instanceFromFields(row.ID, row.TenantID, row.ExperimentID, row.OwnerAccountID, row.GroupID, row.SourceRef, row.SandboxRefs, row.SimSessionRefs, row.Status, row.Score, row.StartedAt, row.FinishedAt, row.LastActiveAt)
}

// instanceFromGetRow 转换查询实例返回行。
func instanceFromGetRow(row sqlcgen.GetExperimentInstanceRow) (ExperimentInstance, error) {
	return instanceFromFields(row.ID, row.TenantID, row.ExperimentID, row.OwnerAccountID, row.GroupID, row.SourceRef, row.SandboxRefs, row.SimSessionRefs, row.Status, row.Score, row.StartedAt, row.FinishedAt, row.LastActiveAt)
}

// instanceFromSourceRefRow 转换按 source_ref 查询实例返回行。
func instanceFromSourceRefRow(row sqlcgen.GetExperimentInstanceBySourceRefRow) (ExperimentInstance, error) {
	return instanceFromFields(row.ID, row.TenantID, row.ExperimentID, row.OwnerAccountID, row.GroupID, row.SourceRef, row.SandboxRefs, row.SimSessionRefs, row.Status, row.Score, row.StartedAt, row.FinishedAt, row.LastActiveAt)
}

// instanceFromUpdateResourcesRow 转换资源更新返回行。
func instanceFromUpdateResourcesRow(row sqlcgen.UpdateInstanceResourcesRow) (ExperimentInstance, error) {
	return instanceFromFields(row.ID, row.TenantID, row.ExperimentID, row.OwnerAccountID, row.GroupID, row.SourceRef, row.SandboxRefs, row.SimSessionRefs, row.Status, row.Score, row.StartedAt, row.FinishedAt, row.LastActiveAt)
}

// instanceFromStatusRow 转换状态更新返回行。
func instanceFromStatusRow(row sqlcgen.SetInstanceStatusRow) (ExperimentInstance, error) {
	return instanceFromFields(row.ID, row.TenantID, row.ExperimentID, row.OwnerAccountID, row.GroupID, row.SourceRef, row.SandboxRefs, row.SimSessionRefs, row.Status, row.Score, row.StartedAt, row.FinishedAt, row.LastActiveAt)
}

// instanceFromFinishRow 转换完成实例返回行。
func instanceFromFinishRow(row sqlcgen.FinishExperimentInstanceRow) (ExperimentInstance, error) {
	return instanceFromFields(row.ID, row.TenantID, row.ExperimentID, row.OwnerAccountID, row.GroupID, row.SourceRef, row.SandboxRefs, row.SimSessionRefs, row.Status, row.Score, row.StartedAt, row.FinishedAt, row.LastActiveAt)
}

// instanceFromRecycleRow 转换后台回收认领行。
func instanceFromRecycleRow(row sqlcgen.ClaimRecyclableInstancesAcrossTenantsRow) (ExperimentInstance, error) {
	return instanceFromFields(row.ID, row.TenantID, row.ExperimentID, row.OwnerAccountID, row.GroupID, row.SourceRef, row.SandboxRefs, row.SimSessionRefs, row.Status, row.Score, row.StartedAt, row.FinishedAt, row.LastActiveAt)
}

// instanceFromFields 汇总实例行字段并解析资源引用 JSON。
func instanceFromFields(id, tenantID, experimentID, ownerAccountID int64, groupID pgtype.Int8, sourceRef string, sandboxRaw, simRaw []byte, status int16, score float64, startedAt, finishedAt, lastActiveAt pgtype.Timestamptz) (ExperimentInstance, error) {
	sandboxes, err := decodeSandboxRefs(sandboxRaw)
	if err != nil {
		return ExperimentInstance{}, err
	}
	sims, err := decodeSimRefs(simRaw)
	if err != nil {
		return ExperimentInstance{}, err
	}
	return ExperimentInstance{ID: id, TenantID: tenantID, ExperimentID: experimentID, OwnerAccountID: ownerAccountID, GroupID: pgtypex.Int8Value(groupID), SourceRef: sourceRef, SandboxRefs: sandboxes, SimSessionRefs: sims, Status: status, Score: score, StartedAt: timex.FromTimestamptz(startedAt), FinishedAt: timex.FromTimestamptz(finishedAt), LastActiveAt: timex.FromTimestamptz(lastActiveAt)}, nil
}

// groupFromRows 组合小组和成员列表。
func groupFromRows(row sqlcgen.ExperimentGroup, members []sqlcgen.GroupMember) ExperimentGroup {
	out := ExperimentGroup{ID: row.ID, TenantID: row.TenantID, ExperimentID: row.ExperimentID, Name: row.Name, CreatedAt: timex.FromTimestamptz(row.CreatedAt)}
	out.Members = make([]GroupMember, 0, len(members))
	for _, member := range members {
		out.Members = append(out.Members, groupMemberFromRow(member))
	}
	return out
}

// groupMemberFromRow 转换小组成员行。
func groupMemberFromRow(row sqlcgen.GroupMember) GroupMember {
	return GroupMember{ID: row.ID, TenantID: row.TenantID, GroupID: row.GroupID, StudentID: row.StudentID, Role: row.Role, CreatedAt: timex.FromTimestamptz(row.CreatedAt)}
}

// checkpointFromRow 转换检查点结果行。
func checkpointFromRow(row sqlcgen.ListCheckpointResultsRow) CheckpointResult {
	return CheckpointResult{ID: row.ID, TenantID: row.TenantID, InstanceID: row.InstanceID, CheckpointID: row.CheckpointID, JudgeTaskRef: pgtypex.TextValue(row.JudgeTaskRef), Passed: row.Passed, Score: row.Score, DetailRef: pgtypex.TextValue(row.DetailRef), JudgedAt: timex.FromTimestamptz(row.JudgedAt)}
}

// checkpointFromUpsertRow 转换检查点 upsert 返回行。
func checkpointFromUpsertRow(row sqlcgen.UpsertCheckpointResultRow) CheckpointResult {
	return CheckpointResult{ID: row.ID, TenantID: row.TenantID, InstanceID: row.InstanceID, CheckpointID: row.CheckpointID, JudgeTaskRef: pgtypex.TextValue(row.JudgeTaskRef), Passed: row.Passed, Score: row.Score, DetailRef: pgtypex.TextValue(row.DetailRef), JudgedAt: timex.FromTimestamptz(row.JudgedAt)}
}

// checkpointFromJudgeTaskRow 转换按判题任务查询返回行。
func checkpointFromJudgeTaskRow(row sqlcgen.GetCheckpointResultByJudgeTaskRow) CheckpointResult {
	return CheckpointResult{ID: row.ID, TenantID: row.TenantID, InstanceID: row.InstanceID, CheckpointID: row.CheckpointID, JudgeTaskRef: pgtypex.TextValue(row.JudgeTaskRef), Passed: row.Passed, Score: row.Score, DetailRef: pgtypex.TextValue(row.DetailRef), JudgedAt: timex.FromTimestamptz(row.JudgedAt)}
}

// reportFromUpsertRow 转换报告提交返回行。
func reportFromUpsertRow(row sqlcgen.UpsertExperimentReportRow) ExperimentReport {
	return ExperimentReport{ID: row.ID, TenantID: row.TenantID, InstanceID: row.InstanceID, StudentID: row.StudentID, ContentRef: row.ContentRef, ManualScore: row.ManualScore, Comment: pgtypex.TextValue(row.Comment), Status: row.Status, SubmittedAt: timex.FromTimestamptz(row.SubmittedAt)}
}

// reportFromGradeRow 转换报告批改返回行。
func reportFromGradeRow(row sqlcgen.GradeExperimentReportRow) ExperimentReport {
	return ExperimentReport{ID: row.ID, TenantID: row.TenantID, InstanceID: row.InstanceID, StudentID: row.StudentID, ContentRef: row.ContentRef, ManualScore: row.ManualScore, Comment: pgtypex.TextValue(row.Comment), Status: row.Status, SubmittedAt: timex.FromTimestamptz(row.SubmittedAt)}
}

// reportFromListRow 转换报告列表返回行。
func reportFromListRow(row sqlcgen.ListExperimentReportsRow) ExperimentReport {
	return ExperimentReport{ID: row.ID, TenantID: row.TenantID, InstanceID: row.InstanceID, StudentID: row.StudentID, ContentRef: row.ContentRef, ManualScore: row.ManualScore, Comment: pgtypex.TextValue(row.Comment), Status: row.Status, SubmittedAt: timex.FromTimestamptz(row.SubmittedAt)}
}

// decodeComponentConfig 解析组件 JSON,空值按空组件处理。
func decodeComponentConfig(raw []byte) (ComponentConfig, error) {
	if len(raw) == 0 {
		return ComponentConfig{Envs: []EnvComponent{}, Sims: []SimComponent{}, Checkpoints: []CheckpointComponent{}}, nil
	}
	var out ComponentConfig
	if err := jsonx.DecodeStrict(raw, &out); err != nil {
		return ComponentConfig{}, apperr.ErrExperimentInvalid.WithCause(err)
	}
	if out.Envs == nil {
		out.Envs = []EnvComponent{}
	}
	if out.Sims == nil {
		out.Sims = []SimComponent{}
	}
	if out.Checkpoints == nil {
		out.Checkpoints = []CheckpointComponent{}
	}
	return out, nil
}

// decodeGroupConfig 解析协作配置 JSON。
func decodeGroupConfig(raw []byte) (GroupConfig, error) {
	if len(raw) == 0 {
		return GroupConfig{}, nil
	}
	var out GroupConfig
	if err := jsonx.DecodeStrict(raw, &out); err != nil {
		return GroupConfig{}, apperr.ErrExperimentGroupInvalid.WithCause(err)
	}
	return out, nil
}

// decodeSandboxRefs 解析沙箱引用 JSON。
func decodeSandboxRefs(raw []byte) ([]SandboxRef, error) {
	var out []SandboxRef
	if len(raw) == 0 {
		return []SandboxRef{}, nil
	}
	if err := jsonx.DecodeStrict(raw, &out); err != nil {
		return nil, apperr.ErrExperimentInstanceInvalid.WithCause(err)
	}
	return out, nil
}

// decodeSimRefs 解析仿真引用 JSON。
func decodeSimRefs(raw []byte) ([]SimSessionRef, error) {
	var out []SimSessionRef
	if len(raw) == 0 {
		return []SimSessionRef{}, nil
	}
	if err := jsonx.DecodeStrict(raw, &out); err != nil {
		return nil, apperr.ErrExperimentInstanceInvalid.WithCause(err)
	}
	return out, nil
}

// encodeJSON 将结构化字段序列化为 JSONB 字节。
func encodeJSON(v any, invalid *apperr.Error) ([]byte, error) {
	raw, err := jsonx.AnyBytes(v, invalid)
	if err != nil {
		return nil, err
	}
	return raw, nil
}
