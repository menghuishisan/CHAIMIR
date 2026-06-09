// auth_test 校验内部服务 HMAC 鉴权和平台级角色守卫的统一边界。
package auth

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
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/response"

	"github.com/gin-gonic/gin"
)

// TestServiceMiddlewareRejectsOrdinaryBearerToken 确认普通用户 JWT 不能冒充内部服务调用。
func TestServiceMiddlewareRejectsOrdinaryBearerToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mgr := NewManager(serviceAuthTestConfig())
	token, err := mgr.IssueAccess(10, 501, 9001, false)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}
	engine := serviceAuthTestEngine(mgr)
	req := httptest.NewRequest(http.MethodPost, "/internal/action", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), `"code":"11008"`) {
		t.Fatalf("expected service auth failure, got %s", rec.Body.String())
	}
}

// TestServiceMiddlewareInjectsTenantIdentity 确认合法服务签名会注入租户上下文供 RLS 使用。
func TestServiceMiddlewareInjectsTenantIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mgr := NewManager(serviceAuthTestConfig())
	engine := serviceAuthTestEngine(mgr)
	req := httptest.NewRequest(http.MethodPost, "/internal/action", strings.NewReader(`{}`))
	signServiceRequest(req, "test-service-hmac-key", "judge", "10", "experiment:2026:instance:55", "trace-1", currentServiceTimestamp())
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Body.String() != "tenant=10" {
		t.Fatalf("expected handler to see tenant identity, got %s", rec.Body.String())
	}
}

// TestServiceMiddlewareMarksInternalServiceIdentity 确认内部服务签名上下文会被明确标记为系统任务身份。
func TestServiceMiddlewareMarksInternalServiceIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mgr := NewManager(serviceAuthTestConfig())
	engine := gin.New()
	engine.Use(response.TraceMiddleware())
	engine.POST("/internal/action", mgr.ServiceMiddleware(), func(c *gin.Context) {
		id, ok := tenant.FromContext(c.Request.Context())
		if !ok {
			c.String(http.StatusInternalServerError, "missing identity")
			return
		}
		if !id.IsSystem {
			c.String(http.StatusInternalServerError, "missing system identity")
			return
		}
		c.String(http.StatusOK, "system")
	})
	req := httptest.NewRequest(http.MethodPost, "/internal/action", strings.NewReader(`{}`))
	signServiceRequest(req, "test-service-hmac-key", "judge", "10", "experiment:2026:instance:55", "trace-1", currentServiceTimestamp())
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Body.String() != "system" {
		t.Fatalf("expected internal service identity, got %s", rec.Body.String())
	}
}

// TestServiceMiddlewareRejectsExpiredTimestamp 确认服务签名不能被长期重放。
func TestServiceMiddlewareRejectsExpiredTimestamp(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mgr := NewManager(serviceAuthTestConfig())
	engine := serviceAuthTestEngine(mgr)
	req := httptest.NewRequest(http.MethodPost, "/internal/action", strings.NewReader(`{}`))
	signServiceRequest(req, "test-service-hmac-key", "judge", "10", "experiment:2026:instance:55", "trace-1", "1")
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), `"code":"11008"`) {
		t.Fatalf("expected expired service signature to fail, got %s", rec.Body.String())
	}
}

// TestServiceMiddlewareInjectsSourceRef 确认签名绑定的来源标识进入上下文。
func TestServiceMiddlewareInjectsSourceRef(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mgr := NewManager(serviceAuthTestConfig())
	engine := gin.New()
	engine.Use(response.TraceMiddleware())
	engine.POST("/internal/action", mgr.ServiceMiddleware(), func(c *gin.Context) {
		sourceRef, ok := ServiceSourceRefFromContext(c.Request.Context())
		if !ok {
			c.String(http.StatusInternalServerError, "missing source_ref")
			return
		}
		c.String(http.StatusOK, sourceRef)
	})
	req := httptest.NewRequest(http.MethodPost, "/internal/action", strings.NewReader(`{}`))
	signServiceRequest(req, "test-service-hmac-key", "judge", "10", "experiment:2026:instance:55", "trace-1", currentServiceTimestamp())
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Body.String() != "experiment:2026:instance:55" {
		t.Fatalf("expected handler to see signed source_ref, got %s", rec.Body.String())
	}
}

// TestValidSourceRefRequiresDocumentedShape 确认服务来源标识统一遵循四段规范。
func TestValidSourceRefRequiresDocumentedShape(t *testing.T) {
	valid := []string{
		"exp:2026:instance:55",
		"contest:2026:submission:abc_123",
		"teaching:2026:submission:9-8",
	}
	for _, ref := range valid {
		if !ValidSourceRef(ref) {
			t.Fatalf("valid source_ref rejected: %s", ref)
		}
	}

	invalid := []string{
		"",
		"contest:2026:55",
		"exp:26:instance:55",
		"Exp:2026:instance:55",
		"exp:2026:1instance:55",
		"exp:2026:instance:55/66",
	}
	for _, ref := range invalid {
		if ValidSourceRef(ref) {
			t.Fatalf("invalid source_ref accepted: %s", ref)
		}
	}
}

