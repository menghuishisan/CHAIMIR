// sandbox events 文件负责注册 M2 事件订阅并把事件解码后委托 service。
package sandbox

import (
	"context"
	"fmt"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/eventbus"
	"chaimir/pkg/apperr"
)

// RegisterEventSubscriptions 订阅新租户初始化事件并建立 M2 自有配额基线。
func RegisterEventSubscriptions(bus eventbus.Bus, svc *Service) error {
	if bus == nil || svc == nil {
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
	return nil
}
