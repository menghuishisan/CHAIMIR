// M6 数据访问层:封装 teaching 自有表的 sqlc 事务入口与 RLS 注入。
package teaching

import (
	"context"

	"chaimir/internal/modules/teaching/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/tenant"

	"github.com/jackc/pgx/v5"
)

// repo 是 M6 模块数据库访问封装。
type repo struct {
	db *db.DB
}

// newRepo 构造 M6 repo。
func newRepo(database *db.DB) *repo { return &repo{db: database} }

// queryFunc 是 M6 sqlc 查询闭包。
type queryFunc func(q *sqlcgen.Queries) error

// inTenant 从请求上下文读取租户并注入 RLS。
func (r *repo) inTenant(ctx context.Context, fn queryFunc) error {
	return r.db.WithTenantTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// inTenantID 用显式租户 ID 注入 RLS,供事件与 contracts 内部入口使用。
func (r *repo) inTenantID(ctx context.Context, tenantID int64, fn queryFunc) error {
	return r.db.WithTenantTxID(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// inPrivileged 使用受控特权连接读取 M6 自有表的跨租户待派发任务。
func (r *repo) inPrivileged(ctx context.Context, fn queryFunc) error {
	return r.db.WithPrivilegedTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// tenantFromContext 读取当前请求租户身份。
func tenantFromContext(ctx context.Context) (tenant.Identity, bool) { return tenant.FromContext(ctx) }
