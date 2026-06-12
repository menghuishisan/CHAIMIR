// grade row_convert 文件负责 M11 sqlc 行到模块 DTO 的转换。
package grade

import (
	"context"
	"time"

	"chaimir/internal/modules/grade/internal/sqlcgen"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"

	"github.com/jackc/pgx/v5/pgtype"
)

// CreateLevelConfig 创建等级映射配置。
func (t *txStore) CreateLevelConfig(ctx context.Context, id, tenantID int64, req LevelConfigRequest) (LevelConfigDTO, error) {
	mapping, err := jsonx.AnyBytes(req.Mapping, apperr.ErrGradeConfigInvalid)
	if err != nil {
		return LevelConfigDTO{}, err
	}
	warnings, err := jsonx.AnyBytes(req.WarningRules, apperr.ErrGradeConfigInvalid)
	if err != nil {
		return LevelConfigDTO{}, err
	}
	row, err := t.q.CreateLevelConfig(ctx, sqlcgen.CreateLevelConfigParams{ID: id, TenantID: tenantID, Name: req.Name, Mapping: mapping, WarningRules: warnings, IsDefault: req.IsDefault})
	if err != nil {
		return LevelConfigDTO{}, err
	}
	return levelConfigDTO(row), nil
}

// ListLevelConfigs 查询等级映射配置。
func (t *txStore) ListLevelConfigs(ctx context.Context) ([]LevelConfigDTO, error) {
	rows, err := t.q.ListLevelConfigs(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]LevelConfigDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, levelConfigDTO(row))
	}
	return out, nil
}

// GetDefaultLevelConfig 查询默认等级映射配置。
func (t *txStore) GetDefaultLevelConfig(ctx context.Context) (LevelConfigDTO, error) {
	row, err := t.q.GetDefaultLevelConfig(ctx)
	if err != nil {
		return LevelConfigDTO{}, err
	}
	return levelConfigDTO(row), nil
}

// UpdateLevelConfig 更新等级映射配置。
func (t *txStore) UpdateLevelConfig(ctx context.Context, id int64, req LevelConfigRequest) (LevelConfigDTO, error) {
	mapping, err := jsonx.AnyBytes(req.Mapping, apperr.ErrGradeConfigInvalid)
	if err != nil {
		return LevelConfigDTO{}, err
	}
	warnings, err := jsonx.AnyBytes(req.WarningRules, apperr.ErrGradeConfigInvalid)
	if err != nil {
		return LevelConfigDTO{}, err
	}
	row, err := t.q.UpdateLevelConfig(ctx, sqlcgen.UpdateLevelConfigParams{ID: id, Name: req.Name, Mapping: mapping, WarningRules: warnings, IsDefault: req.IsDefault})
	if err != nil {
		return LevelConfigDTO{}, err
	}
	return levelConfigDTO(row), nil
}

// CreateSemester 创建学期。
func (t *txStore) CreateSemester(ctx context.Context, id, tenantID int64, req SemesterRequest) (SemesterDTO, error) {
	start, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		return SemesterDTO{}, apperr.ErrGradeConfigInvalid.WithCause(err)
	}
	end, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		return SemesterDTO{}, apperr.ErrGradeConfigInvalid.WithCause(err)
	}
	row, err := t.q.CreateSemester(ctx, sqlcgen.CreateSemesterParams{ID: id, TenantID: tenantID, Name: req.Name, StartDate: pgtypex.Date(start), EndDate: pgtypex.Date(end), IsCurrent: req.IsCurrent})
	if err != nil {
		return SemesterDTO{}, err
	}
	return semesterDTO(row), nil
}

// ListSemesters 查询学期列表。
func (t *txStore) ListSemesters(ctx context.Context) ([]SemesterDTO, error) {
	rows, err := t.q.ListSemesters(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]SemesterDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, semesterDTO(row))
	}
	return out, nil
}

// GetCurrentSemester 查询当前学期。
func (t *txStore) GetCurrentSemester(ctx context.Context) (SemesterDTO, error) {
	row, err := t.q.GetCurrentSemester(ctx)
	if err != nil {
		return SemesterDTO{}, err
	}
	return semesterDTO(row), nil
}

// CreateGradeReview 创建成绩审核。
func (t *txStore) CreateGradeReview(ctx context.Context, id, tenantID, submitterID int64, req ReviewRequest) (ReviewDTO, error) {
	row, err := t.q.CreateGradeReview(ctx, sqlcgen.CreateGradeReviewParams{ID: id, TenantID: tenantID, CourseID: req.CourseID, SemesterID: pgtypex.Int8(req.SemesterID), SubmitterID: submitterID, Comment: pgtypex.Text(req.Comment)})
	if err != nil {
		return ReviewDTO{}, err
	}
	return reviewDTO(row), nil
}

