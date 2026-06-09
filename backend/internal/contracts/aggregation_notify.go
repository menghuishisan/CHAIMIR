// contracts 定义第 3 层通知模块对其他模块开放的站内信与实时推送契约。
package contracts

import "context"

// NotifySendRequest 是模块发送站内信时提交给 M10 的统一请求。
type NotifySendRequest struct {
	TenantID  int64
	Type      string
	Receivers []int64
	Params    map[string]string
	Link      string
}

// NotifyPushRequest 是模块通过 M10 向业务主题推送实时消息的统一请求。
type NotifyPushRequest struct {
	TenantID int64
	Topic    string
	Payload  map[string]any
}

// NotifyService 是 M10 对全平台开放的通知与实时推送契约。
type NotifyService interface {
	// Send 渲染通知模板并写站内信,必要时同步推送红点。
	Send(ctx context.Context, req NotifySendRequest) error
	// Push 把业务实时负载投递到统一 WebSocket 主题。
	Push(ctx context.Context, req NotifyPushRequest) error
}
