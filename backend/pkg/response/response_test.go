// Package response 的测试验证统一响应信封与错误分层暴露规则。
package response

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"

	"github.com/gin-gonic/gin"
)

// TestFailKeepsTechnicalCauseOutOfResponseAndSanitizesLog 确认内部原因只进入脱敏日志,不进入响应体。
func TestFailKeepsTechnicalCauseOutOfResponseAndSanitizesLog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var logs bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })

	router := gin.New()
	router.Use(TraceMiddleware())
	router.GET("/boom", func(c *gin.Context) {
		ctx := logging.WithAttrs(c.Request.Context(), slog.Int64("tenant_id", 10))
		c.Request = c.Request.WithContext(ctx)
		Fail(c, apperr.ErrInternal.WithCause(errors.New("database password=secret unavailable")))
	})

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	req.Header.Set(TraceHeader, "trace-001")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var body Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != apperr.ErrInternal.Code || body.Message != apperr.ErrInternal.Message || body.TraceID != "trace-001" {
		t.Fatalf("unexpected response body: %#v", body)
	}
	if strings.Contains(w.Body.String(), `"msg"`) {
		t.Fatalf("response must use message field, got %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "database") || strings.Contains(w.Body.String(), "secret") {
		t.Fatalf("response leaked technical cause: %s", w.Body.String())
	}

	out := logs.String()
	for _, want := range []string{`"trace_id":"trace-001"`, `"tenant_id":10`, `"error_code":"11500"`, `"error":"[11500] 服务繁忙,请稍后重试: database password=*** unavailable"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("log missing %s: %s", want, out)
		}
	}
	if strings.Contains(out, "secret") {
		t.Fatalf("log leaked sensitive value: %s", out)
	}
}

// TestFailWrapsNonApplicationErrorWithDedicatedCode 确认未分类错误折叠到平台专属错误码,不复用通用内部错误入口。
func TestFailWrapsNonApplicationErrorWithDedicatedCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(TraceMiddleware())
	router.GET("/boom", func(c *gin.Context) {
		Fail(c, errors.New("driver failed"))
	})

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var body Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != apperr.ErrUnhandledFailure.Code {
		t.Fatalf("expected unhandled failure code, got %#v", body)
	}
}

// TestTraceMiddlewareRejectsUnsafeIncomingTrace 确认上游 trace_id 必须是短可见标识,避免日志污染。
func TestTraceMiddlewareRejectsUnsafeIncomingTrace(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(TraceMiddleware())
	router.GET("/ok", func(c *gin.Context) {
		OK(c, gin.H{"trace_id": TraceFromGin(c)})
	})

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	req.Header.Set(TraceHeader, "bad-trace\ninjected")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var body Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.TraceID == "bad-trace\ninjected" || strings.Contains(body.TraceID, "\n") {
		t.Fatalf("unsafe trace id should be replaced, got %q", body.TraceID)
	}
}
