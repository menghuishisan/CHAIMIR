// notify events 文件负责把统一事件总线接入 M10 通知发送和实时推送。
package notify

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/eventbus"
	"chaimir/pkg/apperr"
)

// SubscribeEvents 注册 M10 事件订阅。
func SubscribeEvents(bus eventbus.Bus, svc *Service) ([]eventbus.Subscription, error) {
	if bus == nil || svc == nil {
		return nil, apperr.ErrNotifyChannelUnavailable
	}
	var subs []eventbus.Subscription
	sendSub, err := bus.Subscribe(contracts.SubjectNotifySend, "notify-send", func(ctx context.Context, data []byte) error {
		var req contracts.NotifySendRequest
		if err := eventbus.Decode(data, &req, apperr.ErrNotifyRequestInvalid); err != nil {
			return err
		}
		return svc.Send(ctx, req)
	})
	if err != nil {
		return nil, apperr.ErrNotifyChannelUnavailable.WithCause(err)
	}
	subs = append(subs, sendSub)
	pushSub, err := bus.Subscribe(contracts.SubjectNotifyPush, "notify-push", func(ctx context.Context, data []byte) error {
		var req contracts.NotifyPushRequest
		if err := eventbus.Decode(data, &req, apperr.ErrNotifySubscribeInvalid); err != nil {
			return err
		}
		return svc.Push(ctx, req)
	})
	if err != nil {
		return nil, apperr.ErrNotifyChannelUnavailable.WithCause(err)
	}
	subs = append(subs, pushSub)
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
	return subs, nil
}
