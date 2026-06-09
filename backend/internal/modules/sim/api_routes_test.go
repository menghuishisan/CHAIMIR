// M4 API 路由测试:确认仿真会话内部编排接口与用户交互接口使用不同鉴权边界。
package sim

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/response"

	"github.com/gin-gonic/gin"
)

// TestSimInternalSessionRoutesRequireServiceAuth 确认会话创建与来源回收不能被普通登录态调用。
func TestSimInternalSessionRoutesRequireServiceAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(response.TraceMiddleware(), gin.Recovery())
	api := NewAPI(nil, auth.NewManager(config.AuthConfig{
		JWTSigningKey:             "test-signing-key",
		AccessTTLMin:              15,
		JWTIssuer:                 "chaimir-test",
		HMACKey:                   "test-service-hmac-key",
		ServiceAuthMaxSkewSeconds: 300,
	}), nil, config.UploadConfig{})
	api.Register(engine.Group("/api/v1"))

	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/v1/sim/sessions", `{}`},
		{http.MethodPost, "/api/v1/sim/sessions/recycle", `{}`},
		{http.MethodDelete, "/api/v1/sim/sessions/7001", ``},
		{http.MethodPost, "/api/v1/sim/sessions/7001/checkpoints", `{}`},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		if !strings.Contains(rec.Body.String(), `"code":"11008"`) {
			t.Fatalf("%s %s expected service auth failure, got %s", tc.method, tc.path, rec.Body.String())
		}
	}
}

// TestSimValidationReportAllowsServiceAuth 确认受控预览流程可通过统一服务鉴权回写报告。
func TestSimValidationReportAllowsServiceAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(response.TraceMiddleware(), gin.Recovery())
	authMgr := auth.NewManager(config.AuthConfig{
		JWTSigningKey:             "test-signing-key",
		AccessTTLMin:              15,
		JWTIssuer:                 "chaimir-test",
		HMACKey:                   "test-service-hmac-key",
		ServiceAuthMaxSkewSeconds: 300,
	})
	api := NewAPI(nil, authMgr, nil, config.UploadConfig{})
	api.Register(engine.Group("/api/v1"))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sim/packages/9001/validation-report", strings.NewReader(`{`))
	req.Header.Set("Content-Type", "application/json")
	signSimServiceRequest(req, "test-service-hmac-key", "sim-preview", "10", "experiment:2026:instance:55", "trace-1", currentSimServiceTimestamp())
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if strings.Contains(rec.Body.String(), `"code":"11001"`) || strings.Contains(rec.Body.String(), `"code":"11008"`) {
		t.Fatalf("validation report service route must not require ordinary login, got %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"code":"41005"`) {
		t.Fatalf("service-auth validation report should reach handler validation, got %s", rec.Body.String())
	}
}

// TestSimSharedReplayRouteIsPublic 确认分享码入口不依赖登录态,否则公开分享无法访问。
func TestSimSharedReplayRouteIsPublic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(response.TraceMiddleware(), gin.Recovery())
	api := NewAPI(nil, auth.NewManager(config.AuthConfig{
		JWTSigningKey:             "test-signing-key",
		AccessTTLMin:              15,
		JWTIssuer:                 "chaimir-test",
		HMACKey:                   "test-service-hmac-key",
		ServiceAuthMaxSkewSeconds: 300,
	}), nil, config.UploadConfig{})
	api.Register(engine.Group("/api/v1"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sim/shared/bad-code!", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if strings.Contains(rec.Body.String(), `"code":"11001"`) {
		t.Fatalf("shared replay route must not be blocked by login middleware, got %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"code":"42007"`) {
		t.Fatalf("invalid public share code should return sim share error, got %s", rec.Body.String())
	}
}

// TestSimRouteIDReadsRequestedParam 确认 M4 不把 package code/id、session id 和 review id 混用同一个隐式参数。
func TestSimRouteIDReadsRequestedParam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Params = gin.Params{{Key: "id", Value: "42"}, {Key: "pkg", Value: "9001"}}

	id, ok := simRouteID(c, "id", apperr.ErrSimSessionInvalid)
	if !ok || id != 42 {
		t.Fatalf("expected route id 42, got id=%d ok=%v body=%s", id, ok, rec.Body.String())
	}
	packageID, ok := simRouteID(c, "pkg", apperr.ErrSimPackageInvalid)
	if !ok || packageID != 9001 {
		t.Fatalf("expected package id 9001, got id=%d ok=%v body=%s", packageID, ok, rec.Body.String())
	}
}

