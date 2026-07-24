// server main 是后端服务组合根:加载配置、初始化基础设施、按层装配模块并启动 HTTP/WS。
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"chaimir/internal/modules/sandbox"
	"chaimir/internal/modules/sim"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/httpx"
	platformk8s "chaimir/internal/platform/k8s"
	"chaimir/internal/platform/redis"
	"chaimir/internal/platform/response"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
	"chaimir/pkg/snowflake"

	"github.com/gin-gonic/gin"
)

// main 启动生产 HTTP 服务;所有失败都在启动边界显式终止。
func main() {
	if err := run(); err != nil {
		slog.Error("server exited", slog.String("error", logging.SanitizeError(err.Error())))
		os.Exit(1)
	}
}

// run 负责串联配置、基础设施、模块装配和优雅退出。
func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logging.Setup(cfg.Server.LogLevel, cfg.Server.LogFormat)
	gin.SetMode(ginMode(cfg.Server.AppEnv))

	infra, err := newInfrastructure(ctx, cfg)
	if err != nil {
		return err
	}
	defer infra.close()

	maintenance := &maintenanceGate{}
	router := newRouter(cfg, infra, maintenance)
	if err := router.SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
		return fmt.Errorf("HTTP trusted proxies 配置非法: %w", err)
	}
	if err := assembleModules(ctx, router.Group(""), cfg, infra, maintenance); err != nil {
		return err
	}

	server := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cfg.Server.Addr, cfg.Server.Port),
		Handler:           router,
		ReadHeaderTimeout: time.Duration(cfg.Server.ReadHeaderTimeoutSeconds) * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()
	slog.Info("server started", slog.String("addr", server.Addr))

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.ShutdownTimeoutSeconds)*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("HTTP 服务关闭失败: %w", err)
		}
		return <-errCh
	case err := <-errCh:
		return err
	}
}

// infrastructure 汇总组合根持有的共享基础设施实例。
type infrastructure struct {
	database *db.DB
	redis    *redis.Client
	bus      eventbus.Bus
	storage  *storage.Storage
	k8s      *platformk8s.Client
	auth     *auth.Manager
	wsHub    *ws.Hub
	ids      snowflake.Generator
}

// maintenanceGate 在所有业务路由注册完成后接入 M9 维护配置读取器。
type maintenanceGate struct {
	check func(context.Context) (bool, error)
}

// Handler 在维护模式下仅放行健康、登录和维护配置路由。
func (g *maintenanceGate) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		configRoute := path == "/api/v1/admin/configs" || path == "/api/v1/admin/configs/maintenance_mode" || strings.HasPrefix(path, "/api/v1/admin/configs/maintenance_mode/")
		if g == nil || g.check == nil || strings.HasPrefix(path, "/-/health") || strings.HasPrefix(path, "/api/healthz") || strings.HasPrefix(path, "/api/v1/auth/") || configRoute {
			c.Next()
			return
		}
		enabled, err := g.check(c.Request.Context())
		if err != nil {
			response.Fail(c, err)
			c.Abort()
			return
		}
		if enabled {
			response.Fail(c, apperr.ErrAdminMaintenance)
			c.Abort()
			return
		}
		c.Next()
	}
}

// newInfrastructure 初始化所有模块共享的基础设施,任何依赖不可用都 fail-fast。
func newInfrastructure(ctx context.Context, cfg *config.Config) (*infrastructure, error) {
	database, err := db.New(ctx, cfg.Postgres)
	if err != nil {
		return nil, err
	}
	redisClient, err := redis.New(ctx, cfg.Redis)
	if err != nil {
		return nil, errors.Join(err, closeStartupInfrastructure(database, nil, nil))
	}
	bus, err := eventbus.New(cfg.NATS)
	if err != nil {
		return nil, errors.Join(err, closeStartupInfrastructure(database, redisClient, nil))
	}
	objectStore, err := storage.New(ctx, cfg.MinIO)
	if err != nil {
		return nil, errors.Join(err, closeStartupInfrastructure(database, redisClient, bus))
	}
	if err := objectStore.EnsureBuckets(ctx); err != nil {
		return nil, errors.Join(err, closeStartupInfrastructure(database, redisClient, bus))
	}
	k8sClient, err := platformk8s.New(cfg.Sandbox)
	if err != nil {
		return nil, errors.Join(err, closeStartupInfrastructure(database, redisClient, bus))
	}
	ids, err := snowflake.NewNode(cfg.Snowflake.NodeID)
	if err != nil {
		return nil, errors.Join(err, closeStartupInfrastructure(database, redisClient, bus))
	}
	hub, err := ws.NewHub(ws.NewOriginPolicy(cfg.Server.WSAllowedOrigins), ws.HubOptions{
		ReadTimeout:  time.Duration(cfg.Server.WSReadTimeoutSeconds) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WSWriteTimeoutSeconds) * time.Second,
		PingInterval: time.Duration(cfg.Server.WSPingIntervalSeconds) * time.Second,
		ReadLimit:    cfg.Server.WSReadLimitBytes,
	})
	if err != nil {
		return nil, errors.Join(err, closeStartupInfrastructure(database, redisClient, bus))
	}
	return &infrastructure{
		database: database,
		redis:    redisClient,
		bus:      bus,
		storage:  objectStore,
		k8s:      k8sClient,
		auth:     auth.NewManager(cfg.Auth),
		wsHub:    hub,
		ids:      ids,
	}, nil
}

