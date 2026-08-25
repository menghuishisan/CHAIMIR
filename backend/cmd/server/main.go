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
	"sync/atomic"
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

	router := newRouter(cfg, infra)
	if err := router.SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
		return fmt.Errorf("HTTP trusted proxies 配置非法: %w", err)
	}
	if err := assembleModules(ctx, router.Group(""), cfg, infra); err != nil {
		return err
	}

	server := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cfg.Server.Addr, cfg.Server.Port),
		Handler:           router,
		ReadHeaderTimeout: time.Duration(cfg.Server.ReadHeaderTimeoutSeconds) * time.Second,
		ReadTimeout:       time.Duration(cfg.Server.ReadTimeoutSeconds) * time.Second,
		WriteTimeout:      time.Duration(cfg.Server.WriteTimeoutSeconds) * time.Second,
		IdleTimeout:       time.Duration(cfg.Server.IdleTimeoutSeconds) * time.Second,
		MaxHeaderBytes:    cfg.Server.MaxHeaderBytes,
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
		infra.beginDraining()
		drainTimer := time.NewTimer(time.Duration(cfg.Server.EndpointDrainSeconds) * time.Second)
		<-drainTimer.C
		drainTimer.Stop()
		var shutdownErrs []error
		if err := infra.wsHub.CloseAll("server_draining"); err != nil {
			shutdownErrs = append(shutdownErrs, fmt.Errorf("WebSocket 连接关闭失败: %w", err))
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.ShutdownTimeoutSeconds)*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			shutdownErrs = append(shutdownErrs, fmt.Errorf("HTTP 服务关闭失败: %w", err))
		}
		if err := <-errCh; err != nil {
			shutdownErrs = append(shutdownErrs, err)
		}
		return errors.Join(shutdownErrs...)
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
	files    storage.Service
	k8s      *platformk8s.Client
	auth     *auth.Manager
	wsHub    *ws.Hub
	ids      snowflake.Generator
	draining atomic.Bool
}

