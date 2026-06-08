// M5 数据访问层:只读写 content 模块自有表,跨校共享读取限定在受控特权查询。
package content

import (
	"context"

	"chaimir/internal/modules/content/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/tenant"

	"github.com/jackc/pgx/v5"
)

// repo 封装 content 模块数据库事务入口。
type repo struct {
	db *db.DB
}

// newRepo 构造 M5 repo。
func newRepo(database *db.DB) *repo {
	return &repo{db: database}
}

// queryFunc 是 M5 数据访问闭包,统一接收 sqlc 查询对象。
type queryFunc func(q *sqlcgen.Queries) error

// inTenant 从 ctx 取租户并注入 RLS 后执行查询。
func (r *repo) inTenant(ctx context.Context, fn queryFunc) error {
	return r.db.WithTenantTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// inTenantID 用显式租户 ID 注入 RLS,供 contracts 内部调用使用。
func (r *repo) inTenantID(ctx context.Context, tenantID int64, fn queryFunc) error {
	return r.db.WithTenantTxID(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// inPrivileged 执行共享库受控跨租户读取,调用方必须在 SQL 层限定 shared+published。
func (r *repo) inPrivileged(ctx context.Context, fn queryFunc) error {
	return r.db.WithPrivilegedTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// hasPrivileged 返回是否配置特权池。
func (r *repo) hasPrivileged() bool { return r.db.HasPrivileged() }

// tenantFromContext 读取当前租户身份。
func tenantFromContext(ctx context.Context) (tenant.Identity, bool) {
	return tenant.FromContext(ctx)
}
