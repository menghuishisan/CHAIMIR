// M11 转换工具:ID、分页、JSONB、数值与 sqlc 行转换。
package grade

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"chaimir/internal/modules/grade/internal/sqlcgen"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/timex"

	"github.com/jackc/pgx/v5/pgtype"
)

// pgText 构造可空文本。
func pgText(v string) pgtype.Text {
	return pgtype.Text{String: strings.TrimSpace(v), Valid: strings.TrimSpace(v) != ""}
}

// pgInt8 构造可空 int8。
func pgInt8(v int64) pgtype.Int8 {
	return pgtype.Int8{Int64: v, Valid: v > 0}
}

// pgInt2 构造可空 int2。
func pgInt2(v int16) pgtype.Int2 {
	return pgtype.Int2{Int16: v, Valid: v > 0}
}

// pgNumeric 构造定点 numeric,非法浮点值必须返回错误并由调用方选择业务错误码。
func pgNumeric(v float64) (pgtype.Numeric, error) {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return pgtype.Numeric{}, fmt.Errorf("成绩数值不是有限数字: %v", v)
	}
	var n pgtype.Numeric
	if err := n.Scan(strconv.FormatFloat(v, 'f', 3, 64)); err != nil {
		return pgtype.Numeric{}, err
	}
	return n, nil
}

// numericValue 读取 numeric 为 float64。
func numericValue(v pgtype.Numeric) float64 {
	f, err := strconv.ParseFloat(v.Int.String(), 64)
	if err != nil {
		return 0
	}
	return f * math.Pow10(int(v.Exp))
}

// dateValue 读取 date 为 UTC 时间。
func dateValue(v pgtype.Date) time.Time {
	if !v.Valid {
		return time.Time{}
	}
	return timex.UTC(v.Time)
}

// nullableID 转换可空 ID。
func nullableID(v pgtype.Int8) string {
	if !v.Valid {
		return ""
	}
	return ids.Format(v.Int64)
}

// nullableText 转换可空文本。
func nullableText(v pgtype.Text) string {
	if !v.Valid {
		return ""
	}
	return v.String
}

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
	return SemesterDTO{ID: ids.Format(row.ID), TenantID: ids.Format(row.TenantID), Name: row.Name, StartDate: dateValue(row.StartDate), EndDate: dateValue(row.EndDate), IsCurrent: row.IsCurrent}
}

// reviewDTOFromRow 转换审核行。
func reviewDTOFromRow(row sqlcgen.GradeReview) ReviewDTO {
	return ReviewDTO{
		ID: ids.Format(row.ID), TenantID: ids.Format(row.TenantID), CourseID: ids.Format(row.CourseID),
		SemesterID: nullableID(row.SemesterID), SubmitterID: ids.Format(row.SubmitterID), ReviewerID: nullableID(row.ReviewerID), Status: row.Status,
		IsLocked: row.IsLocked, Comment: nullableText(row.Comment), SubmittedAt: timex.FromTimestamptz(row.SubmittedAt), ReviewedAt: timex.PtrFromTimestamptz(row.ReviewedAt),
	}
}

// semesterGradeDTOFromRow 转换学期 GPA 行。
func semesterGradeDTOFromRow(row sqlcgen.StudentSemesterGrade) SemesterGradeDTO {
	return SemesterGradeDTO{
		ID: ids.Format(row.ID), TenantID: ids.Format(row.TenantID), StudentID: ids.Format(row.StudentID), SemesterID: ids.Format(row.SemesterID),
		TotalCredits: numericValue(row.TotalCredits), GPA: numericValue(row.Gpa), CumulativeGPA: numericValue(row.CumulativeGpa), ComputedAt: timex.FromTimestamptz(row.ComputedAt),
	}
}

// appealDTOFromRow 转换申诉行。
func appealDTOFromRow(row sqlcgen.GradeAppeal) AppealDTO {
	return AppealDTO{
		ID: ids.Format(row.ID), TenantID: ids.Format(row.TenantID), StudentID: ids.Format(row.StudentID), CourseID: ids.Format(row.CourseID),
		Reason: row.Reason, Status: row.Status, HandlerID: nullableID(row.HandlerID), ResultComment: nullableText(row.ResultComment),
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
		SemesterID: nullableID(row.SemesterID), PDFRef: row.PdfRef, GeneratedAt: timex.FromTimestamptz(row.GeneratedAt),
	}
}
