// Package audit 提供统一审计写入辅助与请求审计上下文。
// 依据 CLAUDE.md §3/§7:任何模块写审计都经此写入 identity 的全平台唯一 audit_log 表,
//
//	不自建审计表、不绕过。audit 包定义 Writer 接口,identity(M1)提供实现;
//	main.go 装配时注入,其他模块经 contracts 拿到能力 —— 避免反向 import identity。
package audit

import "context"

// Entry 是一条审计记录(字段对齐 identity 的 audit_log 表)。
type Entry struct {
	TenantID   int64  // 租户范围(平台级操作可为 0)。
	ActorID    int64  // 操作者账号 ID。
	ActorRole  int16  // 操作时角色。
	Action     string // 动作码,如 account.import / auth.login。
	TargetType string // 对象类型(兼来源模块标识)。
	TargetID   int64  // 目标资源 ID。
	Detail     string // 结构化详情(JSON 字符串);敏感值脱敏。
	IP         string // 操作来源 IP。
	TraceID    string // 关联 trace_id。
}

// Writer 是审计写入能力契约;由 identity(M1)实现,写入 audit_log。
type Writer interface {
	Write(ctx context.Context, e Entry) error
}

type requestContextKey struct{}

// RequestContext 是 HTTP/API 边界注入的审计请求元数据。
// 这些字段是横切信息,不属于任何业务模块;放在 platform/audit 可避免各模块私建一套 context key。
type RequestContext struct {
	IP      string
	TraceID string
}

// WithRequestContext 把请求元数据写入 context,供业务成功后构造审计条目。
// 审计只在操作成功后写入,因此这里仅携带横切元数据,不提前推断业务动作或目标对象。
func WithRequestContext(ctx context.Context, req RequestContext) context.Context {
	return context.WithValue(ctx, requestContextKey{}, req)
}

// RequestContextFrom 读取请求审计元数据;缺失时返回空值,由调用方决定是否仍可写审计。
// 非 HTTP 入口(脚本/事件消费者)可能没有 IP,但仍应保留服务端已知的 actor/action 信息。
func RequestContextFrom(ctx context.Context) RequestContext {
	req, _ := ctx.Value(requestContextKey{}).(RequestContext)
	return req
}
