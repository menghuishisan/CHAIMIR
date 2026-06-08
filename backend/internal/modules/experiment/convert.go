// M7 转换工具:在 sqlc 行、HTTP DTO、contracts DTO 与 PostgreSQL 类型之间隔离转换细节。
package experiment

import (
	"strconv"
	"strings"

	"chaimir/internal/modules/experiment/internal/sqlcgen"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"

	"github.com/jackc/pgx/v5/pgtype"
)

// experimentDTOFromRow 转换实验定义行。
func experimentDTOFromRow(row sqlcgen.Experiment) ExperimentDTO {
	return ExperimentDTO{
		ID: ids.Format(row.ID), TenantID: ids.Format(row.TenantID), CourseID: optionalID(row.CourseID), AuthorID: ids.Format(row.AuthorID),
		TemplateRef: textValue(row.TemplateRef), TemplateVersion: textValue(row.TemplateVersion), Name: row.Name,
		Description: row.Description, Components: componentsValue(row.Components), CollabMode: row.CollabMode,
		GroupConfig: jsonx.ObjectMap(row.GroupConfig), RequireReport: row.RequireReport, WizardStep: row.WizardStep,
		Status: row.Status, CreatedAt: timex.FromTimestamptz(row.CreatedAt), UpdatedAt: timex.FromTimestamptz(row.UpdatedAt),
	}
}

// experimentInstanceDTOFromRow 转换实验实例行。
func experimentInstanceDTOFromRow(row sqlcgen.ExperimentInstance) ExperimentInstanceDTO {
	return ExperimentInstanceDTO{
		ID: ids.Format(row.ID), TenantID: ids.Format(row.TenantID), ExperimentID: ids.Format(row.ExperimentID),
		OwnerAccountID: ids.Format(row.OwnerAccountID), GroupID: optionalID(row.GroupID), SourceRef: row.SourceRef,
		Sandboxes: sandboxRefsValue(row.SandboxRefs), Sims: simRefsValue(row.SimSessionRefs),
		Status: row.Status, Score: numericPtr(row.Score), StartedAt: timex.FromTimestamptz(row.StartedAt),
		FinishedAt: timex.FromTimestamptz(row.FinishedAt), LastActiveAt: timex.FromTimestamptz(row.LastActiveAt),
	}
}

// checkpointResultDTOFromRow 转换检查点结果行。
func checkpointResultDTOFromRow(row sqlcgen.CheckpointResult) CheckpointResultDTO {
	return CheckpointResultDTO{
		ID: ids.Format(row.ID), TenantID: row.TenantID, InstanceID: row.InstanceID, CheckpointID: row.CheckpointID,
		JudgeTaskRef: textValue(row.JudgeTaskRef), Passed: row.Passed, Score: numericValue(row.Score), DetailRef: textValue(row.DetailRef),
	}
}

// reportDTOFromRow 转换实验报告行。
func reportDTOFromRow(row sqlcgen.ExperimentReport) ReportDTO {
	at := timex.FromTimestamptz(row.SubmittedAt)
	return ReportDTO{
		ID: ids.Format(row.ID), InstanceID: ids.Format(row.InstanceID), StudentID: ids.Format(row.StudentID), ContentRef: row.ContentRef,
		ManualScore: numericPtr(row.ManualScore), Comment: textValue(row.Comment), Status: row.Status, SubmittedAt: &at,
	}
}

// reportDTOFromAuthorizedRow 转换带授权标记的报告批改返回行。
func reportDTOFromAuthorizedRow(row sqlcgen.GradeExperimentReportAuthorizedRow) ReportDTO {
	at := timex.FromTimestamptz(row.SubmittedAt)
	return ReportDTO{
		ID: ids.Format(row.ID), InstanceID: ids.Format(row.InstanceID), StudentID: ids.Format(row.StudentID), ContentRef: row.ContentRef,
		ManualScore: numericPtr(row.ManualScore), Comment: textValue(row.Comment), Status: row.Status, SubmittedAt: &at,
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

// componentsBytes 序列化组件编排 JSONB。
func componentsBytes(v ExperimentComponents) ([]byte, error) {
	return jsonx.AnyBytes(v, apperr.ErrExperimentInvalid)
}

// componentsValue 解析组件编排 JSONB。
func componentsValue(data []byte) ExperimentComponents {
	return jsonx.Decode(data, ExperimentComponents{})
}

// sandboxRefsBytes 序列化沙箱引用列表。
func sandboxRefsBytes(refs []SandboxRef) ([]byte, error) {
	if refs == nil {
		refs = []SandboxRef{}
	}
	return jsonx.AnyBytes(refs, apperr.ErrExperimentInstanceInvalid)
}

// sandboxRefsValue 解析沙箱引用列表。
func sandboxRefsValue(data []byte) []SandboxRef {
	return jsonx.Decode(data, []SandboxRef{})
}

// simRefsBytes 序列化仿真会话引用列表。
func simRefsBytes(refs []SimSessionRef) ([]byte, error) {
	if refs == nil {
		refs = []SimSessionRef{}
	}
	return jsonx.AnyBytes(refs, apperr.ErrExperimentInstanceInvalid)
}

// simRefsValue 解析仿真会话引用列表。
func simRefsValue(data []byte) []SimSessionRef {
	return jsonx.Decode(data, []SimSessionRef{})
}

// pgNumeric 把分值转换为 PostgreSQL Numeric。
func pgNumeric(v float64) (pgtype.Numeric, error) {
	var n pgtype.Numeric
	if err := n.Scan(strconv.FormatFloat(v, 'f', 2, 64)); err != nil {
		return pgtype.Numeric{}, err
	}
	return n, nil
}

// numericValue 读取 Numeric 值。
func numericValue(v pgtype.Numeric) float64 {
	f, err := v.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return f.Float64
}

// numericPtr 读取可空 Numeric 指针。
func numericPtr(v pgtype.Numeric) *float64 {
	f, err := v.Float64Value()
	if err != nil || !f.Valid {
		return nil
	}
	return &f.Float64
}

// pgText 构造可空文本。
func pgText(v string) pgtype.Text {
	return pgtype.Text{String: v, Valid: strings.TrimSpace(v) != ""}
}

// pgInt8 构造可空 int8。
func pgInt8(v int64) pgtype.Int8 {
	return pgtype.Int8{Int64: v, Valid: v > 0}
}

// pgInt8Filter 构造可选 int8 过滤条件。
func pgInt8Filter(v int64) pgtype.Int8 {
	return pgtype.Int8{Int64: v, Valid: v > 0}
}

// pgInt2Filter 构造可选 smallint 过滤条件。
func pgInt2Filter(v int16) pgtype.Int2 {
	return pgtype.Int2{Int16: v, Valid: v > 0}
}

// textValue 读取可空文本。
func textValue(v pgtype.Text) string {
	if !v.Valid {
		return ""
	}
	return v.String
}

// optionalID 转换可空 ID。
func optionalID(v pgtype.Int8) string {
	if !v.Valid {
		return ""
	}
	return ids.Format(v.Int64)
}
