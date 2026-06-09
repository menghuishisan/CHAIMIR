// M11 行转换层:集中处理 sqlc 行到领域投影和响应 DTO 的纯转换。
package grade

import (
	"chaimir/internal/modules/grade/internal/sqlcgen"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
)

// levelDTOFromRow 转换等级配置行。
func levelDTOFromRow(row sqlcgen.GradeLevelConfig) LevelConfigDTO {
	return LevelConfigDTO{
		ID: ids.Format(row.ID), TenantID: ids.Format(row.TenantID), Name: row.Name, IsDefault: row.IsDefault,
		Mapping:      jsonx.Decode(row.Mapping, []LevelMappingDTO{}),
		WarningRules: jsonx.Decode(row.WarningRules, WarningRuleDTO{}),
	}
}

// semesterDTOFromRow 转换学期行。
func semesterDTOFromRow(row sqlcgen.Semester) SemesterDTO {
	return SemesterDTO{ID: ids.Format(row.ID), TenantID: ids.Format(row.TenantID), Name: row.Name, StartDate: pgtypex.DateValue(row.StartDate), EndDate: pgtypex.DateValue(row.EndDate), IsCurrent: row.IsCurrent}
}

// reviewDTOFromRow 转换审核行。
func reviewDTOFromRow(row sqlcgen.GradeReview) ReviewDTO {
	return ReviewDTO{
		ID: ids.Format(row.ID), TenantID: ids.Format(row.TenantID), CourseID: ids.Format(row.CourseID),
		SemesterID: pgtypex.IDString(row.SemesterID), SubmitterID: ids.Format(row.SubmitterID), ReviewerID: pgtypex.IDString(row.ReviewerID), Status: row.Status,
		IsLocked: row.IsLocked, Comment: pgtypex.TextValue(row.Comment), SubmittedAt: timex.FromTimestamptz(row.SubmittedAt), ReviewedAt: timex.PtrFromTimestamptz(row.ReviewedAt),
	}
}

// semesterGradeDTOFromRow 转换学期 GPA 行。
func semesterGradeDTOFromRow(row sqlcgen.StudentSemesterGrade) SemesterGradeDTO {
	return SemesterGradeDTO{
		ID: ids.Format(row.ID), TenantID: ids.Format(row.TenantID), StudentID: ids.Format(row.StudentID), SemesterID: ids.Format(row.SemesterID),
		TotalCredits: pgtypex.NumericValue(row.TotalCredits), GPA: pgtypex.NumericValue(row.Gpa), CumulativeGPA: pgtypex.NumericValue(row.CumulativeGpa), ComputedAt: timex.FromTimestamptz(row.ComputedAt),
	}
}

// appealDTOFromRow 转换申诉行。
func appealDTOFromRow(row sqlcgen.GradeAppeal) AppealDTO {
	return AppealDTO{
		ID: ids.Format(row.ID), TenantID: ids.Format(row.TenantID), StudentID: ids.Format(row.StudentID), CourseID: ids.Format(row.CourseID),
		Reason: row.Reason, Status: row.Status, HandlerID: pgtypex.IDString(row.HandlerID), ResultComment: pgtypex.TextValue(row.ResultComment),
		CreatedAt: timex.FromTimestamptz(row.CreatedAt), HandledAt: timex.PtrFromTimestamptz(row.HandledAt),
	}
}

// warningDTOFromRow 转换预警行。
func warningDTOFromRow(row sqlcgen.AcademicWarning) WarningDTO {
	return WarningDTO{
		ID: ids.Format(row.ID), TenantID: ids.Format(row.TenantID), StudentID: ids.Format(row.StudentID), SemesterID: ids.Format(row.SemesterID),
		Type: row.Type, Detail: jsonx.Decode(row.Detail, map[string]any{}), Status: row.Status, CreatedAt: timex.FromTimestamptz(row.CreatedAt),
	}
}

// transcriptDTOFromRow 转换成绩单行。
func transcriptDTOFromRow(row sqlcgen.TranscriptRecord) TranscriptDTO {
	return TranscriptDTO{
		ID: ids.Format(row.ID), TenantID: ids.Format(row.TenantID), StudentID: ids.Format(row.StudentID), Scope: row.Scope,
		SemesterID: pgtypex.IDString(row.SemesterID), PDFRef: row.PdfRef, GeneratedAt: timex.FromTimestamptz(row.GeneratedAt),
	}
}
