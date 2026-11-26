// 把 M11 查询结果转换为领域响应。
package grade

import (
	"chaimir/internal/modules/grade/internal/sqlcgen"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
)

func levelConfigDTO(row sqlcgen.GradeLevelConfig) (LevelConfigDTO, error) {
	mapping := []LevelRule{}
	if err := jsonx.DecodeStrictKnownFields(row.Mapping, &mapping); err != nil {
		return LevelConfigDTO{}, apperr.ErrGradeConfigInvalid.WithCause(err)
	}
	warningRules := WarningRules{}
	if err := jsonx.DecodeStrictKnownFields(row.WarningRules, &warningRules); err != nil {
		return LevelConfigDTO{}, apperr.ErrGradeConfigInvalid.WithCause(err)
	}
	return LevelConfigDTO{ID: ids.ID(row.ID), TenantID: ids.ID(row.TenantID), Name: row.Name, Mapping: mapping, WarningRules: warningRules, IsDefault: row.IsDefault, CreatedAt: timex.RFC3339OrEmpty(timex.FromTimestamptz(row.CreatedAt)), UpdatedAt: timex.RFC3339OrEmpty(timex.FromTimestamptz(row.UpdatedAt))}, nil
}

func semesterDTO(row sqlcgen.Semester) SemesterDTO {
	return SemesterDTO{ID: ids.ID(row.ID), TenantID: ids.ID(row.TenantID), Name: row.Name, StartDate: timex.DateOrEmpty(pgtypex.DateValue(row.StartDate)), EndDate: timex.DateOrEmpty(pgtypex.DateValue(row.EndDate)), IsCurrent: row.IsCurrent}
}

func reviewDTO(row sqlcgen.GradeReview) ReviewDTO {
	return ReviewDTO{ID: ids.ID(row.ID), TenantID: ids.ID(row.TenantID), CourseID: ids.ID(row.CourseID), SemesterID: ids.ID(pgtypex.Int8Value(row.SemesterID)), SubmitterID: ids.ID(row.SubmitterID), ReviewerID: ids.ID(pgtypex.Int8Value(row.ReviewerID)), Status: row.Status, IsLocked: row.IsLocked, Comment: pgtypex.TextValue(row.Comment), SubmittedAt: timex.RFC3339OrEmpty(timex.FromTimestamptz(row.SubmittedAt)), ReviewedAt: timex.RFC3339OrEmpty(timex.FromTimestamptz(row.ReviewedAt))}
}

func gradeLockOutbox(row sqlcgen.GradeLockOutbox) GradeLockOutbox {
	return GradeLockOutbox{ID: row.ID, Revision: row.Revision, ChangedAt: timex.FromTimestamptz(row.CreatedAt), TenantID: row.TenantID, ReviewID: row.ReviewID, CourseID: row.CourseID, Locked: row.Locked, Reason: row.Reason, TraceID: row.TraceID, Status: row.Status, RetryCount: row.RetryCount, LastError: pgtypex.TextValue(row.LastError), CreatedAt: timex.RFC3339OrEmpty(timex.FromTimestamptz(row.CreatedAt)), UpdatedAt: timex.RFC3339OrEmpty(timex.FromTimestamptz(row.UpdatedAt)), LeaseToken: row.LeaseToken, LeaseUntil: timex.RFC3339OrEmpty(timex.FromTimestamptz(row.LeaseUntil))}
}

func semesterGradeSummary(row sqlcgen.StudentSemesterGrade) (GradeSummaryDTO, error) {
	totalCredits, err := pgtypex.NumericValue(row.TotalCredits)
	if err != nil {
		return GradeSummaryDTO{}, apperr.ErrGradeAggregationFailed.WithCause(err)
	}
	gpa, err := pgtypex.NumericValue(row.Gpa)
	if err != nil {
		return GradeSummaryDTO{}, apperr.ErrGradeAggregationFailed.WithCause(err)
	}
	cumulativeGPA, err := pgtypex.NumericValue(row.CumulativeGpa)
	if err != nil {
		return GradeSummaryDTO{}, apperr.ErrGradeAggregationFailed.WithCause(err)
	}
	return GradeSummaryDTO{StudentID: ids.ID(row.StudentID), SemesterID: ids.ID(row.SemesterID), TotalCredits: totalCredits, GPA: gpa, CumulativeGPA: cumulativeGPA, ComputedAt: timex.FromTimestamptz(row.ComputedAt)}, nil
}

func appealDTO(row sqlcgen.GradeAppeal) AppealDTO {
	return AppealDTO{UnlockRevision: pgtypex.Int8Value(row.UnlockRevision), ID: ids.ID(row.ID), TenantID: ids.ID(row.TenantID), StudentID: ids.ID(row.StudentID), CourseID: ids.ID(row.CourseID), Reason: row.Reason, Status: row.Status, HandlerID: ids.ID(pgtypex.Int8Value(row.HandlerID)), ResultComment: pgtypex.TextValue(row.ResultComment), CreatedAt: timex.RFC3339OrEmpty(timex.FromTimestamptz(row.CreatedAt)), HandledAt: timex.RFC3339OrEmpty(timex.FromTimestamptz(row.HandledAt))}
}

func warningDTO(row sqlcgen.AcademicWarning) (WarningDTO, error) {
	detail, err := jsonx.ObjectMapStrict(row.Detail)
	if err != nil {
		return WarningDTO{}, apperr.ErrGradeWarningInvalid.WithCause(err)
	}
	return WarningDTO{ID: ids.ID(row.ID), TenantID: ids.ID(row.TenantID), StudentID: ids.ID(row.StudentID), SemesterID: ids.ID(row.SemesterID), Type: row.Type, Detail: detail, Status: row.Status, CreatedAt: timex.RFC3339OrEmpty(timex.FromTimestamptz(row.CreatedAt))}, nil
}

func transcriptDTO(row sqlcgen.TranscriptRecord) TranscriptDTO {
	return TranscriptDTO{ID: ids.ID(row.ID), TenantID: ids.ID(row.TenantID), StudentID: ids.ID(row.StudentID), Scope: row.Scope, SemesterID: ids.ID(pgtypex.Int8Value(row.SemesterID)), PDFRef: row.PdfRef, GeneratedAt: timex.RFC3339OrEmpty(timex.FromTimestamptz(row.GeneratedAt))}
}
