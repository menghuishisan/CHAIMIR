// M11 事件消费:订阅 M6 成绩变更事件,重算 GPA 并完成申诉闭环。
package grade

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/eventbus"
	"chaimir/pkg/apperr"
)

// SubscribeEvents 订阅成绩中心需要消费的事件。
func (s *Service) SubscribeEvents(bus eventbus.Bus) error {
	if bus == nil {
		return apperr.ErrGradeAggregateFailed
	}
	if _, err := bus.Subscribe(contracts.SubjectTeachingGradeUpdated, "grade", s.onTeachingGradeUpdated); err != nil {
		return apperr.ErrGradeAggregateFailed.WithCause(err)
	}
	return nil
}

// onTeachingGradeUpdated 解码 M6 成绩变更事件。
func (s *Service) onTeachingGradeUpdated(ctx context.Context, data []byte) error {
	var event contracts.TeachingGradeUpdatedEvent
	if err := eventbus.Decode(data, &event, apperr.ErrGradeAggregateFailed); err != nil {
		return err
	}
	return s.HandleTeachingGradeUpdated(ctx, event)
}
