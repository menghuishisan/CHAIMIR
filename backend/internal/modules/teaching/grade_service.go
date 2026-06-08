// M6 成绩服务:权重配置、单课程成绩计算、覆盖分和对上层聚合 contracts。
package teaching

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/teaching/internal/sqlcgen"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
)

// SetGradeWeights 保存课程成绩权重。
func (s *Service) SetGradeWeights(ctx context.Context, courseID int64, req []GradeWeightInput) ([]GradeWeightInput, error) {
	if err := validateGradeWeights(req); err != nil {
		return nil, err
	}
	if err := validateGradeWeightSources(req); err != nil {
		return nil, err
	}
	if err := s.ensureTeacherOfCourse(ctx, courseID); err != nil {
		return nil, err
	}
	id, _ := tenantFromContext(ctx)
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		if err := q.DeleteGradeWeightsByCourse(ctx, courseID); err != nil {
			return err
		}
		for _, item := range req {
			weight, err := pgNumeric(item.Weight)
			if err != nil {
				return apperr.ErrGradeWeightInvalid.WithCause(err)
			}
			if _, err = q.CreateGradeWeight(ctx, sqlcgen.CreateGradeWeightParams{ID: s.idgen.Generate(), TenantID: id.TenantID, CourseID: courseID, SourceType: item.SourceType, SourceRef: item.SourceRef, Weight: weight}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return nil, ae
		}
		return nil, apperr.ErrGradeWeightInvalid.WithCause(err)
	}
	return req, s.writeAudit(ctx, id.TenantID, auditActionGradeWeight, auditTargetGrade, courseID, map[string]any{"count": len(req)})
}

// ListGradeWeights 查询课程成绩权重。
func (s *Service) ListGradeWeights(ctx context.Context, courseID int64) ([]GradeWeightInput, error) {
	if err := s.ensureTeacherOfCourse(ctx, courseID); err != nil {
		return nil, err
	}
	var rows []sqlcgen.GradeWeight
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, err := q.ListGradeWeightsByCourse(ctx, courseID)
		rows = found
		return err
	}); err != nil {
		return nil, apperr.ErrGradeQueryFailed.WithCause(err)
	}
	out := make([]GradeWeightInput, 0, len(rows))
	for _, row := range rows {
		out = append(out, GradeWeightInput{SourceType: row.SourceType, SourceRef: row.SourceRef, Weight: numericValue(row.Weight)})
	}
	return out, nil
}

// ComputeGrades 按权重计算课程学生总评。
func (s *Service) ComputeGrades(ctx context.Context, courseID int64) ([]CourseGradeDTO, error) {
	if err := s.ensureTeacherOfCourse(ctx, courseID); err != nil {
		return nil, err
	}
	id, _ := tenantFromContext(ctx)
	var out []CourseGradeDTO
	if err := s.repo.inTenant(ctx, func(q *sqlcgen.Queries) error {
		weights, err := q.ListGradeWeightsByCourse(ctx, courseID)
		if err != nil {
			return err
		}
		scores, err := q.ListLatestAssignmentScoresForCourse(ctx, courseID)
		if err != nil {
			return err
		}
		byStudent := buildWeightedScores(weights, scores)
		for studentID, items := range byStudent {
			total, err := computeWeightedTotal(items)
			if err != nil {
				return err
			}
			autoTotal, err := pgNumeric(total)
			if err != nil {
				return apperr.ErrGradeInvalid.WithCause(err)
			}
			row, err := q.UpsertCourseGrade(ctx, sqlcgen.UpsertCourseGradeParams{ID: s.idgen.Generate(), TenantID: id.TenantID, CourseID: courseID, StudentID: studentID, AutoTotal: autoTotal})
			if err != nil {
				return err
			}
			out = append(out, gradeDTOFromRow(row))
		}
		return nil
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return nil, ae
		}
		return nil, apperr.ErrGradeInvalid.WithCause(err)
	}
	return out, nil
}

// ListGrades 查询课程成绩。
func (s *Service) ListGrades(ctx context.Context, courseID int64, page, size int) ([]CourseGradeDTO, error) {
	if err := s.ensureTeacherOfCourse(ctx, courseID); err != nil {
		return nil, err
	}
	return s.listGradesInTenant(ctx, courseID, page, size)
}

