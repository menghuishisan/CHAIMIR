// M9 配置值保护:敏感配置加密存储,对外响应与历史统一脱敏。
package admin

import (
	"chaimir/internal/platform/secretmap"
	"chaimir/pkg/crypto"
)

// protectConfigValue 递归加密配置值中的敏感字段。
func protectConfigValue(cipher *crypto.Cipher, value map[string]any) (map[string]any, error) {
	return secretmap.Protect(cipher, value, "敏感配置")
}

// maskConfigs 对配置列表逐项脱敏。
func maskConfigs(rows []ConfigDTO) []ConfigDTO {
	out := make([]ConfigDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, maskConfig(row))
	}
	return out
}

// maskConfig 对单项配置脱敏。
func maskConfig(row ConfigDTO) ConfigDTO {
	row.Value = maskConfigValue(row.Value)
	return row
}

// maskConfigHistory 对配置历史中的 old/new 值脱敏。
func maskConfigHistory(rows []ConfigChangeLogDTO) []ConfigChangeLogDTO {
	out := make([]ConfigChangeLogDTO, 0, len(rows))
	for _, row := range rows {
		row.OldValue = maskConfigValue(row.OldValue)
		row.NewValue = maskConfigValue(row.NewValue)
		out = append(out, row)
	}
	return out
}

// maskConfigValue 递归隐藏敏感配置值。
func maskConfigValue(value map[string]any) map[string]any {
	return secretmap.Mask(value)
}
