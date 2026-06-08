// M2 API 路由注册测试:确认文档中的控制面接口进入鉴权链。
package sandbox

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/config"
	"chaimir/pkg/response"

	"github.com/gin-gonic/gin"
)

// TestSandboxRoutesAreProtected 确认沙箱控制面路由已注册且需要登录。
func TestSandboxRoutesAreProtected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(response.TraceMiddleware())
	api := NewAPI(nil, auth.NewManager(config.AuthConfig{
		JWTSigningKey: "test-signing-key",
		AccessTTLMin:  15,
		JWTIssuer:     "chaimir-test",
		HMACKey:       "test-service-hmac-key",
	}))
	api.Register(engine.Group("/api/v1"))

	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v1/sandbox/runtimes", ""},
		{http.MethodPost, "/api/v1/sandbox/runtimes", `{}`},
		{http.MethodPatch, "/api/v1/sandbox/runtimes/1", `{}`},
		{http.MethodPost, "/api/v1/sandbox/runtimes/1/images", `{}`},
		{http.MethodPost, "/api/v1/sandbox/runtimes/1/images/2/prepull", `{}`},
		{http.MethodGet, "/api/v1/sandbox/runtimes/1/images/2/prepull", ""},
		{http.MethodGet, "/api/v1/sandbox/runtimes/1/selftest", ""},
		{http.MethodPost, "/api/v1/sandbox/runtimes/1/selftest", `{}`},
		{http.MethodGet, "/api/v1/sandbox/tools", ""},
		{http.MethodPost, "/api/v1/sandbox/tools", `{}`},
		{http.MethodPost, "/api/v1/sandbox/sandboxes", `{}`},
		{http.MethodGet, "/api/v1/sandbox/sandboxes/1", ""},
		{http.MethodGet, "/api/v1/sandbox/sandboxes/1/progress", ""},
		{http.MethodGet, "/api/v1/sandbox/sandboxes/1/terminal", ""},
		{http.MethodPost, "/api/v1/sandbox/sandboxes/1/pause", `{}`},
		{http.MethodPost, "/api/v1/sandbox/sandboxes/1/resume", `{}`},
		{http.MethodGet, "/api/v1/sandbox/sandboxes/1/files?path=contracts", ""},
		{http.MethodPut, "/api/v1/sandbox/sandboxes/1/files?path=contracts/Counter.sol", `{}`},
		{http.MethodPost, "/api/v1/sandbox/sandboxes/1/files/save", `{}`},
		{http.MethodGet, "/api/v1/sandbox/sandboxes/1/tools/code-server/", ""},
		{http.MethodDelete, "/api/v1/sandbox/sandboxes/1", ""},
		{http.MethodPost, "/api/v1/sandbox/sandboxes/recycle", `{}`},
		{http.MethodPost, "/api/v1/sandbox/sandboxes/1/chain/deploy", `{}`},
		{http.MethodPost, "/api/v1/sandbox/sandboxes/1/chain/tx", `{}`},
		{http.MethodGet, "/api/v1/sandbox/sandboxes/1/chain/query?target=latest", ""},
		{http.MethodPost, "/api/v1/sandbox/sandboxes/1/chain/reset", `{}`},
		{http.MethodGet, "/api/v1/sandbox/quota", ""},
		{http.MethodPatch, "/api/v1/sandbox/quota", `{}`},
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
		if sandboxInternalRoute(tc.method, tc.path) {
			wantCode = `"code":"11008"`
		}
		if !strings.Contains(rec.Body.String(), wantCode) {
			t.Fatalf("%s %s expected %s response, got %s", tc.method, tc.path, wantCode, rec.Body.String())
		}
	}
}

