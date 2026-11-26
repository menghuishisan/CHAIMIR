package sandbox

import (
	"context"
	"errors"
	"fmt"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/eventbus"
	"chaimir/pkg/apperr"
)

// RegisterEventSubscriptions 订阅租户配额初始化及各副本的沙箱会话撤权事件。
func RegisterEventSubscriptions(bus eventbus.Bus, svc *Service) error {
	if bus == nil || svc == nil || svc.wsHub == nil {
		return fmt.Errorf("sandbox event subscriptions require initialized bus and service")
	}
	_, err := bus.Subscribe(contracts.SubjectTenantProvisioned, "sandbox-tenant-provision", func(ctx context.Context, data []byte) error {
		var event contracts.TenantProvisionedEvent
		if err := eventbus.Decode(data, &event, apperr.ErrSandboxQuotaInvalid); err != nil {
			return err
		}
		return svc.EnsureTenantQuota(ctx, event.TenantID)
	})
	if err != nil {
		return fmt.Errorf("sandbox subscribe tenant provision: %w", err)
	}
	fanout, ok := bus.(eventbus.FanoutBus)
	if !ok {
		return apperr.ErrEventBusMissing
	}
	if _, err := fanout.SubscribeFanout(contracts.SubjectSandboxSessionsRevoked, handleSandboxSessionsRevoked(svc.wsHub)); err != nil {
		return fmt.Errorf("sandbox subscribe session revocation: %w", err)
	}
	return nil
}

func publishSandboxSessionsRevoked(ctx context.Context, bus eventbus.Bus, tenantID, sandboxID int64, accountIDs []int64) error {
	if len(accountIDs) == 0 {
		return nil
	}
	fanout, ok := bus.(eventbus.FanoutBus)
	if !ok {
		return apperr.ErrEventBusMissing
	}
	sessions := make([]contracts.SandboxSessionRef, 0, len(accountIDs))
	for _, accountID := range accountIDs {
		if tenantID <= 0 || sandboxID <= 0 || accountID <= 0 {
			return apperr.ErrSandboxContractRequestInvalid
		}
		sessions = append(sessions, contracts.SandboxSessionRef{SandboxID: sandboxID, AccountID: accountID})
	}
	return fanout.PublishFanout(context.WithoutCancel(ctx), contracts.SubjectSandboxSessionsRevoked, contracts.SandboxSessionsRevokedEvent{TenantID: tenantID, Sessions: sessions})
}

func handleSandboxSessionsRevoked(hub sandboxSessionCloser) eventbus.Handler {
	return func(_ context.Context, data []byte) error {
		if hub == nil {
			return apperr.ErrSandboxToolProxyUnavailable
		}
		var event contracts.SandboxSessionsRevokedEvent
		if err := eventbus.Decode(data, &event, apperr.ErrSandboxContractRequestInvalid); err != nil {
			return err
		}
		if event.TenantID <= 0 || len(event.Sessions) == 0 {
			return apperr.ErrSandboxContractRequestInvalid
		}
		var failures []error
		for _, session := range event.Sessions {
			if err := closeSandboxSessionsFor(hub, event.TenantID, session.AccountID, session.SandboxID); err != nil {
				failures = append(failures, err)
			}
		}
		return errors.Join(failures...)
	}
}
