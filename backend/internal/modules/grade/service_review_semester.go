// grade service_review_semester 统一解析报送和审核使用的课程归档学期。
package grade

import (
	"context"
	"fmt"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/pkg/apperr"
)

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
