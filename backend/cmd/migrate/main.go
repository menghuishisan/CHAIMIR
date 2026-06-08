// migrate 命令负责部署期数据库迁移、角色授权和初始化数据写入。
// 它是 deploy/base/migrate/migrate-job.yaml 的真实入口;业务初始化只调用模块服务,不在脚本里复制账号规则。
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chaimir/internal/modules/identity"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/eventbus"
	"chaimir/pkg/crypto"
	"chaimir/pkg/logging"
	"chaimir/pkg/snowflake"
)

const (
	commandMigrateAndSeed = "migrate-and-seed"
	commandMigrateOnly    = "migrate"
	commandSeedOnly       = "seed"
)

// main 解析部署期子命令并统一处理启动失败日志。
func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		logging.Setup("info", "json")
		logging.ErrorContext(context.Background(), "数据库初始化命令执行失败", err.Error())
		os.Exit(1)
	}
}

// supportsCommand 供测试和命令解析共用,确保部署清单里的子命令在 Go 入口中真实存在。
func supportsCommand(name string) bool {
	switch name {
	case commandMigrateAndSeed, commandMigrateOnly, commandSeedOnly:
		return true
	default:
		return false
	}
}

// run 执行部署期子命令;默认完整运行迁移与初始化。
func run(ctx context.Context, args []string) error {
	// 第一步:解析受支持的部署期子命令,避免 Job 参数拼错后执行到错误路径。
	command := commandMigrateAndSeed
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		command = strings.TrimSpace(args[0])
	}
	if !supportsCommand(command) {
		return fmt.Errorf("未知初始化子命令: %s", command)
	}

	// 部署 Job 由 ConfigMap/Secret 注入环境变量,本地调试仍允许读取 backend/.env。
	if err := config.LoadDotEnv(".env"); err != nil {
		return fmt.Errorf("加载 .env 失败: %w", err)
	}
	// 第二步:按子命令只加载需要的配置;纯迁移不要求 Redis/MinIO 等运行期依赖。
	switch command {
	case commandMigrateAndSeed:
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		logging.Setup(cfg.Server.LogLevel, cfg.Server.LogFormat)
		if err := runMigrations(ctx, cfg.Postgres); err != nil {
			return err
		}
		return runSeed(ctx, cfg)
	case commandMigrateOnly:
		pg, err := loadMigrationPostgresConfig()
		if err != nil {
			return err
		}
		logging.Setup("info", "json")
		return runMigrations(ctx, pg)
	case commandSeedOnly:
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		logging.Setup(cfg.Server.LogLevel, cfg.Server.LogFormat)
		return runSeed(ctx, cfg)
	default:
		return fmt.Errorf("未知初始化子命令: %s", command)
	}
}

// runMigrations 调用标准 golang-migrate CLI 应用版本化迁移。
// 迁移只负责 schema/RLS,首个管理员等业务初始化由 seed 阶段调用 M1 服务完成。
func runMigrations(ctx context.Context, pg config.PostgresConfig) error {
	// 第一步:生成属主连接串,迁移和授权必须走受控特权凭据。
	ownerDSN, env, err := privilegedDatabaseArgs(pg)
	if err != nil {
		return err
	}
	migrationsDir, err := migrationsPath()
	if err != nil {
		return err
	}
	// 第二步:运行版本化迁移文件,DSN 通过环境变量传递,避免凭据出现在进程参数中。
	cmd := exec.CommandContext(ctx, "migrate", "-path", migrationsDir, "-database", ownerDSN, "up")
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("应用数据库迁移失败: %w", err)
	}
	// 第三步:迁移成功后刷新 app 角色授权,保证运行期连接不需要属主权限。
	return runRoleGrant(ctx, pg)
}