// ListGradeReviews 查询成绩审核列表。
func (t *txStore) ListGradeReviews(ctx context.Context, status int16, page, size int) ([]ReviewDTO, error) {
	rows, err := t.q.ListGradeReviews(ctx, sqlcgen.ListGradeReviewsParams{Status: status, PageOffset: int32((page - 1) * size), PageLimit: int32(size)})
	if err != nil {
		return nil, err
	}
	out := make([]ReviewDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, reviewDTO(row))
	}
	return out, nil
}

// GetGradeReview 查询成绩审核。
func (t *txStore) GetGradeReview(ctx context.Context, id int64) (ReviewDTO, error) {
	row, err := t.q.GetGradeReview(ctx, id)
	if err != nil {
		return ReviewDTO{}, err
	}
	return reviewDTO(row), nil
}

// GetLatestApprovedReviewByCourse 查询课程最近通过的审核。
func (t *txStore) GetLatestApprovedReviewByCourse(ctx context.Context, courseID int64) (ReviewDTO, error) {
	row, err := t.q.GetLatestApprovedReviewByCourse(ctx, courseID)
	if err != nil {
		return ReviewDTO{}, err
	}
	return reviewDTO(row), nil
}

// ApproveGradeReview 通过成绩审核。
func (t *txStore) ApproveGradeReview(ctx context.Context, id, reviewerID, semesterID int64, comment string) (ReviewDTO, error) {
	row, err := t.q.ApproveGradeReview(ctx, sqlcgen.ApproveGradeReviewParams{ID: id, ReviewerID: pgtypex.Int8(reviewerID), SemesterID: pgtypex.Int8(semesterID), Comment: pgtypex.Text(comment)})
	if err != nil {
		return ReviewDTO{}, err
	}
	return reviewDTO(row), nil
}

// RejectGradeReview 驳回成绩审核。
func (t *txStore) RejectGradeReview(ctx context.Context, id, reviewerID int64, comment string) (ReviewDTO, error) {
	row, err := t.q.RejectGradeReview(ctx, sqlcgen.RejectGradeReviewParams{ID: id, ReviewerID: pgtypex.Int8(reviewerID), Comment: pgtypex.Text(comment)})
	if err != nil {
		return ReviewDTO{}, err
	}
	return reviewDTO(row), nil
}

// UnlockGradeReview 解锁成绩审核。
func (t *txStore) UnlockGradeReview(ctx context.Context, id, reviewerID int64, comment string) (ReviewDTO, error) {
	row, err := t.q.UnlockGradeReview(ctx, sqlcgen.UnlockGradeReviewParams{ID: id, ReviewerID: pgtypex.Int8(reviewerID), Comment: pgtypex.Text(comment)})
	if err != nil {
		return ReviewDTO{}, err
	}
	return reviewDTO(row), nil
}

// UpsertStudentSemesterGrade 保存学生学期 GPA。
func (t *txStore) UpsertStudentSemesterGrade(ctx context.Context, id, tenantID, studentID, semesterID int64, credits, gpa, cumulative float64) (GradeSummaryDTO, error) {
	creditValue, err := pgtypex.NumericScale(credits, 1)
	if err != nil {
		return GradeSummaryDTO{}, apperr.ErrGradeAggregationFailed.WithCause(err)
	}
	gpaValue, err := pgtypex.NumericScale(gpa, 3)
	if err != nil {
		return GradeSummaryDTO{}, apperr.ErrGradeAggregationFailed.WithCause(err)
	}
	cumulativeValue, err := pgtypex.NumericScale(cumulative, 3)
	if err != nil {
		return GradeSummaryDTO{}, apperr.ErrGradeAggregationFailed.WithCause(err)
	}
	row, err := t.q.UpsertStudentSemesterGrade(ctx, sqlcgen.UpsertStudentSemesterGradeParams{ID: id, TenantID: tenantID, StudentID: studentID, SemesterID: semesterID, TotalCredits: creditValue, Gpa: gpaValue, CumulativeGpa: cumulativeValue})
	if err != nil {
		return GradeSummaryDTO{}, err
	}
	return semesterGradeSummary(row), nil
}

// ListStudentSemesterGrades 查询学生学期 GPA。
func (t *txStore) ListStudentSemesterGrades(ctx context.Context, studentID int64) ([]GradeSummaryDTO, error) {
	rows, err := t.q.ListStudentSemesterGrades(ctx, studentID)
	if err != nil {
		return nil, err
	}
	out := make([]GradeSummaryDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, semesterGradeSummary(row))
	}
	return out, nil
}

