// contest events 文件负责订阅跨模块事件、解码载荷并转交 service 处理。
package contest

import (
	"context"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/eventbus"
	"chaimir/pkg/apperr"
)

const contestEventQueue = "contest-workers"

// SubscribeEvents 注册竞赛模块需要消费的跨模块事件。
func SubscribeEvents(bus eventbus.Bus, svc *Service) ([]eventbus.Subscription, error) {
	if bus == nil {
		return nil, apperr.ErrEventBusMissing
	}
	if svc == nil {
		return nil, apperr.ErrEventServiceMissing
	}
	fanout, ok := bus.(eventbus.FanoutBus)
	if !ok {
		return nil, apperr.ErrEventBusMissing
	}
	subs := make([]eventbus.Subscription, 0, 3)
	sub, err := bus.Subscribe(contracts.SubjectJudgeCompleted, contestEventQueue, handleJudgeCompletedEvent(svc))
	if err != nil {
		return nil, apperr.ErrContestEventSubscribeFailed.WithCause(err)
	}
	subs = append(subs, sub)
	sub, err = bus.Subscribe(contracts.SubjectJudgeFailed, contestEventQueue, handleJudgeFailedEvent(svc))
	if err != nil {
		return nil, apperr.ErrContestEventSubscribeFailed.WithCause(err)
	}
	subs = append(subs, sub)
	sub, err = fanout.SubscribeFanout(contracts.SubjectContestSandboxSessionsRevoked, handleContestSandboxSessionsRevoked(svc))
	if err != nil {
		return nil, apperr.ErrContestEventSubscribeFailed.WithCause(err)
	}
	subs = append(subs, sub)
	return subs, nil
}

// handleContestSandboxSessionsRevoked 关闭当前副本上绑定到撤销授权的连接。
func handleContestSandboxSessionsRevoked(svc *Service) eventbus.Handler {
	return func(ctx context.Context, data []byte) error {
		if svc == nil || svc.wsHub == nil {
			return apperr.ErrSandboxToolProxyUnavailable
		}
		var event contracts.ContestSandboxSessionsRevokedEvent
		if err := eventbus.Decode(data, &event, apperr.ErrContestEventPayloadInvalid); err != nil {
			return err
		}
		if event.TenantID <= 0 || len(event.Sessions) == 0 {
			return apperr.ErrContestEventPayloadInvalid
		}
		return closeContestSandboxSessions(svc.wsHub, event.Sessions)
	}
}

// handleJudgeCompletedEvent 解码 M3 判题完成事件。
func handleJudgeCompletedEvent(svc *Service) eventbus.Handler {
	return func(ctx context.Context, data []byte) error {
		var event contracts.JudgeCompletedEvent
		if err := eventbus.Decode(data, &event, apperr.ErrContestEventPayloadInvalid); err != nil {
			return err
		}
		if !auth.ValidSourceRef(event.SourceRef) {
			return apperr.ErrContestEventPayloadInvalid
		}
		if !strings.HasPrefix(strings.TrimSpace(event.SourceRef), "contest:") {
			return nil
		}
		if strings.Contains(event.SourceRef, ":submission:") {
			return svc.HandleSolveJudgeCompleted(ctx, event)
		}
		if strings.Contains(event.SourceRef, ":battle:") {
			return svc.HandleBattleJudgeCompleted(ctx, event)
		}
		return apperr.ErrContestEventSourceMismatch
	}
}

// handleJudgeFailedEvent 解码 M3 判题失败事件。
func handleJudgeFailedEvent(svc *Service) eventbus.Handler {
	return func(ctx context.Context, data []byte) error {
		var event contracts.JudgeFailedEvent
		if err := eventbus.Decode(data, &event, apperr.ErrContestEventPayloadInvalid); err != nil {
			return err
		}
		if !auth.ValidSourceRef(event.SourceRef) {
			return apperr.ErrContestEventPayloadInvalid
		}
		if !strings.HasPrefix(strings.TrimSpace(event.SourceRef), "contest:") {
			return nil
		}
		if strings.Contains(event.SourceRef, ":submission:") {
			return svc.HandleSolveJudgeFailed(ctx, event)
		}
		if strings.Contains(event.SourceRef, ":battle:") {
			return svc.HandleBattleJudgeFailed(ctx, event)
		}
		return apperr.ErrContestEventSourceMismatch
	}
}
