// M4 数据访问层:只读写 sim 模块自有表,全部经 sqlc 生成查询。
package sim

import (
	"context"

	"chaimir/internal/modules/sim/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/tenant"

	"github.com/jackc/pgx/v5"
)

// repo 封装 sim 模块数据库事务入口。
type repo struct {
	db *db.DB
}

// newRepo 构造 M4 repo。
func newRepo(database *db.DB) *repo {
	return &repo{db: database}
}

// queryFunc 是 M4 数据访问闭包,统一接收 sqlc 查询对象。
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

// inApp 访问仿真包和审核平台级配置表。
func (r *repo) inApp(ctx context.Context, fn queryFunc) error {
	return r.db.WithAppTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// tenantFromContext 读取当前租户身份。
func tenantFromContext(ctx context.Context) (tenant.Identity, bool) {
	return tenant.FromContext(ctx)
}