// runRoleGrant 调用 scripts/db/00_role.sql 创建或更新 chaimir_app 角色授权。
// 应用连接必须使用非属主角色,否则租户 RLS 会被绕过。
func runRoleGrant(ctx context.Context, pg config.PostgresConfig) error {
	_, env, err := privilegedDatabaseArgs(pg)
	if err != nil {
		return err
	}
	if strings.TrimSpace(pg.Password) == "" {
		return errors.New("缺少 PG_PASSWORD,无法创建应用数据库角色")
	}
	if strings.TrimSpace(pg.Database) == "" {
		return errors.New("缺少 PG_DATABASE,无法为应用数据库角色授权")
	}
	script, err := roleScriptPath()
	if err != nil {
		return err
	}
	env = append(env, "CHAIMIR_APP_PASSWORD="+pg.Password)
	cmd := exec.CommandContext(ctx, "psql",
		"-v", "ON_ERROR_STOP=1",
		"-v", "db_name="+pg.Database,
		"-f", script,
	)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("应用数据库角色授权失败: %w", err)
	}
	return nil
}

// loadMigrationPostgresConfig 只读取迁移/授权需要的数据库变量。
// 本地 scripts/db/init.sh 会委托 migrate 子命令,因此这里不能强制要求 Redis、MinIO 等运行期配置。
func loadMigrationPostgresConfig() (config.PostgresConfig, error) {
	var missing []string
	req := func(key string) string {
		v := strings.TrimSpace(os.Getenv(key))
		if v == "" {
			missing = append(missing, key)
		}
		return v
	}
	portText := req("PG_PORT")
	port, err := strconv.Atoi(portText)
	if err != nil && portText != "" {
		return config.PostgresConfig{}, fmt.Errorf("环境变量 PG_PORT 需为整数,实际=%q", portText)
	}
	pg := config.PostgresConfig{
		Host:         req("PG_HOST"),
		Port:         port,
		Database:     req("PG_DATABASE"),
		User:         req("PG_USER"),
		Password:     req("PG_PASSWORD"),
		SSLMode:      req("PG_SSLMODE"),
		PrivUser:     req("PG_PRIV_USER"),
		PrivPassword: req("PG_PRIV_PASSWORD"),
	}
	if len(missing) > 0 {
		return config.PostgresConfig{}, fmt.Errorf("缺少迁移所需环境变量: %s", strings.Join(missing, ", "))
	}
	return pg, nil
}

// privilegedDatabaseArgs 用 PG_PRIV_USER/PG_PRIV_PASSWORD 生成迁移连接参数与环境变量。
// 密码只进入子进程环境,不进入命令行参数;部署侧还需用 Job 权限与节点隔离限制环境读取面。
func privilegedDatabaseArgs(pg config.PostgresConfig) (string, []string, error) {
	if strings.TrimSpace(pg.PrivUser) == "" || strings.TrimSpace(pg.PrivPassword) == "" {
		return "", nil, errors.New("缺少 PG_PRIV_USER 或 PG_PRIV_PASSWORD,无法执行迁移/授权特权操作")
	}
	dsn := &url.URL{
		Scheme: "postgres",
		User:   url.User(pg.PrivUser),
		Host:   fmt.Sprintf("%s:%d", pg.Host, pg.Port),
		Path:   pg.Database,
	}
	q := dsn.Query()
	q.Set("sslmode", pg.SSLMode)
	dsn.RawQuery = q.Encode()
	env := []string{
		"PGHOST=" + pg.Host,
		fmt.Sprintf("PGPORT=%d", pg.Port),
		"PGDATABASE=" + pg.Database,
		"PGUSER=" + pg.PrivUser,
		"PGPASSWORD=" + pg.PrivPassword,
		"PGSSLMODE=" + pg.SSLMode,
	}
	return dsn.String(), env, nil
}

