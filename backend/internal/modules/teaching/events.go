// M6 事件处理:订阅 M3 判题完成/失败事件并回写提交状态。
package teaching

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/teaching/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/ids"
	"chaimir/pkg/apperr"
)

// HandleJudgeCompleted 处理 M3 判题完成事件并回写提交分。
func (s *Service) HandleJudgeCompleted(ctx context.Context, event contracts.JudgeCompletedEvent) error {
	if err := s.repo.inTenantID(ctx, event.TenantID, func(q *sqlcgen.Queries) error {
		submission, err := q.GetSubmissionByJudgeTaskRef(ctx, pgText(ids.Format(event.TaskID)))
		if db.IsNoRows(err) {
			return nil
		}
		if err != nil {
			return err
		}
		assignment, err := q.GetAssignmentByID(ctx, submission.AssignmentID)
		if err != nil {
			return err
		}
		finalScore, err := finalScoreForSubmission(assignment, submission, int32(event.Score))
		if err != nil {
			return err
		}
		_, err = q.UpdateSubmissionAutoScore(ctx, sqlcgen.UpdateSubmissionAutoScoreParams{
			ID: submission.ID, AutoScore: pgInt4(int32(event.Score)), FinalScore: pgInt4(finalScore), Status: SubmissionStatusGraded,
		})
		return err
	}); err != nil {
		return apperr.ErrSubmissionEventInvalid.WithCause(err)
	}
	return nil
}

// HandleJudgeFailed 处理 M3 判题失败事件并保留待批改状态。
func (s *Service) HandleJudgeFailed(ctx context.Context, event contracts.JudgeFailedEvent) error {
	if err := s.repo.inTenantID(ctx, event.TenantID, func(q *sqlcgen.Queries) error {
		submission, err := q.GetSubmissionByJudgeTaskRef(ctx, pgText(ids.Format(event.TaskID)))
		if db.IsNoRows(err) {
			return nil
		}
		if err != nil {
			return err
		}
		_, err = q.UpdateSubmissionManualScore(ctx, sqlcgen.UpdateSubmissionManualScoreParams{ID: submission.ID, Comment: pgText("自动判题失败,请等待教师处理"), Status: SubmissionStatusPending})
		return err
	}); err != nil {
		return apperr.ErrSubmissionEventInvalid.WithCause(err)
	}
	return nil
}

// SubscribeEvents 订阅 M3 判题事件。
func (s *Service) SubscribeEvents() error {
	if s.bus == nil {
		return apperr.ErrGradeEventFailed
	}
	if _, err := s.bus.Subscribe(contracts.SubjectJudgeCompleted, "teaching", s.onJudgeCompleted); err != nil {
		return apperr.ErrGradeEventFailed.WithCause(err)
	}
	if _, err := s.bus.Subscribe(contracts.SubjectJudgeFailed, "teaching", s.onJudgeFailed); err != nil {
		return apperr.ErrGradeEventFailed.WithCause(err)
	}
	return nil
}

// onJudgeCompleted 解码判题完成事件。
func (s *Service) onJudgeCompleted(ctx context.Context, data []byte) error {
	var event contracts.JudgeCompletedEvent
	if err := eventbus.Decode(data, &event, apperr.ErrSubmissionEventInvalid); err != nil {
		return err
	}
	return s.HandleJudgeCompleted(ctx, event)
}

// onJudgeFailed 解码判题失败事件。
func (s *Service) onJudgeFailed(ctx context.Context, data []byte) error {
	var event contracts.JudgeFailedEvent
	if err := eventbus.Decode(data, &event, apperr.ErrSubmissionEventInvalid); err != nil {
		return err
	}
	return s.HandleJudgeFailed(ctx, event)
}
