// Package response 实现全平台统一 HTTP 响应信封、trace_id 贯穿与错误分层暴露。
// 依据:
//
//	· docs/总-API接口总览.md §2 统一响应体 {code, message, data}(code=0 成功,非 0 业务码)。
//	· CLAUDE.md §8:响应附 trace_id 供报障;技术细节(堆栈/SQL/内部字段)只进日志。
//
// trace_id 三份文档未规范,平台层在此统一:入口中间件生成 → context/gin → 响应/日志引用。
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

// ---- trace_id 贯穿 ----

type traceCtxKey struct{}

const ginTraceKey = "trace_id"

// TraceHeader 是回传/透传 trace_id 的 HTTP 头(也接受上游传入,跨服务串联)。
const TraceHeader = "X-Trace-Id"

// WithTrace 把 trace_id 写入 context(供 service/repo 经 ctx 取用)。
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

		// 同一个 trace_id 同时进入 gin、标准 context 和日志上下文,保证响应编号、审计记录与错误日志可互相定位。
		c.Set(ginTraceKey, traceID)
		ctx := WithTrace(c.Request.Context(), traceID)
		ctx = logging.WithAttrs(ctx, loggingTraceAttr(traceID))
		c.Request = c.Request.WithContext(ctx)

		c.Header(TraceHeader, traceID)
		c.Next()
	}
}

// isSafeTraceID 限制上游 trace_id 为短可见标识,避免日志换行注入和异常长头传播。
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

// ---- 响应信封 ----

// Envelope 是所有 HTTP 响应的统一结构。
type Envelope struct {
	Code    string `json:"code"`               // "0"=成功;非 0=业务码。
	Message string `json:"message"`            // 用户向友好文案。
	Data    any    `json:"data,omitempty"`     // 成功业务数据。
	TraceID string `json:"trace_id,omitempty"` // 全链路追踪 ID。
}

// Page 是分页 data 标准结构(docs/总-API §2)。
type Page struct {
	List  any   `json:"list"`
	Total int64 `json:"total"`
	Page  int   `json:"page"`
	Size  int   `json:"size"`
}

const codeOK = "0"

// OK 写出统一成功信封,成功响应也带 trace_id 便于前端报障时对齐日志。
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Envelope{Code: codeOK, Message: "ok", Data: data, TraceID: TraceFromGin(c)})
}

// OKPage 按 API 总览的分页结构写出成功信封,避免各模块自定义分页字段。
func OKPage(c *gin.Context, list any, total int64, page, size int) {
	OK(c, Page{List: list, Total: total, Page: page, Size: size})
}

// Fail 写出错误响应,并按错误分层规则记录内部原因。
func Fail(c *gin.Context, err error) {
	traceID := TraceFromGin(c)
	ae, ok := apperr.As(err)
	if !ok {
		// 非应用错误没有稳定用户文案,统一折叠为内部错误,原始原因只进入日志。
		ae = apperr.ErrUnhandledFailure.WithCause(err)
	}

	// 技术原因通过 pkg/logging 脱敏后落日志;响应体只保留用户可理解的 code/message/trace_id。
	logging.ErrorContext(c.Request.Context(), "request failed", ae.Error(), errorCodeAttr(ae.Code))
	c.JSON(http.StatusOK, Envelope{Code: ae.Code, Message: ae.Message, TraceID: traceID})
}

// loggingTraceAttr/errorCodeAttr 隔离日志字段命名,让响应逻辑保持在“信封 + 错误分层”职责内。
func loggingTraceAttr(traceID string) slog.Attr {
	return slog.String("trace_id", traceID)
}

// errorCodeAttr 生成统一错误码日志字段,便于运维按业务码聚合排查。
func errorCodeAttr(code string) slog.Attr {
	return slog.String("error_code", code)
}