// runSeed 根据部署形态执行初始化数据写入。
// SaaS 只创建平台管理员;私有化只创建单校租户和首个学校管理员,保持双形态同构但入口互斥。
func runSeed(ctx context.Context, cfg *config.Config) error {
	// 第一步:只装配 identity 服务,业务初始化必须复用 M1 规则而不是复制 SQL。
	svc, closer, err := buildIdentityService(ctx, cfg)
	if err != nil {
		return err
	}
	defer closer()

	// 第二步:按部署形态选择互斥初始化路径,避免 SaaS/私有化数据混写。
	switch cfg.Deploy.Mode {
	case "school":
		result, err := svc.BootstrapPrivateSchool(ctx, identity.BootstrapPrivateSchoolRequest{
			TenantID:  cfg.Bootstrap.SchoolTenantID,
			Code:      cfg.Bootstrap.SchoolTenantCode,
			Name:      cfg.Bootstrap.SchoolName,
			Type:      cfg.Bootstrap.SchoolType,
			Phone:     cfg.Bootstrap.AdminPhone,
			AdminName: cfg.Bootstrap.AdminName,
			Password:  cfg.Bootstrap.AdminPassword,
		})
		if err != nil {
			return err
		}
		slog.Info("私有化首个学校管理员初始化完成",
			slog.String("tenant_id", result.TenantID),
			slog.String("admin_id", result.AdminID),
			slog.Bool("created", result.Created),
		)
	case "saas":
		result, err := svc.BootstrapPlatformAdmin(ctx, identity.BootstrapPlatformAdminRequest{
			Username: cfg.Bootstrap.PlatformAdminUser,
			Name:     cfg.Bootstrap.PlatformAdminName,
			Password: cfg.Bootstrap.PlatformAdminPassword,
		})
		if err != nil {
			return err
		}
		slog.Info("SaaS 平台管理员初始化完成",
			slog.String("admin_id", result.AdminID),
			slog.Bool("created", result.Created),
		)
	default:
		return fmt.Errorf("不支持的 DEPLOY_MODE: %s", cfg.Deploy.Mode)
	}
	return nil
}

// buildIdentityService 只装配 seed 需要的 M1 依赖,不启动 HTTP、Redis、NATS 或对象存储。
func buildIdentityService(ctx context.Context, cfg *config.Config) (*identity.Service, func(), error) {
	// 第一步:创建 seed 必需的数据库连接,由返回的 closer 交给调用方释放。
	database, err := db.New(ctx, cfg.Postgres)
	if err != nil {
		return nil, nil, err
	}
	// 第二步:初始化 identity 服务的纯进程内依赖,任一失败都关闭数据库避免泄漏。
	idgen, err := snowflake.NewNode(cfg.Snowflake.NodeID)
	if err != nil {
		database.Close()
		return nil, nil, err
	}
	cipher, err := crypto.NewCipher([]byte(cfg.Auth.EncryptionKey))
	if err != nil {
		database.Close()
		return nil, nil, err
	}
	// 第三步:构造 M1 服务;seed 不应发布/订阅事件,因此注入会显式拒绝事件操作的总线。
	svc := identity.NewService(
		database,
		auth.NewManager(cfg.Auth),
		seedEventBus{},
		nil,
		idgen,
		cipher,
		identity.LogSmsSender{},
		[]byte(cfg.Auth.HMACKey),
		cfg.Deploy,
		cfg.Identity,
		time.Duration(cfg.Auth.RefreshTTLDay)*24*time.Hour,
	)
	return svc, database.Close, nil
}

// seedEventBus 是部署期 seed 专用事件总线,任何事件操作都视为初始化路径越界。
type seedEventBus struct{}

// Publish 拒绝 seed 期间的事件发布,避免空实现把意外业务事件吞成成功。
func (seedEventBus) Publish(_ context.Context, subject string, _ any) error {
	return fmt.Errorf("seed 初始化不允许发布事件: %s", subject)
}

// Subscribe 拒绝 seed 期间的事件订阅,初始化命令不应消费运行期事件。
func (seedEventBus) Subscribe(subject, _ string, _ eventbus.Handler) (eventbus.Subscription, error) {
	return nil, fmt.Errorf("seed 初始化不允许订阅事件: %s", subject)
}

// Close 满足事件总线契约;seedEventBus 不持有外部连接。
func (seedEventBus) Close() {}

// migrationsPath 返回仓库内迁移目录;部署镜像和本地运行都以当前工作目录为 backend。
func migrationsPath() (string, error) {
	path, err := filepath.Abs(filepath.Join("db", "migrations"))
	if err != nil {
		return "", fmt.Errorf("解析迁移目录失败: %w", err)
	}
	return path, nil
}

// roleScriptPath 返回应用角色授权脚本路径。
func roleScriptPath() (string, error) {
	path, err := filepath.Abs(filepath.Join("..", "scripts", "db", "00_role.sql"))
	if err != nil {
		return "", fmt.Errorf("解析数据库角色脚本失败: %w", err)
	}
	return path, nil
}
