// SSO 协议适配测试,覆盖 CAS URL 生成与 LDAP 配置校验。
package identity

import (
	"strings"
	"testing"
)

// TestBuildCASLoginURLUsesCASLibraryScheme 确认 CAS 登录地址使用配置的服务端与 service 参数生成。
func TestBuildCASLoginURLUsesCASLibraryScheme(t *testing.T) {
	redirectURL, err := buildCASLoginURL(map[string]any{
		"server_url": "https://sso.example.edu/cas",
	}, "https://chaimir.example.edu/api/v1/auth/sso/pku/callback", []string{"https://chaimir.example.edu"})
	if err != nil {
		t.Fatalf("build cas login url: %v", err)
	}
	if !strings.HasPrefix(redirectURL, "https://sso.example.edu/cas/login?") {
		t.Fatalf("unexpected cas login url: %s", redirectURL)
	}
	if !strings.Contains(redirectURL, "service=https%3A%2F%2Fchaimir.example.edu%2Fapi%2Fv1%2Fauth%2Fsso%2Fpku%2Fcallback") {
		t.Fatalf("service parameter missing or not escaped: %s", redirectURL)
	}
}

// TestBuildCASLoginURLRejectsPrivateEndpoint 确认租户 CAS 配置不能把服务端出站请求引向本机或私网。
func TestBuildCASLoginURLRejectsPrivateEndpoint(t *testing.T) {
	_, err := buildCASLoginURL(map[string]any{
		"server_url": "http://127.0.0.1:8080/cas",
	}, "https://chaimir.example.edu/api/v1/auth/sso/pku/callback", []string{"https://chaimir.example.edu"})
	if err == nil {
		t.Fatalf("expected private CAS endpoint to fail")
	}
}

// TestBuildCASLoginURLRejectsUnlistedServiceOrigin 确认 CAS service 回调域必须来自平台白名单。
func TestBuildCASLoginURLRejectsUnlistedServiceOrigin(t *testing.T) {
	_, err := buildCASLoginURL(map[string]any{
		"server_url": "https://sso.example.edu/cas",
	}, "https://evil.example.net/api/v1/auth/sso/pku/callback", []string{"https://chaimir.example.edu"})
	if err == nil {
		t.Fatalf("expected unlisted CAS service origin to fail")
	}
}

// TestParseLDAPConfigRequiresProductionFields 确认 LDAP 适配器拒绝不完整配置。
func TestParseLDAPConfigRequiresProductionFields(t *testing.T) {
	_, err := parseLDAPConfig(map[string]any{
		"url": "ldaps://ldap.example.edu:636",
	})
	if err == nil {
		t.Fatalf("expected incomplete ldap config to fail")
	}
}

// TestParseLDAPConfigRejectsPlainBind 确认 LDAP 密码认证不能走未加密连接。
func TestParseLDAPConfigRejectsPlainBind(t *testing.T) {
	_, err := parseLDAPConfig(map[string]any{
		"url":             "ldap://ldap.example.edu:389",
		"bind_dn":         "cn=svc,dc=example,dc=edu",
		"bind_password":   "secret",
		"base_dn":         "dc=example,dc=edu",
		"user_filter":     "(uid=%s)",
		"match_attribute": "uid",
	})
	if err == nil {
		t.Fatalf("expected plaintext ldap config to fail")
	}
}

// TestParseLDAPConfigRejectsTLSVerificationBypass 确认生产配置不允许跳过 TLS 校验。
func TestParseLDAPConfigRejectsTLSVerificationBypass(t *testing.T) {
	_, err := parseLDAPConfig(map[string]any{
		"url":                  "ldaps://ldap.example.edu:636",
		"bind_dn":              "cn=svc,dc=example,dc=edu",
		"bind_password":        "secret",
		"base_dn":              "dc=example,dc=edu",
		"user_filter":          "(uid=%s)",
		"match_attribute":      "uid",
		"insecure_skip_verify": true,
	})
	if err == nil {
		t.Fatalf("expected insecure tls bypass to fail")
	}
}

// TestParseLDAPConfigRejectsPrivateEndpoint 确认 LDAP 服务地址也遵守统一出站地址边界。
func TestParseLDAPConfigRejectsPrivateEndpoint(t *testing.T) {
	_, err := parseLDAPConfig(map[string]any{
		"url":             "ldaps://10.0.0.10:636",
		"bind_dn":         "cn=svc,dc=example,dc=edu",
		"bind_password":   "secret",
		"base_dn":         "dc=example,dc=edu",
		"user_filter":     "(uid=%s)",
		"match_attribute": "uid",
	})
	if err == nil {
		t.Fatalf("expected private LDAP endpoint to fail")
	}
}
