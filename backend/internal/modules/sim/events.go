package sim

import (
	"context"
	"errors"
	"fmt"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/eventbus"
	"chaimir/pkg/apperr"
)

// RegisterEventSubscriptions 让每个在线副本复核变更会话，延迟信号不携带过期授权。
func RegisterEventSubscriptions(svc *Service) error {
	if svc == nil || svc.bus == nil || svc.wsHub == nil {
		return apperr.ErrEventBusMissing
	}
	_, err := svc.bus.SubscribeFanout(contracts.SubjectSimSessionChanged, func(ctx context.Context, data []byte) error {
		var event contracts.SimSessionChangedEvent
		if err := eventbus.Decode(data, &event, apperr.ErrSimSessionInvalid); err != nil {
			return err
		}
		if event.TenantID <= 0 || event.SessionID <= 0 {
			return apperr.ErrSimSessionInvalid
		}
		var allowed, executing []int64
		if err := svc.store.TenantTx(ctx, event.TenantID, func(ctx context.Context, tx TxStore) error {
			session, err := tx.GetSession(ctx, event.TenantID, event.SessionID)
			if isNoRows(err) {
				return nil
			}
			if err != nil {
				return apperr.ErrSimSessionQueryFailed.WithCause(err)
			}
			if session.Status == SessionAvailable || session.Status == SessionArchived {
				allowed = append(allowed, session.OwnerAccountID)
				allowed = append(allowed, session.SharedAccountIDs...)
			}
			if session.Status == SessionAvailable {
				executing = allowed
			}
			return nil
		}); err != nil {
			return err
		}
		return errors.Join(
			svc.wsHub.RetainScopeSessions(event.TenantID, simSessionSessionScope(event.SessionID), executing),
			svc.wsHub.RetainScopeSessions(event.TenantID, simReplaySessionScope(event.SessionID), allowed),
		)
	})
	if err != nil {
		return fmt.Errorf("订阅仿真会话变更失败: %w", err)
	}
	return nil
}

func (s *Service) publishSessionChanged(ctx context.Context, tenantID, sessionID int64) error {
	if s.bus == nil {
		return apperr.ErrEventBusMissing
	}
	return s.bus.PublishFanout(context.WithoutCancel(ctx), contracts.SubjectSimSessionChanged, contracts.SimSessionChangedEvent{TenantID: tenantID, SessionID: sessionID})
}
