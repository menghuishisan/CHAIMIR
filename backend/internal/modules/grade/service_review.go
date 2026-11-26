// 报送和处理成绩审核，与锁事件及审计原子提交。
package grade

import (
	"context"
	"fmt"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
)

// SubmitReview 提交课程成绩审核。
func (s *Service) SubmitReview(ctx context.Context, req ReviewRequest) (ReviewDTO, error) {
	id, err := s.requireTeacherAdmin(ctx)
	if err != nil {
		return ReviewDTO{}, err
	}
	if req.CourseID <= 0 {
		return ReviewDTO{}, apperr.ErrGradeReviewInvalid
	}
	actorRole, err := s.gradeActorRole(ctx, id.AccountID)
	if err != nil {
		return ReviewDTO{}, err
	}
	course, err := s.validateReviewCourse(ctx, id, req.CourseID.Int64())
	if err != nil {
		return ReviewDTO{}, err
	}
	semesterID, err := s.resolveReviewSemester(ctx, id.TenantID, course, req.SemesterID.Int64())
	if err != nil {
		return ReviewDTO{}, err
	}
	// 只读 M6 已结算成绩；没有成绩与合法零分不同，读取失败也不能生成待审项。
	grades, err := s.teaching.ListCourseGrades(ctx, id.TenantID, course.CourseID)
	if err != nil {
		return ReviewDTO{}, apperr.ErrGradeReviewInvalid.WithCause(fmt.Errorf("read review course grades: tenant_id=%d course_id=%d: %w", id.TenantID, course.CourseID, err))
	}
	if len(grades) == 0 {
		return ReviewDTO{}, apperr.New(apperr.CodeGradeReviewInvalid, "课程尚无可报送的成绩,请先完成成绩结算").WithCause(fmt.Errorf("review course has no grades: tenant_id=%d course_id=%d", id.TenantID, course.CourseID))
	}
	// 待审记录保存实际归档学期，不把缺省选择留给后续审核猜测。
	req.SemesterID = ids.ID(semesterID)
	var out ReviewDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.CreateGradeReview(ctx, s.ids.Generate(), id.TenantID, id.AccountID, req)
		if err != nil {
			return err
		}
		return s.writeAudit(ctx, id.TenantID, id.AccountID, actorRole, "grade.review.submit", auditTargetGradeReview, out.ID.Int64(), map[string]any{"course_id": course.CourseID, "semester": course.Semester})
	})
	if err != nil {
		return ReviewDTO{}, mapGradeReviewErr(err)
	}
	return out, nil
}

// ListReviews 查询成绩审核分页列表。
func (s *Service) ListReviews(ctx context.Context, status, isLocked int16, semesterID int64, page, size int) ([]ReviewDTO, int64, int, int, pagex.Facets, error) {
	id, err := s.requireSchoolAdmin(ctx)
	if err != nil {
		return nil, 0, page, size, nil, err
	}
	page, size = pagex.Normalize(page, size)
	var out []ReviewDTO
	var total int64
	var byStatus pagex.FacetGroup
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, total, err = tx.ListGradeReviews(ctx, status, isLocked, semesterID, page, size)
		if err != nil {
			return err
		}
		byStatus, err = tx.CountGradeReviewsByStatus(ctx, isLocked, semesterID)
		return err
	})
	if err == nil {
		out, err = s.withReviewNames(ctx, id.TenantID, out)
	}
	return out, total, page, size, pagex.Facets{"status": byStatus}, mapGradeReviewErr(err)
}

// ListOwnReviews 查询当前教师本人提交的成绩审核记录。
func (s *Service) ListOwnReviews(ctx context.Context, status int16, page, size int) ([]ReviewDTO, int64, int, int, error) {
	id, err := s.requireTeacherAdmin(ctx)
	if err != nil {
		return nil, 0, page, size, err
	}
	page, size = pagex.Normalize(page, size)
	var out []ReviewDTO
	var total int64
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, total, err = tx.ListOwnGradeReviews(ctx, id.AccountID, status, page, size)
		return err
	})
	if err == nil {
		out, err = s.withReviewNames(ctx, id.TenantID, out)
	}
	return out, total, page, size, mapGradeReviewErr(err)
}