// TestSimAPIReturnsModuleSpecificValidationCodes 确认 M4 HTTP 边界不把业务校验折叠为通用 BadRequest。
func TestSimAPIReturnsModuleSpecificValidationCodes(t *testing.T) {
	engine, authMgr := newSimRouteEngine([]string{"teacher"})
	teacherToken, err := authMgr.IssueAccess(10, 501, 9001, false)
	if err != nil {
		t.Fatalf("issue teacher token: %v", err)
	}
	platformToken, err := authMgr.IssueAccess(0, 9001, 9001, true)
	if err != nil {
		t.Fatalf("issue platform token: %v", err)
	}

	cases := []struct {
		name   string
		method string
		path   string
		body   string
		token  string
		code   string
	}{
		{name: "missing bundle", method: http.MethodPost, path: "/api/v1/sim/packages", token: teacherToken, code: `"code":"41002"`},
		{name: "package path id", method: http.MethodPost, path: "/api/v1/sim/packages/bad/validation-report", body: `{}`, token: platformToken, code: `"code":"41002"`},
		{name: "validation report body", method: http.MethodPost, path: "/api/v1/sim/packages/9001/validation-report", body: `{`, token: platformToken, code: `"code":"41005"`},
		{name: "review path id", method: http.MethodPost, path: "/api/v1/sim/reviews/bad/approve", body: `{}`, token: platformToken, code: `"code":"43001"`},
		{name: "session path id", method: http.MethodPost, path: "/api/v1/sim/sessions/bad/actions", body: `{}`, token: teacherToken, code: `"code":"42002"`},
		{name: "action body", method: http.MethodPost, path: "/api/v1/sim/sessions/7001/actions", body: `{`, token: teacherToken, code: `"code":"42004"`},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Authorization", "Bearer "+tc.token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s expected unified status 200, got %d", tc.name, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), tc.code) {
			t.Fatalf("%s expected %s response, got %s", tc.name, tc.code, rec.Body.String())
		}
	}
}

// TestSimPackageLifecycleAdminRoutesExist 确认文档要求的下架与重新上架生命周期入口存在。
func TestSimPackageLifecycleAdminRoutesExist(t *testing.T) {
	engine, authMgr := newSimRouteEngine([]string{"teacher"})
	platformToken, err := authMgr.IssueAccess(0, 9001, 9001, true)
	if err != nil {
		t.Fatalf("issue platform token: %v", err)
	}

	for _, tc := range []struct {
		name string
		path string
	}{
		{name: "archive", path: "/api/v1/sim/packages/9001/archive"},
		{name: "republish", path: "/api/v1/sim/packages/9001/republish"},
	} {
		req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer "+platformToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		if rec.Code == http.StatusNotFound || rec.Body.String() == "404 page not found" {
			t.Fatalf("%s lifecycle route must be registered", tc.name)
		}
	}
}

func newSimRouteEngine(roles []string) (*gin.Engine, *auth.Manager) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(response.TraceMiddleware(), gin.Recovery())
	authMgr := auth.NewManager(config.AuthConfig{
		JWTSigningKey:             "test-signing-key",
		AccessTTLMin:              15,
		JWTIssuer:                 "chaimir-test",
		HMACKey:                   "test-service-hmac-key",
		ServiceAuthMaxSkewSeconds: 300,
	})
	api := NewAPI(nil, authMgr, &simRouteIdentity{roles: roles}, config.UploadConfig{})
	api.Register(engine.Group("/api/v1"))
	return engine, authMgr
}

func signSimServiceRequest(req *http.Request, key, service, tenantID, sourceRef, traceID, timestamp string) {
	req.Header.Set(auth.ServiceNameHeader, service)
	req.Header.Set(auth.ServiceTenantHeader, tenantID)
	req.Header.Set(auth.ServiceSourceRefHeader, sourceRef)
	req.Header.Set(auth.ServiceTimestampHeader, timestamp)
	req.Header.Set(response.TraceHeader, traceID)
	req.Header.Set(auth.ServiceSignatureHeader, simServiceSignatureForTest(key, req.Method, req.URL.EscapedPath(), tenantID, sourceRef, timestamp, traceID))
}

func simServiceSignatureForTest(key, method, path, tenantID, sourceRef, timestamp, traceID string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(method + "\n" + path + "\n" + tenantID + "\n" + sourceRef + "\n" + timestamp + "\n" + traceID))
	return hex.EncodeToString(mac.Sum(nil))
}

// currentSimServiceTimestamp 返回内部服务签名测试使用的当前 UTC 秒。
func currentSimServiceTimestamp() string {
	return strconv.FormatInt(timex.Now().Unix(), 10)
}

type simRouteIdentity struct {
	roles []string
}

func (f *simRouteIdentity) GetAccount(context.Context, int64) (contracts.AccountInfo, error) {
	return contracts.AccountInfo{Roles: f.roles}, nil
}

func (f *simRouteIdentity) BatchGetAccounts(context.Context, []int64) ([]contracts.AccountInfo, error) {
	return nil, nil
}

func (f *simRouteIdentity) HasRole(_ context.Context, _ int64, role string) (bool, error) {
	for _, actual := range f.roles {
		if actual == role {
			return true, nil
		}
	}
	return false, nil
}
