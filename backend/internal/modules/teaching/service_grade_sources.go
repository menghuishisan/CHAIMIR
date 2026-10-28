// teaching service_grade_sources 统一课程真实成绩来源、得分读取和实验事件投影。
package teaching

import (
	"context"
	"fmt"
	"math"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/response"
	"chaimir/pkg/apperr"
)

// CourseGradeSource 是课程可选成绩来源的权威最小投影。
type CourseGradeSource struct {
	SourceType int16   `json:"source_type"`
	SourceRef  ids.ID  `json:"source_ref"`
	Title      string  `json:"title"`
	MaxScore   float64 `json:"max_score"`
}

// GradeSourceScore 保存同一学生的最高得分和是否仍有待批改提交。
type GradeSourceScore struct {
	StudentID int64
	Score     float64
	Pending   bool
}

// ListGradeSources 只向授课教师提供本课程全部有效来源，不依赖公开列表的分页上限。
func (s *Service) ListGradeSources(ctx context.Context, courseID int64) ([]CourseGradeSource, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var sources []CourseGradeSource
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		course, err := tx.GetCourse(ctx, id.TenantID, courseID)
		if err != nil {
			return err
		}
		if err := ensureTeacherOwned(course, id.AccountID); err != nil {
			return err
		}
		sources, err = tx.ListCourseGradeSources(ctx, id.TenantID, courseID)
		return err
	})
	return sources, mapGradeError(err)
}

// gradeSourceKey 将作业与考试视为同一提交来源，防止重复加权。
func gradeSourceKey(sourceType int16, ref string) string {
	if sourceType == GradeSourceExam {
		sourceType = GradeSourceAssignment
	}
	return fmt.Sprintf("%d:%s", sourceType, ref)
}

// validGradeScore 排除非有限数、负分和非法满分。
func validGradeScore(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0
}

// validateExperimentSourceEvent 校验事件边界，禁止不完整事实写入教学投影。
func validateExperimentSourceEvent(event contracts.ExperimentCourseSourceChangedEvent) error {
	if _, ok := ids.Parse(event.EventID); !ok {
		return apperr.ErrTeachingGradeInvalid
	}
	if event.TenantID <= 0 || event.ExperimentID <= 0 || event.CourseID < 0 || event.SourceRevision <= 0 || strings.TrimSpace(event.TraceID) == "" || strings.TrimSpace(event.ExperimentName) == "" || !validGradeScore(event.MaxScore) {
		return apperr.ErrTeachingGradeInvalid
	}
	return nil
}

// HandleExperimentSourceChanged 只按递增定义修订更新 M6 自有来源投影。
func (s *Service) HandleExperimentSourceChanged(ctx context.Context, event contracts.ExperimentCourseSourceChangedEvent) error {
	if err := validateExperimentSourceEvent(event); err != nil {
		return err
	}
	ctx = response.WithTrace(ctx, event.TraceID)
	return s.store.TenantTx(ctx, event.TenantID, func(ctx context.Context, tx TxStore) error { return tx.UpsertCourseExperimentSource(ctx, event) })
}

// HandleExperimentScored 在同一事务更新来源及所有实际成员得分，乱序与重投由修订条件拦截。
func (s *Service) HandleExperimentScored(ctx context.Context, event contracts.ExperimentScoredEvent) error {
	source := contracts.ExperimentCourseSourceChangedEvent{EventID: event.EventID, TenantID: event.TenantID, TraceID: event.TraceID, ExperimentID: event.ExperimentID, ExperimentName: event.ExperimentName, CourseID: event.CourseID, SourceRevision: event.SourceRevision, Available: event.Available, MaxScore: event.SourceMaxScore}
	if err := validateExperimentSourceEvent(source); err != nil {
		return err
	}
	if event.InstanceID <= 0 || event.StudentID <= 0 || event.ScoreRevision <= 0 || event.ScoredAt.IsZero() || len(event.StudentIDs) == 0 || !validGradeScore(event.Score) || !validGradeScore(event.MaxScore) || (event.SourceMaxScore > 0 && (event.MaxScore <= 0 || event.Score > event.MaxScore)) {
		return apperr.ErrTeachingGradeInvalid
	}
	seen := make(map[int64]bool, len(event.StudentIDs))
	for _, studentID := range event.StudentIDs {
		if studentID <= 0 || seen[studentID] {
			return apperr.ErrTeachingGradeInvalid
		}
		seen[studentID] = true
	}
	if !seen[event.StudentID] {
		return apperr.ErrTeachingGradeInvalid
	}
	ctx = response.WithTrace(ctx, event.TraceID)
	return s.store.TenantTx(ctx, event.TenantID, func(ctx context.Context, tx TxStore) error {
		if err := tx.UpsertCourseExperimentSource(ctx, source); err != nil {
			return err
		}
		// 没有计分项的实验不进入成绩候选，也不创建虚构的零分记录。
		if event.SourceMaxScore == 0 {
			return nil
		}
		revision, err := tx.GetCourseExperimentScoreRevision(ctx, event.TenantID, event.InstanceID)
		if err != nil {
			return err
		}
		if revision >= event.ScoreRevision {
			return nil
		}
		if err := tx.DeleteCourseExperimentScoresBeforeRevision(ctx, event.TenantID, event.InstanceID, event.ScoreRevision); err != nil {
			return err
		}
		for _, studentID := range event.StudentIDs {
			if err := tx.UpsertCourseExperimentScore(ctx, event, studentID); err != nil {
				return err
			}
		}
		return nil
	})
}
