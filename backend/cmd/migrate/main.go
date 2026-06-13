// migrate main 是部署期迁移、应用角色授权和初始化 seed 的唯一命令入口。
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"

	"chaimir/db/migrations"
	"chaimir/internal/modules/identity"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/redis"
	"chaimir/internal/platform/storage"
	"chaimir/pkg/logging"
	"chaimir/pkg/snowflake"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const appRoleName = "chaimir_app"

// main 分发 migrate / migrate-and-seed 子命令。
func main() {
	if err := run(); err != nil {
		slog.Error("migrate command failed", slog.String("error", logging.SanitizeError(err.Error())))
		os.Exit(1)
	}
}

// run 负责加载配置并按子命令执行部署期编排。
func run() error {
	if len(os.Args) != 2 {
		return fmt.Errorf("用法: migrate [migrate|migrate-and-seed]")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logging.Setup(cfg.Server.LogLevel, cfg.Server.LogFormat)
	ctx := context.Background()
	switch os.Args[1] {
	case "migrate":
		return migrateAndGrant(ctx, cfg)
	case "migrate-and-seed":
		if err := migrateAndGrant(ctx, cfg); err != nil {
			return err
		}
		return seed(ctx, cfg)
	default:
		return fmt.Errorf("未知子命令: %s", os.Args[1])
	}
}

// migrateAndGrant 执行数据库 schema migration,随后幂等授权应用角色。
func migrateAndGrant(ctx context.Context, cfg *config.Config) error {
	if err := runMigrations(cfg.Postgres); err != nil {
		return err
	}
	if err := grantApplicationRole(ctx, cfg.Postgres); err != nil {
		return err
	}
	slog.Info("database migration and role grant completed")
	return nil
}

// runMigrations 使用 golang-migrate 执行嵌入的版本化 SQL。
func runMigrations(pg config.PostgresConfig) error {
	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("创建迁移源失败: %w", err)
	}
	sqlDB, err := sql.Open("pgx", postgresURL(pg, privilegedUser(pg), privilegedPassword(pg)))
	if err != nil {
		return fmt.Errorf("打开迁移数据库连接失败: %w", err)
	}
	defer sqlDB.Close()
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
func grantApplicationRole(ctx context.Context, pg config.PostgresConfig) error {
	if strings.TrimSpace(pg.User) != appRoleName {
		return fmt.Errorf("PG_USER 必须为固定应用角色 %s,实际=%s", appRoleName, pg.User)
	}
	sqlDB, err := sql.Open("pgx", postgresURL(pg, privilegedUser(pg), privilegedPassword(pg)))
	if err != nil {
		return fmt.Errorf("打开授权数据库连接失败: %w", err)
	}
	defer sqlDB.Close()
	grantCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if _, err := sqlDB.ExecContext(grantCtx, "SELECT 1"); err != nil {
		return fmt.Errorf("授权数据库连通性检查失败: %w", err)
	}
	if _, err := sqlDB.ExecContext(grantCtx, `DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'chaimir_app') THEN
    CREATE ROLE chaimir_app LOGIN;
  ELSE
    ALTER ROLE chaimir_app LOGIN;
  END IF;
END $$;`); err != nil {
		return fmt.Errorf("创建应用角色失败: %w", err)
	}
	if _, err := sqlDB.ExecContext(grantCtx, "ALTER ROLE chaimir_app WITH PASSWORD $1", pg.Password); err != nil {
		return fmt.Errorf("更新应用角色密码失败: %w", err)
	}
	statements := []string{
		"GRANT CONNECT ON DATABASE " + quoteIdentifier(pg.Database) + " TO chaimir_app",
		"GRANT USAGE ON SCHEMA public TO chaimir_app",
		"GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO chaimir_app",
		"GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO chaimir_app",
		"ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO chaimir_app",
		"ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO chaimir_app",
	}
	for _, stmt := range statements {
		if _, err := sqlDB.ExecContext(grantCtx, stmt); err != nil {
			return fmt.Errorf("执行授权语句失败: %w", err)
		}
	}
	return nil
}

// seed 执行依赖业务规则的初始化动作,不在 cmd 中复制模块业务逻辑。
func seed(ctx context.Context, cfg *config.Config) error {
	database, err := db.New(ctx, cfg.Postgres)
	if err != nil {
		return err
	}
	defer database.Close()
	redisClient, err := redis.New(ctx, cfg.Redis)
	if err != nil {
		return err
	}
	defer redisClient.Close()
	objectStore, err := storage.New(ctx, cfg.MinIO)
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
	authManager := auth.NewManager(cfg.Auth)
	identitySvc, err := identity.NewService(identity.ServiceDeps{
		Store:          identity.NewStore(database),
		Auth:           authManager,
		Redis:          redisClient,
		IDs:            ids,
		AuthConfig:     cfg.Auth,
		IdentityConfig: cfg.Identity,
		UploadConfig:   cfg.Upload,
		DeployConfig:   cfg.Deploy,
		SMSSender:      identity.NewSMSSender(cfg.SMS),
	})
	if err != nil {
		return err
	}
	if cfg.Deploy.PlatformEnabled {
		return identitySvc.BootstrapPlatformAdmin(ctx, cfg.Bootstrap)
	}
	_, err = identitySvc.BootstrapSchoolTenant(ctx, cfg.Bootstrap)
	return err
}

// privilegedUser 返回迁移/授权使用的数据库特权用户。
func privilegedUser(pg config.PostgresConfig) string {
	if strings.TrimSpace(pg.PrivUser) != "" {
		return pg.PrivUser
	}
	return pg.User
}

// privilegedPassword 返回迁移/授权使用的数据库特权用户密码。
func privilegedPassword(pg config.PostgresConfig) string {
	if strings.TrimSpace(pg.PrivUser) != "" {
		return pg.PrivPassword
	}
	return pg.Password
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
