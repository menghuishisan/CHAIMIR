// teaching service_contract 文件实现 M6 对聚合模块开放的只读 contracts 适配。
package teaching

import (
	"context"

	"chaimir/internal/contracts"
)

// GetCourse 实现 M6 对 M11 的课程归属只读契约。
func (s *Service) GetCourse(ctx context.Context, tenantID, courseID int64) (contracts.TeachingCourseInfo, error) {
	var course Course
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		course, err = tx.GetCourse(ctx, tenantID, courseID)
		return err
	}); err != nil {
		return contracts.TeachingCourseInfo{}, mapCourseError(err)
	}
	return contractCourse(course), nil
}

// GetCourseGrade 实现 M6 对 M11 的学生单课程成绩只读契约。
func (s *Service) GetCourseGrade(ctx context.Context, tenantID, courseID, studentID int64) (contracts.TeachingCourseGrade, error) {
	var grade CourseGrade
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		grade, err = tx.GetCourseGrade(ctx, tenantID, courseID, studentID)
		return err
	}); err != nil {
		return contracts.TeachingCourseGrade{}, mapGradeError(err)
	}
	return contractGrade(grade), nil
}

// IsCourseMember 实现 M6 对 M11 的课程成员只读契约。
func (s *Service) IsCourseMember(ctx context.Context, tenantID, courseID, studentID int64) (bool, error) {
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		_, err := tx.GetCourseMember(ctx, tenantID, courseID, studentID)
		return err
	}); err != nil {
		if isNoRows(err) {
			return false, nil
		}
		return false, mapCourseError(err)
	}
	return true, nil
}

// ListCourseGrades 实现 M6 对 M11 的只读成绩契约。
func (s *Service) ListCourseGrades(ctx context.Context, tenantID, courseID int64) ([]contracts.TeachingCourseGrade, error) {
	grades, err := s.listCourseGradesForTenant(ctx, tenantID, courseID)
	if err != nil {
		return nil, err
	}
	out := make([]contracts.TeachingCourseGrade, 0, len(grades))
	for _, grade := range grades {
		out = append(out, contractGrade(grade))
	}
	return out, nil
}

// ListStudentGrades 实现 M6 对 M11 的学生成绩只读契约。
func (s *Service) ListStudentGrades(ctx context.Context, tenantID, studentID int64, semester string, page, size int) ([]contracts.TeachingCourseGrade, int64, error) {
	var grades []CourseGrade
	var total int64
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		grades, total, err = tx.ListStudentGrades(ctx, tenantID, studentID, semester, page, size)
		return err
	}); err != nil {
		return nil, 0, mapGradeError(err)
	}
	out := make([]contracts.TeachingCourseGrade, 0, len(grades))
	for _, grade := range grades {
		out = append(out, contractGrade(grade))
	}
	return out, total, nil
}

// Stats 实现 M6 对 M9 的教学统计只读契约。
func (s *Service) Stats(ctx context.Context, tenantID int64) (contracts.TeachingStats, error) {
	var stats contracts.TeachingStats
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		got, err := tx.Stats(ctx, tenantID)
		if err != nil {
			return err
		}
		stats = contracts.TeachingStats{TenantID: tenantID, CourseCount: got.CourseCount, ActiveCourseCount: got.ActiveCourseCount, LearningDurationSec: got.LearningDurationSec}
		return nil
	}); err != nil {
		return contracts.TeachingStats{}, mapCourseError(err)
	}
	return stats, nil
}
