// migrate 命令入口测试。
package main

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"chaimir/internal/platform/config"
)

// TestMigrateAndSeedCommandMatchesDeployJob 确认 K8s Job 使用的 migrate-and-seed 子命令由 Go 命令实现。
func TestMigrateAndSeedCommandMatchesDeployJob(t *testing.T) {
	root := repoRoot(t)
	job, err := os.ReadFile(filepath.Join(root, "..", "deploy", "base", "migrate", "migrate-job.yaml"))
	if err != nil {
		t.Fatalf("read migrate job: %v", err)
	}
	if !strings.Contains(string(job), `args: ["migrate-and-seed"]`) {
		t.Fatalf("migrate job must call migrate-and-seed")
	}
	if !supportsCommand("migrate-and-seed") {
		t.Fatalf("cmd/migrate must implement migrate-and-seed used by deploy/base/migrate/migrate-job.yaml")
	}
}

// TestRoleGrantUsesApplicationPassword 确认角色授权复用 PG_PASSWORD,避免同一个 chaimir_app 角色出现两份外部配置。
func TestRoleGrantUsesApplicationPassword(t *testing.T) {
	root := repoRoot(t)
	src, err := os.ReadFile(filepath.Join(root, "cmd", "migrate", "main.go"))
	if err != nil {
		t.Fatalf("read cmd/migrate/main.go: %v", err)
	}
	if strings.Contains(string(src), `os.Getenv("CHAIMIR_APP_PASSWORD")`) || strings.Contains(string(src), `req("CHAIMIR_APP_PASSWORD")`) {
		t.Fatalf("cmd/migrate must read PG_PASSWORD for chaimir_app role password, not a second CHAIMIR_APP_PASSWORD config")
	}
}

// TestMigrateUsesPrivilegedDatabaseConfig 确认迁移使用现有 PG_PRIV_* 特权连接,不新增第三套 owner DSN。
func TestMigrateUsesPrivilegedDatabaseConfig(t *testing.T) {
	root := repoRoot(t)
	src, err := os.ReadFile(filepath.Join(root, "cmd", "migrate", "main.go"))
	if err != nil {
		t.Fatalf("read cmd/migrate/main.go: %v", err)
	}
	if strings.Contains(string(src), "PG_OWNER_DSN") {
		t.Fatalf("cmd/migrate must use PG_PRIV_USER/PG_PRIV_PASSWORD instead of a separate PG_OWNER_DSN")
	}
}

// TestPrivilegedDatabaseArgsKeepPasswordOutOfDSN 确认特权连接参数不把密码放进进程参数。
func TestPrivilegedDatabaseArgsKeepPasswordOutOfDSN(t *testing.T) {
	dsn, env, err := privilegedDatabaseArgs(config.PostgresConfig{
		Host:         "postgres.local",
		Port:         5432,
		Database:     "chaimir",
		SSLMode:      "disable",
		PrivUser:     "owner@example",
		PrivPassword: "p@ss:word/with/slash",
	})
	if err != nil {
		t.Fatalf("privileged database args: %v", err)
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	if password, hasPassword := parsed.User.Password(); parsed.User.Username() != "owner@example" || hasPassword || password != "" {
		t.Fatalf("dsn must contain username only: %s", dsn)
	}
	foundPassword := false
	for _, item := range env {
		if item == "PGPASSWORD=p@ss:word/with/slash" {
			foundPassword = true
		}
	}
	if !foundPassword {
		t.Fatalf("PGPASSWORD env was not preserved: %#v", env)
	}
}

// TestRoleGrantKeepsApplicationPasswordOutOfProcessArgs 确认应用角色口令不进入 psql 命令参数。
func TestRoleGrantKeepsApplicationPasswordOutOfProcessArgs(t *testing.T) {
	root := repoRoot(t)
	src, err := os.ReadFile(filepath.Join(root, "cmd", "migrate", "main.go"))
	if err != nil {
		t.Fatalf("read cmd/migrate/main.go: %v", err)
	}
	text := string(src)
	if strings.Contains(text, `"app_password="+pg.Password`) || strings.Contains(text, `"-v", "app_password=`) {
		t.Fatalf("cmd/migrate must pass chaimir_app password through environment, not psql process arguments")
	}
	if !strings.Contains(text, `"CHAIMIR_APP_PASSWORD="+pg.Password`) {
		t.Fatalf("cmd/migrate must provide chaimir_app password through the child process environment")
	}
}

// TestMigrationCommandsDoNotPassSecretsAsProcessArgument 确认迁移/授权不把口令暴露在进程参数中。
func TestMigrationCommandsDoNotPassSecretsAsProcessArgument(t *testing.T) {
	root := repoRoot(t)
	src, err := os.ReadFile(filepath.Join(root, "cmd", "migrate", "main.go"))
	if err != nil {
		t.Fatalf("read cmd/migrate/main.go: %v", err)
	}
	text := string(src)
	for _, forbidden := range []string{
		`url.UserPassword(pg.PrivUser, pg.PrivPassword)`,
		`"-v", "app_password="+pg.Password`,
		`"app_password="+pg.Password`,
		`exec.CommandContext(ctx, "psql", ownerDSN`,
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("cmd/migrate must not pass database passwords through process arguments; found %s", forbidden)
		}
	}
}

