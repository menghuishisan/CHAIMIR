// sandbox events 文件负责注册 M2 事件订阅并把事件解码后委托 service。
package sandbox

import (
	"fmt"

	"chaimir/internal/platform/eventbus"
)

// RegisterEventSubscriptions 校验 M2 事件装配入口;M2 只发布 sandbox.recycled,不订阅上层事件。
func RegisterEventSubscriptions(bus eventbus.Bus, svc *Service) error {
	if bus == nil || svc == nil {
		return fmt.Errorf("sandbox event subscriptions require initialized bus and service")
	}
	return nil
}
