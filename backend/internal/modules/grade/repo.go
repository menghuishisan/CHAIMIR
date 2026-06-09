// M11 数据访问层:封装成绩中心自有表的 sqlc 查询与 RLS 注入。
package grade

import (
	"context"
	"time"

	"chaimir/internal/modules/grade/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/pgtypex"
	"chaimir/pkg/apperr"

	"github.com/jackc/pgx/v5"
)

// repo 是 M11 模块数据库访问封装。
type repo struct {
	db *db.DB
}

// newRepo 构造 M11 repo。
func newRepo(database *db.DB) *repo { return &repo{db: database} }

// queryFunc 是 M11 sqlc 查询闭包。
type queryFunc func(q *sqlcgen.Queries) error

// inTenant 用显式租户 ID 执行租户表查询。
func (r *repo) inTenant(ctx context.Context, tenantID int64, fn queryFunc) error {
	return r.db.WithTenantTxID(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// ListLevelConfigs 查询等级配置列表。
func (r *repo) ListLevelConfigs(ctx context.Context, tenantID int64) ([]LevelConfigDTO, error) {
	var rows []sqlcgen.GradeLevelConfig
	if err := r.inTenant(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var e error
		rows, e = q.ListGradeLevelConfigs(ctx)
		return e
	}); err != nil {
		return nil, apperr.ErrGradeConfigInvalid.WithCause(err)
	}
	out := make([]LevelConfigDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, levelDTOFromRow(row))
	}
	return out, nil
}

// CreateLevelConfig 创建等级配置。
func (r *repo) CreateLevelConfig(ctx context.Context, tenantID, id int64, req LevelConfigRequest) (LevelConfigDTO, error) {
	mapping, warning, err := configJSON(req)
	if err != nil {
		return LevelConfigDTO{}, err
	}
	var row sqlcgen.GradeLevelConfig
	if err := r.inTenant(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var e error
		row, e = q.CreateGradeLevelConfig(ctx, sqlcgen.CreateGradeLevelConfigParams{ID: id, TenantID: tenantID, Name: req.Name, Mapping: mapping, WarningRules: warning, IsDefault: req.IsDefault})
		return e
	}); err != nil {
		return LevelConfigDTO{}, apperr.ErrGradeConfigInvalid.WithCause(err)
	}
	return levelDTOFromRow(row), nil
}

// UpdateLevelConfig 更新等级配置。
func (r *repo) UpdateLevelConfig(ctx context.Context, tenantID, id int64, req LevelConfigRequest) (LevelConfigDTO, error) {
	mapping, warning, err := configJSON(req)
	if err != nil {
		return LevelConfigDTO{}, err
	}
	var row sqlcgen.GradeLevelConfig
	if err := r.inTenant(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var e error
		row, e = q.UpdateGradeLevelConfig(ctx, sqlcgen.UpdateGradeLevelConfigParams{ID: id, Name: req.Name, Mapping: mapping, WarningRules: warning, IsDefault: req.IsDefault})
		return e
	}); err != nil {
		return LevelConfigDTO{}, apperr.ErrGradeConfigInvalid.WithCause(err)
	}
	return levelDTOFromRow(row), nil
}

// DefaultLevelConfig 读取默认等级配置。
func (r *repo) DefaultLevelConfig(ctx context.Context, tenantID int64) (LevelConfigDTO, error) {
	var row sqlcgen.GradeLevelConfig
	if err := r.inTenant(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var e error
		row, e = q.GetDefaultGradeLevelConfig(ctx)
		return e
	}); err != nil {
		if db.IsNoRows(err) {
			return LevelConfigDTO{}, apperr.ErrGradeConfigNotFound
		}
		return LevelConfigDTO{}, apperr.ErrGradeConfigInvalid.WithCause(err)
	}
	return levelDTOFromRow(row), nil
}

// ListSemesters 查询学期列表。
func (r *repo) ListSemesters(ctx context.Context, tenantID int64) ([]SemesterDTO, error) {
	var rows []sqlcgen.Semester
	if err := r.inTenant(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var e error
		rows, e = q.ListSemesters(ctx)
		return e
	}); err != nil {
		return nil, apperr.ErrGradeSemesterInvalid.WithCause(err)
	}
	out := make([]SemesterDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, semesterDTOFromRow(row))
	}
	return out, nil
}