// TestServiceSourceRefAuthorizedOnlyRestrictsSignedServiceContext 确认只有服务签名上下文才触发来源约束。
func TestServiceSourceRefAuthorizedOnlyRestrictsSignedServiceContext(t *testing.T) {
	ctx := context.Background()
	if !ServiceSourceRefAuthorized(ctx, "exp:2026:instance:55") {
		t.Fatalf("ordinary user context should not be restricted by source_ref")
	}
	signed := WithServiceSourceRef(ctx, "exp:2026:instance:55")
	if !ServiceSourceRefAuthorized(signed, "exp:2026:instance:55") {
		t.Fatalf("matching signed source_ref should pass")
	}
	if ServiceSourceRefAuthorized(signed, "contest:2026:contest:55") {
		t.Fatalf("mismatched signed source_ref should be rejected")
	}
}

// TestRequirePlatformOrAnyRoleAuthorizesTeachersAndPlatform 确认平台层承载通用角色守卫。
func TestRequirePlatformOrAnyRoleAuthorizesTeachersAndPlatform(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := roleAuthTestEngine(
		tenant.Identity{TenantID: 10, AccountID: 501},
		&roleAuthIdentity{roles: []string{contracts.RoleTeacher}},
	)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/teacher", nil)

	engine.ServeHTTP(rec, req)

	if rec.Body.String() != "ok" {
		t.Fatalf("expected teacher role to pass, got %s", rec.Body.String())
	}

	platformEngine := roleAuthTestEngine(tenant.Identity{IsPlatform: true, AccountID: 1}, nil)
	platformRec := httptest.NewRecorder()
	platformReq := httptest.NewRequest(http.MethodGet, "/teacher", nil)
	platformEngine.ServeHTTP(platformRec, platformReq)
	if platformRec.Body.String() != "ok" {
		t.Fatalf("expected platform identity to pass without tenant role lookup, got %s", platformRec.Body.String())
	}
}

// TestRequirePlatformOrAnyRoleRejectsMissingOrMismatchedRole 确认角色不匹配时统一返回禁止访问。
func TestRequirePlatformOrAnyRoleRejectsMissingOrMismatchedRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name     string
		identity contracts.IdentityService
	}{
		{name: "missing identity", identity: nil},
		{name: "student role", identity: &roleAuthIdentity{roles: []string{contracts.RoleStudent}}},
	} {
		engine := roleAuthTestEngine(tenant.Identity{TenantID: 10, AccountID: 501}, tc.identity)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/teacher", nil)

		engine.ServeHTTP(rec, req)

		if !strings.Contains(rec.Body.String(), `"code":"11002"`) {
			t.Fatalf("%s: expected forbidden response, got %s", tc.name, rec.Body.String())
		}
	}
}

// TestRequirePlatformIdentityAcceptsOnlyPlatformContext 确认平台身份入口统一由平台层把关。
func TestRequirePlatformIdentityAcceptsOnlyPlatformContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	platformEngine := platformAuthTestEngine(tenant.Identity{IsPlatform: true, AccountID: 1})
	platformRec := httptest.NewRecorder()
	platformEngine.ServeHTTP(platformRec, httptest.NewRequest(http.MethodGet, "/platform", nil))
	if platformRec.Body.String() != "ok" {
		t.Fatalf("expected platform identity to pass, got %s", platformRec.Body.String())
	}

	tenantEngine := platformAuthTestEngine(tenant.Identity{TenantID: 10, AccountID: 501})
	tenantRec := httptest.NewRecorder()
	tenantEngine.ServeHTTP(tenantRec, httptest.NewRequest(http.MethodGet, "/platform", nil))
	if !strings.Contains(tenantRec.Body.String(), `"code":"11002"`) {
		t.Fatalf("expected tenant identity to be forbidden, got %s", tenantRec.Body.String())
	}
}

// TestRequireTenantAnyRoleRejectsPlatformContext 确认平台身份不会被当作租户角色放行。
func TestRequireTenantAnyRoleRejectsPlatformContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tenantEngine := tenantRoleAuthTestEngine(
		tenant.Identity{TenantID: 10, AccountID: 501},
		&roleAuthIdentity{roles: []string{contracts.RoleSchoolAdmin}},
	)
	tenantRec := httptest.NewRecorder()
	tenantEngine.ServeHTTP(tenantRec, httptest.NewRequest(http.MethodGet, "/tenant-role", nil))
	if tenantRec.Body.String() != "ok" {
		t.Fatalf("expected tenant school admin to pass, got %s", tenantRec.Body.String())
	}

	platformEngine := tenantRoleAuthTestEngine(tenant.Identity{IsPlatform: true, AccountID: 1}, &roleAuthIdentity{roles: []string{contracts.RoleSchoolAdmin}})
	platformRec := httptest.NewRecorder()
	platformEngine.ServeHTTP(platformRec, httptest.NewRequest(http.MethodGet, "/tenant-role", nil))
	if !strings.Contains(platformRec.Body.String(), `"code":"11002"`) {
		t.Fatalf("expected platform context to be forbidden for tenant role route, got %s", platformRec.Body.String())
	}
}

