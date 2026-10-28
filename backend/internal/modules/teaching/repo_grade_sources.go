// teaching repo_grade_sources 只访问 M6 自有成绩来源与得分投影。
package teaching

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/teaching/internal/sqlcgen"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/pgtypex"
)

// ListCourseGradeSources 一次读取课程全部有效来源。
func (s *txStore) ListCourseGradeSources(ctx context.Context, tenantID, courseID int64) ([]CourseGradeSource, error) {
	rows, err := s.q.ListCourseGradeSources(ctx, sqlcgen.ListCourseGradeSourcesParams{TenantID: tenantID, CourseID: courseID})
	if err != nil {
		return nil, err
	}
	out := make([]CourseGradeSource, 0, len(rows))
	for _, row := range rows {
		out = append(out, CourseGradeSource{SourceType: row.SourceType, SourceRef: ids.ID(row.SourceID), Title: row.Title, MaxScore: row.MaxScore})
	}
	return out, nil
}

// ListGradeSourceScores 读取全体提交的最高分和待评分状态，实验分数已按各实例满分归一。
func (s *txStore) ListGradeSourceScores(ctx context.Context, tenantID, sourceID int64, sourceType int16) ([]GradeSourceScore, error) {
	out := make([]GradeSourceScore, 0)
	if sourceType == GradeSourceExperiment {
		rows, err := s.q.ListExperimentGradeSourceScores(ctx, sqlcgen.ListExperimentGradeSourceScoresParams{TenantID: tenantID, ExperimentID: sourceID})
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			out = append(out, GradeSourceScore{StudentID: row.StudentID, Score: row.Score, Pending: row.Pending})
		}
	} else {
		rows, err := s.q.ListAssignmentGradeSourceScores(ctx, sqlcgen.ListAssignmentGradeSourceScoresParams{TenantID: tenantID, AssignmentID: sourceID})
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			out = append(out, GradeSourceScore{StudentID: row.StudentID, Score: row.Score, Pending: row.Pending})
		}
	}
	return out, nil
}

// UpsertCourseExperimentSource 按权威定义修订更新来源，课程零值存为未绑定。
func (s *txStore) UpsertCourseExperimentSource(ctx context.Context, event contracts.ExperimentCourseSourceChangedEvent) error {
	return s.q.UpsertCourseExperimentSource(ctx, sqlcgen.UpsertCourseExperimentSourceParams{TenantID: event.TenantID, ExperimentID: event.ExperimentID, CourseID: pgtypex.Int8(event.CourseID), Title: event.ExperimentName, MaxScore: event.MaxScore, Available: event.Available, SourceRevision: event.SourceRevision})
}

// UpsertCourseExperimentScore 同实例同学生仅接受更新的得分修订。
func (s *txStore) UpsertCourseExperimentScore(ctx context.Context, event contracts.ExperimentScoredEvent, studentID int64) error {
	return s.q.UpsertCourseExperimentScore(ctx, sqlcgen.UpsertCourseExperimentScoreParams{TenantID: event.TenantID, ExperimentID: event.ExperimentID, InstanceID: event.InstanceID, StudentID: studentID, Score: event.Score, MaxScore: event.MaxScore, ScoreRevision: event.ScoreRevision, Pending: event.Pending})
}

// GetCourseExperimentScoreRevision 在来源行锁内读取实例已接受的最新修订。
func (s *txStore) GetCourseExperimentScoreRevision(ctx context.Context, tenantID, instanceID int64) (int64, error) {
	return s.q.GetCourseExperimentScoreRevision(ctx, sqlcgen.GetCourseExperimentScoreRevisionParams{TenantID: tenantID, InstanceID: instanceID})
}

// DeleteCourseExperimentScoresBeforeRevision 同一事件事务替换旧成员集，不保留已移除成员的旧分数。
func (s *txStore) DeleteCourseExperimentScoresBeforeRevision(ctx context.Context, tenantID, instanceID, revision int64) error {
	return s.q.DeleteCourseExperimentScoresBeforeRevision(ctx, sqlcgen.DeleteCourseExperimentScoresBeforeRevisionParams{TenantID: tenantID, InstanceID: instanceID, ScoreRevision: revision})
}