// CreateSemester 创建学期。
func (r *repo) CreateSemester(ctx context.Context, tenantID, id int64, req SemesterRequest) (SemesterDTO, error) {
	start, end, err := parseSemesterDates(req)
	if err != nil {
		return SemesterDTO{}, err
	}
	var row sqlcgen.Semester
	if err := r.inTenant(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var e error
		row, e = q.CreateSemester(ctx, sqlcgen.CreateSemesterParams{ID: id, TenantID: tenantID, Name: req.Name, StartDate: pgtypex.Date(start), EndDate: pgtypex.Date(end), IsCurrent: req.IsCurrent})
		return e
	}); err != nil {
		return SemesterDTO{}, apperr.ErrGradeSemesterInvalid.WithCause(err)
	}
	return semesterDTOFromRow(row), nil
}

// UpsertSemesterGrade 写入或更新学生学期 GPA。
func (r *repo) UpsertSemesterGrade(ctx context.Context, tenantID int64, req SemesterGradeUpsert) (SemesterGradeDTO, error) {
	totalCredits, err := pgtypex.NumericScale(req.TotalCredits, 3)
	if err != nil {
		return SemesterGradeDTO{}, apperr.ErrGradeAggregateFailed.WithCause(err)
	}
	gpa, err := pgtypex.NumericScale(req.GPA, 3)
	if err != nil {
		return SemesterGradeDTO{}, apperr.ErrGradeAggregateFailed.WithCause(err)
	}
	cumulativeGPA, err := pgtypex.NumericScale(req.CumulativeGPA, 3)
	if err != nil {
		return SemesterGradeDTO{}, apperr.ErrGradeAggregateFailed.WithCause(err)
	}
	var row sqlcgen.StudentSemesterGrade
	if err := r.inTenant(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var e error
		row, e = q.UpsertStudentSemesterGrade(ctx, sqlcgen.UpsertStudentSemesterGradeParams{
			ID: req.ID, TenantID: tenantID, StudentID: req.StudentID, SemesterID: req.SemesterID,
			TotalCredits: totalCredits, Gpa: gpa, CumulativeGpa: cumulativeGPA,
		})
		return e
	}); err != nil {
		return SemesterGradeDTO{}, apperr.ErrGradeAggregateFailed.WithCause(err)
	}
	return semesterGradeDTOFromRow(row), nil
}

// ListSemesterGrades 查询某学期全部聚合成绩。
func (r *repo) ListSemesterGrades(ctx context.Context, tenantID, semesterID int64) ([]SemesterGradeDTO, error) {
	var rows []sqlcgen.StudentSemesterGrade
	if err := r.inTenant(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var e error
		rows, e = q.ListSemesterGrades(ctx, semesterID)
		return e
	}); err != nil {
		return nil, apperr.ErrGradeAggregateFailed.WithCause(err)
	}
	out := make([]SemesterGradeDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, semesterGradeDTOFromRow(row))
	}
	return out, nil
}

// ListStudentSemesterGrades 查询学生 GPA 轨迹。
func (r *repo) ListStudentSemesterGrades(ctx context.Context, tenantID, studentID int64) ([]SemesterGradeDTO, error) {
	var rows []sqlcgen.StudentSemesterGrade
	if err := r.inTenant(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var e error
		rows, e = q.ListStudentSemesterGrades(ctx, studentID)
		return e
	}); err != nil {
		return nil, apperr.ErrGradeAggregateFailed.WithCause(err)
	}
	out := make([]SemesterGradeDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, semesterGradeDTOFromRow(row))
	}
	return out, nil
}

// CreateReview 创建或重提交课程成绩审核。
func (r *repo) CreateReview(ctx context.Context, tenantID, id int64, req ReviewCreateRequest, submitterID int64) (ReviewDTO, error) {
	courseID, ok := ids.Parse(req.CourseID)
	if !ok {
		return ReviewDTO{}, apperr.ErrGradeReviewInvalid
	}
	var row sqlcgen.GradeReview
	if err := r.inTenant(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var e error
		row, e = q.CreateGradeReview(ctx, sqlcgen.CreateGradeReviewParams{ID: id, TenantID: tenantID, CourseID: courseID, SubmitterID: submitterID})
		return e
	}); err != nil {
		return ReviewDTO{}, apperr.ErrGradeReviewState.WithCause(err)
	}
	return reviewDTOFromRow(row), nil
}

// GetReview 读取审核记录。
func (r *repo) GetReview(ctx context.Context, tenantID, reviewID int64) (ReviewDTO, error) {
	var row sqlcgen.GradeReview
	if err := r.inTenant(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var e error
		row, e = q.GetGradeReview(ctx, reviewID)
		return e
	}); err != nil {
		return ReviewDTO{}, mapNotFound(err, apperr.ErrGradeReviewNotFound)
	}
	return reviewDTOFromRow(row), nil
}

