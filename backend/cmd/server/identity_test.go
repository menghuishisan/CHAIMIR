// identity 装配测试。
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"chaimir/internal/modules/identity"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/config"
	"chaimir/pkg/snowflake"
)

// newIdentityTestServer 装配 M1 路由测试服务。
func newIdentityTestServer(t *testing.T) *httpServer {
	t.Helper()
	server := newHTTPServer(config.ServerConfig{Addr: "127.0.0.1", Port: 0, AppEnv: "test"}, nil)
	idgen, err := snowflake.NewNode(1)
	if err != nil {
		t.Fatalf("create snowflake node: %v", err)
	}
	authCfg := config.AuthConfig{JWTSigningKey: "test-signing-key", AccessTTLMin: 15, JWTIssuer: "chaimir-test"}
	deps := &moduleDeps{
		cfg: &config.Config{
			Deploy: config.DeployConfig{Mode: "saas", PlatformEnabled: true},
			Server: config.ServerConfig{AppEnv: "test"},
			Auth: config.AuthConfig{
				JWTSigningKey: "test-signing-key",
				AccessTTLMin:  15,
				RefreshTTLDay: 7,
				JWTIssuer:     "chaimir-test",
				EncryptionKey: "12345678901234567890123456789012",
				HMACKey:       "test-hmac-key",
			},
			SMS: config.SMSConfig{Provider: "log"},
		},
		infra: &infra{
			auth:   auth.NewManager(authCfg),
			idgen:  idgen,
			server: server,
		},
	}
	if err := assembleIdentity(deps); err != nil {
		t.Fatalf("assemble identity: %v", err)
	}
	return server
}

// TestAssembleIdentityReturnsConfigError 确认 M1 装配失败返回错误,不使用 panic。
func TestAssembleIdentityReturnsConfigError(t *testing.T) {
	server := newHTTPServer(config.ServerConfig{Addr: "127.0.0.1", Port: 0, AppEnv: "test"}, nil)
	idgen, err := snowflake.NewNode(1)
	if err != nil {
		t.Fatalf("create snowflake node: %v", err)
	}
	deps := &moduleDeps{
		cfg: &config.Config{
			Deploy: config.DeployConfig{Mode: "saas", PlatformEnabled: true},
			Server: config.ServerConfig{AppEnv: "test"},
			Auth: config.AuthConfig{
				JWTSigningKey: "test-signing-key",
				AccessTTLMin:  15,
				RefreshTTLDay: 7,
				JWTIssuer:     "chaimir-test",
				EncryptionKey: "bad-key",
				HMACKey:       "test-hmac-key",
			},
		},
		infra: &infra{
			auth:   auth.NewManager(config.AuthConfig{JWTSigningKey: "test-signing-key", AccessTTLMin: 15, JWTIssuer: "chaimir-test"}),
			idgen:  idgen,
			server: server,
		},
	}
	if err := assembleIdentity(deps); err == nil {
		t.Fatalf("expected assemble identity to return config error")
	}
}

// TestAssembleIdentityRegistersPublicAuthRoutes 确认公开认证路由已装配。
func TestAssembleIdentityRegistersPublicAuthRoutes(t *testing.T) {
	server := newIdentityTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login/phone", strings.NewReader("{"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected unified error response status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"code":"12023"`) {
		t.Fatalf("expected login phone invalid code from identity handler, got body=%s", rec.Body.String())
	}
}

// TestActivateRouteIsPublic 确认激活码开通入口公开且走统一错误响应。
func TestActivateRouteIsPublic(t *testing.T) {
	server := newIdentityTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/activate", strings.NewReader("{"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected unified error response status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"code":"12032"`) {
		t.Fatalf("expected activation invalid code from activation handler, got body=%s", rec.Body.String())
	}
}

// TestImportTemplateRouteIsProtected 确认导入模板接口纳入账号管理权限链。
func TestImportTemplateRouteIsProtected(t *testing.T) {
	server := newIdentityTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/import/template?type=student", nil)
	rec := httptest.NewRecorder()
	server.engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected unified error response status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"code":"11001"`) {
		t.Fatalf("expected unauthorized code from protected route, got body=%s", rec.Body.String())
	}
}

// TestTenantSSORoutesAreProtected 确认租户 SSO 配置接口已纳入学校管理员权限链。
func TestTenantSSORoutesAreProtected(t *testing.T) {
	server := newIdentityTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tenant/sso", nil)
	rec := httptest.NewRecorder()
	server.engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected unified error response status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"code":"11001"`) {
		t.Fatalf("expected unauthorized code from protected route, got body=%s", rec.Body.String())
	}
}

