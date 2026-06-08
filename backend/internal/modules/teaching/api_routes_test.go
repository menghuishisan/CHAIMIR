// M6 API 路由测试:确认教学接口按文档路径注册并进入统一鉴权链。
package teaching

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/response"

	"github.com/gin-gonic/gin"
)

// TestTeachingRoutesAreProtected 确认 M6 用户路径未登录时返回统一未登录错误。
func TestTeachingRoutesAreProtected(t *testing.T) {
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
		{http.MethodGet, "/api/v1/teaching/courses?role=teacher", ""},
		{http.MethodPost, "/api/v1/teaching/courses", `{}`},
		{http.MethodPatch, "/api/v1/teaching/courses/1", `{}`},
		{http.MethodPost, "/api/v1/teaching/courses/1/publish", `{}`},
		{http.MethodPost, "/api/v1/teaching/courses/1/end", `{}`},
		{http.MethodPost, "/api/v1/teaching/courses/1/archive", `{}`},
		{http.MethodPost, "/api/v1/teaching/courses/1/clone", `{}`},
		{http.MethodPost, "/api/v1/teaching/courses/1/share", `{}`},
		{http.MethodPost, "/api/v1/teaching/courses/1/invite-code/refresh", `{}`},
		{http.MethodPost, "/api/v1/teaching/courses/join", `{}`},
		{http.MethodGet, "/api/v1/teaching/courses/1/members", ""},
		{http.MethodPost, "/api/v1/teaching/courses/1/members/batch", `{}`},
		{http.MethodDelete, "/api/v1/teaching/courses/1/members/2", ""},
		{http.MethodGet, "/api/v1/teaching/courses/1/chapters", ""},
		{http.MethodPost, "/api/v1/teaching/courses/1/chapters", `{}`},
		{http.MethodPatch, "/api/v1/teaching/courses/1/chapters/2", `{}`},
		{http.MethodDelete, "/api/v1/teaching/courses/1/chapters/2", ""},
		{http.MethodGet, "/api/v1/teaching/chapters/1/lessons", ""},
		{http.MethodPost, "/api/v1/teaching/chapters/1/lessons", `{}`},
		{http.MethodPatch, "/api/v1/teaching/chapters/1/lessons/2", `{}`},
		{http.MethodDelete, "/api/v1/teaching/chapters/1/lessons/2", ""},
		{http.MethodPost, "/api/v1/teaching/lessons/1/content", `{}`},
		{http.MethodGet, "/api/v1/teaching/courses/1/outline", ""},
		{http.MethodGet, "/api/v1/teaching/lessons/1", ""},
		{http.MethodPost, "/api/v1/teaching/lessons/1/progress", `{}`},
		{http.MethodPost, "/api/v1/teaching/courses/1/assignments", `{}`},
		{http.MethodPatch, "/api/v1/teaching/assignments/1", `{}`},
		{http.MethodPost, "/api/v1/teaching/assignments/1/publish", `{}`},
		{http.MethodGet, "/api/v1/teaching/assignments/1", ""},
		{http.MethodGet, "/api/v1/teaching/assignments/1/draft", ""},
		{http.MethodPost, "/api/v1/teaching/assignments/1/draft", `{}`},
		{http.MethodPost, "/api/v1/teaching/assignments/1/submit", `{}`},
		{http.MethodGet, "/api/v1/teaching/assignments/1/submissions", ""},
		{http.MethodPost, "/api/v1/teaching/submissions/1/grade", `{}`},
		{http.MethodGet, "/api/v1/teaching/submissions/1", ""},
		{http.MethodGet, "/api/v1/teaching/courses/1/posts", ""},
		{http.MethodPost, "/api/v1/teaching/courses/1/posts", `{}`},
		{http.MethodPost, "/api/v1/teaching/posts/1/like", `{}`},
		{http.MethodPost, "/api/v1/teaching/posts/1/pin", `{}`},
		{http.MethodDelete, "/api/v1/teaching/posts/1", ""},
		{http.MethodGet, "/api/v1/teaching/courses/1/announcements", ""},
		{http.MethodPost, "/api/v1/teaching/courses/1/announcements", `{}`},
		{http.MethodPost, "/api/v1/teaching/announcements/1/pin", `{}`},
		{http.MethodPost, "/api/v1/teaching/courses/1/review", `{}`},
		{http.MethodGet, "/api/v1/teaching/courses/1/progress-stats", ""},
		{http.MethodGet, "/api/v1/teaching/courses/1/my-progress", ""},
		{http.MethodPut, "/api/v1/teaching/courses/1/grade-weights", `{}`},
		{http.MethodPost, "/api/v1/teaching/courses/1/grades/compute", `{}`},
		{http.MethodGet, "/api/v1/teaching/courses/1/grades", ""},
		{http.MethodGet, "/api/v1/teaching/courses/1/grades/export", ""},
		{http.MethodPatch, "/api/v1/teaching/courses/1/grades/2", `{}`},
		{http.MethodGet, "/api/v1/teaching/internal/stats?tenant_id=1", ""},
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
		if tc.path == "/api/v1/teaching/internal/stats?tenant_id=1" {
			wantCode = `"code":"11008"`
		}
		if !strings.Contains(rec.Body.String(), wantCode) {
			t.Fatalf("%s %s expected %s response, got %s", tc.method, tc.path, wantCode, rec.Body.String())
		}
	}
}

// TestTeachingInternalStatsRejectsInvalidTenantWithModuleCode 确认内部统计参数错误不返回通用 11004。
func TestTeachingInternalStatsRejectsInvalidTenantWithModuleCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/teaching/internal/stats?tenant_id=bad", nil)
	c.Request = req.WithContext(tenant.WithContext(req.Context(), tenant.Identity{TenantID: 10}))
	api := NewAPI(&Service{}, nil, nil)

	api.internalStats(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected unified status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"63005"`) {
		t.Fatalf("expected teaching stats query code 63005, got %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"code":"11004"`) {
		t.Fatalf("internal stats must not return generic bad request: %s", rec.Body.String())
	}
}
