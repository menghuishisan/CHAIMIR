// HTTP 审计上下文中间件测试。
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/config"
	"chaimir/pkg/response"

	"github.com/gin-gonic/gin"
)

// TestAuditRequestContextMiddlewareInjectsIPAndTrace 确认全局中间件把请求 IP 与 trace_id 注入审计上下文。
func TestAuditRequestContextMiddlewareInjectsIPAndTrace(t *testing.T) {
	server := newHTTPServer(config.ServerConfig{Addr: "127.0.0.1", Port: 0, AppEnv: "test"}, nil)
	server.engine.GET("/audit-context-test", func(c *gin.Context) {
		req := audit.RequestContextFrom(c.Request.Context())
		response.OK(c, gin.H{"ip": req.IP, "trace_id": req.TraceID})
	})

	httpReq := httptest.NewRequest(http.MethodGet, "/audit-context-test", nil)
	httpReq.Header.Set(response.TraceHeader, "trace-test-001")
	httpReq.Header.Set("X-Forwarded-For", "198.51.100.20")
	rec := httptest.NewRecorder()
	server.engine.ServeHTTP(rec, httpReq)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"trace_id":"trace-test-001"`) {
		t.Fatalf("expected trace id in audit request context, got body=%s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"ip":"198.51.100.20"`) {
		t.Fatalf("expected client IP in audit request context, got body=%s", rec.Body.String())
	}
}