// CreateGradeAppeal 创建成绩申诉。
func (t *txStore) CreateGradeAppeal(ctx context.Context, id, tenantID, studentID int64, req AppealRequest) (AppealDTO, error) {
	row, err := t.q.CreateGradeAppeal(ctx, sqlcgen.CreateGradeAppealParams{ID: id, TenantID: tenantID, StudentID: studentID, CourseID: req.CourseID, Reason: req.Reason})
	if err != nil {
		return AppealDTO{}, err
	}
	return appealDTO(row), nil
}

// GetGradeAppeal 查询成绩申诉。
func (t *txStore) GetGradeAppeal(ctx context.Context, id int64) (AppealDTO, error) {
	row, err := t.q.GetGradeAppeal(ctx, id)
	if err != nil {
		return AppealDTO{}, err
	}
	return appealDTO(row), nil
}

// ListGradeAppeals 查询成绩申诉列表。
func (t *txStore) ListGradeAppeals(ctx context.Context, status int16, page, size int) ([]AppealDTO, error) {
	rows, err := t.q.ListGradeAppeals(ctx, sqlcgen.ListGradeAppealsParams{Status: status, PageOffset: int32((page - 1) * size), PageLimit: int32(size)})
	if err != nil {
		return nil, err
	}
	out := make([]AppealDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, appealDTO(row))
	}
	return out, nil
}

// ListAcceptedAppealsByCourseStudent 查询待 M6 改分后收口的申诉。
func (t *txStore) ListAcceptedAppealsByCourseStudent(ctx context.Context, courseID, studentID int64) ([]AppealDTO, error) {
	rows, err := t.q.ListAcceptedAppealsByCourseStudent(ctx, sqlcgen.ListAcceptedAppealsByCourseStudentParams{CourseID: courseID, StudentID: studentID})
	if err != nil {
		return nil, err
	}
	out := make([]AppealDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, appealDTO(row))
	}
	return out, nil
}

// UpdateGradeAppealStatus 更新申诉状态。
func (t *txStore) UpdateGradeAppealStatus(ctx context.Context, id int64, status int16, handlerID int64, comment string) (AppealDTO, error) {
	row, err := t.q.UpdateGradeAppealStatus(ctx, sqlcgen.UpdateGradeAppealStatusParams{ID: id, Status: status, HandlerID: pgtypex.Int8(handlerID), ResultComment: pgtypex.Text(comment)})
	if err != nil {
		return AppealDTO{}, err
	}
	return appealDTO(row), nil
}

// CreateAcademicWarning 创建学业预警。
func (t *txStore) CreateAcademicWarning(ctx context.Context, id, tenantID, studentID, semesterID int64, typ int16, detail map[string]any) (WarningDTO, error) {
	data, err := jsonx.ObjectBytes(detail, apperr.ErrGradeWarningInvalid)
	if err != nil {
		return WarningDTO{}, err
	}
	row, err := t.q.CreateAcademicWarning(ctx, sqlcgen.CreateAcademicWarningParams{ID: id, TenantID: tenantID, StudentID: studentID, SemesterID: semesterID, Type: typ, Detail: data})
	if err != nil {
		return WarningDTO{}, err
	}
	return warningDTO(row), nil
}

// ListAcademicWarnings 查询学业预警。
func (t *txStore) ListAcademicWarnings(ctx context.Context, studentID int64, page, size int) ([]WarningDTO, error) {
	rows, err := t.q.ListAcademicWarnings(ctx, sqlcgen.ListAcademicWarningsParams{StudentID: studentID, PageOffset: int32((page - 1) * size), PageLimit: int32(size)})
	if err != nil {
		return nil, err
	}
	out := make([]WarningDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, warningDTO(row))
	}
	return out, nil
}

// AckAcademicWarning 确认学业预警。
func (t *txStore) AckAcademicWarning(ctx context.Context, id, studentID int64) (WarningDTO, error) {
	row, err := t.q.AckAcademicWarning(ctx, sqlcgen.AckAcademicWarningParams{ID: id, StudentID: studentID})
	if err != nil {
		return WarningDTO{}, err
	}
	return warningDTO(row), nil
}

// CreateTranscriptRecord 创建成绩单记录。
func (t *txStore) CreateTranscriptRecord(ctx context.Context, id, tenantID int64, req TranscriptRequest, pdfRef string) (TranscriptDTO, error) {
	row, err := t.q.CreateTranscriptRecord(ctx, sqlcgen.CreateTranscriptRecordParams{ID: id, TenantID: tenantID, StudentID: req.StudentID, Scope: req.Scope, SemesterID: pgtypex.Int8(req.SemesterID), PdfRef: pdfRef})
	if err != nil {
		return TranscriptDTO{}, err
	}
	return transcriptDTO(row), nil
}

