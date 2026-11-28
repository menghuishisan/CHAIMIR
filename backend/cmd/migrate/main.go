// migrate main 是部署期迁移、应用角色授权和初始化 seed 的唯一命令入口。
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"

	"chaimir/db/migrations"
	"chaimir/internal/modules/identity"
	"chaimir/internal/modules/sim"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/storage"
	"chaimir/pkg/logging"
	"chaimir/pkg/snowflake"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const appRoleName = "chaimir_app"

// grantMaintenanceRoleSQL 在数据库内安全引用动态标识符,避免部署命令拼接维护角色 DDL。
const grantMaintenanceRoleSQL = `DO $$
DECLARE
  maintenance_role text := current_setting('chaimir.migration.maintenance_role');
  table_name text;
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = maintenance_role) THEN
    EXECUTE format('CREATE ROLE %I NOLOGIN BYPASSRLS', maintenance_role);
  ELSE
    EXECUTE format('ALTER ROLE %I NOLOGIN BYPASSRLS', maintenance_role);
  END IF;

  EXECUTE format('GRANT CONNECT ON DATABASE %I TO %I', current_database(), maintenance_role);
  EXECUTE format('GRANT USAGE ON SCHEMA public TO %I', maintenance_role);
  EXECUTE format('REVOKE ALL PRIVILEGES ON ALL TABLES IN SCHEMA public FROM %I', maintenance_role);
  FOR table_name IN
    SELECT jsonb_array_elements_text(current_setting('chaimir.migration.maintenance_tables')::jsonb)
  LOOP
    EXECUTE format('GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE %I TO %I', table_name, maintenance_role);
  END LOOP;
  IF current_setting('chaimir.migration.maintenance_audit')::boolean THEN
    EXECUTE format('GRANT INSERT ON TABLE audit_log TO %I', maintenance_role);
  END IF;
  EXECUTE format('GRANT %I TO %I', maintenance_role, session_user);
END $$;`

// main 分发数据库迁移、初始化和受控验收数据子命令。
func main() {
	if len(os.Args) == 2 && os.Args[1] == "--selftest" {
		fmt.Fprintln(os.Stdout, "迁移命令自检通过")
		return
	}
	if err := run(); err != nil {
		slog.Error("migrate command failed", slog.String("error", logging.SanitizeError(err.Error())))
		os.Exit(1)
	}
}

