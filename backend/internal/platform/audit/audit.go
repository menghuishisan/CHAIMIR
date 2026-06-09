// audit 提供统一审计写入契约与请求级审计上下文,供各模块复用同一条审计链路。
package audit

import "context"

// Entry 表示一条要写入 M1 audit_log 的审计记录。
type Entry struct {
	TenantID   int64
	ActorID    int64
	ActorRole  int16
	Action     string
	TargetType string
	TargetID   int64
	Detail     string
	IP         string
	TraceID    string
}

// Writer 是审计写入能力契约,由 identity 模块实现并在装配时注入。
type Writer interface {
	// Write 写入一条审计记录到全平台唯一 audit_log。
	Write(ctx context.Context, e Entry) error
}

type requestContextKey struct{}

// RequestContext 是从 HTTP 或事件入口抽出的横切审计元数据。
type RequestContext struct {
	IP      string
	TraceID string
}

// WithRequestContext 把请求元数据写入 context,供业务成功后构造审计条目。
func WithRequestContext(ctx context.Context, req RequestContext) context.Context {
	return context.WithValue(ctx, requestContextKey{}, req)
}

// RequestContextFrom 读取请求元数据;缺失时返回零值。
func RequestContextFrom(ctx context.Context) RequestContext {
	req, _ := ctx.Value(requestContextKey{}).(RequestContext)
	return req
}
