// M9 API 路由测试:确认管理后台接口按文档路径注册并执行鉴权与角色边界。
package admin

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/config"

	"github.com/gin-gonic/gin"
)

// TestAdminRoutesAreProtected 确认 M9 管理后台路径未登录时返回统一未登录错误。
func TestAdminRoutesAreProtected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	api := NewAPI(newTestService(&fakeAdminStore{}), newTestAuth(), nil, config.DeployConfig{PlatformEnabled: true})
	api.Register(engine.Group("/api/v1"))

	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v1/admin/platform/dashboard", ""},
		{http.MethodGet, "/api/v1/admin/platform/statistics", ""},
		{http.MethodGet, "/api/v1/admin/platform/tenants", ""},
		{http.MethodGet, "/api/v1/admin/platform/applications", ""},
		{http.MethodPost, "/api/v1/admin/platform/applications/1/approve", `{}`},
		{http.MethodPost, "/api/v1/admin/platform/applications/1/reject", `{}`},
		{http.MethodGet, "/api/v1/admin/school/dashboard", ""},
		{http.MethodGet, "/api/v1/admin/school/statistics", ""},
		{http.MethodGet, "/api/v1/admin/audit", ""},
		{http.MethodGet, "/api/v1/admin/audit/export", ""},
		{http.MethodGet, "/api/v1/admin/configs", ""},
		{http.MethodPut, "/api/v1/admin/configs/system.rate", `{}`},
		{http.MethodGet, "/api/v1/admin/configs/system.rate/history", ""},
		{http.MethodPost, "/api/v1/admin/configs/system.rate/rollback", `{}`},
		{http.MethodGet, "/api/v1/admin/alert-rules", ""},
		{http.MethodPost, "/api/v1/admin/alert-rules", `{}`},
		{http.MethodPatch, "/api/v1/admin/alert-rules/1", `{}`},
		{http.MethodGet, "/api/v1/admin/alert-events", ""},
		{http.MethodPost, "/api/v1/admin/alert-events/1/handle", `{}`},
		{http.MethodGet, "/api/v1/admin/monitoring/panels", ""},
		{http.MethodGet, "/api/v1/admin/backups", ""},
		{http.MethodPost, "/api/v1/admin/backups/trigger", `{}`},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s expected unified status 200, got %d", tc.method, tc.path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `"code":"11001"`) {
			t.Fatalf("%s %s expected unauthorized response, got %s", tc.method, tc.path, rec.Body.String())
		}
	}
}

// TestSchoolAdminCannotAccessPlatformDashboard 确认学校管理员不能访问平台管理接口。
func TestSchoolAdminCannotAccessPlatformDashboard(t *testing.T) {
	engine, authMgr := newAdminRouteEngine()
	token, err := authMgr.IssueAccess(1001, 2001, 3001, false)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/platform/dashboard", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), `"code":"11002"`) {
		t.Fatalf("expected forbidden response, got %s", rec.Body.String())
	}
}

// TestPlatformAdminCanAccessPlatformDashboard 确认平台管理员可以进入平台看板接口。
func TestPlatformAdminCanAccessPlatformDashboard(t *testing.T) {
	engine, authMgr := newAdminRouteEngine()
	token, err := authMgr.IssueAccess(0, 9001, 3001, true)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/platform/dashboard", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"0"`) {
		t.Fatalf("expected ok response, got %s", rec.Body.String())
	}
}

// TestSchoolAdminCannotAccessMonitoringPanels 确认基础监控入口仅平台管理员可访问。
func TestSchoolAdminCannotAccessMonitoringPanels(t *testing.T) {
	engine, authMgr := newAdminRouteEngine()
	token, err := authMgr.IssueAccess(1001, 2001, 3001, false)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/monitoring/panels", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), `"code":"11002"`) {
		t.Fatalf("expected forbidden response, got %s", rec.Body.String())
	}
}

// TestAdminAPIErrorsUseModuleCodesAndRoleContracts 防止 M9 API 边界退回通用错误码或硬编码角色。
func TestAdminAPIErrorsUseModuleCodesAndRoleContracts(t *testing.T) {
	apiSrc, err := os.ReadFile("api.go")
	if err != nil {
		t.Fatalf("read api: %v", err)
	}
	text := string(apiSrc)
	if strings.Contains(text, "ErrBadRequest") {
		t.Fatalf("M9 API must use admin module error codes instead of ErrBadRequest")
	}
	if strings.Contains(text, `"school_admin"`) {
		t.Fatalf("M9 API role checks must use contracts.RoleSchoolAdmin")
	}
	for _, required := range []string{"contracts.RoleSchoolAdmin", "ErrAdminStatisticsQueryInvalid", "ErrAdminAuditQueryInvalid"} {
		if !strings.Contains(text, required) {
			t.Fatalf("M9 API boundary missing %s", required)
		}
	}
	codes, err := os.ReadFile("../../../pkg/apperr/admin_codes.go")
	if err != nil {
		t.Fatalf("read admin codes: %v", err)
	}
	codeText := string(codes)
	for _, required := range []string{"ErrAdminStatisticsQueryInvalid", "ErrAdminAuditQueryInvalid"} {
		if !strings.Contains(codeText, required) {
			t.Fatalf("M9 error code registry missing %s", required)
		}
	}
}

// TestAdminInvalidQueryParamsReturnPreciseCodes 确认统计日期和审计时间解析错误使用不同 M9 专属错误码。
func TestAdminInvalidQueryParamsReturnPreciseCodes(t *testing.T) {
	engine, authMgr := newAdminRouteEngine()
	token, err := authMgr.IssueAccess(0, 9001, 3001, true)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}
	cases := []struct {
		path string
		code string
	}{
		{"/api/v1/admin/platform/statistics?from=bad-date", "91003"},
		{"/api/v1/admin/audit?from=bad-time", "92003"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		if !strings.Contains(rec.Body.String(), `"code":"`+tc.code+`"`) {
			t.Fatalf("%s expected code %s, got %s", tc.path, tc.code, rec.Body.String())
		}
	}
}

// newAdminRouteEngine 构造带 M9 路由的测试 Gin 引擎。
func newAdminRouteEngine() (*gin.Engine, *auth.Manager) {
	gin.SetMode(gin.TestMode)
	authMgr := newTestAuth()
	engine := gin.New()
	svc := newTestService(&fakeAdminStore{})
	svc.deploy = config.DeployConfig{PlatformEnabled: true}
	api := NewAPI(svc, authMgr, &fakeIdentityReader{roles: []string{contracts.RoleSchoolAdmin}}, config.DeployConfig{PlatformEnabled: true})
	api.Register(engine.Group("/api/v1"))
	return engine, authMgr
}

// newTestAuth 构造测试用 JWT 管理器。
func newTestAuth() *auth.Manager {
	return auth.NewManager(config.AuthConfig{JWTSigningKey: "test-signing-key", AccessTTLMin: 15, JWTIssuer: "chaimir-test"})
}
