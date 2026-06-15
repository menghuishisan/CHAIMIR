// db 封装 PostgreSQL 连接池、租户事务和受控特权事务入口。
package db

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"chaimir/internal/platform/config"
	"chaimir/internal/platform/tenant"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB 封装应用池和可选特权池。
type DB struct {
	app  *pgxpool.Pool
	priv *pgxpool.Pool
}

// TxFunc 是在事务内执行的函数签名。
type TxFunc func(ctx context.Context, tx pgx.Tx) error

// New 创建 app 与可选 priv 连接池,并在启动阶段检查连通性。
func New(ctx context.Context, cfg config.PostgresConfig) (*DB, error) {
	app, err := openPool(ctx, cfg, cfg.User, cfg.Password)
	if err != nil {
		return nil, fmt.Errorf("创建 app 连接池失败: %w", err)
	}
	database := &DB{app: app}

	if strings.TrimSpace(cfg.PrivUser) != "" {
		priv, err := openPool(ctx, cfg, cfg.PrivUser, cfg.PrivPassword)
		if err != nil {
			app.Close()
			return nil, fmt.Errorf("创建 priv 连接池失败: %w", err)
		}
		database.priv = priv
	}
	return database, nil
}

// IsNoRows 统一识别 pgx 未命中错误,供上层转换为业务错误码。
func IsNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// Close 关闭所有已创建的连接池。
func (d *DB) Close() {
	if d.priv != nil {
		d.priv.Close()
	}
	if d.app != nil {
		d.app.Close()
	}
}

// Ping 检查 app 池连通性。
func (d *DB) Ping(ctx context.Context) error {
	if d == nil || d.app == nil {
		return fmt.Errorf("app 连接池未初始化")
	}
	return d.app.Ping(ctx)
}

// HasPrivileged 判断是否配置了特权连接池。
func (d *DB) HasPrivileged() bool {
	if d == nil {
		return false
	}
	return d.priv != nil
}

// AppPool 暴露 app 池给需要直接构造 sqlc 查询对象的 repo 装配层使用。
func (d *DB) AppPool() *pgxpool.Pool {
	if d == nil {
		return nil
	}
	return d.app
}

// PrivilegedPool 暴露特权池给受控装配层使用;未配置时返回 nil。
func (d *DB) PrivilegedPool() *pgxpool.Pool {
	if d == nil {
		return nil
	}
	return d.priv
}

// WithTenantTx 从上下文读取租户身份并注入 RLS 会话变量后执行事务。
func (d *DB) WithTenantTx(ctx context.Context, fn TxFunc) error {
	if d == nil {
		return fmt.Errorf("数据库未初始化")
	}
	id, ok := tenant.FromContext(ctx)
	if !ok {
		return fmt.Errorf("数据访问缺少租户上下文(未鉴权或未注入 tenant)")
	}
	return d.WithTenantTxID(ctx, id.TenantID, fn)
}

// WithTenantTxID 用显式租户 ID 注入 RLS 会话变量后执行事务。
func (d *DB) WithTenantTxID(ctx context.Context, tenantID int64, fn TxFunc) error {
	if d == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if tenantID <= 0 {
		return fmt.Errorf("tenant_id 必须大于 0")
	}
	return runTx(ctx, d.app, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", fmt.Sprintf("%d", tenantID)); err != nil {
			return fmt.Errorf("注入 app.tenant_id 失败: %w", err)
		}
		return fn(ctx, tx)
	})
}

// WithAppTx 在应用连接池中开启普通事务,用于访问无需 RLS 的平台级表。
func (d *DB) WithAppTx(ctx context.Context, fn TxFunc) error {
	if d == nil {
		return fmt.Errorf("数据库未初始化")
	}
	return runTx(ctx, d.app, fn)
}

// WithPrivilegedTx 在特权连接池中开启事务,仅用于受控平台路径或预认证定位。
func (d *DB) WithPrivilegedTx(ctx context.Context, fn TxFunc) error {
	if d == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if d.priv == nil {
		return fmt.Errorf("未配置特权连接(PG_PRIV_USER),无法执行跨租户查询")
	}
	return runTx(ctx, d.priv, fn)
}

// WithPrivilegedModuleTx 仅限模块后台维护任务扫描本模块自有 RLS 表。
func (d *DB) WithPrivilegedModuleTx(ctx context.Context, module string, fn TxFunc) error {
	if d == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if strings.TrimSpace(module) == "" {
		return fmt.Errorf("模块维护特权事务缺少模块名")
	}
	if d.priv == nil {
		return fmt.Errorf("未配置特权连接(PG_PRIV_USER),无法执行模块维护任务")
	}
	return runTx(ctx, d.priv, fn)
}

// openPool 构造并连通检查一个连接池。
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

// buildPoolConfig 通过结构化字段构造连接池配置,避免凭据特殊字符破坏 DSN。
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

// runTx 统一开启、提交和回滚事务,并保留完整错误链。
func runTx(ctx context.Context, pool *pgxpool.Pool, fn TxFunc) error {
	if pool == nil {
		return fmt.Errorf("数据库连接池未初始化")
	}
	if fn == nil {
		return fmt.Errorf("事务函数不能为空")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}

	if err := fn(ctx, tx); err != nil {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
			return fmt.Errorf("事务执行失败: %w", errors.Join(err, fmt.Errorf("回滚失败: %w", rollbackErr)))
		}
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
			return fmt.Errorf("提交事务失败: %w", errors.Join(err, fmt.Errorf("回滚失败: %w", rollbackErr)))
		}
		return fmt.Errorf("提交事务失败: %w", err)
	}
	return nil
}
