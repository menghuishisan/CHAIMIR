// teaching events 文件负责订阅跨模块事件、解码载荷并转交 service 处理。
package teaching

import (
	"context"
	"fmt"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/eventbus"
	"chaimir/pkg/apperr"
)

const teachingEventQueue = "teaching-workers"

// SubscribeEvents 注册教学模块需要消费的跨模块事件。
func SubscribeEvents(bus eventbus.Bus, svc *Service) ([]eventbus.Subscription, error) {
	if bus == nil {
		return nil, apperr.ErrEventBusMissing
	}
	if svc == nil {
		return nil, apperr.ErrEventServiceMissing
	}
	subs := make([]eventbus.Subscription, 0, 3)
	if err := subscribeTeachingEvent(bus, &subs, contracts.SubjectJudgeCompleted, handleJudgeCompletedEvent(svc)); err != nil {
		return nil, err
	}
	if err := subscribeTeachingEvent(bus, &subs, contracts.SubjectJudgeFailed, handleJudgeFailedEvent(svc)); err != nil {
		return nil, err
	}
	if err := subscribeTeachingEvent(bus, &subs, contracts.SubjectGradeReviewLockChanged, handleGradeLockChangedEvent(svc)); err != nil {
		return nil, err
	}
	return subs, nil
}

// subscribeTeachingEvent 注册单个主题并保留订阅句柄。
func subscribeTeachingEvent(bus eventbus.Bus, subs *[]eventbus.Subscription, subject string, handler eventbus.Handler) error {
	sub, err := bus.Subscribe(subject, teachingEventQueue, handler)
	if err != nil {
		return fmt.Errorf("订阅 teaching 事件 %s 失败: %w", subject, err)
	}
	*subs = append(*subs, sub)
	return nil
}

// handleJudgeCompletedEvent 解码 M3 判题完成事件。
func handleJudgeCompletedEvent(svc *Service) eventbus.Handler {
	return func(ctx context.Context, data []byte) error {
		var event contracts.JudgeCompletedEvent
		if err := eventbus.Decode(data, &event, apperr.ErrTeachingSubmissionInvalid); err != nil {
			return err
		}
		return svc.HandleJudgeCompleted(ctx, event)
	}
}

// handleJudgeFailedEvent 解码 M3 判题失败事件。
func handleJudgeFailedEvent(svc *Service) eventbus.Handler {
	return func(ctx context.Context, data []byte) error {
		var event contracts.JudgeFailedEvent
		if err := eventbus.Decode(data, &event, apperr.ErrTeachingSubmissionInvalid); err != nil {
			return err
		}
		return svc.HandleJudgeFailed(ctx, event)
	}
}

// handleGradeLockChangedEvent 解码 M11 成绩锁变化事件。
func handleGradeLockChangedEvent(svc *Service) eventbus.Handler {
	return func(ctx context.Context, data []byte) error {
		var event contracts.GradeReviewLockChangedEvent
		if err := eventbus.Decode(data, &event, apperr.ErrTeachingGradeInvalid); err != nil {
			return err
		}
		return svc.HandleGradeLockChanged(ctx, event)
	}
}
