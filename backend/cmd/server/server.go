// HTTP 服务器组装:Gin 引擎、全局中间件、健康检查端点。
// 依据 deploy/base/backend/deployment.yaml:就绪 /api/healthz、存活 /api/livez。
// 全局中间件:trace_id(贯穿日志/响应)、Recovery(panic 转统一错误)。
package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/config"
	"chaimir/pkg/apperr"
	"chaimir/pkg/response"

	"github.com/gin-gonic/gin"
)

// healthChecker 是被纳入就绪检查的依赖(如 db/redis)。
type healthChecker interface {
	Ping(ctx context.Context) error
}

// httpServer 封装 Gin 引擎与监听配置。
type httpServer struct {
	engine *gin.Engine
	cfg    config.ServerConfig
	checks map[string]healthChecker
}

// newHTTPServer 创建 HTTP 服务器,装配全局中间件与健康检查路由。
func newHTTPServer(cfg config.ServerConfig, checks map[string]healthChecker) *httpServer {
	if cfg.AppEnv == "prod" {
		gin.SetMode(gin.ReleaseMode)
	}
	engine := gin.New()
	// trace_id 优先建立,随后把同一 trace/IP 注入审计上下文;Recovery 放最后,确保 panic
	// 也能走统一响应和带 trace_id 的日志,同时审计层不会自己生成第二套追踪编号。
	engine.Use(response.TraceMiddleware())
	engine.Use(auditRequestContextMiddleware())
	engine.Use(recoveryMiddleware())

	s := &httpServer{engine: engine, cfg: cfg, checks: checks}
	s.registerHealth()
	return s
}

// apiV1 返回 /api/v1 路由组,供各模块装配挂载业务路由。
func (s *httpServer) apiV1() *gin.RouterGroup { return s.engine.Group("/api/v1") }

// registerHealth 注册存活/就绪探针。
func (s *httpServer) registerHealth() {
	s.engine.GET("/api/livez", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "alive"})
	})
	s.engine.GET("/api/healthz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(s.cfg.HealthTimeoutSeconds)*time.Second)
		defer cancel()
		for name, hc := range s.checks {
			if err := hc.Ping(ctx); err != nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "failed": name})
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
}

// run 启动 HTTP 服务并阻塞;ctx 取消时优雅关闭。
func (s *httpServer) run(ctx context.Context) error {
	srv := &http.Server{Addr: fmt.Sprintf("%s:%d", s.cfg.Addr, s.cfg.Port), Handler: s.engine}
	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(s.cfg.ShutdownTimeoutSeconds)*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return fmt.Errorf("HTTP 服务异常退出: %w", err)
	}
}

// auditRequestContextMiddleware 把 HTTP 请求元数据注入 platform/audit 上下文。
// 该中间件放在 cmd/server 而不是 pkg/response,是为了保持 pkg 通用可复用、不反向依赖 internal;
// 审计记录复用响应层生成的 trace_id,避免用户报障编号和 audit_log.trace_id 不一致。
func auditRequestContextMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqCtx := audit.RequestContext{
			IP:      c.ClientIP(),
			TraceID: response.TraceFromGin(c),
		}
		c.Request = c.Request.WithContext(audit.WithRequestContext(c.Request.Context(), reqCtx))
		c.Next()
	}
}

// recoveryMiddleware 捕获 panic,转为统一内部错误响应(不泄漏堆栈到 body)。
func recoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				response.Fail(c, apperr.ErrPanicRecovered.WithCause(fmt.Errorf("panic: %v", r)))
				c.Abort()
			}
		}()
		c.Next()
	}
}