// OverrideGrade 写入教师手动覆盖成绩。
func (s *Service) OverrideGrade(ctx context.Context, courseID, studentID int64, req GradeOverrideRequest) (CourseGradeDTO, error) {
	if err := validateScore(req.Score); err != nil {
		return CourseGradeDTO{}, err
	}
	if err := s.ensureTeacherOfCourse(ctx, courseID); err != nil {
		return CourseGradeDTO{}, err
	}
	id, _ := tenantFromContext(ctx)
	before, err := s.getCourseGradeSnapshot(ctx, id.TenantID, courseID, studentID)
	if err != nil {
		return CourseGradeDTO{}, err
	}
	row, err := s.upsertOverrideGrade(ctx, id.TenantID, courseID, studentID, req.Score)
	if err != nil {
		return CourseGradeDTO{}, err
	}
	after := gradeAuditSnapshot(row)
	if err := s.writeAudit(ctx, id.TenantID, auditActionGradeOverride, auditTargetGrade, courseID, map[string]any{
		"student_id": ids.Format(studentID),
		"reason":     req.Reason,
		"before":     before,
		"after":      after,
	}); err != nil {
		return CourseGradeDTO{}, err
	}
	if err := s.publishGradeUpdated(ctx, id.TenantID, courseID, studentID); err != nil {
		return CourseGradeDTO{}, err
	}
	return gradeDTOFromRow(row), nil
}

// ListCourseGrades 实现 contracts.TeachingService 的成绩读取能力。
func (s *Service) ListCourseGrades(ctx context.Context, tenantID, courseID int64) ([]contracts.TeachingCourseGrade, error) {
	var course sqlcgen.Course
	var rows []sqlcgen.CourseGrade
	if err := s.repo.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var err error
		course, err = q.GetCourseByID(ctx, courseID)
		if err != nil {
			return err
		}
		rows, err = q.ListCourseGrades(ctx, sqlcgen.ListCourseGradesParams{CourseID: courseID, LimitCount: int32(s.courseGradesMaxRows)})
		return err
	}); err != nil {
		return nil, apperr.ErrGradeQueryFailed.WithCause(err)
	}
	out := make([]contracts.TeachingCourseGrade, 0, len(rows))
	for _, row := range rows {
		out = append(out, contractGradeFromRows(course, row))
	}
	return out, nil
}

// ListStudentGrades 实现 contracts.TeachingService 的学生跨课程成绩读取能力。
func (s *Service) ListStudentGrades(ctx context.Context, tenantID, studentID int64) ([]contracts.TeachingCourseGrade, error) {
	var rows []sqlcgen.ListStudentCourseGradesRow
	if err := s.repo.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var err error
		rows, err = q.ListStudentCourseGrades(ctx, studentID)
		return err
	}); err != nil {
		return nil, apperr.ErrGradeQueryFailed.WithCause(err)
	}
	out := make([]contracts.TeachingCourseGrade, 0, len(rows))
	for _, row := range rows {
		out = append(out, contractGradeFromStudentRow(row))
	}
	return out, nil
}

// Stats 实现 M9 看板教学统计契约。
func (s *Service) Stats(ctx context.Context, tenantID int64) (contracts.TeachingStats, error) {
	var stats contracts.TeachingStats
	stats.TenantID = tenantID
	if err := s.repo.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var err error
		stats.CourseCount, err = q.CountCourses(ctx)
		if err != nil {
			return err
		}
		stats.ActiveCourseCount, err = q.CountActiveCourses(ctx)
		if err != nil {
			return err
		}
		stats.LearningDurationSec, err = q.SumLearningDuration(ctx)
		return err
	}); err != nil {
		return contracts.TeachingStats{}, apperr.ErrTeachingStatsQueryInvalid.WithCause(err)
	}
	return stats, nil
}

// publishGradeUpdated 发布单课程成绩变更事件,供 M11 订阅后重算 GPA。
func (s *Service) publishGradeUpdated(ctx context.Context, tenantID, courseID, studentID int64) error {
	if s.bus == nil {
		return apperr.ErrGradeEventFailed
	}
	if err := s.bus.Publish(ctx, contracts.SubjectTeachingGradeUpdated, contracts.TeachingGradeUpdatedEvent{
		TenantID: tenantID, CourseID: courseID, StudentID: studentID, UpdatedAt: timex.Now(),
	}); err != nil {
		return apperr.ErrGradeEventFailed.WithCause(err)
	}
	return nil
}

// StatsDTO 返回 HTTP 内部统计 DTO。
func (s *Service) StatsDTO(ctx context.Context, tenantID int64) (StatsDTO, error) {
	stats, err := s.Stats(ctx, tenantID)
	if err != nil {
		return StatsDTO{}, err
	}
	return StatsDTO{TenantID: ids.Format(stats.TenantID), CourseCount: stats.CourseCount, ActiveCourseCount: stats.ActiveCourseCount, LearningDurationSec: stats.LearningDurationSec}, nil
}
