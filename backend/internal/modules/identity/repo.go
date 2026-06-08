// M1 数据访问层(repo)。
// 边界(CLAUDE.md §3 / §6):只读写 identity 自己的表;数据访问全部经 sqlc 生成的查询,
//
//	不手写 SQL。通过 platform/db 的事务入口执行:
//	· 租户上下文 → WithTenantTx(注入 SET LOCAL app.tenant_id,RLS 生效);
//	· 显式租户 → WithTenantTxID(鉴权前流程:登录定位租户后加载账号);
//	· 平台级表(无 RLS)→ WithAppTx;
//	· 受控特权路径 → WithPrivilegedTx(属主连接绕 RLS,仅限预认证定位与 tenant_id=NULL 场景)。
package identity

import (
	"context"
	"errors"
	"fmt"

	"chaimir/internal/modules/identity/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/tenant"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// repo 封装数据库访问,仅暴露事务执行入口给 service。
type repo struct {
	db *db.DB
}

// newRepo 绑定平台数据库入口,repo 本身不持有业务状态。
func newRepo(database *db.DB) *repo {
	return &repo{db: database}
}

// queryFunc 是在某事务上执行的查询闭包,拿到 sqlc 的 *Queries。
type queryFunc func(q *sqlcgen.Queries) error

// inTenant 在租户事务内执行(从 ctx 取租户,RLS 生效)。
func (r *repo) inTenant(ctx context.Context, fn queryFunc) error {
	return r.db.WithTenantTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// inTenantID 用显式租户 ID 执行(鉴权前流程,RLS 生效)。
func (r *repo) inTenantID(ctx context.Context, tenantID int64, fn queryFunc) error {
	return r.db.WithTenantTxID(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// inApp 在 app 池普通事务执行(仅访问无 RLS 的平台级表:tenant/platform_admin/application)。
func (r *repo) inApp(ctx context.Context, fn queryFunc) error {
	return r.db.WithAppTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// inAppTenantID 在同一 app 事务内先处理平台表,再注入指定租户 RLS 继续处理租户表。
// 用于入驻审核这类必须原子完成“平台申请/租户 + 新租户首个管理员”的流程。
func (r *repo) inAppTenantID(ctx context.Context, tenantID int64, fn queryFunc) error {
	return r.db.WithAppTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", fmt.Sprintf("%d", tenantID)); err != nil {
			return fmt.Errorf("注入 app.tenant_id 失败: %w", err)
		}
		return fn(sqlcgen.New(tx))
	})
}

// inPrivileged 在特权池(属主,绕 RLS)执行,仅限预认证定位与 tenant_id=NULL 平台级记录。
func (r *repo) inPrivileged(ctx context.Context, fn queryFunc) error {
	return r.db.WithPrivilegedTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// hasPrivileged 是否配置了特权连接(受控预认证/平台级空租户路径可用)。
func (r *repo) hasPrivileged() bool { return r.db.HasPrivileged() }

// pgErrCode 提取 PostgreSQL 错误码(如 23505 唯一冲突);非 PG 错误返回空串。
func pgErrCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

// tenantIdentity 从 ctx 取租户身份(已由鉴权中间件注入)。
func tenantIdentity(ctx context.Context) (int64, bool) {
	id, ok := tenant.FromContext(ctx)
	return id.TenantID, ok
}
