// M1 API 边界测试:守护身份模块 HTTP 参数错误使用模块专属错误码。
package identity

import (
	"os"
	"strings"
	"testing"
)

// TestAuthAPIErrorsUseDedicatedIdentityCodes 防止认证入口退回通用 ErrBadRequest。
func TestAuthAPIErrorsUseDedicatedIdentityCodes(t *testing.T) {
	src, err := os.ReadFile("api_auth.go")
	if err != nil {
		t.Fatalf("read api_auth: %v", err)
	}
	text := string(src)
	if strings.Contains(text, "ErrBadRequest") {
		t.Fatalf("M1 auth API must use dedicated identity error codes instead of ErrBadRequest")
	}
	for _, required := range []string{
		"ErrLoginPhoneInvalid",
		"ErrPlatformLoginInvalid",
		"ErrLoginNoInvalid",
		"ErrLoginSmsInvalid",
		"ErrSsoLoginInvalid",
		"ErrSsoCallbackInvalid",
		"ErrLDAPLoginInvalid",
		"ErrSmsRequestInvalid",
		"ErrRefreshRequestInvalid",
		"ErrResetPasswordInvalid",
		"ErrActivationInvalid",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("M1 auth API missing dedicated error %s", required)
		}
	}
}

// TestPlatformAdminLoginRouteExists 确认 SaaS 平台管理员有独立认证入口。
func TestPlatformAdminLoginRouteExists(t *testing.T) {
	src, err := os.ReadFile("api.go")
	if err != nil {
		t.Fatalf("read api.go: %v", err)
	}
	if !strings.Contains(string(src), `authG.POST("/login/platform", a.loginPlatform)`) {
		t.Fatalf("M1 must expose /auth/login/platform for platform administrators")
	}
}

// TestManagementAPIErrorsUseDedicatedIdentityCodes 防止账号、组织、租户和导入入口退回通用 ErrBadRequest。
func TestManagementAPIErrorsUseDedicatedIdentityCodes(t *testing.T) {
	for _, path := range []string{"api_account.go", "api_platform.go", "api_org.go"} {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if strings.Contains(string(src), "ErrBadRequest") {
			t.Fatalf("%s must use dedicated identity error codes instead of ErrBadRequest", path)
		}
	}
}