// TestLoadMigrationPostgresConfigOnlyRequiresDatabaseEnv 确认 migrate 子命令只要求数据库相关变量。
func TestLoadMigrationPostgresConfigOnlyRequiresDatabaseEnv(t *testing.T) {
	t.Setenv("PG_HOST", "postgres")
	t.Setenv("PG_PORT", "5432")
	t.Setenv("PG_DATABASE", "chaimir")
	t.Setenv("PG_USER", "chaimir_app")
	t.Setenv("PG_PASSWORD", "app-secret")
	t.Setenv("PG_SSLMODE", "require")
	t.Setenv("PG_PRIV_USER", "postgres")
	t.Setenv("PG_PRIV_PASSWORD", "owner-secret")

	cfg, err := loadMigrationPostgresConfig()
	if err != nil {
		t.Fatalf("load migration postgres config: %v", err)
	}
	if cfg.User != "chaimir_app" || cfg.Password != "app-secret" ||
		cfg.PrivUser != "postgres" || cfg.PrivPassword != "owner-secret" ||
		cfg.SSLMode != "require" {
		t.Fatalf("unexpected migration postgres config: %#v", cfg)
	}
}

// TestLoadMigrationPostgresConfigRequiresSSLMode 确认迁移入口不为 PG_SSLMODE 写代码默认值。
func TestLoadMigrationPostgresConfigRequiresSSLMode(t *testing.T) {
	t.Setenv("PG_HOST", "postgres")
	t.Setenv("PG_PORT", "5432")
	t.Setenv("PG_DATABASE", "chaimir")
	t.Setenv("PG_USER", "chaimir_app")
	t.Setenv("PG_PASSWORD", "app-secret")
	t.Setenv("PG_PRIV_USER", "postgres")
	t.Setenv("PG_PRIV_PASSWORD", "owner-secret")

	if _, err := loadMigrationPostgresConfig(); err == nil || !strings.Contains(err.Error(), "PG_SSLMODE") {
		t.Fatalf("missing PG_SSLMODE must fail migration config loading, got %v", err)
	}
}

// TestSeedEventBusRejectsUnexpectedEvents 确认 seed 入口不会用空事件总线吞掉意外事件发布。
func TestSeedEventBusRejectsUnexpectedEvents(t *testing.T) {
	bus := seedEventBus{}
	if err := bus.Publish(context.Background(), "identity.created", map[string]string{"id": "1"}); err == nil {
		t.Fatalf("seed event bus must reject unexpected publish")
	}
	if _, err := bus.Subscribe("identity.created", "", nil); err == nil {
		t.Fatalf("seed event bus must reject unexpected subscribe")
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}