// run 负责加载配置并按子命令执行部署期编排。
func run() error {
	if len(os.Args) != 2 {
		return fmt.Errorf("用法: migrate [migrate|migrate-and-seed|seed-acceptance]")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logging.Setup(cfg.Server.Logging)
	ctx := context.Background()
	switch os.Args[1] {
	case "migrate":
		return migrateAndGrant(ctx, cfg)
	case "migrate-and-seed":
		if err := migrateAndGrant(ctx, cfg); err != nil {
			return err
		}
		return seed(ctx, cfg)
	case "seed-acceptance":
		return seedAcceptance(ctx, cfg)
	default:
		return fmt.Errorf("未知子命令: %s", os.Args[1])
	}
}

// migrateAndGrant 执行数据库 schema migration,随后幂等授权应用角色。
func migrateAndGrant(ctx context.Context, cfg *config.Config) error {
	if err := runMigrations(ctx, cfg.Postgres); err != nil {
		return err
	}
	if err := grantApplicationRole(ctx, cfg.Postgres); err != nil {
		return err
	}
	if err := grantMaintenanceRoles(ctx, cfg.Postgres); err != nil {
		return err
	}
	slog.Info("database migration and role grant completed")
	return nil
}

// grantMaintenanceRoles 创建不可登录的模块维护角色,并把跨租户权限限制在各模块自有表。
func grantMaintenanceRoles(ctx context.Context, pg config.PostgresConfig) (resultErr error) {
	if pg.GrantTimeoutSeconds <= 0 {
		return fmt.Errorf("PG_GRANT_TIMEOUT_SECONDS 必须大于 0")
	}
	privilegedRole := strings.TrimSpace(pg.PrivUser)
	if privilegedRole == "" {
		return fmt.Errorf("模块维护角色授权缺少 PG_PRIV_USER")
	}
	sqlDB, err := sql.Open("pgx", postgresURL(pg, privilegedRole, pg.PrivPassword))
	if err != nil {
		return fmt.Errorf("打开模块维护角色授权连接失败: %w", err)
	}
	defer closeSQLDatabase(sqlDB, "关闭模块维护角色授权连接失败", &resultErr)
	grantCtx, cancel := context.WithTimeout(ctx, time.Duration(pg.GrantTimeoutSeconds)*time.Second)
	defer cancel()
	tx, err := sqlDB.BeginTx(grantCtx, nil)
	if err != nil {
		return fmt.Errorf("开始模块维护角色授权事务失败: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			resultErr = errors.Join(resultErr, fmt.Errorf("回滚模块维护角色授权事务失败: %w", rollbackErr))
		}
	}()
	for _, policy := range db.MaintenancePolicies() {
		tablesJSON, err := json.Marshal(policy.Tables)
		if err != nil {
			return fmt.Errorf("编码模块维护角色 %s 表策略失败: %w", policy.Module, err)
		}
		if _, err := tx.ExecContext(grantCtx, `SELECT
  set_config('chaimir.migration.maintenance_role', $1, true),
  set_config('chaimir.migration.maintenance_tables', $2, true),
  set_config('chaimir.migration.maintenance_audit', $3::boolean::text, true)`, policy.Role, string(tablesJSON), policy.WritesAudit); err != nil {
			return fmt.Errorf("设置模块维护角色 %s 授权参数失败: %w", policy.Module, err)
		}
		if _, err := tx.ExecContext(grantCtx, grantMaintenanceRoleSQL); err != nil {
			return fmt.Errorf("授权模块维护角色 %s 失败: %w", policy.Module, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交模块维护角色授权事务失败: %w", err)
	}
	return resultErr
}

// closeSQLDatabase 关闭部署期数据库连接，并把关闭错误合并到原始返回错误中。
func closeSQLDatabase(sqlDB *sql.DB, message string, resultErr *error) {
	if err := sqlDB.Close(); err != nil {
		*resultErr = errors.Join(*resultErr, fmt.Errorf("%s: %w", message, err))
	}
}

// runMigrations 等待 PostgreSQL 就绪后使用 golang-migrate 执行嵌入的版本化 SQL。
func runMigrations(ctx context.Context, pg config.PostgresConfig) (resultErr error) {
	if err := db.WaitForPostgres(ctx, pg, pg.PrivUser, pg.PrivPassword); err != nil {
		return fmt.Errorf("等待迁移数据库就绪失败: %w", err)
	}
	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("创建迁移源失败: %w", err)
	}
	sqlDB, err := sql.Open("pgx", postgresURL(pg, pg.PrivUser, pg.PrivPassword))
	if err != nil {
		return fmt.Errorf("打开迁移数据库连接失败: %w", err)
	}
	defer closeSQLDatabase(sqlDB, "关闭迁移数据库连接失败", &resultErr)
	driver, err := postgres.WithInstance(sqlDB, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("创建迁移 driver 失败: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		return fmt.Errorf("创建迁移器失败: %w", err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("执行数据库迁移失败: %w", err)
	}
	return nil
}

// grantApplicationRole 幂等创建固定应用角色并授予 public schema 最小表权限。
func grantApplicationRole(ctx context.Context, pg config.PostgresConfig) (resultErr error) {
	if strings.TrimSpace(pg.User) != appRoleName {
		return fmt.Errorf("PG_USER 必须为固定应用角色 %s,实际=%s", appRoleName, pg.User)
	}
	if pg.GrantTimeoutSeconds <= 0 {
		return fmt.Errorf("PG_GRANT_TIMEOUT_SECONDS 必须大于 0")
	}
	sqlDB, err := sql.Open("pgx", postgresURL(pg, pg.PrivUser, pg.PrivPassword))
	if err != nil {
		return fmt.Errorf("打开授权数据库连接失败: %w", err)
	}
	defer closeSQLDatabase(sqlDB, "关闭授权数据库连接失败", &resultErr)
	grantCtx, cancel := context.WithTimeout(ctx, time.Duration(pg.GrantTimeoutSeconds)*time.Second)
	defer cancel()
	tx, err := sqlDB.BeginTx(grantCtx, nil)
	if err != nil {
		return fmt.Errorf("开始应用角色授权事务失败: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			resultErr = errors.Join(resultErr, rollbackErr)
		}
	}()
	if _, err := tx.ExecContext(grantCtx, `DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'chaimir_app') THEN
    CREATE ROLE chaimir_app LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
  ELSE
    ALTER ROLE chaimir_app LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
  END IF;
END $$;`); err != nil {
		return fmt.Errorf("创建应用角色失败: %w", err)
	}
	if _, err := tx.ExecContext(grantCtx, `DO $$
DECLARE app_oid oid := 'chaimir_app'::regrole;
BEGIN
  IF EXISTS (SELECT 1 FROM pg_auth_members WHERE member = app_oid) THEN
    RAISE EXCEPTION 'chaimir_app must not be a member of another role';
  END IF;
  IF EXISTS (
    SELECT 1 FROM pg_shdepend
    WHERE refclassid = 'pg_authid'::regclass AND refobjid = app_oid AND deptype = 'o'
      AND (dbid = (SELECT oid FROM pg_database WHERE datname = current_database())
           OR (classid = 'pg_database'::regclass AND objid = (SELECT oid FROM pg_database WHERE datname = current_database())))
  ) THEN
    RAISE EXCEPTION 'chaimir_app must not own database objects';
  END IF;
  IF has_schema_privilege('chaimir_app', 'public', 'CREATE') THEN
    RAISE EXCEPTION 'chaimir_app must not have CREATE on schema public';
  END IF;
END $$;`); err != nil {
		return fmt.Errorf("应用数据库角色不满足隔离要求: %w", err)
	}
	passwordLiteral, err := quoteRolePasswordLiteral(grantCtx, tx, pg.Password)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(grantCtx, "ALTER ROLE chaimir_app WITH PASSWORD "+passwordLiteral); err != nil {
		return fmt.Errorf("更新应用角色密码失败: %w", err)
	}
	statements := []string{
		"GRANT CONNECT ON DATABASE " + quoteIdentifier(pg.Database) + " TO chaimir_app",
		"GRANT USAGE ON SCHEMA public TO chaimir_app",
		"GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO chaimir_app",
		"GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO chaimir_app",
		"ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE SELECT, INSERT, UPDATE, DELETE ON TABLES FROM chaimir_app",
		"REVOKE ALL ON TABLE platform_admin, platform_auth_session, tenant_application, tenant_provision_outbox, schema_migrations FROM chaimir_app, PUBLIC",
		"REVOKE INSERT, DELETE ON tenant FROM chaimir_app, PUBLIC",
		"ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO chaimir_app",
	}
	for _, stmt := range statements {
		if _, err := tx.ExecContext(grantCtx, stmt); err != nil {
			return fmt.Errorf("执行授权语句失败: %w", err)
		}
	}
	return tx.Commit()
}

// quoteRolePasswordLiteral 使用 PostgreSQL 自身的字面量转义规则生成角色密码片段。
func quoteRolePasswordLiteral(ctx context.Context, tx *sql.Tx, password string) (string, error) {
	var literal string
	if err := tx.QueryRowContext(ctx, "SELECT quote_literal($1)", password).Scan(&literal); err != nil {
		return "", fmt.Errorf("转义应用角色密码失败: %w", err)
	}
	if strings.TrimSpace(literal) == "" {
		return "", fmt.Errorf("转义应用角色密码结果为空")
	}
	return literal, nil
}

// seed 执行依赖业务规则的初始化动作,不在 cmd 中复制模块业务逻辑。
// 两件事:①把平台内置仿真包标准库入库(平台交付物,与租户无关);②建首个管理员/学校租户。
func seed(ctx context.Context, cfg *config.Config) (resultErr error) {
	database, err := db.New(ctx, cfg.Postgres)
	if err != nil {
		return err
	}
	defer database.Close()
	objectStore, err := storage.New(ctx, cfg.ObjectStorage)
	if err != nil {
		return err
	}
	if err := objectStore.EnsureBuckets(ctx); err != nil {
		return err
	}
	ids, err := snowflake.NewNode(cfg.Snowflake.NodeID)
	if err != nil {
		return err
	}
	if err := seedBuiltinSimPackages(ctx, database, ids); err != nil {
		return err
	}
	if err := seedPlatformCatalog(ctx, database); err != nil {
		return err
	}
	store, err := identity.NewStore(database)
	if err != nil {
		return err
	}
	return identity.Bootstrap(ctx, store, ids, cfg.Deploy, cfg.Auth, cfg.Bootstrap)
}

// seedBuiltinSimPackages 调用 M4 模块入口同步内置仿真包标准库。
// 内置包的协议声明来自前端 sim-sdk 并由 sim 模块 go:embed 校验入库,cmd 只负责编排,
// 不在此复制 SQL 或协议解析(工程目录设计 §2.0:部署期命令可装配模块入口,不承载业务逻辑)。
func seedBuiltinSimPackages(ctx context.Context, database *db.DB, ids snowflake.Generator) error {
	result, err := sim.SyncBuiltinPackages(ctx, sim.NewStore(database), ids)
	if err != nil {
		return err
	}
	slog.Info("builtin simulation packages synced",
		slog.Int("upserted", result.Upserted),
		slog.Int("archived", len(result.Archived)))
	return nil
}

// postgresURL 通过结构化字段构造 DSN,避免凭据特殊字符破坏连接串。
func postgresURL(pg config.PostgresConfig, user, password string) string {
	dsn := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, password),
		Host:   fmt.Sprintf("%s:%d", pg.Host, pg.Port),
		Path:   pg.Database,
	}
	q := dsn.Query()
	q.Set("sslmode", pg.SSLMode)
	dsn.RawQuery = q.Encode()
	return dsn.String()
}

// quoteIdentifier 为 PostgreSQL 标识符执行最小安全引用。
func quoteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
