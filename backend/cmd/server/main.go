// server main 是后端服务组合根:加载配置、初始化基础设施、按层装配模块并启动 HTTP/WS。
package main

import (
	"chaimir/internal/contracts"
	"context"
	"errors"
	"fmt"
	"io"
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
	"chaimir/internal/platform/background"
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

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--selftest" {
		fmt.Fprintln(os.Stdout, "后端服务自检通过")
		return
	}
	if err := run(); err != nil {
		slog.Error("server exited", slog.String("error", logging.SanitizeError(err.Error())))
		os.Exit(1)
	}
}

func run() (err error) {
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
	tasks := background.New(ctx)
	var server *http.Server
	var requests *httpx.DrainingHandler
	defer func() {
		stop()
		if closeErr := infra.wsHub.CloseAll("server_shutdown"); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("WebSocket 连接关闭失败: %w", closeErr))
		}
		if server != nil {
			if closeErr := server.Close(); closeErr != nil {
				err = errors.Join(err, fmt.Errorf("HTTP 连接关闭失败: %w", closeErr))
			}
			requests.Drain()
		}
		tasks.Wait()
		err = errors.Join(err, infra.close())
	}()

	router := newRouter(cfg, infra)
	if err := router.SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
		return fmt.Errorf("HTTP trusted proxies 配置非法: %w", err)
	}
	if err := assembleModules(tasks, router.Group(""), cfg, infra); err != nil {
		return err
	}

	requests = httpx.NewDrainingHandler(router)
	server = &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cfg.Server.Addr, cfg.Server.Port),
		Handler:           requests,
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
		<-time.After(time.Duration(cfg.Server.EndpointDrainSeconds) * time.Second)
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

type infrastructure struct {
	database *db.DB
	redis    *redis.Client
	bus      eventbus.ManagedBus
	storage  *storage.Storage
	files    storage.Service
	k8s      *platformk8s.Client
	auth     *auth.Manager
	wsHub    *ws.Hub
	sandbox  io.Closer
	ids      snowflake.Generator
	draining atomic.Bool
}

// beginDraining 标记当前副本正在下线,让 readiness 先于 HTTP 关闭失效。
func (i *infrastructure) beginDraining() {
	i.draining.Store(true)
}

func newInfrastructure(ctx context.Context, cfg *config.Config) (_ *infrastructure, err error) {
	infra := &infrastructure{}
	defer func() {
		if err != nil {
			err = errors.Join(err, infra.close())
		}
	}()

	if infra.database, err = db.New(ctx, cfg.Postgres); err != nil {
		return nil, err
	}
	if infra.redis, err = redis.New(ctx, cfg.Redis); err != nil {
		return nil, err
	}
	if infra.bus, err = eventbus.New(cfg.NATS); err != nil {
		return nil, err
	}
	if infra.storage, err = storage.New(ctx, cfg.ObjectStorage); err != nil {
		return nil, err
	}
	if err := infra.storage.EnsureBuckets(ctx); err != nil {
		return nil, err
	}
	if infra.files, err = storage.NewServiceFromConfig(cfg.Auth, cfg.ObjectStorage, cfg.Upload); err != nil {
		return nil, err
	}
	if infra.k8s, err = platformk8s.New(cfg.Sandbox); err != nil {
		return nil, err
	}
	if infra.ids, err = snowflake.NewNode(cfg.Snowflake.NodeID); err != nil {
		return nil, err
	}
	if infra.wsHub, err = ws.NewHub(ws.NewOriginPolicy(cfg.Server.WSAllowedOrigins, cfg.Server.TrustedProxies), ws.HubOptions{
		ReadTimeout:  time.Duration(cfg.Server.WSReadTimeoutSeconds) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WSWriteTimeoutSeconds) * time.Second,
		PingInterval: time.Duration(cfg.Server.WSPingIntervalSeconds) * time.Second,
		ReadLimit:    cfg.Server.WSReadLimitBytes,
	}); err != nil {
		return nil, err
	}
	infra.auth = auth.NewManager(cfg.Auth)
	return infra, nil
}

