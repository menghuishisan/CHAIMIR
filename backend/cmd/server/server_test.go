// HTTP 服务器测试:覆盖全局中间件与 panic recovery 错误分层。
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"chaimir/internal/platform/config"
	"chaimir/pkg/apperr"

	"github.com/gin-gonic/gin"
)

// TestRecoveryMiddlewareUsesDedicatedPanicCode 确认 panic recovery 不复用通用内部错误码。
func TestRecoveryMiddlewareUsesDedicatedPanicCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := newHTTPServer(config.ServerConfig{Addr: "127.0.0.1", Port: 0, AppEnv: "test"}, nil)
	server.engine.GET("/panic", func(*gin.Context) {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()
	server.engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"code":"`+apperr.ErrPanicRecovered.Code+`"`) {
		t.Fatalf("expected panic recovery code, status=%d body=%s", rec.Code, rec.Body.String())
	}
}
