// M8 事件处理:订阅 M3 判题结果,回写解题赛提交并刷新排行榜。
package contest

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/eventbus"
	"chaimir/pkg/apperr"
)

// SubscribeEvents 订阅 M3 判题事件。
func (s *Service) SubscribeEvents() error {
	if s.bus == nil {
		return apperr.ErrContestEventSubscribeFailed
	}
	if _, err := s.bus.Subscribe(contracts.SubjectJudgeCompleted, "contest", s.onJudgeCompleted); err != nil {
		return apperr.ErrContestEventSubscribeFailed.WithCause(err)
	}
	if _, err := s.bus.Subscribe(contracts.SubjectJudgeFailed, "contest", s.onJudgeFailed); err != nil {
		return apperr.ErrContestEventSubscribeFailed.WithCause(err)
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
