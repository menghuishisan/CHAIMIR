// M7 行转换层:集中处理 sqlc 行到领域投影和响应 DTO 的纯转换。
package experiment

import (
	"chaimir/internal/modules/experiment/internal/sqlcgen"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
)

// experimentDTOFromRow 转换实验定义行。
func experimentDTOFromRow(row sqlcgen.Experiment) ExperimentDTO {
	return ExperimentDTO{
		ID: ids.Format(row.ID), TenantID: ids.Format(row.TenantID), CourseID: pgtypex.IDString(row.CourseID), AuthorID: ids.Format(row.AuthorID),
		TemplateRef: pgtypex.TextValue(row.TemplateRef), TemplateVersion: pgtypex.TextValue(row.TemplateVersion), Name: row.Name,
		Description: row.Description, Components: componentsValue(row.Components), CollabMode: row.CollabMode,
		GroupConfig: jsonx.ObjectMap(row.GroupConfig), RequireReport: row.RequireReport, WizardStep: row.WizardStep,
		Status: row.Status, CreatedAt: timex.FromTimestamptz(row.CreatedAt), UpdatedAt: timex.FromTimestamptz(row.UpdatedAt),
	}
}

// experimentInstanceDTOFromRow 转换实验实例行。
func experimentInstanceDTOFromRow(row sqlcgen.ExperimentInstance) ExperimentInstanceDTO {
	return ExperimentInstanceDTO{
		ID: ids.Format(row.ID), TenantID: ids.Format(row.TenantID), ExperimentID: ids.Format(row.ExperimentID),
		OwnerAccountID: ids.Format(row.OwnerAccountID), GroupID: pgtypex.IDString(row.GroupID), SourceRef: row.SourceRef,
		Sandboxes: sandboxRefsValue(row.SandboxRefs), Sims: simRefsValue(row.SimSessionRefs),
		Status: row.Status, Score: pgtypex.NumericPtrValue(row.Score), StartedAt: timex.FromTimestamptz(row.StartedAt),
		FinishedAt: timex.FromTimestamptz(row.FinishedAt), LastActiveAt: timex.FromTimestamptz(row.LastActiveAt),
	}
}

// checkpointResultDTOFromRow 转换检查点结果行。
func checkpointResultDTOFromRow(row sqlcgen.CheckpointResult) CheckpointResultDTO {
	return CheckpointResultDTO{
		ID: ids.Format(row.ID), TenantID: row.TenantID, InstanceID: row.InstanceID, CheckpointID: row.CheckpointID,
		JudgeTaskRef: pgtypex.TextValue(row.JudgeTaskRef), Passed: row.Passed, Score: pgtypex.NumericValue(row.Score), DetailRef: pgtypex.TextValue(row.DetailRef),
	}
}

// reportDTOFromRow 转换实验报告行。
func reportDTOFromRow(row sqlcgen.ExperimentReport) ReportDTO {
	at := timex.FromTimestamptz(row.SubmittedAt)
	return ReportDTO{
		ID: ids.Format(row.ID), InstanceID: ids.Format(row.InstanceID), StudentID: ids.Format(row.StudentID), ContentRef: row.ContentRef,
		ManualScore: pgtypex.NumericPtrValue(row.ManualScore), Comment: pgtypex.TextValue(row.Comment), Status: row.Status, SubmittedAt: &at,
	}
}

// reportDTOFromAuthorizedRow 转换带授权标记的报告批改返回行。
func reportDTOFromAuthorizedRow(row sqlcgen.GradeExperimentReportAuthorizedRow) ReportDTO {
	at := timex.FromTimestamptz(row.SubmittedAt)
	return ReportDTO{
		ID: ids.Format(row.ID), InstanceID: ids.Format(row.InstanceID), StudentID: ids.Format(row.StudentID), ContentRef: row.ContentRef,
		ManualScore: pgtypex.NumericPtrValue(row.ManualScore), Comment: pgtypex.TextValue(row.Comment), Status: row.Status, SubmittedAt: &at,
	}
}

// groupDTOFromRows 转换协作小组与成员行。
func groupDTOFromRows(group sqlcgen.ExperimentGroup, members []sqlcgen.GroupMember) GroupDTO {
	out := GroupDTO{ID: ids.Format(group.ID), ExperimentID: ids.Format(group.ExperimentID), Name: group.Name, Members: make([]GroupMemberDTO, 0, len(members))}
	for _, member := range members {
		out.Members = append(out.Members, groupMemberDTOFromRow(member))
	}
	return out
}

// groupMemberDTOFromRow 转换协作小组成员行。
func groupMemberDTOFromRow(row sqlcgen.GroupMember) GroupMemberDTO {
	return GroupMemberDTO{ID: ids.Format(row.ID), GroupID: ids.Format(row.GroupID), StudentID: ids.Format(row.StudentID), Role: row.Role}
}

// groupMemberDTOFromAuthorizedRow 转换带授权标记的小组成员写入返回行。
func groupMemberDTOFromAuthorizedRow(row sqlcgen.AddGroupMemberAuthorizedRow) GroupMemberDTO {
	return GroupMemberDTO{ID: ids.Format(row.ID), GroupID: ids.Format(row.GroupID), StudentID: ids.Format(row.StudentID), Role: row.Role}
}
