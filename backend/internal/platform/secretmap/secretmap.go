// secretmap 统一处理 JSON 配置 map 中敏感字段的加密、脱敏和还原。
package secretmap

import (
	"fmt"
	"strings"

	"chaimir/pkg/crypto"
	"chaimir/pkg/privacy"
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
		if privacy.IsCredentialKey(key) {
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
		protected, err := protectValue(cipher, raw, label)
		if err != nil {
			return nil, err
		}
		out[key] = protected
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
		if privacy.IsCredentialKey(key) {
			out[key] = MaskedValue
			continue
		}
		out[key] = maskValue(raw)
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
		revealed, err := revealValue(cipher, raw, label, key)
		if err != nil {
			return nil, err
		}
		out[key] = revealed
	}
	return out, nil
}

// protectValue 递归处理任意 JSON 值,让数组中的配置对象也复用同一敏感字段加密口径。
func protectValue(cipher *crypto.Cipher, raw any, label string) (any, error) {
	switch v := raw.(type) {
	case map[string]any:
		return Protect(cipher, v, label)
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			protected, err := protectValue(cipher, item, label)
			if err != nil {
				return nil, err
			}
			out = append(out, protected)
		}
		return out, nil
	default:
		return raw, nil
	}
}

// maskValue 递归脱敏任意 JSON 值,确保响应体中数组里的凭据字段也不会泄露。
func maskValue(raw any) any {
	switch v := raw.(type) {
	case map[string]any:
		return Mask(v)
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, maskValue(item))
		}
		return out
	default:
		return raw
	}
}

// revealValue 递归还原任意 JSON 值,仅在服务端内部适配器需要明文时使用。
func revealValue(cipher *crypto.Cipher, raw any, label, key string) (any, error) {
	switch v := raw.(type) {
	case map[string]any:
		if encrypted, ok := v["encrypted"].(bool); ok && encrypted {
			ciphertext, ok := v["value"].(string)
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
			return plain, nil
		}
		return Reveal(cipher, v, label)
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			revealed, err := revealValue(cipher, item, label, key)
			if err != nil {
				return nil, err
			}
			out = append(out, revealed)
		}
		return out, nil
	default:
		return raw, nil
	}
}