// ApproveReview 将审核、GPA、预警、锁 outbox 和审计原子提交。
func (s *Service) ApproveReview(ctx context.Context, reviewID int64, req ReviewDecisionRequest) (ReviewDTO, error) {
	id, err := s.requireSchoolAdmin(ctx)
	if err != nil {
		return ReviewDTO{}, err
	}
	review, err := s.getReview(ctx, id.TenantID, reviewID)
	if err != nil {
		return ReviewDTO{}, err
	}
	course, err := s.validateReviewCourse(ctx, id, review.CourseID.Int64())
	if err != nil {
		return ReviewDTO{}, err
	}
	// 显式选择优先，其次沿用报送学期；省略编号也由同一规则解析并复核。
	semesterID := req.SemesterID.Int64()
	if semesterID == 0 {
		semesterID = review.SemesterID.Int64()
	}
	semesterID, err = s.resolveReviewSemester(ctx, id.TenantID, course, semesterID)
	if err != nil {
		return ReviewDTO{}, err
	}
	var out ReviewDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if err := tx.LockGradeCourseScope(ctx, id.TenantID, review.CourseID.Int64()); err != nil {
			return err
		}
		var err error
		accepted, err := tx.HasAcceptedAppealsByCourse(ctx, review.CourseID.Int64())
		if err != nil {
			return err
		}
		if accepted {
			return apperr.ErrGradeReviewStateInvalid
		}
		out, err = tx.ApproveGradeReview(ctx, reviewID, id.AccountID, semesterID, req.Comment)
		if err != nil {
			return err
		}
		if err := s.enqueueLockOutbox(ctx, tx, out, true, "review_approved"); err != nil {
			return err
		}
		if err := s.recomputeCourseTx(ctx, tx, id.TenantID, out.CourseID.Int64(), out.SemesterID.Int64()); err != nil {
			return err
		}
		return s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumSchoolAdmin, "grade.review.approve", auditTargetGradeReview, out.ID.Int64(), map[string]any{"course_id": out.CourseID.String(), "semester_id": out.SemesterID.String()})
	})
	if err != nil {
		return ReviewDTO{}, mapGradeReviewErr(err)
	}
	s.drainLockOutboxBestEffort(ctx)
	s.invalidateCourseAggregateCache(ctx, id.TenantID, out.CourseID.Int64())
	return out, nil
}

// RejectReview 驳回成绩审核。
func (s *Service) RejectReview(ctx context.Context, reviewID int64, req ReviewDecisionRequest) (ReviewDTO, error) {
	id, err := s.requireSchoolAdmin(ctx)
	if err != nil {
		return ReviewDTO{}, err
	}
	review, err := s.getReview(ctx, id.TenantID, reviewID)
	if err != nil {
		return ReviewDTO{}, err
	}
	if _, err := s.teaching.GetCourse(ctx, id.TenantID, review.CourseID.Int64()); err != nil {
		return ReviewDTO{}, apperr.ErrGradeReviewInvalid.WithCause(err)
	}
	var out ReviewDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if err := tx.LockGradeCourseScope(ctx, id.TenantID, review.CourseID.Int64()); err != nil {
			return err
		}
		var err error
		accepted, err := tx.HasAcceptedAppealsByCourse(ctx, review.CourseID.Int64())
		if err != nil {
			return err
		}
		if accepted {
			return apperr.ErrGradeReviewStateInvalid
		}
		out, err = tx.RejectGradeReview(ctx, reviewID, id.AccountID, req.Comment)
		if err != nil {
			return err
		}
		return s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumSchoolAdmin, "grade.review.reject", auditTargetGradeReview, out.ID.Int64(), map[string]any{"course_id": out.CourseID.String()})
	})
	if err != nil {
		return ReviewDTO{}, mapGradeReviewErr(err)
	}
	if out.SemesterID > 0 {
		s.invalidateCourseAggregateCache(ctx, id.TenantID, out.CourseID.Int64())
	}
	return out, nil
}

// UnlockReview 解锁课程成绩,供审核重开或申诉受理后由 M6 改分。
func (s *Service) UnlockReview(ctx context.Context, reviewID int64, req ReviewDecisionRequest) (ReviewDTO, error) {
	id, err := s.requireSchoolAdmin(ctx)
	if err != nil {
		return ReviewDTO{}, err
	}
	review, err := s.getReview(ctx, id.TenantID, reviewID)
	if err != nil {
		return ReviewDTO{}, err
	}
	if _, err := s.teaching.GetCourse(ctx, id.TenantID, review.CourseID.Int64()); err != nil {
		return ReviewDTO{}, apperr.ErrGradeReviewInvalid.WithCause(err)
	}
	var out ReviewDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if err := tx.LockGradeCourseScope(ctx, id.TenantID, review.CourseID.Int64()); err != nil {
			return err
		}
		var err error
		out, err = tx.UnlockGradeReview(ctx, reviewID, id.AccountID, req.Comment)
		if err != nil {
			return err
		}
		if err := s.enqueueLockOutbox(ctx, tx, out, false, "review_unlocked"); err != nil {
			return err
		}
		if err := s.recomputeCourseTx(ctx, tx, id.TenantID, out.CourseID.Int64(), out.SemesterID.Int64()); err != nil {
			return err
		}
		return s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumSchoolAdmin, "grade.review.unlock", auditTargetGradeReview, out.ID.Int64(), map[string]any{"course_id": out.CourseID.String()})
	})
	if err != nil {
		return ReviewDTO{}, mapGradeReviewErr(err)
	}
	s.drainLockOutboxBestEffort(ctx)
	if out.SemesterID > 0 {
		s.invalidateCourseAggregateCache(ctx, id.TenantID, out.CourseID.Int64())
	}
	return out, nil
}

