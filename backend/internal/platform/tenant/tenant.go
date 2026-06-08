// Package tenant 提供租户上下文的注入与读取。
// 依据 docs/总-数据库表总览.md §4:多租户经 RLS 隔离,每请求 SET app.tenant_id;
//   租户 ID 从服务端会话确立,不接受客户端传参(CLAUDE.md §7)。
package tenant

import "context"

type ctxKey struct{}

// Identity 是经鉴权确立的租户上下文身份。
type Identity struct {
	TenantID   int64 // 租户(学校)ID;RLS 隔离键。
	AccountID  int64 // 当前账号 ID。
	IsPlatform bool  // 平台管理员上下文(可访问平台级表)。
}

// WithContext 把租户身份注入 context。
func WithContext(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// FromContext 取租户身份;ok=false 表示未鉴权上下文。
func FromContext(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(ctxKey{}).(Identity)
	return id, ok
}
