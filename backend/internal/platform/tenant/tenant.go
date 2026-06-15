// tenant 提供租户身份上下文注入与读取,供鉴权、RLS 与审计统一复用。
package tenant

import "context"

type ctxKey struct{}

// Identity 是经服务端鉴权后确立的租户身份上下文。
type Identity struct {
	TenantID   int64
	AccountID  int64
	IsPlatform bool
	IsSystem   bool
}

// WithContext 把已验证身份注入上下文,供下游基础设施读取。
func WithContext(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// FromContext 读取已注入的租户身份;缺失时返回 ok=false。
func FromContext(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(ctxKey{}).(Identity)
	return id, ok
}
