// M7 事件处理:订阅 M3 判题结果与 M2 沙箱回收事件,并发布实验得分事件。
package experiment

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/eventbus"
	"chaimir/pkg/apperr"
)

// SubscribeEvents 订阅 M3 判题事件与 M2 沙箱回收事件。
func (s *Service) SubscribeEvents() error {
	if s.bus == nil {
		return apperr.ErrExperimentEventSubscribeFailed
	}
	if _, err := s.bus.Subscribe(contracts.SubjectJudgeCompleted, "experiment", s.onJudgeCompleted); err != nil {
		return apperr.ErrExperimentEventSubscribeFailed.WithCause(err)
	}
	if _, err := s.bus.Subscribe(contracts.SubjectJudgeFailed, "experiment", s.onJudgeFailed); err != nil {
		return apperr.ErrExperimentEventSubscribeFailed.WithCause(err)
	}
	if _, err := s.bus.Subscribe(contracts.SubjectSandboxRecycled, "experiment", s.onSandboxRecycled); err != nil {
		return apperr.ErrExperimentEventSubscribeFailed.WithCause(err)
	}
	return nil
}

// onJudgeCompleted 解码判题完成事件。
func (s *Service) onJudgeCompleted(ctx context.Context, data []byte) error {
	var event contracts.JudgeCompletedEvent
	if err := eventbus.Decode(data, &event, apperr.ErrExperimentEventInvalid); err != nil {
		return err
	}
	return s.HandleJudgeCompleted(ctx, event)
}

// onJudgeFailed 解码判题失败事件。
func (s *Service) onJudgeFailed(ctx context.Context, data []byte) error {
	var event contracts.JudgeFailedEvent
	if err := eventbus.Decode(data, &event, apperr.ErrExperimentEventInvalid); err != nil {
		return err
	}
	return s.HandleJudgeFailed(ctx, event)
}

// onSandboxRecycled 解码沙箱回收事件。
func (s *Service) onSandboxRecycled(ctx context.Context, data []byte) error {
	var event contracts.SandboxRecycledEvent
	if err := eventbus.Decode(data, &event, apperr.ErrExperimentEventInvalid); err != nil {
		return err
	}
	return s.HandleSandboxRecycled(ctx, event)
}
