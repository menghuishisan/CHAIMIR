// M6 转换工具:在 sqlc 行、contracts DTO 与 HTTP DTO 之间做类型隔离。
package teaching

import (
	"strconv"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/teaching/internal/sqlcgen"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/timex"

	"github.com/jackc/pgx/v5/pgtype"
)

// courseDTOFromRow 转换课程数据库行。
func courseDTOFromRow(row sqlcgen.Course) CourseDTO {
	return CourseDTO{
		ID: ids.Format(row.ID), TeacherID: ids.Format(row.TeacherID), Name: row.Name, Description: row.Description,
		Type: row.Type, Difficulty: row.Difficulty, CoverURL: textValue(row.CoverUrl), Semester: row.Semester,
		Credits: numericValue(row.Credits), Schedule: jsonx.ObjectMap(row.Schedule), InviteCode: row.InviteCode,
		Status: row.Status, Visibility: row.Visibility, CreatedAt: timex.FromTimestamptz(row.CreatedAt), UpdatedAt: timex.FromTimestamptz(row.UpdatedAt),
	}
}

// courseDTOsFromRows 批量转换课程行。
func courseDTOsFromRows(rows []sqlcgen.Course) []CourseDTO {
	out := make([]CourseDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, courseDTOFromRow(row))
	}
	return out
}

// chapterDTOFromRow 转换章节行。
func chapterDTOFromRow(row sqlcgen.Chapter) ChapterDTO {
	return ChapterDTO{ID: ids.Format(row.ID), CourseID: ids.Format(row.CourseID), Title: row.Title, Sort: row.Sort}
}

// lessonDTOFromRow 转换课时行。
func lessonDTOFromRow(row sqlcgen.Lesson) LessonDTO {
	return LessonDTO{
		ID: ids.Format(row.ID), ChapterID: ids.Format(row.ChapterID), Title: row.Title,
		ContentType: row.ContentType, ContentRef: jsonx.ObjectMap(row.ContentRef), Sort: row.Sort,
	}
}

// assignmentDTOFromRow 转换作业行。
func assignmentDTOFromRow(row sqlcgen.Assignment, items []AssignmentItemDTO) AssignmentDTO {
	return AssignmentDTO{
		ID: ids.Format(row.ID), CourseID: ids.Format(row.CourseID), Title: row.Title, ChapterID: optionalID(row.ChapterID),
		DueAt: timex.FromTimestamptz(row.DueAt), MaxAttempts: row.MaxAttempts, LatePolicy: row.LatePolicy,
		LatePenalty: jsonx.ObjectMap(row.LatePenalty), Status: row.Status, Items: items,
	}
}

// assignmentItemDTOFromRow 转换作业题目行。
func assignmentItemDTOFromRow(row sqlcgen.AssignmentItem, face map[string]any) AssignmentItemDTO {
	return AssignmentItemDTO{
		ID: ids.Format(row.ID), ItemCode: row.ItemCode, ItemVersion: row.ItemVersion, Score: row.Score,
		Seq: row.Seq, GradingMode: row.GradingMode, JudgerCode: textValue(row.JudgerCode), Face: face,
	}
}

// submissionDTOFromRow 转换提交行。
func submissionDTOFromRow(row sqlcgen.Submission) SubmissionDTO {
	return SubmissionDTO{
		ID: ids.Format(row.ID), AssignmentID: ids.Format(row.AssignmentID), StudentID: ids.Format(row.StudentID),
		AttemptNo: row.AttemptNo, ContentRef: jsonx.ObjectMap(row.ContentRef), JudgeTaskRef: textValue(row.JudgeTaskRef),
		AutoScore: int4Ptr(row.AutoScore), ManualScore: int4Ptr(row.ManualScore), FinalScore: int4Ptr(row.FinalScore),
		Comment: textValue(row.Comment), IsLate: row.IsLate, Status: row.Status, SubmittedAt: timex.FromTimestamptz(row.SubmittedAt),
	}
}

// gradeDTOFromRow 转换课程成绩行。
func gradeDTOFromRow(row sqlcgen.CourseGrade) CourseGradeDTO {
	override := numericPtr(row.OverrideTotal)
	final := numericValue(row.AutoTotal)
	if override != nil {
		final = *override
	}
	return CourseGradeDTO{
		ID: ids.Format(row.ID), CourseID: ids.Format(row.CourseID), StudentID: ids.Format(row.StudentID),
		AutoTotal: numericValue(row.AutoTotal), OverrideTotal: override, FinalTotal: final, IsOverridden: row.IsOverridden,
	}
}

// contractGradeFromRows 转换 M6 成绩为 contracts DTO。
func contractGradeFromRows(course sqlcgen.Course, row sqlcgen.CourseGrade) contracts.TeachingCourseGrade {
	dto := gradeDTOFromRow(row)
	return contracts.TeachingCourseGrade{
		TenantID: row.TenantID, CourseID: row.CourseID, StudentID: row.StudentID,
		AutoTotal: dto.AutoTotal, OverrideTotal: dto.OverrideTotal, FinalTotal: dto.FinalTotal,
		IsOverridden: row.IsOverridden, Credits: numericValue(course.Credits),
	}
}

// contractGradeFromStudentRow 转换学生跨课程成绩查询行。
func contractGradeFromStudentRow(row sqlcgen.ListStudentCourseGradesRow) contracts.TeachingCourseGrade {
	finalTotal := numericValue(row.AutoTotal)
	var override *float64
	if row.OverrideTotal.Valid {
		v := numericValue(row.OverrideTotal)
		override = &v
		finalTotal = v
	}
	return contracts.TeachingCourseGrade{
		TenantID:      row.TenantID,
		CourseID:      row.CourseID,
		StudentID:     row.StudentID,
		AutoTotal:     numericValue(row.AutoTotal),
		OverrideTotal: override,
		FinalTotal:    finalTotal,
		IsOverridden:  row.IsOverridden,
		Credits:       numericValue(row.Credits),
	}
}

// pgNumeric 把 float64 转换为 pgtype.Numeric。
func pgNumeric(v float64) (pgtype.Numeric, error) {
	var n pgtype.Numeric
	if err := n.Scan(strconv.FormatFloat(v, 'f', 2, 64)); err != nil {
		return pgtype.Numeric{}, err
	}
	return n, nil
}

// nullableNumeric 构造可空 Numeric。
func nullableNumeric(v *float64) (pgtype.Numeric, error) {
	if v == nil {
		return pgtype.Numeric{}, nil
	}
	return pgNumeric(*v)
}

// numericValue 读取 Numeric 的 float64 值。
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

// pgInt4 构造可空 int4。
func pgInt4(v int32) pgtype.Int4 {
	return pgtype.Int4{Int32: v, Valid: true}
}

// pgOptionalInt4 构造可空 int4。
func pgOptionalInt4(v *int32) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: *v, Valid: true}
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

// int4Ptr 转换可空 int4。
func int4Ptr(v pgtype.Int4) *int32 {
	if !v.Valid {
		return nil
	}
	return &v.Int32
}
