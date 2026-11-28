// Package response 实现全平台统一 HTTP 响应信封、trace_id 贯穿与错误分层暴露。
package response

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"

	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// TraceHeader 是回传/透传 trace_id 的 HTTP 头。
const TraceHeader = "X-Trace-Id"

// Envelope 是后端统一 JSON 响应体。
type Envelope struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
	TraceID string `json:"trace_id,omitempty"`
}

// Page 是分页接口统一返回结构。
type Page struct {
	List   any                         `json:"list"`
	Total  int64                       `json:"total"`
	Page   int                         `json:"page"`
	Size   int                         `json:"size"`
	Facets map[string]map[string]int64 `json:"facets,omitempty"`
}

// SetRequestTrace 同时更新请求上下文与响应头。上游编号必须先通过内部鉴权。
func SetRequestTrace(c *gin.Context, ctx context.Context, traceID string) {
	c.Request = c.Request.WithContext(logging.WithTrace(ctx, traceID))
	c.Header(TraceHeader, traceID)
}

// TraceMiddleware 设置请求追踪编号并通过响应头回传。
func TraceMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := logging.WithAttrs(c.Request.Context(), slog.String("method", c.Request.Method), requestPathAttr(c.FullPath(), c.Request.URL.Path))
		SetRequestTrace(c, ctx, uuid.NewString())
		c.Next()
	}
}

// RecoveryMiddleware 将 panic 收敛到统一错误信封,避免默认 recovery 泄露非规范响应形态。
func RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if v := recover(); v != nil {
				responseErr := apperr.ErrPanicRecovered.WithCause(fmt.Errorf("panic: %v\n%s", v, debug.Stack()))
				Fail(c, responseErr)
				c.Abort()
			}
		}()
		c.Next()
	}
}

// OK 写出统一成功信封。
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Envelope{Code: apperr.CodeOK, Message: apperr.MessageOK, Data: data, TraceID: logging.TraceFromContext(c.Request.Context())})
}

// OKPage 写出统一分页信封。
func OKPage(c *gin.Context, list any, total int64, page, size int) {
	OK(c, Page{List: list, Total: total, Page: page, Size: size})
}

// OKPageWithFacets 输出带全量分组计数的分页响应,列表与聚合使用同一权限口径。
func OKPageWithFacets(c *gin.Context, list any, total int64, page, size int, facets map[string]map[string]int64) {
	OK(c, Page{List: list, Total: total, Page: page, Size: size, Facets: facets})
}

// Fail 写出错误响应,并按错误分层规则记录内部原因。
func Fail(c *gin.Context, err error) {
	ae := apperr.AsAppError(err)
	logging.ErrorContext(c.Request.Context(), "request failed", ae.LogString(), slog.String("error_code", ae.UserCode()))
	c.JSON(http.StatusOK, Envelope{Code: ae.UserCode(), Message: ae.UserMessage(), TraceID: logging.TraceFromContext(c.Request.Context())})
}

// NoRoute 将未注册路径收敛为统一错误信封,避免 Gin 默认 404 空响应。
func NoRoute(c *gin.Context) {
	Fail(c, apperr.ErrNotFound)
}

// ValidTraceID 限制上游编号为可写入审计列的短标识。
func ValidTraceID(traceID string) bool {
	if len(traceID) == 0 || len(traceID) > 64 {
		return false
	}
	for _, r := range traceID {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == '-' || r == '_' || r == '.' || r == ':' {
			continue
		}
		return false
	}
	return true
}

// requestPathAttr 生成请求路径日志字段,优先使用 Gin 路由模板避免记录敏感路径片段。
func requestPathAttr(routePath, rawPath string) slog.Attr {
	path := strings.TrimSpace(routePath)
	if path == "" {
		path = rawPath
	}
	return slog.String("path", path)
}
