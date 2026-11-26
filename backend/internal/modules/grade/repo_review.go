// 访问成绩审核记录及课程锁。
package grade

import (
	"context"
	"time"

	"chaimir/internal/modules/grade/internal/sqlcgen"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
	pkgcrypto "chaimir/pkg/crypto"
)

// CreateGradeReview 创建成绩审核。
func (t *txStore) CreateGradeReview(ctx context.Context, id, tenantID, submitterID int64, req ReviewRequest) (ReviewDTO, error) {
	row, err := t.q.CreateGradeReview(ctx, sqlcgen.CreateGradeReviewParams{ID: id, TenantID: tenantID, CourseID: req.CourseID.Int64(), SemesterID: pgtypex.Int8(req.SemesterID.Int64()), SubmitterID: submitterID, Comment: pgtypex.Text(req.Comment)})
	if err != nil {
		return ReviewDTO{}, err
	}
	return reviewDTO(row), nil
}

// CountGradeReviewsByStatus 按状态聚合成绩审核(套用锁定与学期筛选,不套用状态)。
func (t *txStore) CountGradeReviewsByStatus(ctx context.Context, isLocked int16, semesterID int64) (pagex.FacetGroup, error) {
	rows, err := t.q.CountGradeReviewsByStatus(ctx, sqlcgen.CountGradeReviewsByStatusParams{IsLocked: isLocked, SemesterID: semesterID})
	if err != nil {
		return nil, err
	}
	group := pagex.FacetGroup{}
	for _, row := range rows {
		group.Add(row.Status, row.Count)
	}
	return group, nil
}

// ListGradeReviews 查询成绩审核分页列表和总数;semesterID 传 0 表示不按学期筛选。
func (t *txStore) ListGradeReviews(ctx context.Context, status, isLocked int16, semesterID int64, page, size int) ([]ReviewDTO, int64, error) {
	limit, offset := pagex.LimitOffset(page, size)
	rows, err := t.q.ListGradeReviews(ctx, sqlcgen.ListGradeReviewsParams{Status: status, IsLocked: isLocked, SemesterID: semesterID, PageOffset: offset, PageLimit: limit})
	if err != nil {
		return nil, 0, err
	}
	out := make([]ReviewDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, reviewDTO(row))
	}
	total, err := t.q.CountGradeReviews(ctx, sqlcgen.CountGradeReviewsParams{Status: status, IsLocked: isLocked, SemesterID: semesterID})
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

// LockGradeCourseScope 在更新审核/申诉前串行化同课程权威状态与修订分配。
func (t *txStore) LockGradeCourseScope(ctx context.Context, tenantID, courseID int64) error {
	return t.q.LockGradeCourseScope(ctx, sqlcgen.LockGradeCourseScopeParams{Column1: tenantID, Column2: courseID})
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
func (t *txStore) ClaimPendingGradeLockOutbox(ctx context.Context, limit, maxAttempts int32, staleBefore, leaseUntil time.Time) ([]GradeLockOutbox, error) {
	leaseToken, err := pkgcrypto.RandomToken(48)
	if err != nil {
		return nil, err
	}
	rows, err := t.q.ClaimPendingGradeLockOutbox(ctx, sqlcgen.ClaimPendingGradeLockOutboxParams{LeaseToken: leaseToken, LeaseUntil: timex.Timestamptz(leaseUntil), StaleBefore: timex.Timestamptz(staleBefore), PageLimit: limit, MaxAttempts: maxAttempts})
	if err != nil {
		return nil, err
	}
	out := make([]GradeLockOutbox, 0, len(rows))
	for _, row := range rows {
		out = append(out, gradeLockOutbox(row))
	}
	return out, nil
}

// RequeueExhaustedGradeLockOutbox 把冷却期后的失败事件重新放回队列,用于依赖恢复后的自动回放。
func (t *txStore) RequeueExhaustedGradeLockOutbox(ctx context.Context, delaySeconds int32) (int64, error) {
	return t.q.RequeueExhaustedGradeLockOutbox(ctx, int64(delaySeconds))
}

// MarkGradeLockOutboxPublished 标记锁定事件发布成功。
func (t *txStore) MarkGradeLockOutboxPublished(ctx context.Context, tenantID, id int64, leaseToken string) (GradeLockOutbox, error) {
	row, err := t.q.MarkGradeLockOutboxPublished(ctx, sqlcgen.MarkGradeLockOutboxPublishedParams{TenantID: tenantID, ID: id, LeaseToken: leaseToken})
	if err != nil {
		return GradeLockOutbox{}, err
	}
	return gradeLockOutbox(row), nil
}

// MarkGradeLockOutboxFailed 标记锁定事件发布失败并保留脱敏原因。
func (t *txStore) MarkGradeLockOutboxFailed(ctx context.Context, tenantID, id int64, reason, leaseToken string) (GradeLockOutbox, error) {
	row, err := t.q.MarkGradeLockOutboxFailed(ctx, sqlcgen.MarkGradeLockOutboxFailedParams{TenantID: tenantID, ID: id, LastError: pgtypex.Text(reason), LeaseToken: leaseToken})
	if err != nil {
		return GradeLockOutbox{}, err
	}
	return gradeLockOutbox(row), nil
}
