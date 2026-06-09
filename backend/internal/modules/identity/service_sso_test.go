// identity service_sso_test 文件验证统一认证配置安全边界。
package identity

import (
	"testing"

	"chaimir/internal/platform/config"
	"chaimir/pkg/crypto"
)

// TestValidateSSOConfigRequiresSecureEndpoints 验证 CAS/LDAP 配置必须使用安全协议。
func TestValidateSSOConfigRequiresSecureEndpoints(t *testing.T) {
	s := &Service{}
	if err := s.validateSSOConfig(SSOConfigRequest{Type: SSOTypeCAS, MatchField: SSOMatchNo, Config: map[string]any{"server_url": "http://cas.example.edu/cas"}}); err == nil {
		t.Fatalf("期望拒绝非 HTTPS CAS 地址")
	}
	if err := s.validateSSOConfig(SSOConfigRequest{Type: SSOTypeCAS, MatchField: SSOMatchNo, Config: map[string]any{"server_url": "https://10.0.0.10/cas"}}); err == nil {
		t.Fatalf("期望拒绝指向内网的 CAS 地址")
	}
	if err := s.validateSSOConfig(SSOConfigRequest{Type: SSOTypeLDAP, MatchField: SSOMatchNo, Config: map[string]any{"url": "ldap://ldap.example.edu"}}); err == nil {
		t.Fatalf("期望拒绝非 LDAPS 地址")
	}
	if err := s.validateSSOConfig(SSOConfigRequest{Type: SSOTypeCAS, MatchField: SSOMatchNo, Config: map[string]any{"server_url": "https://cas.example.edu/cas"}}); err != nil {
		t.Fatalf("期望 HTTPS CAS 地址通过: %v", err)
	}
}

// TestSecureSSOConfigEncryptsLDAPBindPassword 验证 LDAP 绑定密码入库前必须加密。
func TestSecureSSOConfigEncryptsLDAPBindPassword(t *testing.T) {
	cipher, err := crypto.NewCipher([]byte("12345678901234567890123456789012"))
	if err != nil {
		t.Fatalf("初始化测试加密器失败: %v", err)
	}
	s := &Service{cipher: cipher}

	out, err := s.secureSSOConfig(SSOConfigRequest{Type: SSOTypeLDAP, Config: map[string]any{
		"url":             "ldaps://ldap.example.edu",
		"bind_dn":         "cn=reader,dc=example,dc=edu",
		"bind_password":   "secret-password",
		"base_dn":         "dc=example,dc=edu",
		"user_filter":     "(uid={username})",
		"match_attribute": "employeeNumber",
	}})
	if err != nil {
		t.Fatalf("加密 LDAP 配置失败: %v", err)
	}
	stored, _ := out["bind_password"].(string)
	if stored == "" || stored == "secret-password" {
		t.Fatalf("LDAP bind_password 不应明文存储: %q", stored)
	}
	plain, err := s.decryptLDAPBindPassword(stored)
	if err != nil {
		t.Fatalf("解密 LDAP bind_password 失败: %v", err)
	}
	if plain != "secret-password" {
		t.Fatalf("LDAP bind_password 解密结果不正确: %q", plain)
	}
}

// TestServiceOriginAllowedUsesConfiguredWhitelist 验证 CAS service 回调 origin 只接受部署白名单。
func TestServiceOriginAllowedUsesConfiguredWhitelist(t *testing.T) {
	s := &Service{cfg: config.IdentityConfig{SSOAllowedServiceOrigins: []string{"https://chaimir.example.edu"}}}
	if !s.serviceOriginAllowed("https://chaimir.example.edu/api/v1/auth/sso/demo/callback") {
		t.Fatalf("期望白名单内 origin 通过")
	}
	if s.serviceOriginAllowed("https://evil.example.com/callback") {
		t.Fatalf("期望白名单外 origin 被拒绝")
	}
}
