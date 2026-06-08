// M2 数据访问层(repo):只读写 sandbox 模块自有表,全部经 sqlc 生成查询。
package sandbox

import (
	"context"

	"chaimir/internal/modules/sandbox/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/tenant"

	"github.com/jackc/pgx/v5"
)

// repo 封装 sandbox 模块数据库事务入口。
type repo struct {
	db *db.DB
}

// newRepo 绑定平台数据库入口,所有查询仍通过显式事务方法进入。
func newRepo(database *db.DB) *repo {
	return &repo{db: database}
}

// queryFunc 是在 sqlc 查询对象上执行的事务闭包。
type queryFunc func(q *sqlcgen.Queries) error

// inTenant 从 ctx 取租户并注入 RLS 后执行查询。
func (r *repo) inTenant(ctx context.Context, fn queryFunc) error {
	return r.db.WithTenantTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// inTenantID 用显式租户 ID 注入 RLS,供内部 contracts 调用使用。
func (r *repo) inTenantID(ctx context.Context, tenantID int64, fn queryFunc) error {
	return r.db.WithTenantTxID(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// inApp 访问 runtime/tool 等无 RLS 平台级配置表。
func (r *repo) inApp(ctx context.Context, fn queryFunc) error {
	return r.db.WithAppTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// tenantFromContext 读取当前租户身份。
func tenantFromContext(ctx context.Context) (tenant.Identity, bool) {
	return tenant.FromContext(ctx)
}
