// M8 漏洞源配置安全:加密保存密钥类字段,服务端同步前解密,响应前脱敏。
package contest

import (
	"chaimir/internal/platform/secretmap"
	"chaimir/pkg/crypto"
)

// protectVulnSourceConfig 加密漏洞源配置中的敏感字段。
func protectVulnSourceConfig(cipher *crypto.Cipher, cfg map[string]any) (map[string]any, error) {
	return secretmap.Protect(cipher, cfg, "漏洞源敏感配置")
}

// revealVulnSourceConfig 解密漏洞源配置,仅供服务端同步适配器使用。
func revealVulnSourceConfig(cipher *crypto.Cipher, cfg map[string]any) (map[string]any, error) {
	return secretmap.Reveal(cipher, cfg, "漏洞源敏感配置")
}

// maskVulnSourceConfig 脱敏漏洞源配置响应,不返回明文或密文。
func maskVulnSourceConfig(cfg map[string]any) map[string]any {
	return secretmap.Mask(cfg)
}
