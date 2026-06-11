// sandbox events 文件负责注册 M2 事件订阅并把事件解码后委托 service。
package sandbox

import (
	"fmt"

	"chaimir/internal/platform/eventbus"
)

// RegisterEventSubscriptions 注册沙箱模块事件订阅;当前 M2 只发布 sandbox.recycled,不消费上层事件。
func RegisterEventSubscriptions(bus eventbus.Bus, svc *Service) error {
	if bus == nil || svc == nil {
		return fmt.Errorf("sandbox event subscriptions require initialized bus and service")
	}
	return nil
}