// closeStartupInfrastructure 释放启动阶段已创建的依赖，并返回需要向上报告的关闭错误。
func closeStartupInfrastructure(database *db.DB, redisClient *redis.Client, bus eventbus.Bus) error {
	if bus != nil {
		bus.Close()
	}
	var closeErr error
	if redisClient != nil {
		if err := redisClient.Close(); err != nil {
			closeErr = fmt.Errorf("关闭 Redis 客户端失败: %w", err)
		}
	}
	if database != nil {
		database.Close()
	}
	return closeErr
}

// close 释放长连接资源;HTTP server 自身由 run 的 Shutdown 管理。
func (i *infrastructure) close() {
	if i == nil {
		return
	}
	if i.bus != nil {
		i.bus.Close()
	}
	if i.redis != nil {
		if err := i.redis.Close(); err != nil {
			slog.Error("close redis failed", slog.String("error", logging.SanitizeError(err.Error())))
		}
	}
	if i.database != nil {
		i.database.Close()
	}
}

// newRouter 创建 HTTP 路由器、统一 trace 中间件和健康探针。
func newRouter(cfg *config.Config, infra *infrastructure, maintenance *maintenanceGate) *gin.Engine {
	r := gin.New()
	r.Use(response.TraceMiddleware(), httpx.AuditContextMiddleware(), response.RecoveryMiddleware(), maintenance.Handler())
	r.NoRoute(response.NoRoute)
	r.GET("/-/healthz", func(c *gin.Context) {
		response.OK(c, map[string]string{"status": "ok"})
	})
	r.GET("/api/healthz", func(c *gin.Context) {
		response.OK(c, map[string]string{"status": "ok"})
	})
	r.GET("/-/readyz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(cfg.Server.HealthTimeoutSeconds)*time.Second)
		defer cancel()
		if err := infra.database.Ping(ctx); err != nil {
			response.Fail(c, err)
			return
		}
		if err := infra.redis.Ping(ctx); err != nil {
			response.Fail(c, err)
			return
		}
		if err := infra.k8s.Healthz(ctx); err != nil {
			response.Fail(c, err)
			return
		}
		response.OK(c, map[string]string{"status": "ready"})
	})
	return r
}

