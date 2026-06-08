// 第3层 聚合/横切:notify(M10)对外接口契约。
// 依据铁律:任何模块发通知都经 contracts.NotifyService(不自写通知表/不自建 WS 广播)。
// 当前文件定义 M10 已对外承诺的最小通知契约,供各模块经 contracts 调用。
package contracts

import "context"

// NotifySendRequest 是模块发送站内信的统一请求。
// Type 对应 notification_template.type,Params 用于模板变量渲染。
type NotifySendRequest struct {
	TenantID  int64
	Type      string
	Receivers []int64
	Params    map[string]string
	Link      string
}

// NotifyPushRequest 是模块推送业务实时数据的统一请求。
// Topic 使用不含租户前缀的业务 topic,M10 负责按 tenant_id 做隔离。
type NotifyPushRequest struct {
	TenantID int64
	Topic    string
	Payload  map[string]any
}

// NotifyService 是 notify 模块对外提供的通知能力。
type NotifyService interface {
	// Send 渲染模板并发送站内信,同时推送个人未读红点。
	Send(ctx context.Context, req NotifySendRequest) error
	// Push 向订阅 topic 的在线连接推送业务实时数据。
	Push(ctx context.Context, req NotifyPushRequest) error
}
