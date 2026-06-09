// M6 成绩服务:权重配置、单课程成绩计算、覆盖分和对上层聚合 contracts。
package teaching

import (
	"context"

	"chaimir/internal/contracts"
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
	if err := s.repo.replaceGradeWeights(ctx, id.TenantID, courseID, req, s.idgen.Generate); err != nil {
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
	rows, err := s.repo.listGradeWeights(ctx, courseID)
	if err != nil {
		return nil, apperr.ErrGradeQueryFailed.WithCause(err)
	}
	return rows, nil
}

// ComputeGrades 按权重计算课程学生总评。
func (s *Service) ComputeGrades(ctx context.Context, courseID int64) ([]CourseGradeDTO, error) {
	if err := s.ensureTeacherOfCourse(ctx, courseID); err != nil {
		return nil, err
	}
	id, _ := tenantFromContext(ctx)
	var out []CourseGradeDTO
	weights, err := s.repo.listGradeWeights(ctx, courseID)
	if err != nil {
		return nil, apperr.ErrGradeInvalid.WithCause(err)
	}
	scores, err := s.repo.listLatestAssignmentScoresForCourse(ctx, courseID)
	if err != nil {
		return nil, apperr.ErrGradeInvalid.WithCause(err)
	}
	byStudent := buildWeightedScores(weights, scores)
	for studentID, items := range byStudent {
		total, err := computeWeightedTotal(items)
		if err != nil {
			return nil, err
		}
		row, err := s.repo.upsertAutoCourseGrade(ctx, id.TenantID, s.idgen.Generate(), courseID, studentID, total)
		if err != nil {
			if ae, ok := apperr.As(err); ok {
				return nil, ae
			}
			return nil, apperr.ErrGradeInvalid.WithCause(err)
		}
		out = append(out, row)
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
	return gradeDTOFromCourseGradeSnapshot(row), nil
}

// ListCourseGrades 实现 contracts.TeachingService 的成绩读取能力。
func (s *Service) ListCourseGrades(ctx context.Context, tenantID, courseID int64) ([]contracts.TeachingCourseGrade, error) {
	rows, err := s.repo.listCourseGradesWithCourseInTenant(ctx, tenantID, courseID, s.courseGradesMaxRows)
	if err != nil {
		return nil, apperr.ErrGradeQueryFailed.WithCause(err)
	}
	out := make([]contracts.TeachingCourseGrade, 0, len(rows))
	for _, row := range rows {
		out = append(out, contractGradeFromSnapshot(row))
	}
	return out, nil
}

// ListStudentGrades 实现 contracts.TeachingService 的学生跨课程成绩读取能力。
func (s *Service) ListStudentGrades(ctx context.Context, tenantID, studentID int64) ([]contracts.TeachingCourseGrade, error) {
	rows, err := s.repo.listStudentCourseGradesInTenant(ctx, tenantID, studentID)
	if err != nil {
		return nil, apperr.ErrGradeQueryFailed.WithCause(err)
	}
	out := make([]contracts.TeachingCourseGrade, 0, len(rows))
	for _, row := range rows {
		out = append(out, contractGradeFromSnapshot(row))
	}
	return out, nil
}

// Stats 实现 M9 看板教学统计契约。
func (s *Service) Stats(ctx context.Context, tenantID int64) (contracts.TeachingStats, error) {
	stats, err := s.repo.teachingStatsInTenant(ctx, tenantID)
	if err != nil {
		return contracts.TeachingStats{}, apperr.ErrTeachingStatsQueryInvalid.WithCause(err)
	}
	return contracts.TeachingStats{
		TenantID: stats.TenantID, CourseCount: stats.CourseCount,
		ActiveCourseCount: stats.ActiveCourseCount, LearningDurationSec: stats.LearningDurationSec,
	}, nil
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
