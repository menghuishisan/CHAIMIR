// grade events 文件负责订阅教学成绩更新事件并驱动 M11 聚合重算。
package grade

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/eventbus"
	"chaimir/pkg/apperr"
)

// SubscribeEvents 注册 M11 事件订阅。
func SubscribeEvents(bus eventbus.Bus, svc *Service) ([]eventbus.Subscription, error) {
	if bus == nil || svc == nil {
		return nil, apperr.ErrGradeAggregationFailed
	}
	sub, err := bus.Subscribe(contracts.SubjectTeachingGradeUpdated, "grade-center", func(ctx context.Context, data []byte) error {
		var evt contracts.TeachingGradeUpdatedEvent
		if err := eventbus.Decode(data, &evt, apperr.ErrGradeAggregationFailed); err != nil {
			return err
		}
		return svc.HandleGradeUpdated(ctx, evt)
	})
	if err != nil {
		return nil, apperr.ErrGradeAggregationFailed.WithCause(err)
	}
	return []eventbus.Subscription{sub}, nil
}
