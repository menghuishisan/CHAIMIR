// experiment events 文件负责订阅跨模块事件、解码载荷并转交 service 处理。
package experiment

import (
	"context"
	"fmt"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/eventbus"
	"chaimir/pkg/apperr"
)

const experimentEventQueue = "experiment-workers"

// SubscribeEvents 注册实验模块需要消费的跨模块事件。
func SubscribeEvents(bus eventbus.Bus, svc *Service) ([]eventbus.Subscription, error) {
	if bus == nil {
		return nil, apperr.ErrEventBusMissing
	}
	if svc == nil {
		return nil, apperr.ErrEventServiceMissing
	}
	subs := make([]eventbus.Subscription, 0, 3)
	if err := subscribeExperimentEvent(bus, &subs, contracts.SubjectJudgeCompleted, handleJudgeCompletedEvent(svc)); err != nil {
		return nil, err
	}
	if err := subscribeExperimentEvent(bus, &subs, contracts.SubjectJudgeFailed, handleJudgeFailedEvent(svc)); err != nil {
		return nil, err
	}
	if err := subscribeExperimentEvent(bus, &subs, contracts.SubjectSandboxRecycled, handleSandboxRecycledEvent(svc)); err != nil {
		return nil, err
	}
	return subs, nil
}

// subscribeExperimentEvent 注册单个主题并保留订阅句柄。
func subscribeExperimentEvent(bus eventbus.Bus, subs *[]eventbus.Subscription, subject string, handler eventbus.Handler) error {
	sub, err := bus.Subscribe(subject, experimentEventQueue, handler)
	if err != nil {
		return fmt.Errorf("订阅 experiment 事件 %s 失败: %w", subject, err)
	}
	*subs = append(*subs, sub)
	return nil
}

// handleJudgeCompletedEvent 解码 M3 判题完成事件。
func handleJudgeCompletedEvent(svc *Service) eventbus.Handler {
	return func(ctx context.Context, data []byte) error {
		var event contracts.JudgeCompletedEvent
		if err := eventbus.Decode(data, &event, apperr.ErrExperimentCheckpointInvalid); err != nil {
			return err
		}
		if !auth.ValidSourceRef(event.SourceRef) {
			return apperr.ErrExperimentCheckpointInvalid
		}
		if !strings.HasPrefix(strings.TrimSpace(event.SourceRef), "experiment:") {
			return nil
		}
		return svc.HandleJudgeCompleted(ctx, event)
	}
}

// handleJudgeFailedEvent 解码 M3 判题失败事件。
func handleJudgeFailedEvent(svc *Service) eventbus.Handler {
	return func(ctx context.Context, data []byte) error {
		var event contracts.JudgeFailedEvent
		if err := eventbus.Decode(data, &event, apperr.ErrExperimentCheckpointInvalid); err != nil {
			return err
		}
		if !auth.ValidSourceRef(event.SourceRef) {
			return apperr.ErrExperimentCheckpointInvalid
		}
		if !strings.HasPrefix(strings.TrimSpace(event.SourceRef), "experiment:") {
			return nil
		}
		return svc.HandleJudgeFailed(ctx, event)
	}
}

// handleSandboxRecycledEvent 解码 M2 沙箱回收事件。
func handleSandboxRecycledEvent(svc *Service) eventbus.Handler {
	return func(ctx context.Context, data []byte) error {
		var event contracts.SandboxRecycledEvent
		if err := eventbus.Decode(data, &event, apperr.ErrExperimentInstanceInvalid); err != nil {
			return err
		}
		return svc.HandleSandboxRecycled(ctx, event)
	}
}

// HandleSandboxRecycled 消费 M2 回收事件并将仍在进行的实例标记为环境已释放。
func (s *Service) HandleSandboxRecycled(ctx context.Context, event contracts.SandboxRecycledEvent) error {
	if event.TenantID <= 0 || !validExperimentSourceRef(event.SourceRef) {
		return apperr.ErrExperimentSourceRefInvalid
	}
	return s.store.TenantTx(ctx, event.TenantID, func(ctx context.Context, tx TxStore) error {
		inst, err := tx.GetInstanceBySourceRef(ctx, event.TenantID, event.SourceRef)
		if err != nil {
			return err
		}
		if inst.Status == InstanceStatusRunning || inst.Status == InstanceStatusPaused || inst.Status == InstanceStatusCreating {
			_, err = tx.SetInstanceStatus(ctx, event.TenantID, inst.ID, InstanceStatusReleased)
			return err
		}
		return nil
	})
}
