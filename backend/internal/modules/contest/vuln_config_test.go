// M8 漏洞源配置安全测试:确认密钥字段加密保存、响应脱敏、服务端同步可解密。
package contest

import (
	"testing"

	"chaimir/pkg/crypto"
)

// TestVulnSourceConfigProtectMaskReveal 确认漏洞源配置中的密钥不会明文存储或回传。
func TestVulnSourceConfigProtectMaskReveal(t *testing.T) {
	cipher, err := crypto.NewCipher([]byte("12345678901234567890123456789012"))
	if err != nil {
		t.Fatalf("create cipher: %v", err)
	}
	protected, err := protectVulnSourceConfig(cipher, map[string]any{
		"endpoint": "https://vuln.example.edu/feed",
		"headers":  map[string]any{"Authorization": "Bearer secret-token"},
	})
	if err != nil {
		t.Fatalf("protect config: %v", err)
	}
	headers, ok := protected["headers"].(map[string]any)
	if !ok {
		t.Fatalf("headers missing: %#v", protected)
	}
	if headers["Authorization"] == "Bearer secret-token" {
		t.Fatalf("sensitive header was not encrypted")
	}
	masked := maskVulnSourceConfig(protected)
	maskedHeaders, ok := masked["headers"].(map[string]any)
	if !ok {
		t.Fatalf("masked headers missing: %#v", masked)
	}
	if maskedHeaders["Authorization"] != "已配置" {
		t.Fatalf("sensitive header not masked: %#v", maskedHeaders["Authorization"])
	}
	revealed, err := revealVulnSourceConfig(cipher, protected)
	if err != nil {
		t.Fatalf("reveal config: %v", err)
	}
	revealedHeaders, ok := revealed["headers"].(map[string]any)
	if !ok {
		t.Fatalf("revealed headers missing: %#v", revealed)
	}
	if revealedHeaders["Authorization"] != "Bearer secret-token" {
		t.Fatalf("unexpected revealed header: %#v", revealedHeaders["Authorization"])
	}
}