// beginDraining 标记当前副本正在下线,让 readiness 先于 HTTP 关闭失效。
func (i *infrastructure) beginDraining() {
	if i != nil {
		i.draining.Store(true)
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
	objectStore, err := storage.New(ctx, cfg.ObjectStorage)
	if err != nil {
		return nil, errors.Join(err, closeStartupInfrastructure(database, redisClient, bus))
	}
	if err := objectStore.EnsureBuckets(ctx); err != nil {
		return nil, errors.Join(err, closeStartupInfrastructure(database, redisClient, bus))
	}
	fileService, err := storage.NewServiceFromConfig(cfg.Auth, cfg.ObjectStorage, cfg.Upload)
	if err != nil {
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
	hub, err := ws.NewHub(ws.NewOriginPolicy(cfg.Server.WSAllowedOrigins, cfg.Server.TrustedProxies), ws.HubOptions{
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
		files:    fileService,
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
func newRouter(cfg *config.Config, infra *infrastructure) *gin.Engine {
	r := gin.New()
	r.Use(
		response.TraceMiddleware(),
		httpx.RequestBodyLimitMiddleware(cfg.Server.MaxJSONBodyBytes, maxMultipartBodyBytes(cfg)),
		httpx.AuditContextMiddleware(),
		response.RecoveryMiddleware(),
	)
	r.NoRoute(response.NoRoute)
	r.GET("/-/healthz", livenessProbe)
	r.GET("/api/healthz", livenessProbe)
	r.GET("/-/readyz", readinessProbe(cfg, infra))
	return r
}

// maxMultipartBodyBytes 返回所有已注册 multipart 路由允许的最大请求体。
// 业务 handler 仍逐项校验自己的文件额度；此处只确保 Gin 解析前存在硬上限。
func maxMultipartBodyBytes(cfg *config.Config) int64 {
	limits := []int64{
		cfg.Upload.ImportMaxBytes,
		cfg.Upload.ContentAttachmentMaxBytes,
		cfg.Upload.ExperimentReportMaxBytes,
		cfg.Upload.CourseMaterialMaxBytes,
		cfg.Upload.TenantLogoMaxBytes,
		cfg.Upload.CourseCoverMaxBytes,
		cfg.Upload.SimBundleMaxBytes + cfg.Upload.SimBundleMetadataMaxBytes,
	}
	maxBytes := limits[0]
	for _, limit := range limits[1:] {
		if limit > maxBytes {
			maxBytes = limit
		}
	}
	// 为 multipart 边界和非文件字段预留固定且很小的开销，避免业务额度被编码开销误拒绝。
	return maxBytes + 1<<20
}

// livenessProbe 回存活明文。探针消费者是 kubelet 与网关,不是终端用户,
// 因此不套业务 JSON 信封:HTTP 状态码本身就是探针唯一可判定的信号
// (与 storage 下载流一样,是 docs/总-API接口总览.md 声明的显式非信封出口)。
func livenessProbe(c *gin.Context) {
	c.String(http.StatusOK, "ok")
}

// readinessProbe 依次探测启动期强依赖,任一不可用即回 503 让 kubelet 摘掉本副本流量。
// 失败原因(哪个依赖、原始错误链)只进结构化日志并带 trace_id;响应体不含依赖名,
// 既不向调用者播报内部拓扑,也不吞错(§8 错误暴露分层)。
func readinessProbe(cfg *config.Config, infra *infrastructure) gin.HandlerFunc {
	dependencies := []struct {
		name  string
		check func(*infrastructure, context.Context) error
	}{
		{"postgres", func(i *infrastructure, ctx context.Context) error { return i.database.Ping(ctx) }},
		{"redis", func(i *infrastructure, ctx context.Context) error { return i.redis.Ping(ctx) }},
		{"kubernetes", func(i *infrastructure, ctx context.Context) error { return i.k8s.Healthz(ctx) }},
	}
	return func(c *gin.Context) {
		if infra != nil && infra.draining.Load() {
			c.String(http.StatusServiceUnavailable, "not ready")
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(cfg.Server.HealthTimeoutSeconds)*time.Second)
		defer cancel()
		for _, dependency := range dependencies {
			if err := dependency.check(infra, ctx); err != nil {
				logging.ErrorContext(c.Request.Context(), "readiness probe failed",
					fmt.Errorf("依赖 %s 不可用: %w", dependency.name, err).Error(),
					slog.String("dependency", dependency.name))
				c.String(http.StatusServiceUnavailable, "not ready")
				return
			}
		}
		c.String(http.StatusOK, "ready")
	}
}

// assembleModules 按地基、引擎、业务、聚合顺序装配 11 个模块和基础层路由。
func assembleModules(ctx context.Context, router gin.IRouter, cfg *config.Config, infra *infrastructure) error {
	identitySvc, err := RegisterIdentityModule(IdentityModuleDeps{
		Router:      router,
		Database:    infra.database,
		Auth:        infra.auth,
		Redis:       infra.redis,
		IDs:         infra.ids,
		Config:      *cfg,
		Storage:     infra.storage,
		FileService: infra.files,
		EventBus:    infra.bus,
	})
	if err != nil {
		return err
	}
	auditWriter := identitySvc.AuditWriter()
	transferSvc, err := RegisterTransfer(TransferDeps{Router: router, Database: infra.database, IDs: infra.ids, Config: cfg.Transfer, FileService: infra.files, Auth: infra.auth, Roles: identitySvc})
	if err != nil {
		return err
	}
	if err := storage.RegisterDownloadRoutes(router, infra.storage, infra.redis, infra.files, infra.auth); err != nil {
		return err
	}
	contentSvc, err := RegisterContentModule(ContentModuleDeps{Router: router, Database: infra.database, IDs: infra.ids, Upload: cfg.Upload, FileService: infra.files, Storage: infra.storage, Audit: auditWriter, Auth: infra.auth, Roles: identitySvc})
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
	simSvc, err := RegisterSimModule(ctx, SimModuleDeps{Router: router, Database: infra.database, IDs: infra.ids, Upload: cfg.Upload, FileService: infra.files, SimBackend: cfg.SimBackend, Storage: infra.storage, Audit: auditWriter, WSHub: infra.wsHub, Auth: infra.auth, Roles: identitySvc, BackendAdapters: stdioJSONAdapters})
	if err != nil {
		return err
	}
	// notify 模块通过事件总线订阅业务推送/通知,注册后无需被其他模块直接依赖。
	if _, err := RegisterNotifyModule(ctx, NotifyModuleDeps{Router: router, Database: infra.database, IDs: infra.ids, Redis: infra.redis, Hub: infra.wsHub, Config: cfg.Notify, EventBus: infra.bus, Auth: infra.auth, Roles: identitySvc, Audit: auditWriter}); err != nil {
		return err
	}
	teachingSvc, err := RegisterTeachingModule(ctx, TeachingModuleDeps{Router: router, Database: infra.database, IDs: infra.ids, Config: cfg.Teaching, Upload: cfg.Upload, FileService: infra.files, Content: contentSvc, Judge: judgeSvc, Transfer: transferSvc, Storage: infra.storage, Audit: auditWriter, EventBus: infra.bus, Auth: infra.auth, Roles: identitySvc})
	if err != nil {
		return err
	}
	experimentSvc, err := RegisterExperimentModule(ctx, ExperimentModuleDeps{Router: router, Database: infra.database, IDs: infra.ids, Config: cfg.Experiment, Content: contentSvc, Sandbox: sandboxSvc, Judge: judgeSvc, Sim: simSvc, Audit: auditWriter, EventBus: infra.bus, Storage: infra.storage, Upload: cfg.Upload, FileService: infra.files, Auth: infra.auth, Roles: identitySvc})
	if err != nil {
		return err
	}
	contestSvc, err := RegisterContestModule(ctx, ContestModuleDeps{Router: router, Database: infra.database, IDs: infra.ids, Config: cfg.Contest, AuthConfig: cfg.Auth, FileService: infra.files, Storage: infra.storage, Content: contentSvc, ContentImport: contentSvc, Sandbox: sandboxSvc, Judge: judgeSvc, Fingerprint: judgeSvc, Audit: auditWriter, EventBus: infra.bus, WSHub: infra.wsHub, Auth: infra.auth, Roles: identitySvc})
	if err != nil {
		return err
	}
	if _, err := RegisterAdminModule(ctx, AdminModuleDeps{Router: router, Database: infra.database, IDs: infra.ids, Audit: auditWriter, Identity: identitySvc, Stats: identitySvc, AuditRead: identitySvc, Teaching: teachingSvc, Sandbox: sandboxSvc, Experiment: experimentSvc, Contest: contestSvc, EventBus: infra.bus, Monitoring: cfg.Monitoring, Config: cfg.Admin, AuthConfig: cfg.Auth, FileService: infra.files, Transfer: transferSvc, Storage: infra.storage, Auth: infra.auth, Roles: identitySvc}); err != nil {
		return err
	}
	if _, err := RegisterGradeModule(ctx, GradeModuleDeps{Router: router, Database: infra.database, IDs: infra.ids, Audit: auditWriter, Teaching: teachingSvc, EventBus: infra.bus, Redis: infra.redis, Storage: infra.storage, FileService: infra.files, Config: cfg.Grade, Auth: infra.auth, Roles: identitySvc}); err != nil {
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