// GetTranscriptRecord 查询成绩单记录。
func (t *txStore) GetTranscriptRecord(ctx context.Context, id int64) (TranscriptDTO, error) {
	row, err := t.q.GetTranscriptRecord(ctx, id)
	if err != nil {
		return TranscriptDTO{}, err
	}
	return transcriptDTO(row), nil
}

// ListTranscriptRecords 查询成绩单记录。
func (t *txStore) ListTranscriptRecords(ctx context.Context, studentID int64, page, size int) ([]TranscriptDTO, error) {
	rows, err := t.q.ListTranscriptRecords(ctx, sqlcgen.ListTranscriptRecordsParams{StudentID: studentID, PageOffset: int32((page - 1) * size), PageLimit: int32(size)})
	if err != nil {
		return nil, err
	}
	out := make([]TranscriptDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, transcriptDTO(row))
	}
	return out, nil
}

// levelConfigDTO 转换等级配置行。
func levelConfigDTO(row sqlcgen.GradeLevelConfig) LevelConfigDTO {
	return LevelConfigDTO{ID: row.ID, TenantID: row.TenantID, Name: row.Name, Mapping: jsonx.Decode(row.Mapping, []LevelRule{}), WarningRules: jsonx.Decode(row.WarningRules, WarningRules{}), IsDefault: row.IsDefault, CreatedAt: formatTime(row.CreatedAt), UpdatedAt: formatTime(row.UpdatedAt)}
}

// semesterDTO 转换学期行。
func semesterDTO(row sqlcgen.Semester) SemesterDTO {
	return SemesterDTO{ID: row.ID, TenantID: row.TenantID, Name: row.Name, StartDate: pgtypex.DateValue(row.StartDate).Format("2006-01-02"), EndDate: pgtypex.DateValue(row.EndDate).Format("2006-01-02"), IsCurrent: row.IsCurrent}
}

// reviewDTO 转换审核行。
func reviewDTO(row sqlcgen.GradeReview) ReviewDTO {
	return ReviewDTO{ID: row.ID, TenantID: row.TenantID, CourseID: row.CourseID, SemesterID: pgtypex.Int8Value(row.SemesterID), SubmitterID: row.SubmitterID, ReviewerID: pgtypex.Int8Value(row.ReviewerID), Status: row.Status, IsLocked: row.IsLocked, Comment: pgtypex.TextValue(row.Comment), SubmittedAt: formatTime(row.SubmittedAt), ReviewedAt: formatOptionalTime(row.ReviewedAt)}
}

// semesterGradeSummary 转换学期成绩聚合行。
func semesterGradeSummary(row sqlcgen.StudentSemesterGrade) GradeSummaryDTO {
	return GradeSummaryDTO{StudentID: row.StudentID, SemesterID: row.SemesterID, TotalCredits: pgtypex.NumericValue(row.TotalCredits), GPA: pgtypex.NumericValue(row.Gpa), CumulativeGPA: pgtypex.NumericValue(row.CumulativeGpa), ComputedAt: timex.FromTimestamptz(row.ComputedAt)}
}

// appealDTO 转换申诉行。
func appealDTO(row sqlcgen.GradeAppeal) AppealDTO {
	return AppealDTO{ID: row.ID, TenantID: row.TenantID, StudentID: row.StudentID, CourseID: row.CourseID, Reason: row.Reason, Status: row.Status, HandlerID: pgtypex.Int8Value(row.HandlerID), ResultComment: pgtypex.TextValue(row.ResultComment), CreatedAt: formatTime(row.CreatedAt), HandledAt: formatOptionalTime(row.HandledAt)}
}

// warningDTO 转换预警行。
func warningDTO(row sqlcgen.AcademicWarning) WarningDTO {
	return WarningDTO{ID: row.ID, TenantID: row.TenantID, StudentID: row.StudentID, SemesterID: row.SemesterID, Type: row.Type, Detail: jsonx.ObjectMap(row.Detail), Status: row.Status, CreatedAt: formatTime(row.CreatedAt)}
}

// transcriptDTO 转换成绩单记录行。
func transcriptDTO(row sqlcgen.TranscriptRecord) TranscriptDTO {
	return TranscriptDTO{ID: row.ID, TenantID: row.TenantID, StudentID: row.StudentID, Scope: row.Scope, SemesterID: pgtypex.Int8Value(row.SemesterID), PDFRef: row.PdfRef, GeneratedAt: formatTime(row.GeneratedAt)}
}

// formatTime 格式化必填时间。
func formatTime(v pgtype.Timestamptz) string {
	return timex.FromTimestamptz(v).Format(time.RFC3339)
}

// formatOptionalTime 格式化可空时间。
func formatOptionalTime(v pgtype.Timestamptz) string {
	if !v.Valid {
		return ""
	}
	return timex.FromTimestamptz(v).Format(time.RFC3339)
}
