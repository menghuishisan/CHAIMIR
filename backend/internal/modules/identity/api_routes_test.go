// identity api_routes_test 文件守护身份模块文档要求的 HTTP 路由注册。
package identity

import (
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/config"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestDocumentedRoutesAreRegistered 验证接口设计文档中的关键路由已进入 Gin 路由表。
func TestDocumentedRoutesAreRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, &Service{deploy: config.DeployConfig{PlatformEnabled: true}}, testAuthManager())

	routes := map[string]bool{}
	for _, route := range r.Routes() {
		routes[route.Method+" "+route.Path] = true
	}

	expected := []string{
		http.MethodPost + " /api/v1/auth/login/platform",
		http.MethodPost + " /api/v1/auth/login/phone",
		http.MethodPost + " /api/v1/auth/login/no",
		http.MethodPost + " /api/v1/auth/login/sms",
		http.MethodPost + " /api/v1/auth/sms/send",
		http.MethodPost + " /api/v1/auth/refresh",
		http.MethodPost + " /api/v1/auth/logout",
		http.MethodPost + " /api/v1/auth/password/reset",
		http.MethodPost + " /api/v1/auth/activate",
		http.MethodGet + " /api/v1/auth/sso/:tenant_code/login",
		http.MethodGet + " /api/v1/auth/sso/:tenant_code/callback",
		http.MethodPost + " /api/v1/auth/sso/:tenant_code/ldap",
		http.MethodPost + " /api/v1/platform/applications",
		http.MethodGet + " /api/v1/platform/applications",
		http.MethodPost + " /api/v1/platform/applications/:id/approve",
		http.MethodPost + " /api/v1/platform/applications/:id/reject",
		http.MethodGet + " /api/v1/platform/tenants",
		http.MethodGet + " /api/v1/platform/tenants/:id",
		http.MethodPatch + " /api/v1/platform/tenants/:id",
		http.MethodGet + " /api/v1/tenant/config",
		http.MethodPatch + " /api/v1/tenant/config",
		http.MethodGet + " /api/v1/tenant/sso",
		http.MethodPut + " /api/v1/tenant/sso",
		http.MethodGet + " /api/v1/org/departments",
		http.MethodPost + " /api/v1/org/departments",
		http.MethodPatch + " /api/v1/org/departments/:id",
		http.MethodDelete + " /api/v1/org/departments/:id",
		http.MethodGet + " /api/v1/org/majors",
		http.MethodPost + " /api/v1/org/majors",
		http.MethodPatch + " /api/v1/org/majors/:id",
		http.MethodDelete + " /api/v1/org/majors/:id",
		http.MethodGet + " /api/v1/org/classes",
		http.MethodPost + " /api/v1/org/classes",
		http.MethodPatch + " /api/v1/org/classes/:id",
		http.MethodDelete + " /api/v1/org/classes/:id",
		http.MethodPost + " /api/v1/org/import/preview",
		http.MethodPost + " /api/v1/org/import/commit",
		http.MethodGet + " /api/v1/org/import/template",
		http.MethodPost + " /api/v1/org/classes/archive",
		http.MethodPost + " /api/v1/org/classes/promote",
		http.MethodGet + " /api/v1/accounts",
		http.MethodPost + " /api/v1/accounts",
		http.MethodPatch + " /api/v1/accounts/:id",
		http.MethodPost + " /api/v1/accounts/:id/disable",
		http.MethodPost + " /api/v1/accounts/:id/enable",
		http.MethodPost + " /api/v1/accounts/:id/archive",
		http.MethodPost + " /api/v1/accounts/:id/restore",
		http.MethodPost + " /api/v1/accounts/:id/cancel",
		http.MethodPost + " /api/v1/accounts/:id/force-logout",
		http.MethodPost + " /api/v1/accounts/:id/reset-password",
		http.MethodPost + " /api/v1/accounts/:id/grant-admin",
		http.MethodPost + " /api/v1/accounts/:id/revoke-admin",
		http.MethodPost + " /api/v1/accounts/batch/disable",
		http.MethodPost + " /api/v1/accounts/batch/archive",
		http.MethodPost + " /api/v1/accounts/batch/restore",
		http.MethodPost + " /api/v1/accounts/import/preview",
		http.MethodPost + " /api/v1/accounts/import/commit",
		http.MethodGet + " /api/v1/accounts/import/template",
		http.MethodGet + " /api/v1/accounts/import/batches",
		http.MethodGet + " /api/v1/me",
		http.MethodPost + " /api/v1/me/password",
		http.MethodPost + " /api/v1/me/phone",
		http.MethodGet + " /api/v1/me/sessions",
		http.MethodGet + " /api/v1/audit",
	}

	for _, route := range expected {
		if !routes[route] {
			t.Fatalf("documented route not registered: %s", route)
		}
	}
}

// TestPrivateDeployDoesNotRegisterPlatformRoutes 验证私有化部署不会暴露 SaaS 平台层路由。
func TestPrivateDeployDoesNotRegisterPlatformRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r, &Service{deploy: config.DeployConfig{PlatformEnabled: false}}, testAuthManager())

	for _, route := range r.Routes() {
		if route.Path == "/api/v1/platform/applications" || route.Path == "/api/v1/platform/tenants" {
			t.Fatalf("private deployment must not register platform route: %s %s", route.Method, route.Path)
		}
	}
}

// testAuthManager 构造路由装配测试所需的真实鉴权管理器,不为受保护路由提供无鉴权捷径。
func testAuthManager() *auth.Manager {
	return auth.NewManager(config.AuthConfig{
		JWTSigningKey:             "identity-route-test-signing-key",
		HMACKey:                   "identity-route-test-hmac-key",
		AccessTTLMin:              15,
		JWTIssuer:                 "identity-route-test",
		ServiceAuthMaxSkewSeconds: 60,
	})
}