func (s *Service) getReview(ctx context.Context, tenantID, reviewID int64) (ReviewDTO, error) {
	var review ReviewDTO
	err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		review, err = tx.GetGradeReview(ctx, reviewID)
		return err
	})
	if err != nil {
		return ReviewDTO{}, apperr.ErrGradeReviewStateInvalid.WithCause(err)
	}
	return review, nil
}

// validateReviewCourse 校验审核课程存在且教师只能操作本人课程。
func (s *Service) validateReviewCourse(ctx context.Context, id tenant.Identity, courseID int64) (contracts.TeachingCourseInfo, error) {
	course, err := s.teaching.GetCourse(ctx, id.TenantID, courseID)
	if err != nil {
		return contracts.TeachingCourseInfo{}, apperr.ErrGradeReviewInvalid.WithCause(err)
	}
	if course.TenantID != id.TenantID || course.CourseID != courseID {
		return contracts.TeachingCourseInfo{}, apperr.ErrGradeReviewInvalid
	}
	hasAdmin, err := s.isSchoolAdmin(ctx, id.AccountID)
	if err != nil {
		return contracts.TeachingCourseInfo{}, err
	}
	if !hasAdmin && course.TeacherID != id.AccountID {
		return contracts.TeachingCourseInfo{}, apperr.ErrGradeForbidden
	}
	return course, nil
}

// mapGradeReviewErr 保留审核、锁事件和审计的专属错误，未知仓储错误保留原因链。
func mapGradeReviewErr(err error) error {
	if err == nil {
		return nil
	}
	if appErr, ok := apperr.As(err); ok {
		return appErr
	}
	return apperr.ErrGradeReviewStateInvalid.WithCause(err)
}

// resolveReviewSemester 只接受同租户且名称精确匹配课程的学期，不按日期或近似名称归档。
func (s *Service) resolveReviewSemester(ctx context.Context, tenantID int64, course contracts.TeachingCourseInfo, requestedID int64) (int64, error) {
	if requestedID < 0 {
		return 0, apperr.ErrGradeReviewInvalid.WithCause(fmt.Errorf("negative review semester: tenant_id=%d course_id=%d semester_id=%d", tenantID, course.CourseID, requestedID))
	}
	if strings.TrimSpace(course.Semester) == "" {
		return 0, apperr.New(apperr.CodeGradeReviewInvalid, "课程尚未填写学期,请先完善课程信息").WithCause(fmt.Errorf("review course semester is empty: tenant_id=%d course_id=%d", tenantID, course.CourseID))
	}
	var semesters []SemesterDTO
	err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		semesters, err = tx.ListSemesters(ctx)
		return err
	})
	if err != nil {
		return 0, apperr.ErrGradeConfigInvalid.WithCause(fmt.Errorf("list review semesters: tenant_id=%d course_id=%d: %w", tenantID, course.CourseID, err))
	}
	// 数据库以租户和名称保证唯一；显式编号与缺省名称共用这一校验入口。
	for _, semester := range semesters {
		if requestedID > 0 && semester.ID.Int64() != requestedID {
			continue
		}
		if semester.Name == course.Semester {
			return semester.ID.Int64(), nil
		}
		if requestedID > 0 {
			return 0, apperr.New(apperr.CodeGradeReviewInvalid, "所选学期与课程学期不一致,请先核对课程信息").WithCause(fmt.Errorf("review semester mismatch: tenant_id=%d course_id=%d course_semester=%q semester_id=%d semester_name=%q", tenantID, course.CourseID, course.Semester, requestedID, semester.Name))
		}
	}
	if requestedID > 0 {
		return 0, apperr.New(apperr.CodeGradeReviewInvalid, "所选学期不可用,请重新选择").WithCause(fmt.Errorf("review semester not found: tenant_id=%d course_id=%d semester_id=%d", tenantID, course.CourseID, requestedID))
	}
	return 0, apperr.New(apperr.CodeGradeReviewInvalid, "课程学期尚未在学校登记,请联系学校管理员核对后重试").WithCause(fmt.Errorf("review course semester not registered: tenant_id=%d course_id=%d course_semester=%q", tenantID, course.CourseID, course.Semester))
}
