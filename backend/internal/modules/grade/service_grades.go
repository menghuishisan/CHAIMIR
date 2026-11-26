// 查询已发布学业记录，并在租户事务内重算 GPA 与预警。
package grade

import (
	"context"
	"sort"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
)

// StudentGradePage 只向本人学生和校管返回已发布课程的当前页与学期/累计汇总。
func (s *Service) StudentGradePage(ctx context.Context, studentID, semesterID int64, page, size int) (StudentGradePageDTO, error) {
	id, studentID, err := s.normalizeReadableStudent(ctx, studentID)
	if err != nil {
		return StudentGradePageDTO{}, err
	}
	page, size = pagex.Normalize(page, size)
	return s.readStudentGrades(ctx, id.TenantID, studentID, semesterID, page, size)
}

// StudentGPA 查询学生已落库的学期与累计 GPA 聚合结果。
func (s *Service) StudentGPA(ctx context.Context, studentID int64) ([]GradeSummaryDTO, error) {
	id, studentID, err := s.normalizeReadableStudent(ctx, studentID)
	if err != nil {
		return nil, err
	}
	var out []GradeSummaryDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.ListStudentSemesterGrades(ctx, studentID)
		return err
	})
	if err != nil {
		return nil, apperr.ErrGradeAggregationFailed.WithCause(err)
	}
	return out, nil
}

// RecomputeStudentGrade 重算指定学生学期 GPA 并返回聚合结果。
func (s *Service) RecomputeStudentGrade(ctx context.Context, studentID int64, req RecomputeRequest) (GradeSummaryDTO, error) {
	id, err := s.requireSchoolAdmin(ctx)
	if err != nil {
		return GradeSummaryDTO{}, err
	}
	if studentID <= 0 || req.SemesterID <= 0 {
		return GradeSummaryDTO{}, apperr.ErrGradeAggregationFailed
	}
	out := GradeSummaryDTO{StudentID: ids.ID(studentID), SemesterID: req.SemesterID, ComputedAt: timex.Now()}
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if err := tx.LockGradeStudentScope(ctx, id.TenantID, studentID); err != nil {
			return err
		}
		if _, err := s.recomputeStudentWarningsTx(ctx, tx, id.TenantID, studentID, req.SemesterID.Int64()); err != nil {
			return err
		}
		summaries, err := tx.ListStudentSemesterGrades(ctx, studentID)
		if err != nil {
			return err
		}
		for _, summary := range summaries {
			if summary.SemesterID == req.SemesterID {
				out = summary
				return nil
			}
		}
		// 撤回全部课程后没有该学期摘要，保留正常空范围与当前累计值。
		if len(summaries) > 0 {
			out.CumulativeGPA = summaries[0].CumulativeGPA
		}
		return nil
	})
	if err != nil {
		return GradeSummaryDTO{}, apperr.ErrGradeAggregationFailed.WithCause(err)
	}
	s.invalidateGradeAggregateCache(ctx, id.TenantID, studentID)
	s.drainLockOutboxBestEffort(ctx)
	return out, nil
}

// recomputeCourseTx 在课程状态事务内按学生编号取得锁，再重算全部受影响学生。
func (s *Service) recomputeCourseTx(ctx context.Context, tx TxStore, tenantID, courseID, semesterID int64) error {
	grades, err := s.teaching.ListCourseGrades(ctx, tenantID, courseID)
	if err != nil {
		return apperr.ErrGradeAggregationFailed.WithCause(err)
	}
	students := make(map[int64]struct{}, len(grades))
	for _, row := range grades {
		students[row.StudentID] = struct{}{}
	}
	ordered := make([]int64, 0, len(students))
	for studentID := range students {
		ordered = append(ordered, studentID)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	for _, studentID := range ordered {
		if err := tx.LockGradeStudentScope(ctx, tenantID, studentID); err != nil {
			return err
		}
		if _, err := s.recomputeStudentWarningsTx(ctx, tx, tenantID, studentID, semesterID); err != nil {
			return err
		}
	}
	return nil
}

// recomputeStudentWarnings 重算学生 GPA 并返回本次新建的预警数量。
func (s *Service) recomputeStudentWarnings(ctx context.Context, tenantID, studentID, semesterID int64) (int, error) {
	var created int
	err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		if err := tx.LockGradeStudentScope(ctx, tenantID, studentID); err != nil {
			return err
		}
		var err error
		created, err = s.recomputeStudentWarningsTx(ctx, tx, tenantID, studentID, semesterID)
		return err
	})
	if err != nil {
		return 0, err
	}
	s.invalidateGradeAggregateCache(ctx, tenantID, studentID)
	s.drainLockOutboxBestEffort(ctx)
	return created, nil
}

// recomputeStudentWarningsTx 在学生锁内替换全部已发布学期聚合，并更新目标学期预警。
func (s *Service) recomputeStudentWarningsTx(ctx context.Context, tx TxStore, tenantID, studentID, semesterID int64) (int, error) {
	semesters, err := tx.ListSemesters(ctx)
	if err != nil {
		return 0, err
	}
	found := false
	for _, semester := range semesters {
		found = found || semester.ID.Int64() == semesterID
	}
	if !found {
		return 0, apperr.ErrGradeConfigInvalid
	}
	publications, err := tx.ListGradePublications(ctx)
	if err != nil {
		return 0, apperr.ErrGradeAggregationFailed.WithCause(err)
	}
	published, _ := publicationScope(publications)
	var cfg LevelConfigDTO
	var mapping []LevelRule
	bySemester := make(map[int64][]CourseGradeInput)
	var cumulative gpaTotals
	err = s.walkPublishedGrades(ctx, tenantID, studentID, published, func(row contracts.TeachingCourseGrade, term int64) error {
		if mapping == nil {
			var err error
			cfg, err = tx.GetDefaultLevelConfig(ctx)
			if err != nil {
				return apperr.ErrGradeConfigInvalid.WithCause(err)
			}
			if err := validateGPAMapping(cfg.Mapping); err != nil {
				return err
			}
			mapping = orderedMapping(cfg.Mapping)
		}
		input := CourseGradeInput{CourseID: ids.ID(row.CourseID), StudentID: ids.ID(studentID), FinalTotal: row.FinalTotal, Credits: row.Credits}
		bySemester[term] = append(bySemester[term], input)
		return cumulative.add(input, mapping)
	})
	if err != nil {
		return 0, err
	}
	if err := tx.DeleteStudentSemesterGrades(ctx, studentID); err != nil {
		return 0, apperr.ErrGradeAggregationFailed.WithCause(err)
	}
	var targetGPA float64
	for _, semester := range semesters {
		inputs := bySemester[semester.ID.Int64()]
		if len(inputs) == 0 {
			continue
		}
		gpa, credits, err := ComputeGPA(inputs, cfg.Mapping)
		if err != nil {
			return 0, err
		}
		if _, err := tx.UpsertStudentSemesterGrade(ctx, s.ids.Generate(), tenantID, studentID, semester.ID.Int64(), credits, gpa, cumulative.gpa()); err != nil {
			return 0, apperr.ErrGradeAggregationFailed.WithCause(err)
		}
		if semester.ID.Int64() == semesterID {
			targetGPA = gpa
		}
	}
	inputs := bySemester[semesterID]
	if len(inputs) == 0 {
		return 0, nil
	}
	return s.createWarnings(ctx, tx, tenantID, studentID, semesterID, inputs, targetGPA, cfg.WarningRules, cfg.Mapping)
}
