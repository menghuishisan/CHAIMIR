// grade service_grade_events 在单事务内消费M6成绩修订,核对申诉解锁因果并更新聚合。
package grade

import (
	"context"
	"fmt"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/response"
	"chaimir/pkg/apperr"
)

// HandleGradeUpdated 只消费更新修订;课程/学生串行锁内重读状态,避免旧事件完成新申诉。
func (s *Service) HandleGradeUpdated(ctx context.Context, evt contracts.TeachingGradeUpdatedEvent) error {
	if evt.TenantID <= 0 || evt.CourseID <= 0 || evt.StudentID <= 0 || evt.Revision <= 0 || evt.LockRevision < 0 || evt.UpdatedAt.IsZero() || strings.TrimSpace(evt.TraceID) == "" {
		return apperr.ErrGradeAggregationFailed
	}
	ctx = response.WithTrace(ctx, evt.TraceID)
	var semesters []SemesterDTO
	err := s.store.TenantTx(ctx, evt.TenantID, func(ctx context.Context, tx TxStore) error {
		if err := tx.LockGradeCourseScope(ctx, evt.TenantID, evt.CourseID); err != nil {
			return err
		}
		if err := tx.LockGradeStudentScope(ctx, evt.TenantID, evt.StudentID); err != nil {
			return err
		}
		var err error
		// 重投也失效缓存,覆盖上次提交后、删除缓存前进程中断的情况。
		semesters, err = tx.ListSemesters(ctx)
		if err != nil {
			return err
		}
		applied, err := tx.ApplyGradeEventRevision(ctx, evt.TenantID, evt.CourseID, evt.StudentID, evt.Revision)
		if err != nil || !applied {
			return err
		}
		// 待审新报送不改变锁权威,仍按最后一次锁事件关联的审核处理在途申诉。
		authority, err := tx.GetLatestGradeLockOutboxByCourse(ctx, evt.CourseID)
		hasAuthority := err == nil
		if err != nil && !isNoRows(err) {
			return err
		}
		var review ReviewDTO
		if hasAuthority {
			if authority.TenantID != evt.TenantID || authority.CourseID != evt.CourseID || authority.Revision <= 0 || evt.LockRevision > authority.Revision {
				return apperr.ErrGradeReviewStateInvalid
			}
			review, err = tx.GetGradeReview(ctx, authority.ReviewID)
		} else {
			if evt.LockRevision != 0 {
				return apperr.ErrGradeReviewStateInvalid
			}
			review, err = tx.GetLatestReviewByCourse(ctx, evt.CourseID)
		}
		if isNoRows(err) && !hasAuthority {
			// 首次报送前不生成审核或申诉状态,正常确认这条成绩事件。
			return nil
		}
		if err != nil {
			return err
		}
		if review.SemesterID <= 0 || review.TenantID.Int64() != evt.TenantID || review.CourseID.Int64() != evt.CourseID {
			return apperr.ErrGradeReviewStateInvalid
		}
		_, err = s.recomputeStudentWarningsTx(ctx, tx, evt.TenantID, evt.StudentID, review.SemesterID.Int64())
		if err != nil {
			return err
		}
		appeals, err := tx.ListAcceptedAppealsByCourseStudent(ctx, evt.CourseID, evt.StudentID)
		if err != nil || len(appeals) == 0 {
			return err
		}
		if !hasAuthority || authority.ReviewID != review.ID.Int64() {
			return apperr.ErrGradeReviewStateInvalid.WithCause(fmt.Errorf("成绩修订与课程审核不一致: course_id=%d review_id=%d event_revision=%d lock_revision=%d", evt.CourseID, review.ID, evt.Revision, evt.LockRevision))
		}
		completed := false
		for _, appeal := range appeals {
			if appeal.UnlockRevision <= 0 {
				return apperr.ErrGradeReviewStateInvalid.WithCause(fmt.Errorf("受理申诉缺少解锁修订: appeal_id=%d", appeal.ID))
			}
			if evt.LockRevision < appeal.UnlockRevision {
				continue
			}
			if _, err := tx.UpdateGradeAppealStatus(ctx, appeal.ID.Int64(), AppealStatusAccepted, AppealStatusCompleted, appeal.HandlerID.Int64(), "成绩已更新"); err != nil {
				return err
			}
			completed = true
		}
		if !completed {
			return nil
		}
		remaining, err := tx.HasAcceptedAppealsByCourse(ctx, evt.CourseID)
		if err != nil || remaining || review.IsLocked {
			return err
		}
		relocked, err := tx.RelockGradeReview(ctx, review.ID.Int64(), review.ReviewerID.Int64(), "成绩已更新并完成复核")
		if err != nil {
			return err
		}
		return s.enqueueLockOutbox(ctx, tx, relocked, true, "grade_updated")
	})
	if err != nil {
		return apperr.ErrGradeAggregationFailed.WithCause(err)
	}
	s.invalidateSummaryCache(ctx, evt.TenantID, evt.StudentID, 0)
	for _, semester := range semesters {
		s.invalidateSummaryCache(ctx, evt.TenantID, evt.StudentID, semester.ID.Int64())
	}
	s.drainLockOutboxBestEffort(ctx)
	return nil
}