// serviceAuthTestConfig 给出服务签名测试所需的最小鉴权配置。
func serviceAuthTestConfig() config.AuthConfig {
	return config.AuthConfig{
		JWTSigningKey:             "test-signing-key",
		AccessTTLMin:              15,
		JWTIssuer:                 "chaimir-test",
		HMACKey:                   "test-service-hmac-key",
		ServiceAuthMaxSkewSeconds: 300,
	}
}

// serviceAuthTestEngine 构造带统一 trace 与服务鉴权的测试路由。
func serviceAuthTestEngine(mgr *Manager) *gin.Engine {
	engine := gin.New()
	engine.Use(response.TraceMiddleware())
	engine.POST("/internal/action", mgr.ServiceMiddleware(), func(c *gin.Context) {
		id, ok := tenant.FromContext(c.Request.Context())
		if !ok {
			c.String(http.StatusInternalServerError, "missing identity")
			return
		}
		c.String(http.StatusOK, "tenant=%d", id.TenantID)
	})
	return engine
}

// signServiceRequest 用测试密钥生成内部服务鉴权头。
func signServiceRequest(req *http.Request, key, service, tenantID, sourceRef, traceID, timestamp string) {
	req.Header.Set(ServiceNameHeader, service)
	req.Header.Set(ServiceTenantHeader, tenantID)
	req.Header.Set(ServiceSourceRefHeader, sourceRef)
	req.Header.Set(ServiceTimestampHeader, timestamp)
	req.Header.Set(response.TraceHeader, traceID)
	req.Header.Set(ServiceSignatureHeader, serviceSignatureForTest(key, req.Method, req.URL.EscapedPath(), tenantID, sourceRef, timestamp, traceID))
}

// serviceSignatureForTest 按生产同一签名输入顺序生成测试签名。
func serviceSignatureForTest(key, method, path, tenantID, sourceRef, timestamp, traceID string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(method + "\n" + path + "\n" + tenantID + "\n" + sourceRef + "\n" + timestamp + "\n" + traceID))
	return hex.EncodeToString(mac.Sum(nil))
}

// currentServiceTimestamp 返回服务签名测试使用的 UTC 秒级时间戳。
func currentServiceTimestamp() string {
	return strconv.FormatInt(timex.Now().Unix(), 10)
}

// roleAuthTestEngine 注入已登录身份后执行平台层通用角色守卫。
func roleAuthTestEngine(id tenant.Identity, identity contracts.IdentityService) *gin.Engine {
	engine := gin.New()
	engine.Use(response.TraceMiddleware())
	engine.GET("/teacher", func(c *gin.Context) {
		c.Request = c.Request.WithContext(tenant.WithContext(c.Request.Context(), id))
		c.Next()
	}, RequirePlatformOrAnyRole(identity, contracts.RoleTeacher, contracts.RoleSchoolAdmin), func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	return engine
}

// tenantRoleAuthTestEngine 注入已登录身份后执行租户角色守卫。
func tenantRoleAuthTestEngine(id tenant.Identity, identity contracts.IdentityService) *gin.Engine {
	engine := gin.New()
	engine.Use(response.TraceMiddleware())
	engine.GET("/tenant-role", func(c *gin.Context) {
		c.Request = c.Request.WithContext(tenant.WithContext(c.Request.Context(), id))
		c.Next()
	}, RequireTenantAnyRole(identity, contracts.RoleSchoolAdmin), func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	return engine
}

// platformAuthTestEngine 注入已登录身份后执行平台身份守卫。
func platformAuthTestEngine(id tenant.Identity) *gin.Engine {
	engine := gin.New()
	engine.Use(response.TraceMiddleware())
	engine.GET("/platform", func(c *gin.Context) {
		c.Request = c.Request.WithContext(tenant.WithContext(c.Request.Context(), id))
		c.Next()
	}, RequirePlatformIdentity(), func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	return engine
}

// roleAuthIdentity 是角色中间件测试用的最小身份契约实现。
type roleAuthIdentity struct {
	roles []string
}

// GetAccount 返回带测试角色的账号摘要。
func (f *roleAuthIdentity) GetAccount(context.Context, int64) (contracts.AccountInfo, error) {
	return contracts.AccountInfo{Roles: f.roles}, nil
}

// BatchGetAccounts 不参与该组测试。
func (f *roleAuthIdentity) BatchGetAccounts(context.Context, []int64) ([]contracts.AccountInfo, error) {
	return nil, nil
}

// HasRole 按测试角色集合判断角色命中。
func (f *roleAuthIdentity) HasRole(_ context.Context, _ int64, role string) (bool, error) {
	return contracts.HasAnyRole(f.roles, role), nil
}