// assembleModules 按地基、引擎、业务、聚合顺序装配 11 个模块和基础层路由。
func assembleModules(ctx context.Context, router gin.IRouter, cfg *config.Config, infra *infrastructure, maintenance *maintenanceGate) error {
	identitySvc, err := RegisterIdentityModule(IdentityModuleDeps{
		Router:   router,
		Database: infra.database,
		Auth:     infra.auth,
		Redis:    infra.redis,
		IDs:      infra.ids,
		Config:   *cfg,
		EventBus: infra.bus,
	})
	if err != nil {
		return err
	}
	auditWriter := identitySvc.AuditWriter()
	transferSvc, err := RegisterTransfer(TransferDeps{Router: router, Database: infra.database, IDs: infra.ids, Config: cfg.Transfer, AuthConfig: cfg.Auth, Auth: infra.auth, Roles: identitySvc})
	if err != nil {
		return err
	}
	if err := storage.RegisterDownloadRoutes(router, infra.storage, infra.redis, cfg.Auth.HMACKey, infra.auth); err != nil {
		return err
	}
	contentSvc, err := RegisterContentModule(ContentModuleDeps{Router: router, Database: infra.database, IDs: infra.ids, Upload: cfg.Upload, MinIO: cfg.MinIO, AuthCfg: cfg.Auth, Storage: infra.storage, Audit: auditWriter, Auth: infra.auth, Roles: identitySvc})
	if err != nil {
		return err
	}
	sandboxSvc, err := RegisterSandboxModule(ctx, SandboxModuleDeps{
		Router:       router,
		Database:     infra.database,
		IDs:          infra.ids,
		Config:       cfg.Sandbox,
		Storage:      infra.storage,
		K8s:          infra.k8s,
		Audit:        auditWriter,
		EventBus:     infra.bus,
		WSHub:        infra.wsHub,
		Auth:         infra.auth,
		Roles:        identitySvc,
		Capabilities: map[string]sandbox.ChainCapability{},
	})
	if err != nil {
		return err
	}
	if err := StartIdentityBackgroundTasks(ctx, cfg.Identity, identitySvc); err != nil {
		return err
	}
	judgeSvc, err := RegisterJudgeModule(ctx, JudgeModuleDeps{Router: router, Database: infra.database, IDs: infra.ids, Config: cfg.Judge, AuthCfg: cfg.Auth, Storage: infra.storage, Sandbox: sandboxSvc, Content: contentSvc, Audit: auditWriter, EventBus: infra.bus, WSHub: infra.wsHub, Auth: infra.auth, Roles: identitySvc})
	if err != nil {
		return err
	}
	stdioJSONAdapters, err := sim.NewStdioJSONBackendRegistry(infra.k8s, cfg.SimBackend, cfg.Sandbox)
	if err != nil {
		return fmt.Errorf("装配 M4 stdio-json 后端计算能力失败: %w", err)
	}
	simSvc, err := RegisterSimModule(SimModuleDeps{Router: router, Database: infra.database, IDs: infra.ids, Upload: cfg.Upload, MinIO: cfg.MinIO, AuthConfig: cfg.Auth, Storage: infra.storage, Audit: auditWriter, WSHub: infra.wsHub, Auth: infra.auth, Roles: identitySvc, BackendAdapters: stdioJSONAdapters})
	if err != nil {
		return err
	}
	notifySvc, err := RegisterNotifyModule(ctx, NotifyModuleDeps{Router: router, Database: infra.database, IDs: infra.ids, Redis: infra.redis, Hub: infra.wsHub, Config: cfg.Notify, EventBus: infra.bus, Auth: infra.auth, Roles: identitySvc, Audit: auditWriter})
	if err != nil {
		return err
	}
	teachingSvc, err := RegisterTeachingModule(ctx, TeachingModuleDeps{Router: router, Database: infra.database, IDs: infra.ids, Config: cfg.Teaching, Upload: cfg.Upload, MinIO: cfg.MinIO, AuthCfg: cfg.Auth, Content: contentSvc, Judge: judgeSvc, Transfer: transferSvc, Storage: infra.storage, Audit: auditWriter, EventBus: infra.bus, Auth: infra.auth, Roles: identitySvc})
	if err != nil {
		return err
	}
	experimentSvc, err := RegisterExperimentModule(ctx, ExperimentModuleDeps{Router: router, Database: infra.database, IDs: infra.ids, Config: cfg.Experiment, Upload: cfg.Upload, MinIO: cfg.MinIO, AuthCfg: cfg.Auth, Content: contentSvc, Sandbox: sandboxSvc, Judge: judgeSvc, Sim: simSvc, Audit: auditWriter, EventBus: infra.bus, Storage: infra.storage, Auth: infra.auth, Roles: identitySvc})
	if err != nil {
		return err
	}
	contestSvc, err := RegisterContestModule(ctx, ContestModuleDeps{Router: router, Database: infra.database, IDs: infra.ids, Config: cfg.Contest, AuthConfig: cfg.Auth, Content: contentSvc, ContentImport: contentSvc, Sandbox: sandboxSvc, Judge: judgeSvc, Fingerprint: judgeSvc, Notify: notifySvc, Audit: auditWriter, EventBus: infra.bus, Auth: infra.auth, Roles: identitySvc})
	if err != nil {
		return err
	}
	adminSvc, err := RegisterAdminModule(ctx, AdminModuleDeps{Router: router, Database: infra.database, IDs: infra.ids, Audit: auditWriter, Identity: identitySvc, Stats: identitySvc, AuditRead: identitySvc, Teaching: teachingSvc, Sandbox: sandboxSvc, Experiment: experimentSvc, Contest: contestSvc, Notify: notifySvc, Monitoring: cfg.Monitoring, Config: cfg.Admin, Upload: cfg.Upload, MinIO: cfg.MinIO, AuthConfig: cfg.Auth, Transfer: transferSvc, Storage: infra.storage, Auth: infra.auth, Roles: identitySvc})
	if err != nil {
		return err
	}
	maintenance.check = adminSvc.MaintenanceEnabled
	if _, err := RegisterGradeModule(ctx, GradeModuleDeps{Router: router, Database: infra.database, IDs: infra.ids, Audit: auditWriter, Teaching: teachingSvc, Notify: notifySvc, EventBus: infra.bus, Redis: infra.redis, Storage: infra.storage, Upload: cfg.Upload, MinIO: cfg.MinIO, AuthConfig: cfg.Auth, Config: cfg.Grade, Auth: infra.auth, Roles: identitySvc}); err != nil {
		return err
	}
	return nil
}

// ginMode 把部署环境映射为 Gin 运行模式。
func ginMode(appEnv string) string {
	if appEnv == "prod" || appEnv == "production" {
		return gin.ReleaseMode
	}
	return gin.DebugMode
}