// close 也用于启动失败时释放已经初始化的依赖。
func (i *infrastructure) close() error {
	var err error
	if i.bus != nil {
		i.bus.StopConsumers()
	}
	if i.sandbox != nil {
		if closeErr := i.sandbox.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("等待沙箱后台任务失败: %w", closeErr))
		}
	}
	if i.bus != nil {
		if closeErr := i.bus.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("关闭事件总线失败: %w", closeErr))
		}
	}
	if i.redis != nil {
		if closeErr := i.redis.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("关闭 Redis 客户端失败: %w", closeErr))
		}
	}
	if i.database != nil {
		i.database.Close()
	}
	return err
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
	// 为 multipart 边界和非文件字段预留开销，避免误拒绝额度内的文件。
	return max(
		cfg.Upload.ImportMaxBytes,
		cfg.Upload.ContentAttachmentMaxBytes,
		cfg.Upload.ExperimentReportMaxBytes,
		cfg.Upload.CourseMaterialMaxBytes,
		cfg.Upload.TenantLogoMaxBytes,
		cfg.Upload.CourseCoverMaxBytes,
		cfg.Upload.SimBundleMaxBytes+cfg.Upload.SimBundleMetadataMaxBytes,
	) + 1<<20
}

// 探针用 HTTP 状态码向 kubelet 和网关报告结果。
func livenessProbe(c *gin.Context) {
	c.String(http.StatusOK, "ok")
}

// readinessProbe 依次探测启动期强依赖,任一不可用即回 503 让 kubelet 摘掉本副本流量。
// 分项和总耗时、失败原因只进结构化日志并带 trace_id;响应体不含依赖名,
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
		if infra.draining.Load() {
			c.String(http.StatusServiceUnavailable, "not ready")
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(cfg.Server.HealthTimeoutSeconds)*time.Second)
		defer cancel()
		started := time.Now()
		results := make([]slog.Attr, 0, len(dependencies))
		for _, dependency := range dependencies {
			dependencyStarted := time.Now()
			err := dependency.check(infra, ctx)
			if err == nil {
				err = ctx.Err()
			}
			results = append(results, slog.Group(dependency.name,
				slog.Duration("duration", time.Since(dependencyStarted)), slog.Bool("ready", err == nil)))
			if err != nil {
				logging.ErrorContext(c.Request.Context(), "readiness probe failed",
					fmt.Errorf("依赖 %s 不可用: %w", dependency.name, err).Error(),
					slog.String("dependency", dependency.name),
					slog.Duration("duration", time.Since(started)),
					slog.Attr{Key: "checks", Value: slog.GroupValue(results...)})
				c.String(http.StatusServiceUnavailable, "not ready")
				return
			}
		}
		slog.Default().LogAttrs(ctx, slog.LevelDebug, "readiness dependencies succeeded",
			logging.AttrsFromContext(ctx, slog.Duration("duration", time.Since(started)),
				slog.Attr{Key: "checks", Value: slog.GroupValue(results...)})...)
		// 探测期间也可能开始退出，不能以刚完成的依赖检查重新开放流量。
		if infra.draining.Load() {
			c.String(http.StatusServiceUnavailable, "not ready")
			return
		}
		c.String(http.StatusOK, "ready")
	}
}

