// Chaimir 后端总入口。
// 依据 docs/总-工程目录设计.md §2.0:
//
//	加载配置 → 初始化基础设施(db/redis/nats/k8s/storage)→ 按分层装配各模块 → 启动 HTTP/WS。
//
// 装配顺序遵循分层(地基→引擎→业务→聚合),保证依赖就绪;反向通信走事件总线。
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/k8s"
	"chaimir/internal/platform/redis"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/logging"
	"chaimir/pkg/snowflake"
)

// main 是进程入口,把致命启动错误统一记录后退出。
func main() {
	if err := run(); err != nil {
		logStartupFailure(err)
		os.Exit(1)
	}
}

// run 完成装配并阻塞运行;返回错误即致命退出。
func run() error {
	// 第一步:启动边界只从环境变量读取配置,任何缺失或格式错误立即 fail-fast。
	cfg, err := loadRuntimeConfig(".env")
	if err != nil {
		return err
	}
	logging.Setup(cfg.Server.LogLevel, cfg.Server.LogFormat)
	slog.Info("配置加载完成",
		slog.String("deploy_mode", cfg.Deploy.Mode),
		slog.Bool("platform_enabled", cfg.Deploy.PlatformEnabled),
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 第二步:先初始化数据库、缓存、事件总线、对象存储和 K8s 等强依赖。
	app, err := initInfra(ctx, cfg)
	if err != nil {
		return err
	}
	defer app.close()

	// 第三步:基础设施就绪后再按分层顺序装配模块,防止 nil 依赖进入生产服务。
	// 按分层顺序装配各模块(地基→引擎→业务→聚合)。
	if err := assembleModules(&moduleDeps{ctx: ctx, cfg: cfg, infra: app}); err != nil {
		return err
	}

	// 第四步:全部依赖和路由注册完成后才启动 HTTP/WS 服务。
	slog.Info("Chaimir 后端启动", slog.String("addr", cfg.Server.Addr), slog.Int("port", cfg.Server.Port))
	return app.server.run(ctx)
}

// loadRuntimeConfig 加载本地 .env 后读取运行配置;任何配置错误都 fail-fast。
func loadRuntimeConfig(dotEnvPath string) (*config.Config, error) {
	if err := config.LoadDotEnv(dotEnvPath); err != nil {
		return nil, err
	}
	return config.Load()
}

// infra 持有所有基础设施单例。
type infra struct {
	db            *db.DB
	redis         *redis.Client
	bus           eventbus.Bus
	store         *storage.Storage
	k8s           *k8s.Client
	auth          *auth.Manager
	audit         audit.Writer
	identity      contracts.IdentityService
	identityAdmin contracts.IdentityAdminService
	content       contracts.ContentReadService
	contentImport contracts.ContentImportService
	contentJudge  contracts.ContentJudgeService
	teaching      contracts.TeachingService
	experiment    contracts.ExperimentService
	contest       contracts.ContestService
	sandbox       contracts.SandboxService
	judge         contracts.JudgeService
	sim           contracts.SimService
	notify        contracts.NotifyService
	hub           *ws.Hub
	idgen         *snowflake.Node
	server        *httpServer
}

// close 释放基础设施资源。
func (a *infra) close() {
	if a.bus != nil {
		a.bus.Close()
	}
	if a.redis != nil {
		if err := a.redis.Close(); err != nil {
			logInfraCloseFailure("redis", err)
		}
	}
	if a.db != nil {
		a.db.Close()
	}
}

// logStartupFailure 记录致命启动错误。
func logStartupFailure(err error) {
	// 启动失败通常发生在依赖连接或配置加载阶段,可能携带连接串/凭据片段。
	logging.ErrorContext(context.Background(), "服务启动失败", err.Error())
}

// logInfraCloseFailure 记录资源释放错误。
func logInfraCloseFailure(resource string, err error) {
	// 关闭阶段不能再向上返回影响进程退出,但仍必须结构化记录,避免问题被静默吞掉。
	logging.ErrorContext(context.Background(), "基础设施资源关闭失败", err.Error(), slog.String("resource", resource))
}

// initInfra 初始化全部基础设施;关键依赖(db/redis/nats/minio)失败即 fail-fast。
func initInfra(ctx context.Context, cfg *config.Config) (*infra, error) {
	a := &infra{}

	// 第一步:创建全局 ID 生成器,后续模块装配统一复用同一个节点配置。
	idgen, err := snowflake.NewNode(cfg.Snowflake.NodeID)
	if err != nil {
		return nil, err
	}
	a.idgen = idgen

	// 第二步:初始化持久化、缓存、事件总线和对象存储;这些生产强依赖缺失即停止启动。
	if a.db, err = db.New(ctx, cfg.Postgres); err != nil {
		return nil, err
	}
	slog.Info("PostgreSQL 连接就绪", slog.Bool("privileged_pool", a.db.HasPrivileged()))

	if a.redis, err = redis.New(ctx, cfg.Redis); err != nil {
		return nil, err
	}
	slog.Info("Redis 连接就绪")

	if a.bus, err = eventbus.New(cfg.NATS); err != nil {
		return nil, err
	}
	slog.Info("NATS 事件总线就绪")

	if a.store, err = storage.New(ctx, cfg.MinIO); err != nil {
		return nil, err
	}
	if err := a.store.EnsureBuckets(ctx); err != nil {
		return nil, err
	}
	slog.Info("MinIO 对象存储就绪")

	// 第三步:初始化无外部连接的进程内基础设施,供 API 鉴权和 WebSocket 复用。
	a.auth = auth.NewManager(cfg.Auth)
	a.hub = ws.NewHub(ws.NewOriginPolicy(cfg.Server.WSAllowedOrigins))

	// 第四步:初始化沙箱编排强依赖;学生代码执行能力不能在运行期切换为无沙箱路径。
	// K8s 客户端(M2 沙箱编排)。M2 已进入生产实现阶段,启动失败必须直接暴露。
	if a.k8s, err = k8s.New(cfg.Sandbox); err != nil {
		return nil, err
	}
	slog.Info("K8s 客户端就绪")

	// 第五步:最后构造 HTTP 服务器,把基础依赖纳入健康/就绪检查。
	// HTTP 服务器 + 就绪检查(db/redis 纳入就绪探针)。
	a.server = newHTTPServer(cfg.Server, map[string]healthChecker{
		"postgres": a.db,
		"redis":    a.redis,
	})
	return a, nil
}
