// M6 事件处理:订阅并解码 M3 判题完成/失败事件,实际业务回写委托服务层完成。
package teaching

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/eventbus"
	"chaimir/pkg/apperr"
)

// SubscribeEvents 订阅 M3 判题事件。
func (s *Service) SubscribeEvents() error {
	if s.bus == nil {
		return apperr.ErrSubmissionEventSubscribeFailed
	}
	if _, err := s.bus.Subscribe(contracts.SubjectJudgeCompleted, "teaching", s.onJudgeCompleted); err != nil {
		return apperr.ErrSubmissionEventSubscribeFailed.WithCause(err)
	}
	if _, err := s.bus.Subscribe(contracts.SubjectJudgeFailed, "teaching", s.onJudgeFailed); err != nil {
		return apperr.ErrSubmissionEventSubscribeFailed.WithCause(err)
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
