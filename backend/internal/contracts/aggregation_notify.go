// contracts 定义第 3 层通知模块对其他模块开放的站内信与实时推送契约。
package contracts

import (
	"encoding/json"
)

// NotifySendRequest 是模块发送站内信时提交给 M10 的统一请求。
type NotifySendRequest struct {
	TenantID  int64             `json:"tenant_id"`
	Type      string            `json:"type"`
	Receivers []int64           `json:"receivers"`
	Params    map[string]string `json:"params"`
	Link      string            `json:"link"`
}

// NotifyPushRequest 是模块通过 M10 向业务主题推送实时消息的统一请求。
type NotifyPushRequest struct {
	TenantID int64 `json:"tenant_id"`
	// Topic 必须使用 M10 唯一实时 topic 规范:
	// tenant:{tenant_id}:notify:{account_id}、tenant:{tenant_id}:alert、
	// tenant:{tenant_id}:{contest|sandbox|sim|experiment|course|judge}:{resource_id}:{channel}。
	// 不得回退到无租户前缀 topic,否则无法在 M10 边界独立校验租户隔离。
	Topic   string          `json:"topic"`
	Payload json.RawMessage `json:"payload"`
}
