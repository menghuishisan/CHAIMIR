// SSO 配置安全处理测试。
package identity

import (
	"testing"

	"chaimir/pkg/apperr"
	"chaimir/pkg/crypto"
)

// TestProtectSsoConfigEncryptsSensitiveValues 确认密码类字段保存前加密。
func TestProtectSsoConfigEncryptsSensitiveValues(t *testing.T) {
	cipher, err := crypto.NewCipher([]byte("12345678901234567890123456789012"))
	if err != nil {
		t.Fatalf("create cipher: %v", err)
	}
	cfg, err := protectSsoConfig(cipher, map[string]any{
		"server_url":    "https://sso.example.edu",
		"bind_password": "secret-password",
	})
	if err != nil {
		t.Fatalf("protect config: %v", err)
	}
	if cfg["bind_password"] == "secret-password" {
		t.Fatalf("sensitive value was not encrypted")
	}
	encrypted, ok := cfg["bind_password"].(map[string]any)
	if !ok {
		t.Fatalf("encrypted value format: %#v", cfg["bind_password"])
	}
	value, ok := encrypted["value"].(string)
	if !ok {
		t.Fatalf("encrypted value missing: %#v", encrypted)
	}
	plain, err := cipher.DecryptString(value)
	if err != nil {
		t.Fatalf("decrypt protected value: %v", err)
	}
	if plain != "secret-password" {
		t.Fatalf("unexpected decrypted value: %q", plain)
	}
	if cfg["server_url"] != "https://sso.example.edu" {
		t.Fatalf("non-sensitive value changed: %#v", cfg["server_url"])
	}
}

// TestMaskSsoConfigHidesSensitiveValues 确认响应不返回密码密文或明文。
func TestMaskSsoConfigHidesSensitiveValues(t *testing.T) {
	cfg := maskSsoConfig(map[string]any{
		"bind_password": map[string]any{"encrypted": true, "value": "ciphertext"},
		"server_url":    "https://sso.example.edu",
	})
	if cfg["bind_password"] != "已配置" {
		t.Fatalf("expected masked password, got %#v", cfg["bind_password"])
	}
	if cfg["server_url"] != "https://sso.example.edu" {
		t.Fatalf("non-sensitive value changed: %#v", cfg["server_url"])
	}
}

// TestRevealSsoConfigDecryptsSensitiveValues 确认服务端协议适配器能还原加密配置。
func TestRevealSsoConfigDecryptsSensitiveValues(t *testing.T) {
	cipher, err := crypto.NewCipher([]byte("12345678901234567890123456789012"))
	if err != nil {
		t.Fatalf("create cipher: %v", err)
	}
	protected, err := protectSsoConfig(cipher, map[string]any{
		"url":           "ldaps://ldap.example.edu",
		"bind_password": "secret-password",
	})
	if err != nil {
		t.Fatalf("protect config: %v", err)
	}
	revealed, err := revealSsoConfig(cipher, protected)
	if err != nil {
		t.Fatalf("reveal config: %v", err)
	}
	if revealed["bind_password"] != "secret-password" {
		t.Fatalf("expected decrypted password, got %#v", revealed["bind_password"])
	}
	if revealed["url"] != "ldaps://ldap.example.edu" {
		t.Fatalf("non-sensitive value changed: %#v", revealed["url"])
	}
}

// TestValidateSsoConfigForStorageRejectsIncompleteLDAP 确认保存 SSO 配置时即拒绝不完整 LDAP 配置。
func TestValidateSsoConfigForStorageRejectsIncompleteLDAP(t *testing.T) {
	err := validateSsoConfigForStorage(SsoTypeLDAP, map[string]any{"url": "ldaps://ldap.example.edu"})
	if err != apperr.ErrSsoLDAPConfigInvalid {
		t.Fatalf("expected bad request for incomplete ldap config, got %v", err)
	}
}

// TestValidateSsoConfigForStorageRejectsInvalidCAS 确认保存 CAS 配置时即校验服务端地址。
func TestValidateSsoConfigForStorageRejectsInvalidCAS(t *testing.T) {
	err := validateSsoConfigForStorage(SsoTypeCAS, map[string]any{"server_url": "not-a-url"})
	if err != apperr.ErrSsoCASURLInvalid {
		t.Fatalf("expected bad request for invalid cas config, got %v", err)
	}
}
