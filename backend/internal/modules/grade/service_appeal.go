// 校验申诉归属并处理受理、解锁与驳回。
package grade

import (
	"context"
	"fmt"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
)

// CreateAppeal 创建成绩申诉。
func (s *Service) CreateAppeal(ctx context.Context, req AppealRequest) (AppealDTO, error) {
	id, err := s.requireStudent(ctx)
	if err != nil {
		return AppealDTO{}, err
	}
	if req.CourseID <= 0 || strings.TrimSpace(req.Reason) == "" {
		return AppealDTO{}, apperr.ErrGradeAppealInvalid
	}
	if err := s.validateAppealCourse(ctx, id.TenantID, req.CourseID.Int64(), id.AccountID); err != nil {
		return AppealDTO{}, err
	}
	var out AppealDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		review, err := tx.GetLatestApprovedReviewByCourse(ctx, req.CourseID.Int64())
		if err != nil {
			return apperr.ErrGradeAppealInvalid.WithCause(err)
		}
		reviewedAt, err := timex.ParseRFC3339(review.ReviewedAt)
		if err != nil {
			return apperr.ErrGradeAppealInvalid.WithCause(err)
		}
		if err := EnsureAppealWithinWindow(reviewedAt, timex.Now(), s.cfg.AppealWindowDays); err != nil {
			return err
		}
		hasOpen, err := tx.HasOpenGradeAppeal(ctx, req.CourseID.Int64(), id.AccountID)
		if err != nil {
			return apperr.ErrGradeAppealInvalid.WithCause(err)
		}
		if hasOpen {
			return apperr.ErrGradeAppealInvalid
		}
		out, err = tx.CreateGradeAppeal(ctx, s.ids.Generate(), id.TenantID, id.AccountID, req)
		if err != nil {
			return err
		}
		return s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumStudent, "grade.appeal.create", auditTargetAppeal, out.ID.Int64(), map[string]any{"course_id": req.CourseID.String()})
	})
	if err != nil {
		return AppealDTO{}, mapGradeAppealErr(err)
	}
	s.invalidateGradeAggregateCache(ctx, id.TenantID, id.AccountID)
	return out, nil
}

// ListAppeals 按服务端角色在分页前限制申诉课程范围,教师不能读取其他教师的申诉。
func (s *Service) ListAppeals(ctx context.Context, status int16, page, size int) ([]AppealDTO, int64, int, int, pagex.Facets, error) {
	id, err := requireTenantUser(ctx)
	if err != nil {
		return nil, 0, page, size, nil, err
	}
	page, size = pagex.Normalize(page, size)
	// 角色取自服务端会话:校管优先、其次教师,只有学生身份的看本人
	actorRole, err := s.gradeActorRole(ctx, id.AccountID)
	if err != nil {
		return nil, 0, page, size, nil, err
	}
	query := appealListScope(actorRole, id.AccountID)
	query.Status, query.Page, query.Size = status, page, size
	if !query.AllCourses && query.StudentID == 0 {
		query.CourseIDs, err = s.teaching.ListTeacherCourseIDs(ctx, id.TenantID, id.AccountID)
		if err != nil {
			return nil, 0, page, size, nil, apperr.ErrGradeForbidden.WithCause(fmt.Errorf("读取教师 %d 的申诉课程范围: %w", id.AccountID, err))
		}
	}
	var out []AppealDTO
	var total int64
	var byStatus pagex.FacetGroup
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, total, err = tx.ListGradeAppeals(ctx, query)
		if err != nil {
			return err
		}
		byStatus, err = tx.CountGradeAppealsByStatus(ctx, query)
		return err
	})
	if err == nil {
		out, err = s.withAppealNames(ctx, id.TenantID, out)
	}
	return out, total, page, size, pagex.Facets{"status": byStatus}, mapGradeAppealErr(err)
}

// appealListScope 使用已校验的服务端角色，教师课程范围由调用方经 M6 填充。
func appealListScope(actorRole int16, accountID int64) AppealListQuery {
	switch actorRole {
	case contracts.RoleNumSchoolAdmin:
		return AppealListQuery{AllCourses: true}
	case contracts.RoleNumTeacher:
		return AppealListQuery{}
	default:
		return AppealListQuery{StudentID: accountID}
	}
}

// AcceptAppeal 受理申诉并解锁对应课程审核。
func (s *Service) AcceptAppeal(ctx context.Context, appealID int64, req AppealDecisionRequest) (AppealDTO, error) {
	return s.decideAppeal(ctx, appealID, AppealStatusAccepted, req.Comment)
}

// RejectAppeal 驳回申诉。
func (s *Service) RejectAppeal(ctx context.Context, appealID int64, req AppealDecisionRequest) (AppealDTO, error) {
	return s.decideAppeal(ctx, appealID, AppealStatusRejected, req.Comment)
}

