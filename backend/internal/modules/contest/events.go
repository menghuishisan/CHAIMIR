// M8 事件处理:订阅 M3 判题结果,回写解题赛提交并刷新排行榜。
package contest

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/eventbus"
	"chaimir/pkg/apperr"
)

// HandleJudgeCompleted 处理 M3 判题完成事件。
func (s *Service) HandleJudgeCompleted(ctx context.Context, event contracts.JudgeCompletedEvent) error {
	pending, err := s.store.PendingSubmissionByJudgeTask(ctx, event.TenantID, event.TaskID)
	if err != nil {
		return err
	}
	if pending.SourceRef != event.SourceRef {
		return apperr.ErrContestEventInvalid
	}
	return s.applySolveJudgement(ctx, pending, event.Score > 0, int32(event.Score))
}

// HandleJudgeFailed 处理 M3 判题失败事件,保留 0 分结果。
func (s *Service) HandleJudgeFailed(ctx context.Context, event contracts.JudgeFailedEvent) error {
	pending, err := s.store.PendingSubmissionByJudgeTask(ctx, event.TenantID, event.TaskID)
	if err != nil {
		return err
	}
	if pending.SourceRef != event.SourceRef {
		return apperr.ErrContestEventInvalid
	}
	return s.applySolveJudgement(ctx, pending, false, 0)
}

// SubscribeEvents 订阅 M3 判题事件。
func (s *Service) SubscribeEvents() error {
	if s.bus == nil {
		return apperr.ErrContestEventFailed
	}
	if _, err := s.bus.Subscribe(contracts.SubjectJudgeCompleted, "contest", s.onJudgeCompleted); err != nil {
		return apperr.ErrContestEventFailed.WithCause(err)
	}
	if _, err := s.bus.Subscribe(contracts.SubjectJudgeFailed, "contest", s.onJudgeFailed); err != nil {
		return apperr.ErrContestEventFailed.WithCause(err)
	}
	return nil
}

// onJudgeCompleted 解码判题完成事件。
func (s *Service) onJudgeCompleted(ctx context.Context, data []byte) error {
	var event contracts.JudgeCompletedEvent
	if err := eventbus.Decode(data, &event, apperr.ErrContestEventInvalid); err != nil {
		return err
	}
	return s.HandleJudgeCompleted(ctx, event)
}

// onJudgeFailed 解码判题失败事件。
func (s *Service) onJudgeFailed(ctx context.Context, data []byte) error {
	var event contracts.JudgeFailedEvent
	if err := eventbus.Decode(data, &event, apperr.ErrContestEventInvalid); err != nil {
		return err
	}
	return s.HandleJudgeFailed(ctx, event)
}
