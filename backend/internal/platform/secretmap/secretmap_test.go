// secretmap 测试覆盖敏感配置 map 的统一保护、脱敏和还原规则。
package secretmap

import (
	"testing"

	"chaimir/pkg/crypto"
)

// TestProtectMaskRevealNestedSensitiveValues 确认嵌套敏感配置被加密、响应脱敏、内部使用可还原。
func TestProtectMaskRevealNestedSensitiveValues(t *testing.T) {
	cipher, err := crypto.NewCipher([]byte("12345678901234567890123456789012"))
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	protected, err := Protect(cipher, map[string]any{
		"endpoint": "https://example.test/feed",
		"headers": map[string]any{
			"Authorization": "Bearer secret-token",
		},
		"bind_password": "secret-password",
	}, "测试配置")
	if err != nil {
		t.Fatalf("protect: %v", err)
	}
	if protected["bind_password"] == "secret-password" {
		t.Fatalf("sensitive value must not remain plaintext: %#v", protected)
	}
	masked := Mask(protected)
	if masked["bind_password"] != MaskedValue {
		t.Fatalf("sensitive value must be masked, got %#v", masked["bind_password"])
	}
	maskedHeaders := masked["headers"].(map[string]any)
	if maskedHeaders["Authorization"] != MaskedValue {
		t.Fatalf("nested sensitive value must be masked, got %#v", maskedHeaders)
	}
	revealed, err := Reveal(cipher, protected, "测试配置")
	if err != nil {
		t.Fatalf("reveal: %v", err)
	}
	if revealed["bind_password"] != "secret-password" {
		t.Fatalf("expected decrypted password, got %#v", revealed["bind_password"])
	}
	revealedHeaders := revealed["headers"].(map[string]any)
	if revealedHeaders["Authorization"] != "Bearer secret-token" {
		t.Fatalf("expected decrypted authorization header, got %#v", revealedHeaders)
	}
}

// TestProtectRequiresCipherForSensitiveValue 防止敏感配置在缺少加密器时静默明文保存。
func TestProtectRequiresCipherForSensitiveValue(t *testing.T) {
	if _, err := Protect(nil, map[string]any{"api_key": "secret"}, "测试配置"); err == nil {
		t.Fatalf("expected missing cipher error")
	}
}

// TestIsSensitiveKeyCoversProjectCredentialVariants 确认项目常见密钥字段都会进入统一脱敏/加密范围。
func TestIsSensitiveKeyCoversProjectCredentialVariants(t *testing.T) {
	cases := []string{"private_key", "access_key", "signing_key", "session_secret", "api_key"}
	for _, key := range cases {
		if !IsSensitiveKey(key) {
			t.Fatalf("expected %q to be treated as sensitive", key)
		}
	}
}