// TestAuditRouteIsProtected 确认审计查询入口需要登录;具体平台/学校权限由 identity 包单测覆盖。
func TestAuditRouteIsProtected(t *testing.T) {
	server := newIdentityTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit", nil)
	rec := httptest.NewRecorder()
	server.engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected unified status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"code":"11001"`) {
		t.Fatalf("expected unauthorized code from protected audit route, got body=%s", rec.Body.String())
	}
}

// TestIdentityRemainingDocumentedRoutesAreProtected 确认 M1 文档列出的剩余账号/会话路由已注册。
func TestIdentityRemainingDocumentedRoutesAreProtected(t *testing.T) {
	server := newIdentityTestServer(t)
	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/v1/org/import", `{"departments":[{"name":"计算机学院"}]}`},
		{http.MethodPatch, "/api/v1/org/majors/1", `{"name":"软件工程"}`},
		{http.MethodPatch, "/api/v1/org/classes/1", `{"name":"软工 2301","enrollment_year":2023}`},
		{http.MethodPost, "/api/v1/org/classes/archive", `{"class_ids":["1"]}`},
		{http.MethodPost, "/api/v1/org/classes/promote", `{"rows":[{"class_id":"1","name":"计科 2301","enrollment_year":2023}]}`},
		{http.MethodGet, "/api/v1/accounts/import/batches", ""},
		{http.MethodPost, "/api/v1/accounts/batch/disable", `{"account_ids":["1"]}`},
		{http.MethodPost, "/api/v1/accounts/batch/archive", `{"account_ids":["1"]}`},
		{http.MethodPost, "/api/v1/accounts/batch/restore", `{"account_ids":["1"]}`},
		{http.MethodGet, "/api/v1/me/sessions", ""},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.engine.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s expected unified status 200, got %d body=%s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), `"code":"11001"`) {
			t.Fatalf("%s %s expected unauthorized code, got body=%s", tc.method, tc.path, rec.Body.String())
		}
	}
}

// TestIdentitySSOLoginRoutesArePublic 确认 SSO 登录入口是公开路由,不被 JWT 中间件拦截。
func TestIdentitySSOLoginRoutesArePublic(t *testing.T) {
	server := newIdentityTestServer(t)
	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v1/auth/sso/pku/login", ""},
		{http.MethodGet, "/api/v1/auth/sso/pku/callback", ""},
		{http.MethodPost, "/api/v1/auth/sso/pku/ldap", `{`},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.engine.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s expected unified status 200, got %d body=%s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), `"code":"11001"`) {
			t.Fatalf("%s %s must be public, got unauthorized body=%s", tc.method, tc.path, rec.Body.String())
		}
	}
}

// TestBuildIdentitySmsSenderRejectsProdLogSender 确认生产环境不能使用日志短信。
func TestBuildIdentitySmsSenderRejectsProdLogSender(t *testing.T) {
	_, err := buildIdentitySmsSender(&config.Config{Server: config.ServerConfig{AppEnv: "prod"}})
	if err == nil {
		t.Fatalf("expected prod sms sender config to fail")
	}
}

// TestBuildIdentitySmsSenderAllowsDevLogSender 确认非生产环境可使用开发日志短信。
func TestBuildIdentitySmsSenderAllowsDevLogSender(t *testing.T) {
	sender, err := buildIdentitySmsSender(&config.Config{
		Server: config.ServerConfig{AppEnv: "dev"},
		SMS:    config.SMSConfig{Provider: "log"},
	})
	if err != nil {
		t.Fatalf("build dev sender: %v", err)
	}
	if _, ok := sender.(identity.LogSmsSender); !ok {
		t.Fatalf("expected dev log sender, got %T", sender)
	}
}

// TestBuildIdentitySmsSenderBuildsHTTPProvider 确认生产可配置真实 HTTP 短信网关。
func TestBuildIdentitySmsSenderBuildsHTTPProvider(t *testing.T) {
	sender, err := buildIdentitySmsSender(&config.Config{
		Server: config.ServerConfig{AppEnv: "prod"},
		SMS: config.SMSConfig{
			Provider:       "http",
			Endpoint:       "https://sms-gateway.example.edu/send",
			Token:          "secret-token",
			LoginTemplate:  "login",
			ResetTemplate:  "reset",
			ChangeTemplate: "change",
			TimeoutSeconds: 5,
		},
	})
	if err != nil {
		t.Fatalf("build sms sender: %v", err)
	}
	if _, ok := sender.(*identity.HTTPSmsSender); !ok {
		t.Fatalf("expected http sms sender, got %T", sender)
	}
}