// decideAppeal 按目标状态处理申诉并在受理时发布解锁事件。
func (s *Service) decideAppeal(ctx context.Context, appealID int64, status int16, comment string) (AppealDTO, error) {
	id, err := s.requireTeacherAdmin(ctx)
	if err != nil {
		return AppealDTO{}, err
	}
	actorRole, err := s.gradeActorRole(ctx, id.AccountID)
	if err != nil {
		return AppealDTO{}, err
	}
	var existing AppealDTO
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		existing, err = tx.GetGradeAppeal(ctx, appealID)
		return err
	}); err != nil {
		return AppealDTO{}, mapGradeAppealErr(err)
	}
	if err := s.ensureAppealHandlerCanAccessCourse(ctx, id, actorRole, existing.CourseID.Int64()); err != nil {
		return AppealDTO{}, err
	}
	var out AppealDTO
	action := "grade.appeal.reject"
	if status == AppealStatusAccepted {
		action = "grade.appeal.accept"
	}
	var affectedSemesterID int64
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		if err := tx.LockGradeCourseScope(ctx, id.TenantID, existing.CourseID.Int64()); err != nil {
			return err
		}
		if status != AppealStatusAccepted {
			out, err = tx.UpdateGradeAppealStatus(ctx, appealID, AppealStatusPending, status, id.AccountID, comment)
		} else {
			// 以最后权威锁事件找到已审核的课程;第二名学生受理时课程可能已解锁。
			authority, err := tx.GetLatestGradeLockOutboxByCourse(ctx, existing.CourseID.Int64())
			if err != nil {
				return err
			}
			review, err := tx.GetGradeReview(ctx, authority.ReviewID)
			if err != nil {
				return err
			}
			if review.IsLocked {
				review, err = tx.UnlockGradeReview(ctx, review.ID.Int64(), id.AccountID, comment)
				if err != nil {
					return err
				}
			} else if review.Status != ReviewStatusPending || authority.Locked {
				return apperr.ErrGradeReviewStateInvalid
			}
			affectedSemesterID = review.SemesterID.Int64()
			unlock, err := s.createLockOutbox(ctx, tx, review, false, "appeal_accepted")
			if err != nil {
				return err
			}
			out, err = tx.AcceptGradeAppeal(ctx, appealID, id.AccountID, comment, unlock.Revision)
			if err != nil {
				return err
			}
			if err := s.recomputeCourseTx(ctx, tx, id.TenantID, existing.CourseID.Int64(), affectedSemesterID); err != nil {
				return err
			}
		}
		if err != nil {
			return err
		}
		return s.writeAudit(ctx, id.TenantID, id.AccountID, actorRole, action, auditTargetAppeal, out.ID.Int64(), map[string]any{"course_id": out.CourseID.String()})
	})
	if err != nil {
		return AppealDTO{}, mapGradeAppealErr(err)
	}
	if status == AppealStatusAccepted {
		s.drainLockOutboxBestEffort(ctx)
		s.invalidateCourseAggregateCache(ctx, id.TenantID, out.CourseID.Int64())
	} else {
		s.invalidateGradeAggregateCache(ctx, id.TenantID, out.StudentID.Int64())
	}
	return out, nil
}

func (s *Service) validateAppealCourse(ctx context.Context, tenantID, courseID, studentID int64) error {
	member, err := s.teaching.IsCourseMember(ctx, tenantID, courseID, studentID)
	if err != nil {
		return apperr.ErrGradeAppealInvalid.WithCause(err)
	}
	if !member {
		return apperr.ErrGradeForbidden
	}
	if _, err := s.teaching.GetCourseGrade(ctx, tenantID, courseID, studentID); err != nil {
		return apperr.ErrGradeAppealInvalid.WithCause(err)
	}
	return nil
}

// ensureAppealHandlerCanAccessCourse 限制教师只能处理本人课程的申诉,管理员可处理全校。
func (s *Service) ensureAppealHandlerCanAccessCourse(ctx context.Context, id tenant.Identity, actorRole int16, courseID int64) error {
	course, err := s.teaching.GetCourse(ctx, id.TenantID, courseID)
	if err != nil {
		return apperr.ErrGradeAppealInvalid.WithCause(err)
	}
	if actorRole == contracts.RoleNumTeacher && course.TeacherID != id.AccountID {
		return apperr.ErrGradeForbidden
	}
	return nil
}

// mapGradeAppealErr 保留申诉窗口等专属错误码。
func mapGradeAppealErr(err error) error {
	if err == nil {
		return nil
	}
	if isNoRows(err) {
		return apperr.ErrGradeAppealInvalid.WithCause(err)
	}
	return apperr.AsAppError(err)
}
