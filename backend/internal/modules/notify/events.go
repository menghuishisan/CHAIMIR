// notify events 文件负责把统一事件总线接入 M10 通知发送和实时推送。
package notify

import (
	"context"
	"fmt"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/ids"
	"chaimir/pkg/apperr"
)

// SubscribeEvents 注册 M10 事件订阅。
func SubscribeEvents(bus eventbus.Bus, svc *Service) ([]eventbus.Subscription, error) {
	if bus == nil || svc == nil {
		return nil, apperr.ErrNotifyChannelUnavailable
	}
	var subs []eventbus.Subscription
	revokeSub, err := bus.Subscribe(contracts.SubjectIdentitySessionRevoked, "notify-session", func(ctx context.Context, data []byte) error {
		var evt contracts.IdentitySessionRevokedEvent
		if err := eventbus.Decode(data, &evt, apperr.ErrNotifySubscribeInvalid); err != nil {
			return err
		}
		return svc.CloseSession(ctx, evt.TenantID, evt.AccountID)
	})
	if err != nil {
		return nil, apperr.ErrNotifyChannelUnavailable.WithCause(err)
	}
	subs = append(subs, revokeSub)
	sendSub, err := bus.Subscribe(contracts.SubjectNotifySendRequested, "notify-send", func(ctx context.Context, data []byte) error {
		var evt contracts.NotifySendRequestedEvent
		if err := eventbus.Decode(data, &evt, apperr.ErrNotifyRequestInvalid); err != nil {
			return err
		}
		return svc.Send(ctx, contracts.NotifySendRequest{RequestID: evt.RequestID, TenantID: evt.TenantID, Type: evt.Type, Receivers: evt.Receivers, Params: evt.Params, Link: evt.Link})
	})
	if err != nil {
		return nil, apperr.ErrNotifyChannelUnavailable.WithCause(err)
	}
	subs = append(subs, sendSub)
	scoreSub, err := bus.Subscribe(contracts.SubjectExperimentScored, "notify-experiment-score", func(ctx context.Context, data []byte) error {
		var evt contracts.ExperimentScoredEvent
		if err := eventbus.Decode(data, &evt, apperr.ErrNotifyRequestInvalid); err != nil {
			return err
		}
		if _, ok := ids.Parse(strings.TrimSpace(evt.EventID)); !ok || strings.TrimSpace(evt.ExperimentName) == "" || evt.StudentID <= 0 || evt.ScoredAt.IsZero() {
			return apperr.ErrNotifyRequestInvalid
		}
		requestID := fmt.Sprintf("experiment:%04d:score:%s", evt.ScoredAt.Year(), evt.EventID)
		return svc.Send(ctx, contracts.NotifySendRequest{RequestID: requestID, TenantID: evt.TenantID, Type: "experiment.completed", Receivers: []int64{evt.StudentID}, Params: map[string]string{"experiment": evt.ExperimentName}, Link: "/student/experiment-detail?id=" + ids.Format(evt.ExperimentID)})
	})
	if err != nil {
		return nil, apperr.ErrNotifyChannelUnavailable.WithCause(err)
	}
	subs = append(subs, scoreSub)
	pushSub, err := bus.Subscribe(contracts.SubjectNotifyPushRequested, "notify-push", func(ctx context.Context, data []byte) error {
		var evt contracts.NotifyPushRequestedEvent
		if err := eventbus.Decode(data, &evt, apperr.ErrNotifySubscribeInvalid); err != nil {
			return err
		}
		return svc.Push(ctx, contracts.NotifyPushRequest{TenantID: evt.TenantID, Topic: evt.Topic, Payload: evt.Payload})
	})
	if err != nil {
		return nil, apperr.ErrNotifyChannelUnavailable.WithCause(err)
	}
	subs = append(subs, pushSub)
	return subs, nil
}
