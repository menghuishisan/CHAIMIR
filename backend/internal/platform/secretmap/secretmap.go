// Package secretmap 统一处理 JSON 配置 map 中敏感字段的加密、脱敏和还原。
package secretmap

import (
	"fmt"
	"strings"

	"chaimir/pkg/crypto"
)

// MaskedValue 是响应中展示已配置敏感值的统一用户向文案。
const MaskedValue = "已配置"

// Protect 递归加密 map 中带凭据语义的字符串字段。
func Protect(cipher *crypto.Cipher, value map[string]any, label string) (map[string]any, error) {
	if value == nil {
		return map[string]any{}, nil
	}
	out := make(map[string]any, len(value))
	for key, raw := range value {
		if IsSensitiveKey(key) {
			text, ok := raw.(string)
			if !ok || strings.TrimSpace(text) == "" {
				out[key] = raw
				continue
			}
			if cipher == nil {
				return nil, fmt.Errorf("%s %s 缺少加密器", label, key)
			}
			encrypted, err := cipher.EncryptString(text)
			if err != nil {
				return nil, err
			}
			out[key] = map[string]any{"encrypted": true, "value": encrypted}
			continue
		}
		if nested, ok := raw.(map[string]any); ok {
			protected, err := Protect(cipher, nested, label)
			if err != nil {
				return nil, err
			}
			out[key] = protected
			continue
		}
		out[key] = raw
	}
	return out, nil
}

// Mask 递归隐藏 map 中带凭据语义的字段,不返回明文或密文。
func Mask(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(value))
	for key, raw := range value {
		if IsSensitiveKey(key) {
			out[key] = MaskedValue
			continue
		}
		if nested, ok := raw.(map[string]any); ok {
			out[key] = Mask(nested)
			continue
		}
		out[key] = raw
	}
	return out
}

// Reveal 递归解密已保护的 map,仅供服务端内部适配器使用。
func Reveal(cipher *crypto.Cipher, value map[string]any, label string) (map[string]any, error) {
	if value == nil {
		return map[string]any{}, nil
	}
	out := make(map[string]any, len(value))
	for key, raw := range value {
		if nested, ok := raw.(map[string]any); ok {
			if encrypted, ok := nested["encrypted"].(bool); ok && encrypted {
				ciphertext, ok := nested["value"].(string)
				if !ok || strings.TrimSpace(ciphertext) == "" {
					return nil, fmt.Errorf("%s %s 缺少密文", label, key)
				}
				if cipher == nil {
					return nil, fmt.Errorf("%s %s 缺少解密器", label, key)
				}
				plain, err := cipher.DecryptString(ciphertext)
				if err != nil {
					return nil, err
				}
				out[key] = plain
				continue
			}
			revealed, err := Reveal(cipher, nested, label)
			if err != nil {
				return nil, err
			}
			out[key] = revealed
			continue
		}
		out[key] = raw
	}
	return out, nil
}

// IsSensitiveKey 判断配置键名是否携带凭据语义。
func IsSensitiveKey(key string) bool {
	normalized := strings.ToLower(key)
	return strings.Contains(normalized, "password") ||
		strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "token") ||
		strings.Contains(normalized, "credential") ||
		strings.Contains(normalized, "authorization") ||
		strings.Contains(normalized, "api_key")
}