// GetReviewByCourse 读取课程当前审核记录。
func (r *repo) GetReviewByCourse(ctx context.Context, tenantID, courseID int64) (ReviewDTO, error) {
	var row sqlcgen.GradeReview
	if err := r.inTenant(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var e error
		row, e = q.GetGradeReviewByCourse(ctx, courseID)
		return e
	}); err != nil {
		return ReviewDTO{}, mapNotFound(err, apperr.ErrGradeReviewNotFound)
	}
	return reviewDTOFromRow(row), nil
}

// ListReviews 查询审核记录列表。
func (r *repo) ListReviews(ctx context.Context, tenantID int64, status int16, page, size int) ([]ReviewDTO, int64, error) {
	page, size = pagex.Normalize(page, size)
	var rows []sqlcgen.GradeReview
	var total int64
	if err := r.inTenant(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var e error
		total, e = q.CountGradeReviews(ctx, pgtypex.Int2(status))
		if e != nil {
			return e
		}
		rows, e = q.ListGradeReviews(ctx, sqlcgen.ListGradeReviewsParams{Status: pgtypex.Int2(status), LimitCount: int32(size), OffsetCount: int32((page - 1) * size)})
		return e
	}); err != nil {
		return nil, 0, apperr.ErrGradeReviewState.WithCause(err)
	}
	out := make([]ReviewDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, reviewDTOFromRow(row))
	}
	return out, total, nil
}

// ApproveReview 审核通过并锁定。
func (r *repo) ApproveReview(ctx context.Context, tenantID, reviewerID, reviewID, semesterID int64, comment string) (ReviewDTO, error) {
	var row sqlcgen.GradeReview
	if err := r.inTenant(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var e error
		row, e = q.ApproveGradeReview(ctx, sqlcgen.ApproveGradeReviewParams{ID: reviewID, ReviewerID: pgtypex.Int8(reviewerID), SemesterID: pgtypex.Int8(semesterID), Comment: pgtypex.Text(comment)})
		return e
	}); err != nil {
		return ReviewDTO{}, apperr.ErrGradeReviewState.WithCause(err)
	}
	return reviewDTOFromRow(row), nil
}

// RejectReview 驳回待审成绩。
func (r *repo) RejectReview(ctx context.Context, tenantID, reviewerID, reviewID int64, comment string) (ReviewDTO, error) {
	var row sqlcgen.GradeReview
	if err := r.inTenant(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var e error
		row, e = q.RejectGradeReview(ctx, sqlcgen.RejectGradeReviewParams{ID: reviewID, ReviewerID: pgtypex.Int8(reviewerID), Comment: pgtypex.Text(comment)})
		return e
	}); err != nil {
		return ReviewDTO{}, apperr.ErrGradeReviewState.WithCause(err)
	}
	return reviewDTOFromRow(row), nil
}

// UnlockReview 解锁已审核课程成绩。
func (r *repo) UnlockReview(ctx context.Context, tenantID, reviewerID, reviewID int64, comment string) (ReviewDTO, error) {
	var row sqlcgen.GradeReview
	if err := r.inTenant(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var e error
		row, e = q.UnlockGradeReview(ctx, sqlcgen.UnlockGradeReviewParams{ID: reviewID, ReviewerID: pgtypex.Int8(reviewerID), Comment: pgtypex.Text(comment)})
		return e
	}); err != nil {
		return ReviewDTO{}, apperr.ErrGradeReviewState.WithCause(err)
	}
	return reviewDTOFromRow(row), nil
}

// RelockReviewByCourse 在 M6 改分事件闭环后重新锁定课程审核。
func (r *repo) RelockReviewByCourse(ctx context.Context, tenantID, courseID int64) (ReviewDTO, error) {
	var row sqlcgen.GradeReview
	if err := r.inTenant(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var e error
		row, e = q.RelockGradeReviewByCourse(ctx, courseID)
		return e
	}); err != nil {
		return ReviewDTO{}, apperr.ErrGradeReviewState.WithCause(err)
	}
	return reviewDTOFromRow(row), nil
}

// CreateAppeal 创建学生申诉。
func (r *repo) CreateAppeal(ctx context.Context, tenantID, appealID int64, req AppealCreateRequest, studentID int64) (AppealDTO, error) {
	courseID, ok := ids.Parse(req.CourseID)
	if !ok {
		return AppealDTO{}, apperr.ErrGradeAppealInvalid
	}
	var row sqlcgen.GradeAppeal
	if err := r.inTenant(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var e error
		row, e = q.CreateGradeAppeal(ctx, sqlcgen.CreateGradeAppealParams{ID: appealID, TenantID: tenantID, StudentID: studentID, CourseID: courseID, Reason: req.Reason})
		return e
	}); err != nil {
		return AppealDTO{}, apperr.ErrGradeAppealInvalid.WithCause(err)
	}
	return appealDTOFromRow(row), nil
}