// assembleModules 按地基、引擎、业务、聚合顺序装配 11 个模块和基础层路由。
func assembleModules(tasks *background.Runner, router gin.IRouter, cfg *config.Config, infra *infrastructure) error {
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
	sandboxSvc, err := RegisterSandboxModule(tasks, SandboxModuleDeps{
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
	infra.sandbox = sandboxSvc
	if err := StartIdentityBackgroundTasks(tasks, cfg.Identity, identitySvc); err != nil {
		return err
	}
	judgeSvc, err := RegisterJudgeModule(tasks, JudgeModuleDeps{Router: router, Database: infra.database, IDs: infra.ids, Config: cfg.Judge, AuthCfg: cfg.Auth, Storage: infra.storage, Sandbox: sandboxSvc, Content: contentSvc, Audit: auditWriter, EventBus: infra.bus, WSHub: infra.wsHub, Auth: infra.auth, Roles: identitySvc})
	if err != nil {
		return err
	}
	stdioJSONAdapters, err := sim.NewStdioJSONBackendRegistry(infra.k8s, cfg.SimBackend, cfg.Sandbox, infra.redis)
	if err != nil {
		return fmt.Errorf("装配 M4 stdio-json 后端计算能力失败: %w", err)
	}
	simSvc, err := RegisterSimModule(tasks, SimModuleDeps{Router: router, Database: infra.database, IDs: infra.ids, Upload: cfg.Upload, FileService: infra.files, SimBackend: cfg.SimBackend, Storage: infra.storage, Audit: auditWriter, WSHub: infra.wsHub, Auth: infra.auth, Roles: identitySvc, EventBus: infra.bus, BackendAdapters: stdioJSONAdapters})
	if err != nil {
		return err
	}
	experimentSvc, err := RegisterExperimentModule(tasks, ExperimentModuleDeps{Router: router, Database: infra.database, IDs: infra.ids, Config: cfg.Experiment, Content: contentSvc, ContentClone: contentSvc, Sandbox: sandboxSvc, SandboxClone: sandboxSvc, Judge: judgeSvc, Sim: simSvc, Audit: auditWriter, EventBus: infra.bus, Storage: infra.storage, Upload: cfg.Upload, FileService: infra.files, Auth: infra.auth, Roles: identitySvc})
	if err != nil {
		return err
	}
	teachingSvc, err := RegisterTeachingModule(tasks, TeachingModuleDeps{Router: router, Database: infra.database, IDs: infra.ids, Config: cfg.Teaching, Upload: cfg.Upload, FileService: infra.files, Content: contentSvc, ContentClone: contentSvc, ExperimentClone: experimentSvc, Sim: simSvc, Judge: judgeSvc, Transfer: transferSvc, Storage: infra.storage, Audit: auditWriter, EventBus: infra.bus, Auth: infra.auth, Roles: identitySvc})
	if err != nil {
		return err
	}
	contestSvc, err := RegisterContestModule(tasks, ContestModuleDeps{CleanupTimeout: time.Duration(cfg.Upload.CleanupTimeoutSeconds) * time.Second, Router: router, Database: infra.database, IDs: infra.ids, Config: cfg.Contest, AuthConfig: cfg.Auth, FileService: infra.files, Storage: infra.storage, Content: contentSvc, ContentImport: contentSvc, Sandbox: sandboxSvc, Judge: judgeSvc, Fingerprint: judgeSvc, Audit: auditWriter, EventBus: infra.bus, WSHub: infra.wsHub, Auth: infra.auth, Roles: identitySvc})
	if err != nil {
		return err
	}
	if _, err := RegisterNotifyModule(tasks, NotifyModuleDeps{Router: router, Database: infra.database, IDs: infra.ids, Redis: infra.redis, Hub: infra.wsHub, Config: cfg.Notify, EventBus: infra.bus, Auth: infra.auth, Roles: identitySvc, Contests: contestSvc, Experiments: experimentSvc, Audit: auditWriter}); err != nil {
		return err
	}
	gradeSvc, err := RegisterGradeModule(tasks, GradeModuleDeps{CleanupTimeout: time.Duration(cfg.Upload.CleanupTimeoutSeconds) * time.Second, Router: router, Database: infra.database, IDs: infra.ids, Audit: auditWriter, Teaching: teachingSvc, EventBus: infra.bus, Redis: infra.redis, Storage: infra.storage, FileService: infra.files, Config: cfg.Grade, Auth: infra.auth, Roles: identitySvc})
	if err != nil {
		return err
	}
	if _, err := RegisterAdminModule(tasks, AdminModuleDeps{CleanupTimeout: time.Duration(cfg.Upload.CleanupTimeoutSeconds) * time.Second, Metrics: []contracts.AlertMetricReader{identitySvc, sandboxSvc, judgeSvc, transferSvc, gradeSvc}, Recipients: identitySvc, Router: router, Database: infra.database, IDs: infra.ids, Audit: auditWriter, Identity: identitySvc, Stats: identitySvc, AuditRead: identitySvc, Teaching: teachingSvc, Sandbox: sandboxSvc, Experiment: experimentSvc, Contest: contestSvc, EventBus: infra.bus, Monitoring: cfg.Monitoring, Config: cfg.Admin, FileService: infra.files, Transfer: transferSvc, Storage: infra.storage, Auth: infra.auth, Roles: identitySvc}); err != nil {
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
