// Package db 封装 PostgreSQL 连接池与多租户 RLS 会话变量注入。
// 依据 docs/总-技术选型.md §2.3(sqlc + pgx)与 docs/01 §8(RLS + 独立连接角色绕 RLS)。
//
// 两连接池模型:
//
//	· app 池(PG_USER=chaimir_app,非属主):承载全部租户请求,RLS 强制生效 —— 默认路径。
//	· priv 池(PG_PRIV_USER=属主,可选):仅用于普通 RLS 租户事务无法表达的受控路径
//	  (预认证定位、tenant_id=NULL 的平台审计/验证码、激活码定位)。属主绕过 RLS,
//	  调用方必须在模块内收敛用途,不得作为普通跨租户读写入口。
//
// 租户隔离在事务内用 SET LOCAL app.tenant_id 注入(LOCAL=仅本事务,随结束失效,防连接池串号)。
package db

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"chaimir/internal/platform/config"
	"chaimir/internal/platform/tenant"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// IsNoRows 统一识别 pgx 未命中错误,调用方据此转换为各模块自己的业务错误码。
func IsNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// DB 封装 app 与可选 priv 连接池,提供租户感知的事务入口。
type DB struct {
	app  *pgxpool.Pool
	priv *pgxpool.Pool // 可空:未配置 PG_PRIV_USER 时为 nil。
}

// New 创建 app/priv 连接池并验证连通;app 池是租户请求的唯一默认数据入口。
func New(ctx context.Context, cfg config.PostgresConfig) (*DB, error) {
	app, err := openPool(ctx, cfg, cfg.User, cfg.Password)
	if err != nil {
		return nil, fmt.Errorf("创建 app 连接池失败: %w", err)
	}
	d := &DB{app: app}

	// 特权池仅在配置了独立特权用户时创建。
	if cfg.PrivUser != "" {
		priv, err := openPool(ctx, cfg, cfg.PrivUser, cfg.PrivPassword)
		if err != nil {
			app.Close()
			return nil, fmt.Errorf("创建 priv 连接池失败: %w", err)
		}
		d.priv = priv
	}
	return d, nil
}

// openPool 用结构化连接配置建池并 Ping,避免手写 DSN 造成凭据转义问题。
func openPool(ctx context.Context, cfg config.PostgresConfig, user, password string) (*pgxpool.Pool, error) {
	pc, err := buildPoolConfig(cfg, user, password)
	if err != nil {
		return nil, fmt.Errorf("解析连接配置失败: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, pc)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("连通性检查失败: %w", err)
	}
	return pool, nil
}

// buildPoolConfig 通过 pgx 结构化字段构造连接池配置,确保用户名和密码按原值传递给驱动。
func buildPoolConfig(cfg config.PostgresConfig, user, password string) (*pgxpool.Config, error) {
	dsn := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, password),
		Host:   fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Path:   cfg.Database,
	}
	q := dsn.Query()
	q.Set("sslmode", cfg.SSLMode)
	q.Set("pool_max_conns", fmt.Sprintf("%d", cfg.MaxConns))
	q.Set("pool_min_conns", fmt.Sprintf("%d", cfg.MinConns))
	dsn.RawQuery = q.Encode()
	pc, err := pgxpool.ParseConfig(dsn.String())
	if err != nil {
		return nil, err
	}
	pc.MaxConns = int32(cfg.MaxConns)
	pc.MinConns = int32(cfg.MinConns)
	return pc, nil
}

// Close 按存在性关闭所有连接池,由进程退出流程统一调用。
func (d *DB) Close() {
	if d.priv != nil {
		d.priv.Close()
	}
	d.app.Close()
}

// Ping 健康检查(app 池)。
func (d *DB) Ping(ctx context.Context) error { return d.app.Ping(ctx) }

// HasPrivileged 是否配置了特权池(受控预认证/平台级空租户路径可用)。
func (d *DB) HasPrivileged() bool { return d.priv != nil }

// TxFunc 是在事务内执行的业务函数。
type TxFunc func(ctx context.Context, tx pgx.Tx) error

// WithTenantTx 从 ctx 取租户身份,注入 RLS 后在 app 池开启事务执行 fn。
// 缺少租户上下文即拒绝(防无租户穿透 RLS)。
func (d *DB) WithTenantTx(ctx context.Context, fn TxFunc) error {
	id, ok := tenant.FromContext(ctx)
	if !ok {
		return fmt.Errorf("数据访问缺少租户上下文(未鉴权或未注入 tenant)")
	}
	return d.WithTenantTxID(ctx, id.TenantID, fn)
}

// WithTenantTxID 用显式租户 ID 在 app 池注入 RLS 后执行 fn(鉴权前流程:登录定位租户后加载账号)。
func (d *DB) WithTenantTxID(ctx context.Context, tenantID int64, fn TxFunc) error {
	return runTx(ctx, d.app, func(ctx context.Context, tx pgx.Tx) error {
		// SET LOCAL:仅本事务有效,防连接复用串号。
		if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)",
			fmt.Sprintf("%d", tenantID)); err != nil {
			return fmt.Errorf("注入 app.tenant_id 失败: %w", err)
		}
		return fn(ctx, tx)
	})
}

// WithAppTx 在 app 池开启普通事务(用于访问无 RLS 的平台级表:tenant/platform_admin/application)。
// app 角色对这些表有 DML 权,且它们无 RLS,故无需特权池。
func (d *DB) WithAppTx(ctx context.Context, fn TxFunc) error {
	return runTx(ctx, d.app, fn)
}

// WithPrivilegedTx 在特权池(属主,绕 RLS)开启事务,仅限受控预认证/平台级空租户路径。
// 未配置特权池则报错(调用方须先判 HasPrivileged 或确保部署已配)。
func (d *DB) WithPrivilegedTx(ctx context.Context, fn TxFunc) error {
	if d.priv == nil {
		return fmt.Errorf("未配置特权连接(PG_PRIV_USER),无法执行跨租户查询")
	}
	return runTx(ctx, d.priv, fn)
}

// runTx 统一事务执行:开启→执行→提交/回滚,错误不吞、用 %w 保留链。
func runTx(ctx context.Context, pool *pgxpool.Pool, fn TxFunc) error {
	// 第一步:事务必须由统一入口开启,保证后续提交/回滚语义一致。
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}

	// 第二步:业务函数失败时必须回滚;回滚失败也要带回原始业务错误。
	if err := fn(ctx, tx); err != nil {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
			return fmt.Errorf("事务执行失败: %w; 回滚失败: %v", err, rollbackErr)
		}
		return err
	}
	// 第三步:提交失败后尝试回滚,并保留提交失败作为主错误链。
	if err := tx.Commit(ctx); err != nil {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
			return fmt.Errorf("提交事务失败: %w; 回滚失败: %v", err, rollbackErr)
		}
		return fmt.Errorf("提交事务失败: %w", err)
	}
	return nil
}
