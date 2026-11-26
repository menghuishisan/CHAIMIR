// 保存和查询学期 GPA 聚合。
package grade

import (
	"context"

	"chaimir/internal/modules/grade/internal/sqlcgen"
	"chaimir/internal/platform/pgtypex"
	"chaimir/pkg/apperr"
)

// ListGradePublications 读取 M11 当前发布范围，包括解锁修订以隔离旧缓存。
func (t *txStore) ListGradePublications(ctx context.Context) ([]gradePublication, error) {
	rows, err := t.q.ListGradePublications(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]gradePublication, 0, len(rows))
	for _, row := range rows {
		out = append(out, gradePublication{CourseID: row.CourseID, SemesterID: row.SemesterID, Revision: row.Revision, Published: row.Published})
	}
	return out, nil
}

// DeleteStudentSemesterGrades 与本次全部已发布学期重算同事务执行。
func (t *txStore) DeleteStudentSemesterGrades(ctx context.Context, studentID int64) error {
	return t.q.DeleteStudentSemesterGrades(ctx, studentID)
}

// UpsertStudentSemesterGrade 保存学生学期 GPA。
func (t *txStore) UpsertStudentSemesterGrade(ctx context.Context, id, tenantID, studentID, semesterID int64, credits, gpa, cumulative float64) (GradeSummaryDTO, error) {
	creditValue, err := pgtypex.NumericScale(credits, 1)
	if err != nil {
		return GradeSummaryDTO{}, apperr.ErrGradeAggregationFailed.WithCause(err)
	}
	gpaValue, err := pgtypex.NumericScale(gpa, 3)
	if err != nil {
		return GradeSummaryDTO{}, apperr.ErrGradeAggregationFailed.WithCause(err)
	}
	cumulativeValue, err := pgtypex.NumericScale(cumulative, 3)
	if err != nil {
		return GradeSummaryDTO{}, apperr.ErrGradeAggregationFailed.WithCause(err)
	}
	row, err := t.q.UpsertStudentSemesterGrade(ctx, sqlcgen.UpsertStudentSemesterGradeParams{ID: id, TenantID: tenantID, StudentID: studentID, SemesterID: semesterID, TotalCredits: creditValue, Gpa: gpaValue, CumulativeGpa: cumulativeValue})
	if err != nil {
		return GradeSummaryDTO{}, err
	}
	return semesterGradeSummary(row)
}

// ListStudentSemesterGrades 查询学生学期 GPA。
func (t *txStore) ListStudentSemesterGrades(ctx context.Context, studentID int64) ([]GradeSummaryDTO, error) {
	rows, err := t.q.ListStudentSemesterGrades(ctx, studentID)
	if err != nil {
		return nil, err
	}
	out := make([]GradeSummaryDTO, 0, len(rows))
	for _, row := range rows {
		item, err := semesterGradeSummary(row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// ListGradeWarningStudents 按指定学期取得已有聚合的学生，不读取历史学期摘要。
func (t *txStore) ListGradeWarningStudents(ctx context.Context, studentID, semesterID int64) ([]int64, error) {
	return t.q.ListGradeWarningStudents(ctx, sqlcgen.ListGradeWarningStudentsParams{StudentID: studentID, SemesterID: semesterID})
}