// FindOpenAppeal 查询同一学生课程未闭环申诉。
func (r *repo) FindOpenAppeal(ctx context.Context, tenantID, studentID, courseID int64) (AppealDTO, bool, error) {
	var row sqlcgen.GradeAppeal
	if err := r.inTenant(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var e error
		row, e = q.FindOpenGradeAppeal(ctx, sqlcgen.FindOpenGradeAppealParams{StudentID: studentID, CourseID: courseID})
		return e
	}); err != nil {
		if db.IsNoRows(err) {
			return AppealDTO{}, false, nil
		}
		return AppealDTO{}, false, apperr.ErrGradeAppealInvalid.WithCause(err)
	}
	return appealDTOFromRow(row), true, nil
}

// GetAppeal 读取申诉记录。
func (r *repo) GetAppeal(ctx context.Context, tenantID, appealID int64) (AppealDTO, error) {
	var row sqlcgen.GradeAppeal
	if err := r.inTenant(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var e error
		row, e = q.GetGradeAppeal(ctx, appealID)
		return e
	}); err != nil {
		return AppealDTO{}, mapNotFound(err, apperr.ErrGradeAppealNotFound)
	}
	return appealDTOFromRow(row), nil
}

// ListAppeals 查询申诉列表。
func (r *repo) ListAppeals(ctx context.Context, tenantID int64, status int16, page, size int) ([]AppealDTO, int64, error) {
	page, size = pagex.Normalize(page, size)
	var rows []sqlcgen.GradeAppeal
	var total int64
	if err := r.inTenant(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var e error
		total, e = q.CountGradeAppeals(ctx, pgtypex.Int2(status))
		if e != nil {
			return e
		}
		rows, e = q.ListGradeAppeals(ctx, sqlcgen.ListGradeAppealsParams{Status: pgtypex.Int2(status), LimitCount: int32(size), OffsetCount: int32((page - 1) * size)})
		return e
	}); err != nil {
		return nil, 0, apperr.ErrGradeAppealInvalid.WithCause(err)
	}
	out := make([]AppealDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, appealDTOFromRow(row))
	}
	return out, total, nil
}

// UpdateAppealStatus 更新申诉状态。
func (r *repo) UpdateAppealStatus(ctx context.Context, tenantID, appealID, handlerID int64, status int16, comment string) (AppealDTO, error) {
	var row sqlcgen.GradeAppeal
	if err := r.inTenant(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var e error
		row, e = q.UpdateGradeAppealStatus(ctx, sqlcgen.UpdateGradeAppealStatusParams{ID: appealID, HandlerID: pgtypex.Int8(handlerID), Status: status, ResultComment: pgtypex.Text(comment)})
		return e
	}); err != nil {
		return AppealDTO{}, apperr.ErrGradeAppealState.WithCause(err)
	}
	return appealDTOFromRow(row), nil
}

// CreateWarning 写入学业预警。
func (r *repo) CreateWarning(ctx context.Context, tenantID, warningID int64, req WarningCreate) (WarningDTO, error) {
	detail, err := jsonx.AnyBytes(req.Detail, apperr.ErrGradeWarningInvalid)
	if err != nil {
		return WarningDTO{}, err
	}
	var row sqlcgen.AcademicWarning
	if err := r.inTenant(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var e error
		row, e = q.CreateAcademicWarning(ctx, sqlcgen.CreateAcademicWarningParams{ID: warningID, TenantID: tenantID, StudentID: req.StudentID, SemesterID: req.SemesterID, Type: req.Type, Detail: detail})
		return e
	}); err != nil {
		return WarningDTO{}, apperr.ErrGradeWarningFailed.WithCause(err)
	}
	return warningDTOFromRow(row), nil
}

// DeleteWarning 删除刚创建但通知失败的预警,避免留下仅落库未通知的半成功状态。
func (r *repo) DeleteWarning(ctx context.Context, tenantID, warningID int64) error {
	if err := r.inTenant(ctx, tenantID, func(q *sqlcgen.Queries) error {
		return q.DeleteAcademicWarning(ctx, warningID)
	}); err != nil {
		return apperr.ErrGradeWarningFailed.WithCause(err)
	}
	return nil
}