func sandboxInternalRoute(method, path string) bool {
	switch {
	case method == http.MethodPost && path == "/api/v1/sandbox/sandboxes":
		return true
	case method == http.MethodPost && path == "/api/v1/sandbox/sandboxes/recycle":
		return true
	case method == http.MethodPost && (strings.Contains(path, "/pause") || strings.Contains(path, "/resume")):
		return true
	case method == http.MethodPost && strings.Contains(path, "/chain/"):
		return true
	case method == http.MethodGet && strings.Contains(path, "/chain/query"):
		return true
	case method == http.MethodDelete && strings.HasPrefix(path, "/api/v1/sandbox/sandboxes/"):
		return true
	default:
		return false
	}
}

func signSandboxServiceRequest(req *http.Request, service, tenantID, sourceRef, traceID, timestamp string) {
	req.Header.Set(auth.ServiceNameHeader, service)
	req.Header.Set(auth.ServiceTenantHeader, tenantID)
	req.Header.Set(auth.ServiceSourceRefHeader, sourceRef)
	req.Header.Set(auth.ServiceTimestampHeader, timestamp)
	req.Header.Set(response.TraceHeader, traceID)
	req.Header.Set(auth.ServiceSignatureHeader, sandboxServiceSignature(req.Method, req.URL.EscapedPath(), tenantID, sourceRef, timestamp, traceID))
}

func sandboxServiceSignature(method, path, tenantID, sourceRef, timestamp, traceID string) string {
	mac := hmac.New(sha256.New, []byte("test-service-hmac-key"))
	mac.Write([]byte(method + "\n" + path + "\n" + tenantID + "\n" + sourceRef + "\n" + timestamp + "\n" + traceID))
	return hex.EncodeToString(mac.Sum(nil))
}

// TestRuntimeAdminRoutesRejectTenantUsers 确认运行时/工具管理接口只允许平台管理员访问。
func TestRuntimeAdminRoutesRejectTenantUsers(t *testing.T) {
	engine, mgr := sandboxTestEngine(&Service{})
	token, err := mgr.IssueAccess(1001, 2001, 3001, false)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sandbox/runtimes", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"code":"11002"`) {
		t.Fatalf("tenant user must be forbidden from runtime admin routes, status=%d body=%s", rec.Code, rec.Body.String())
	}
}

// TestQuotaUpdateRequiresSchoolOrPlatformAdmin 确认配额调整不能只靠普通登录态。
func TestQuotaUpdateRequiresSchoolOrPlatformAdmin(t *testing.T) {
	engine, mgr := sandboxTestEngine(&Service{identity: fakeIdentityService{roles: []string{contracts.RoleStudent}}})
	token, err := mgr.IssueAccess(1001, 2001, 3001, false)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}

	body := `{"max_concurrent_sandbox":10,"max_cpu":100,"max_memory_mb":102400,"idle_timeout_min":30,"max_lifetime_min":240,"max_keepalive_min":120,"max_snapshot_retention_min":1440}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/sandbox/quota", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"code":"11002"`) {
		t.Fatalf("student must be forbidden from quota update, status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func sandboxTestEngine(svc *Service) (*gin.Engine, *auth.Manager) {
	gin.SetMode(gin.TestMode)
	mgr := auth.NewManager(config.AuthConfig{
		JWTSigningKey: "test-signing-key",
		AccessTTLMin:  15,
		JWTIssuer:     "chaimir-test",
		HMACKey:       "test-service-hmac-key",
	})
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(response.TraceMiddleware())
	NewAPI(svc, mgr).Register(engine.Group("/api/v1"))
	return engine, mgr
}

type fakeIdentityService struct {
	roles []string
}

func (f fakeIdentityService) GetAccount(context.Context, int64) (contracts.AccountInfo, error) {
	return contracts.AccountInfo{Roles: f.roles}, nil
}

func (f fakeIdentityService) BatchGetAccounts(context.Context, []int64) ([]contracts.AccountInfo, error) {
	return nil, nil
}

func (f fakeIdentityService) HasRole(_ context.Context, _ int64, role string) (bool, error) {
	return contracts.HasAnyRole(f.roles, role), nil
}
