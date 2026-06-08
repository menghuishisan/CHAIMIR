// M5 API 路由测试:确认题库接口按文档路径注册并进入统一鉴权链。
package content

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
	"chaimir/pkg/response"

	"github.com/gin-gonic/gin"
)

// TestContentRoutesAreProtected 确认 M5 用户与内部路径未登录时返回统一未登录错误。
func TestContentRoutesAreProtected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	mgr := auth.NewManager(config.AuthConfig{
		JWTSigningKey:             "test-signing-key",
		AccessTTLMin:              15,
		JWTIssuer:                 "chaimir-test",
		HMACKey:                   "test-service-hmac-key",
		ServiceAuthMaxSkewSeconds: 300,
	})
	engine.Use(response.TraceMiddleware())
	api := NewAPI(nil, mgr, nil)
	api.Register(engine.Group("/api/v1"))

	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v1/content/items", ""},
		{http.MethodPost, "/api/v1/content/items", `{}`},
		{http.MethodGet, "/api/v1/content/items/problem-1/1.0.0", ""},
		{http.MethodGet, "/api/v1/content/items/problem-1/1.0.0/full", ""},
		{http.MethodPatch, "/api/v1/content/items/1", `{}`},
		{http.MethodPost, "/api/v1/content/items/1/publish", `{}`},
		{http.MethodPost, "/api/v1/content/items/1/deprecate", `{}`},
		{http.MethodDelete, "/api/v1/content/items/1", ""},
		{http.MethodPost, "/api/v1/content/items/system-import", `{}`},
		{http.MethodGet, "/api/v1/content/items/problem-1/versions", ""},
		{http.MethodPost, "/api/v1/content/items/problem-1/new-version", `{}`},
		{http.MethodPost, "/api/v1/content/items/problem-1/1.0.0/clone", `{}`},
		{http.MethodPost, "/api/v1/content/items/1/share", `{}`},
		{http.MethodPost, "/api/v1/content/items/1/unshare", `{}`},
		{http.MethodGet, "/api/v1/content/shared", ""},
		{http.MethodGet, "/api/v1/content/categories", ""},
		{http.MethodPost, "/api/v1/content/categories", `{}`},
		{http.MethodPatch, "/api/v1/content/categories/1", `{}`},
		{http.MethodDelete, "/api/v1/content/categories/1", ""},
		{http.MethodGet, "/api/v1/content/papers", ""},
		{http.MethodPost, "/api/v1/content/papers", `{}`},
		{http.MethodGet, "/api/v1/content/papers/1", ""},
		{http.MethodPost, "/api/v1/content/papers/1/regenerate", `{}`},
		{http.MethodPost, "/api/v1/content/items/batch", `{}`},
		{http.MethodPost, "/api/v1/content/items/problem-1/1.0.0/usage", `{}`},
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
		if contentInternalRoute(tc.method, tc.path) {
			wantCode = `"code":"11008"`
		}
		if !strings.Contains(rec.Body.String(), wantCode) {
			t.Fatalf("%s %s expected %s response, got %s", tc.method, tc.path, wantCode, rec.Body.String())
		}
	}
}

func contentInternalRoute(method, path string) bool {
	switch {
	case method == http.MethodPost && path == "/api/v1/content/items/system-import":
		return true
	case method == http.MethodPost && path == "/api/v1/content/items/batch":
		return true
	case method == http.MethodPost && strings.Contains(path, "/usage"):
		return true
	default:
		return false
	}
}

// TestContentItemActionsUseDocumentedIDPaths 确认状态与共享操作只接受文档定义的 item ID,不把 code 转成 ID 兼容旧入口。
func TestContentItemActionsUseDocumentedIDPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	mgr := auth.NewManager(config.AuthConfig{
		JWTSigningKey: "test-signing-key",
		AccessTTLMin:  15,
		JWTIssuer:     "chaimir-test",
	})
	api := NewAPI(nil, mgr, contentRouteIdentity{})
	api.Register(engine.Group("/api/v1"))
	token, err := mgr.IssueAccess(10, 501, 9001, false)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}

	codeActionPaths := []string{
		"/api/v1/content/items/problem-1/publish",
		"/api/v1/content/items/problem-1/deprecate",
		"/api/v1/content/items/problem-1/share",
		"/api/v1/content/items/problem-1/unshare",
	}
	for _, path := range codeActionPaths {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s expected unified status 200, got %d", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `"code":"51008"`) {
			t.Fatalf("%s should reject non-ID action path, got %s", path, rec.Body.String())
		}
	}
}

// TestStudentCannotDirectlyFetchContentFace 确认学生不能绕过业务引用直接访问题库题面接口。
func TestStudentCannotDirectlyFetchContentFace(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	mgr := auth.NewManager(config.AuthConfig{
		JWTSigningKey: "test-signing-key",
		AccessTTLMin:  15,
		JWTIssuer:     "chaimir-test",
	})
	api := NewAPI(nil, mgr, studentContentRouteIdentity{})
	api.Register(engine.Group("/api/v1"))
	token, err := mgr.IssueAccess(10, 501, 9001, false)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/content/items/problem-1/1.0.0", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected unified status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"11002"`) {
		t.Fatalf("student direct content fetch must be forbidden, got %s", rec.Body.String())
	}
}

