// grade repo_data 文件负责 M11 repo 查询写入与 sqlc 行到模块 DTO 的转换。
package grade

import (
	"context"
	"time"

	"chaimir/internal/modules/grade/internal/sqlcgen"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
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
	return levelConfigDTO(row)
}

// ListLevelConfigs 查询等级映射配置。
func (t *txStore) ListLevelConfigs(ctx context.Context) ([]LevelConfigDTO, error) {
	rows, err := t.q.ListLevelConfigs(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]LevelConfigDTO, 0, len(rows))
	for _, row := range rows {
		item, err := levelConfigDTO(row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// GetDefaultLevelConfig 查询默认等级映射配置。
func (t *txStore) GetDefaultLevelConfig(ctx context.Context) (LevelConfigDTO, error) {
	row, err := t.q.GetDefaultLevelConfig(ctx)
	if err != nil {
		return LevelConfigDTO{}, err
	}
	return levelConfigDTO(row)
}

// LockGradeLevelDefaultScope 串行化当前租户默认等级切换。
func (t *txStore) LockGradeLevelDefaultScope(ctx context.Context, lockKey int64) error {
	return t.q.LockGradeLevelDefaultScope(ctx, lockKey)
}

// ClearDefaultLevelConfigs 清除当前租户既有默认等级标记。
func (t *txStore) ClearDefaultLevelConfigs(ctx context.Context) error {
	return t.q.ClearDefaultLevelConfigs(ctx)
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
	return levelConfigDTO(row)
}

// CreateSemester 创建学期。
func (t *txStore) CreateSemester(ctx context.Context, id, tenantID int64, req SemesterRequest) (SemesterDTO, error) {
	start, err := timex.ParseDate(req.StartDate)
	if err != nil {
		return SemesterDTO{}, apperr.ErrGradeConfigInvalid.WithCause(err)
	}
	end, err := timex.ParseDate(req.EndDate)
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

// LockSemesterCurrentScope 串行化当前租户当前学期切换。
func (t *txStore) LockSemesterCurrentScope(ctx context.Context, lockKey int64) error {
	return t.q.LockSemesterCurrentScope(ctx, lockKey)
}

// ClearCurrentSemesters 清除当前租户既有当前学期标记。
func (t *txStore) ClearCurrentSemesters(ctx context.Context) error {
	return t.q.ClearCurrentSemesters(ctx)
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
	row, err := t.q.CreateGradeReview(ctx, sqlcgen.CreateGradeReviewParams{ID: id, TenantID: tenantID, CourseID: req.CourseID.Int64(), SemesterID: pgtypex.Int8(req.SemesterID.Int64()), SubmitterID: submitterID, Comment: pgtypex.Text(req.Comment)})
	if err != nil {
		return ReviewDTO{}, err
	}
	return reviewDTO(row), nil
}

// ListGradeReviews 查询成绩审核分页列表和总数。
func (t *txStore) ListGradeReviews(ctx context.Context, status int16, page, size int) ([]ReviewDTO, int64, error) {
	limit, offset := pagex.LimitOffset(page, size)
	rows, err := t.q.ListGradeReviews(ctx, sqlcgen.ListGradeReviewsParams{Status: status, PageOffset: offset, PageLimit: limit})
	if err != nil {
		return nil, 0, err
	}
	out := make([]ReviewDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, reviewDTO(row))
	}
	total, err := t.q.CountGradeReviews(ctx, status)
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// ListOwnGradeReviews 查询指定提交人的成绩审核分页列表。
func (t *txStore) ListOwnGradeReviews(ctx context.Context, submitterID int64, status int16, page, size int) ([]ReviewDTO, int64, error) {
	limit, offset := pagex.LimitOffset(page, size)
	rows, err := t.q.ListOwnGradeReviews(ctx, sqlcgen.ListOwnGradeReviewsParams{SubmitterID: submitterID, Status: status, PageOffset: offset, PageLimit: limit})
	if err != nil {
		return nil, 0, err
	}
	out := make([]ReviewDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, reviewDTO(row))
	}
	total, err := t.q.CountOwnGradeReviews(ctx, sqlcgen.CountOwnGradeReviewsParams{SubmitterID: submitterID, Status: status})
	return out, total, err
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

// GetLatestReviewByCourse 查询课程最近一次成绩审核。
func (t *txStore) GetLatestReviewByCourse(ctx context.Context, courseID int64) (ReviewDTO, error) {
	row, err := t.q.GetLatestReviewByCourse(ctx, courseID)
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

// RelockGradeReview 重新锁定申诉改分后的审核状态。
func (t *txStore) RelockGradeReview(ctx context.Context, id, reviewerID int64, comment string) (ReviewDTO, error) {
	row, err := t.q.RelockGradeReview(ctx, sqlcgen.RelockGradeReviewParams{ID: id, ReviewerID: pgtypex.Int8(reviewerID), Comment: pgtypex.Text(comment)})
	if err != nil {
		return ReviewDTO{}, err
	}
	return reviewDTO(row), nil
}

// CreateGradeLockOutbox 在审核状态事务内写入锁定事件 outbox。
func (t *txStore) CreateGradeLockOutbox(ctx context.Context, id int64, review ReviewDTO, locked bool, reason, traceID string) (GradeLockOutbox, error) {
	row, err := t.q.CreateGradeLockOutbox(ctx, sqlcgen.CreateGradeLockOutboxParams{ID: id, TenantID: review.TenantID.Int64(), ReviewID: review.ID.Int64(), CourseID: review.CourseID.Int64(), Locked: locked, Reason: reason, TraceID: traceID})
	if err != nil {
		return GradeLockOutbox{}, err
	}
	return gradeLockOutbox(row), nil
}

// ClaimPendingGradeLockOutbox 跨租户领取待发布、失败待重试或卡住超时的锁定事件。
func (t *txStore) ClaimPendingGradeLockOutbox(ctx context.Context, limit int32, staleBefore time.Time) ([]GradeLockOutbox, error) {
	rows, err := t.q.ClaimPendingGradeLockOutbox(ctx, sqlcgen.ClaimPendingGradeLockOutboxParams{StaleBefore: timex.Timestamptz(staleBefore), PageLimit: limit})
	if err != nil {
		return nil, err
	}
	out := make([]GradeLockOutbox, 0, len(rows))
	for _, row := range rows {
		out = append(out, gradeLockOutbox(row))
	}
	return out, nil
}

// MarkGradeLockOutboxPublished 标记锁定事件发布成功。
func (t *txStore) MarkGradeLockOutboxPublished(ctx context.Context, tenantID, id int64) (GradeLockOutbox, error) {
	row, err := t.q.MarkGradeLockOutboxPublished(ctx, sqlcgen.MarkGradeLockOutboxPublishedParams{TenantID: tenantID, ID: id})
	if err != nil {
		return GradeLockOutbox{}, err
	}
	return gradeLockOutbox(row), nil
}

// MarkGradeLockOutboxFailed 标记锁定事件发布失败并保留脱敏原因。
func (t *txStore) MarkGradeLockOutboxFailed(ctx context.Context, tenantID, id int64, reason string) (GradeLockOutbox, error) {
	row, err := t.q.MarkGradeLockOutboxFailed(ctx, sqlcgen.MarkGradeLockOutboxFailedParams{TenantID: tenantID, ID: id, LastError: pgtypex.Text(reason)})
	if err != nil {
		return GradeLockOutbox{}, err
	}
	return gradeLockOutbox(row), nil
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
	return semesterGradeSummary(row)
}

// ListStudentSemesterGrades 查询学生学期 GPA。
func (t *txStore) ListStudentSemesterGrades(ctx context.Context, studentID int64) ([]GradeSummaryDTO, error) {
	rows, err := t.q.ListStudentSemesterGrades(ctx, studentID)
	if err != nil {
		return nil, err
	}
	out := make([]GradeSummaryDTO, 0, len(rows))
	for _, row := range rows {
		item, err := semesterGradeSummary(row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// ListKnownStudentSemesterGrades 查询已有 GPA 聚合记录,供预警周期扫描确定范围。
func (t *txStore) ListKnownStudentSemesterGrades(ctx context.Context, studentID int64) ([]GradeSummaryDTO, error) {
	rows, err := t.q.ListKnownStudentSemesterGrades(ctx, studentID)
	if err != nil {
		return nil, err
	}
	out := make([]GradeSummaryDTO, 0, len(rows))
	for _, row := range rows {
		item, err := semesterGradeSummary(row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// CreateGradeAppeal 创建成绩申诉。
func (t *txStore) CreateGradeAppeal(ctx context.Context, id, tenantID, studentID int64, req AppealRequest) (AppealDTO, error) {
	row, err := t.q.CreateGradeAppeal(ctx, sqlcgen.CreateGradeAppealParams{ID: id, TenantID: tenantID, StudentID: studentID, CourseID: req.CourseID.Int64(), Reason: req.Reason})
	if err != nil {
		return AppealDTO{}, err
	}
	return appealDTO(row), nil
}

// HasOpenGradeAppeal 判断同一学生同一课程是否已有待处理或受理中的申诉。
func (t *txStore) HasOpenGradeAppeal(ctx context.Context, courseID, studentID int64) (bool, error) {
	return t.q.HasOpenGradeAppeal(ctx, sqlcgen.HasOpenGradeAppealParams{CourseID: courseID, StudentID: studentID})
}

// GetGradeAppeal 查询成绩申诉。
func (t *txStore) GetGradeAppeal(ctx context.Context, id int64) (AppealDTO, error) {
	row, err := t.q.GetGradeAppeal(ctx, id)
	if err != nil {
		return AppealDTO{}, err
	}
	return appealDTO(row), nil
}

// ListGradeAppeals 查询成绩申诉分页列表和总数。
func (t *txStore) ListGradeAppeals(ctx context.Context, status int16, page, size int) ([]AppealDTO, int64, error) {
	limit, offset := pagex.LimitOffset(page, size)
	rows, err := t.q.ListGradeAppeals(ctx, sqlcgen.ListGradeAppealsParams{Status: status, PageOffset: offset, PageLimit: limit})
	if err != nil {
		return nil, 0, err
	}
	out := make([]AppealDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, appealDTO(row))
	}
	total, err := t.q.CountGradeAppeals(ctx, status)
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
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

// UpdateGradeAppealStatus 按明确前置状态更新申诉,避免重复处理旧状态。
func (t *txStore) UpdateGradeAppealStatus(ctx context.Context, id int64, fromStatus, toStatus int16, handlerID int64, comment string) (AppealDTO, error) {
	row, err := t.q.UpdateGradeAppealStatus(ctx, sqlcgen.UpdateGradeAppealStatusParams{ID: id, FromStatus: fromStatus, ToStatus: toStatus, HandlerID: handlerID, ResultComment: comment})
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
	return warningDTO(row)
}

// ListAcademicWarnings 查询学业预警分页列表和总数。
// status 传 0 表示不按确认状态过滤,与 SQL 中的可选条件一致。
func (t *txStore) ListAcademicWarnings(ctx context.Context, studentID int64, status int16, page, size int) ([]WarningDTO, int64, error) {
	limit, offset := pagex.LimitOffset(page, size)
	rows, err := t.q.ListAcademicWarnings(ctx, sqlcgen.ListAcademicWarningsParams{StudentID: studentID, Status: status, PageOffset: offset, PageLimit: limit})
	if err != nil {
		return nil, 0, err
	}
	out := make([]WarningDTO, 0, len(rows))
	for _, row := range rows {
		item, err := warningDTO(row)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, item)
	}
	total, err := t.q.CountAcademicWarnings(ctx, sqlcgen.CountAcademicWarningsParams{StudentID: studentID, Status: status})
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// AckAcademicWarning 确认学业预警。
func (t *txStore) AckAcademicWarning(ctx context.Context, id, studentID int64) (WarningDTO, error) {
	row, err := t.q.AckAcademicWarning(ctx, sqlcgen.AckAcademicWarningParams{ID: id, StudentID: studentID})
	if err != nil {
		return WarningDTO{}, err
	}
	return warningDTO(row)
}

// CreateTranscriptRecord 创建成绩单记录。
func (t *txStore) CreateTranscriptRecord(ctx context.Context, id, tenantID int64, req TranscriptRequest, pdfRef string) (TranscriptDTO, error) {
	row, err := t.q.CreateTranscriptRecord(ctx, sqlcgen.CreateTranscriptRecordParams{ID: id, TenantID: tenantID, StudentID: req.StudentID.Int64(), Scope: req.Scope, SemesterID: pgtypex.Int8(req.SemesterID.Int64()), PdfRef: pdfRef})
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
	limit, offset := pagex.LimitOffset(page, size)
	rows, err := t.q.ListTranscriptRecords(ctx, sqlcgen.ListTranscriptRecordsParams{StudentID: studentID, PageOffset: offset, PageLimit: limit})
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

// semesterDTO 转换学期行。
func semesterDTO(row sqlcgen.Semester) SemesterDTO {
	return SemesterDTO{ID: ids.ID(row.ID), TenantID: ids.ID(row.TenantID), Name: row.Name, StartDate: timex.DateOrEmpty(pgtypex.DateValue(row.StartDate)), EndDate: timex.DateOrEmpty(pgtypex.DateValue(row.EndDate)), IsCurrent: row.IsCurrent}
}

// reviewDTO 转换审核行。
func reviewDTO(row sqlcgen.GradeReview) ReviewDTO {
	return ReviewDTO{ID: ids.ID(row.ID), TenantID: ids.ID(row.TenantID), CourseID: ids.ID(row.CourseID), SemesterID: ids.ID(pgtypex.Int8Value(row.SemesterID)), SubmitterID: ids.ID(row.SubmitterID), ReviewerID: ids.ID(pgtypex.Int8Value(row.ReviewerID)), Status: row.Status, IsLocked: row.IsLocked, Comment: pgtypex.TextValue(row.Comment), SubmittedAt: timex.RFC3339OrEmpty(timex.FromTimestamptz(row.SubmittedAt)), ReviewedAt: timex.RFC3339OrEmpty(timex.FromTimestamptz(row.ReviewedAt))}
}

// gradeLockOutbox 转换成绩锁事件 outbox 行。
func gradeLockOutbox(row sqlcgen.GradeLockOutbox) GradeLockOutbox {
	return GradeLockOutbox{ID: row.ID, TenantID: row.TenantID, ReviewID: row.ReviewID, CourseID: row.CourseID, Locked: row.Locked, Reason: row.Reason, TraceID: row.TraceID, Status: row.Status, RetryCount: row.RetryCount, LastError: pgtypex.TextValue(row.LastError), CreatedAt: timex.RFC3339OrEmpty(timex.FromTimestamptz(row.CreatedAt)), UpdatedAt: timex.RFC3339OrEmpty(timex.FromTimestamptz(row.UpdatedAt))}
}

// semesterGradeSummary 转换学期成绩聚合行。
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

// appealDTO 转换申诉行。
func appealDTO(row sqlcgen.GradeAppeal) AppealDTO {
	return AppealDTO{ID: ids.ID(row.ID), TenantID: ids.ID(row.TenantID), StudentID: ids.ID(row.StudentID), CourseID: ids.ID(row.CourseID), Reason: row.Reason, Status: row.Status, HandlerID: ids.ID(pgtypex.Int8Value(row.HandlerID)), ResultComment: pgtypex.TextValue(row.ResultComment), CreatedAt: timex.RFC3339OrEmpty(timex.FromTimestamptz(row.CreatedAt)), HandledAt: timex.RFC3339OrEmpty(timex.FromTimestamptz(row.HandledAt))}
}

// warningDTO 转换预警行。
func warningDTO(row sqlcgen.AcademicWarning) (WarningDTO, error) {
	detail, err := jsonx.ObjectMapStrict(row.Detail)
	if err != nil {
		return WarningDTO{}, apperr.ErrGradeWarningInvalid.WithCause(err)
	}
	return WarningDTO{ID: ids.ID(row.ID), TenantID: ids.ID(row.TenantID), StudentID: ids.ID(row.StudentID), SemesterID: ids.ID(row.SemesterID), Type: row.Type, Detail: detail, Status: row.Status, CreatedAt: timex.RFC3339OrEmpty(timex.FromTimestamptz(row.CreatedAt))}, nil
}

// transcriptDTO 转换成绩单记录行。
func transcriptDTO(row sqlcgen.TranscriptRecord) TranscriptDTO {
	return TranscriptDTO{ID: ids.ID(row.ID), TenantID: ids.ID(row.TenantID), StudentID: ids.ID(row.StudentID), Scope: row.Scope, SemesterID: ids.ID(pgtypex.Int8Value(row.SemesterID)), PDFRef: row.PdfRef, GeneratedAt: timex.RFC3339OrEmpty(timex.FromTimestamptz(row.GeneratedAt))}
}