// ListWarnings 查询学业预警。
func (r *repo) ListWarnings(ctx context.Context, tenantID, studentID, semesterID int64, status int16, page, size int) ([]WarningDTO, int64, error) {
	page, size = pagex.Normalize(page, size)
	var rows []sqlcgen.AcademicWarning
	var total int64
	if err := r.inTenant(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var e error
		params := sqlcgen.CountAcademicWarningsParams{StudentID: pgtypex.Int8(studentID), SemesterID: pgtypex.Int8(semesterID), Status: pgtypex.Int2(status)}
		total, e = q.CountAcademicWarnings(ctx, params)
		if e != nil {
			return e
		}
		rows, e = q.ListAcademicWarnings(ctx, sqlcgen.ListAcademicWarningsParams{StudentID: params.StudentID, SemesterID: params.SemesterID, Status: params.Status, LimitCount: int32(size), OffsetCount: int32((page - 1) * size)})
		return e
	}); err != nil {
		return nil, 0, apperr.ErrGradeWarningFailed.WithCause(err)
	}
	out := make([]WarningDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, warningDTOFromRow(row))
	}
	return out, total, nil
}

// AcknowledgeWarning 标记学生本人的预警已知悉。
func (r *repo) AcknowledgeWarning(ctx context.Context, tenantID, studentID, warningID int64) (WarningDTO, error) {
	var row sqlcgen.AcademicWarning
	if err := r.inTenant(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var e error
		row, e = q.AcknowledgeAcademicWarning(ctx, sqlcgen.AcknowledgeAcademicWarningParams{ID: warningID, StudentID: studentID})
		return e
	}); err != nil {
		return WarningDTO{}, apperr.ErrGradeWarningFailed.WithCause(err)
	}
	return warningDTOFromRow(row), nil
}

// CreateTranscript 创建成绩单记录。
func (r *repo) CreateTranscript(ctx context.Context, tenantID, id int64, req TranscriptRequest, pdfRef string) (TranscriptDTO, error) {
	studentID, ok := ids.Parse(req.StudentID)
	if !ok {
		return TranscriptDTO{}, apperr.ErrGradeTranscriptInvalid
	}
	var row sqlcgen.TranscriptRecord
	if err := r.inTenant(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var e error
		row, e = q.CreateTranscriptRecord(ctx, sqlcgen.CreateTranscriptRecordParams{ID: id, TenantID: tenantID, StudentID: studentID, Scope: req.Scope, SemesterID: pgtypex.Int8(ids.ParseOrZero(req.SemesterID)), PdfRef: pdfRef})
		return e
	}); err != nil {
		return TranscriptDTO{}, apperr.ErrGradeTranscriptFailed.WithCause(err)
	}
	return transcriptDTOFromRow(row), nil
}

// GetTranscript 读取成绩单记录。
func (r *repo) GetTranscript(ctx context.Context, tenantID, transcriptID int64) (TranscriptDTO, error) {
	var row sqlcgen.TranscriptRecord
	if err := r.inTenant(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var e error
		row, e = q.GetTranscriptRecord(ctx, transcriptID)
		return e
	}); err != nil {
		return TranscriptDTO{}, apperr.ErrGradeTranscriptFailed.WithCause(err)
	}
	return transcriptDTOFromRow(row), nil
}

// configJSON 序列化等级映射与预警规则。
func configJSON(req LevelConfigRequest) ([]byte, []byte, error) {
	mapping, err := jsonx.AnyBytes(req.Mapping, apperr.ErrGradeConfigInvalid)
	if err != nil {
		return nil, nil, err
	}
	warning, err := jsonx.AnyBytes(req.WarningRules, apperr.ErrGradeConfigInvalid)
	if err != nil {
		return nil, nil, err
	}
	return mapping, warning, nil
}

// parseSemesterDates 解析学期日期。
func parseSemesterDates(req SemesterRequest) (time.Time, time.Time, error) {
	start, err := time.Parse(time.DateOnly, req.StartDate)
	if err != nil {
		return time.Time{}, time.Time{}, apperr.ErrGradeSemesterInvalid.WithCause(err)
	}
	end, err := time.Parse(time.DateOnly, req.EndDate)
	if err != nil {
		return time.Time{}, time.Time{}, apperr.ErrGradeSemesterInvalid.WithCause(err)
	}
	if start.After(end) {
		return time.Time{}, time.Time{}, apperr.ErrGradeSemesterInvalid
	}
	return start, end, nil
}

// mapNotFound 把未命中映射为业务不存在错误。
func mapNotFound(err error, appErr *apperr.Error) error {
	if db.IsNoRows(err) {
		return appErr
	}
	return appErr.WithCause(err)
}