// TestInternalServiceCanReachFullContentRoute 确认文档标注的 full 内部 HTTP 入口接受服务签名。
func TestInternalServiceCanReachFullContentRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	mgr := auth.NewManager(config.AuthConfig{
		JWTSigningKey:             "test-signing-key",
		AccessTTLMin:              15,
		JWTIssuer:                 "chaimir-test",
		HMACKey:                   "test-service-hmac-key",
		ServiceAuthMaxSkewSeconds: 300,
	})
	engine.Use(response.TraceMiddleware())
	api := NewAPI(nil, mgr, nil)
	api.Register(engine.Group("/api/v1"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/content/items/problem-1/1.0.0/full", nil)
	signContentServiceRequest(req, "test-service-hmac-key", "judge", "10", "judge:task:1", "trace-1", currentContentServiceTimestamp())
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected unified status 200, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), `"code":"11008"`) || strings.Contains(rec.Body.String(), `"code":"11001"`) {
		t.Fatalf("service-signed full route should pass auth boundary, got %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"code":"51010"`) {
		t.Fatalf("service-signed full route should reach content layer, got %s", rec.Body.String())
	}
}

// TestContentAPIBoundaryErrorsUseModuleCodes 确认 M5 HTTP 边界的 JSON 与 ID 错误不再落到通用 11004。
func TestContentAPIBoundaryErrorsUseModuleCodes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	mgr := auth.NewManager(config.AuthConfig{
		JWTSigningKey: "test-signing-key",
		AccessTTLMin:  15,
		JWTIssuer:     "chaimir-test",
	})
	api := NewAPI(nil, mgr, contentRouteIdentity{})
	api.Register(engine.Group("/api/v1"))
	token, err := mgr.IssueAccess(10, 501, 9001, false)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}

	cases := []struct {
		method string
		path   string
		body   string
		code   string
	}{
		{http.MethodPost, "/api/v1/content/items", `{`, "51007"},
		{http.MethodPatch, "/api/v1/content/items/not-id", `{}`, "51008"},
		{http.MethodPost, "/api/v1/content/items/problem-1/new-version", `{`, "52003"},
		{http.MethodPost, "/api/v1/content/items/problem-1/1.0.0/clone", `{`, "53003"},
		{http.MethodPost, "/api/v1/content/categories", `{`, "51009"},
		{http.MethodPatch, "/api/v1/content/categories/not-id", `{}`, "51009"},
		{http.MethodPost, "/api/v1/content/papers", `{`, "54004"},
		{http.MethodPost, "/api/v1/content/papers/not-id/regenerate", `{}`, "54005"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s expected unified status 200, got %d", tc.method, tc.path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `"code":"`+tc.code+`"`) {
			t.Fatalf("%s %s expected module code %s, got %s", tc.method, tc.path, tc.code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), `"code":"11004"`) {
			t.Fatalf("%s %s must not return generic bad request: %s", tc.method, tc.path, rec.Body.String())
		}
	}
}

type contentRouteIdentity struct{}

func (contentRouteIdentity) GetAccount(context.Context, int64) (contracts.AccountInfo, error) {
	return contracts.AccountInfo{Roles: []string{"teacher"}}, nil
}

func (contentRouteIdentity) BatchGetAccounts(context.Context, []int64) ([]contracts.AccountInfo, error) {
	return nil, nil
}

func (contentRouteIdentity) HasRole(context.Context, int64, string) (bool, error) {
	return true, nil
}

type studentContentRouteIdentity struct{}

func (studentContentRouteIdentity) GetAccount(context.Context, int64) (contracts.AccountInfo, error) {
	return contracts.AccountInfo{Roles: []string{"student"}}, nil
}

func (studentContentRouteIdentity) BatchGetAccounts(context.Context, []int64) ([]contracts.AccountInfo, error) {
	return nil, nil
}

func (studentContentRouteIdentity) HasRole(context.Context, int64, string) (bool, error) {
	return false, nil
}

func signContentServiceRequest(req *http.Request, key, service, tenantID, sourceRef, traceID, timestamp string) {
	req.Header.Set(auth.ServiceNameHeader, service)
	req.Header.Set(auth.ServiceTenantHeader, tenantID)
	req.Header.Set(auth.ServiceSourceRefHeader, sourceRef)
	req.Header.Set(auth.ServiceTimestampHeader, timestamp)
	req.Header.Set(response.TraceHeader, traceID)
	req.Header.Set(auth.ServiceSignatureHeader, contentServiceSignature(key, req.Method, req.URL.EscapedPath(), tenantID, sourceRef, timestamp, traceID))
}

func contentServiceSignature(key, method, path, tenantID, sourceRef, timestamp, traceID string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(method + "\n" + path + "\n" + tenantID + "\n" + sourceRef + "\n" + timestamp + "\n" + traceID))
	return hex.EncodeToString(mac.Sum(nil))
}

// currentContentServiceTimestamp 返回内部服务签名测试使用的当前 UTC 秒。
func currentContentServiceTimestamp() string {
	return strconv.FormatInt(timex.Now().Unix(), 10)
}
