// M7 API 路由测试:确认实验接口按文档路径注册并进入统一鉴权链。
package experiment

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/config"
	"chaimir/pkg/response"

	"github.com/gin-gonic/gin"
)

// TestExperimentRoutesAreProtected 确认 M7 用户路径未登录时返回统一未登录错误。
func TestExperimentRoutesAreProtected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(response.TraceMiddleware())
	api := NewAPI(nil, auth.NewManager(config.AuthConfig{
		JWTSigningKey: "test-signing-key",
		AccessTTLMin:  15,
		JWTIssuer:     "chaimir-test",
		HMACKey:       "test-service-hmac-key",
	}), nil)
	api.Register(engine.Group("/api/v1"))

	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v1/experiment/experiments?course_id=1&status=1", ""},
		{http.MethodPost, "/api/v1/experiment/experiments", `{}`},
		{http.MethodPatch, "/api/v1/experiment/experiments/1", `{}`},
		{http.MethodPost, "/api/v1/experiment/experiments/1/validate", `{}`},
		{http.MethodPost, "/api/v1/experiment/experiments/1/publish", `{}`},
		{http.MethodPost, "/api/v1/experiment/experiments/1/unpublish", `{}`},
		{http.MethodPost, "/api/v1/experiment/experiments/1/instances", `{}`},
		{http.MethodGet, "/api/v1/experiment/instances/1", ""},
		{http.MethodGet, "/api/v1/experiment/instances/1/progress", ""},
		{http.MethodPost, "/api/v1/experiment/instances/1/pause", `{}`},
		{http.MethodPost, "/api/v1/experiment/instances/1/resume", `{}`},
		{http.MethodPost, "/api/v1/experiment/instances/1/finish", `{}`},
		{http.MethodDelete, "/api/v1/experiment/instances/1", ""},
		{http.MethodPost, "/api/v1/experiment/instances/1/checkpoints/cp1/judge", `{}`},
		{http.MethodPost, "/api/v1/experiment/instances/1/report", `{}`},
		{http.MethodGet, "/api/v1/experiment/experiments/1/reports", ""},
		{http.MethodPost, "/api/v1/experiment/reports/1/grade", `{}`},
		{http.MethodPost, "/api/v1/experiment/experiments/1/groups", `{}`},
		{http.MethodPost, "/api/v1/experiment/groups/1/members", `{}`},
		{http.MethodGet, "/api/v1/experiment/groups/1", ""},
		{http.MethodGet, "/api/v1/experiment/instances/1/score", ""},
		{http.MethodGet, "/api/v1/experiment/internal/stats?tenant_id=1&course_id=1", ""},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s expected unified status 200, got %d", tc.method, tc.path, rec.Code)
		}
		wantCode := `"code":"11001"`
		if experimentInternalRoute(tc.path) {
			wantCode = `"code":"11008"`
		}
		if !strings.Contains(rec.Body.String(), wantCode) {
			t.Fatalf("%s %s expected %s response, got %s", tc.method, tc.path, wantCode, rec.Body.String())
		}
	}
}

func experimentInternalRoute(path string) bool {
	return strings.Contains(path, "/instances/1/score") || strings.Contains(path, "/internal/stats")
}
