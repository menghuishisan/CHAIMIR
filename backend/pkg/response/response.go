// response 实现全平台统一 HTTP 响应信封、trace_id 贯穿与错误分层暴露。
package response

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type traceCtxKey struct{}

const ginTraceKey = "trace_id"

// TraceHeader 是回传/透传 trace_id 的 HTTP 头。
const TraceHeader = "X-Trace-Id"

// Body 是后端统一 JSON 响应体。
type Envelope struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
	TraceID string `json:"trace_id,omitempty"`
}

// Page 是分页接口统一返回结构。
type Page struct {
	List  any   `json:"list"`
	Total int64 `json:"total"`
	Page  int   `json:"page"`
	Size  int   `json:"size"`
}

const codeOK = "0"

// WithTrace 把 trace_id 写入 context。
func WithTrace(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceCtxKey{}, traceID)
}

// TraceFromContext 从 context 取 trace_id。
func TraceFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(traceCtxKey{}).(string); ok {
		return v
	}
	return ""
}

// TraceFromGin 从 gin.Context 取 trace_id。
func TraceFromGin(c *gin.Context) string {
	if v, ok := c.Get(ginTraceKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// TraceMiddleware 为每个请求确立 trace_id,注入 gin/request.Context,并回写响应头。
func TraceMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := strings.TrimSpace(c.GetHeader(TraceHeader))
		if !isSafeTraceID(traceID) {
			traceID = uuid.NewString()
		}
		c.Set(ginTraceKey, traceID)
		ctx := WithTrace(c.Request.Context(), traceID)
		ctx = logging.WithAttrs(ctx, loggingTraceAttr(traceID))
		c.Request = c.Request.WithContext(ctx)
		c.Header(TraceHeader, traceID)
		c.Next()
	}
}

// OK 写出统一成功信封。
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Envelope{Code: codeOK, Message: "ok", Data: data, TraceID: TraceFromGin(c)})
}

// OKPage 写出统一分页信封。
func OKPage(c *gin.Context, list any, total int64, page, size int) {
	OK(c, Page{List: list, Total: total, Page: page, Size: size})
}

// Fail 写出错误响应,并按错误分层规则记录内部原因。
func Fail(c *gin.Context, err error) {
	traceID := TraceFromGin(c)
	ae, ok := apperr.As(err)
	if !ok {
		ae = apperr.ErrUnhandledFailure.WithCause(err)
	}
	logging.ErrorContext(c.Request.Context(), "request failed", ae.LogString(), errorCodeAttr(ae.Code))
	c.JSON(http.StatusOK, Envelope{Code: ae.Code, Message: ae.Message, TraceID: traceID})
}

// Error 保留为纯数据转换辅助,用于不直接依赖 Gin 的边界测试和轻量场景。
func Error(err error, traceID string) Envelope {
	ae := apperr.AsAppError(err)
	if ae == nil {
		ae = apperr.ErrInternal
	}
	return Envelope{Code: ae.UserCode(), Message: ae.UserMessage(), TraceID: traceID}
}

// isSafeTraceID 限制上游 trace_id 为短可见标识,避免日志污染。
func isSafeTraceID(traceID string) bool {
	if len(traceID) == 0 || len(traceID) > 128 {
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

// loggingTraceAttr 生成 trace_id 日志字段。
func loggingTraceAttr(traceID string) slog.Attr {
	return slog.String("trace_id", traceID)
}

// errorCodeAttr 生成错误码日志字段。
func errorCodeAttr(code string) slog.Attr {
	return slog.String("error_code", code)
}
