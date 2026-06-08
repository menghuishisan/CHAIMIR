// Package audit 的详情序列化边界。
// 审计 detail 最终写入 identity.audit_log JSONB,各业务模块只提供结构化 map。
package audit

import (
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
)

// DetailString 将审计详情转换为稳定 JSON 对象字符串;nil 统一落为空对象。
func DetailString(detail map[string]any) (string, error) {
	data, err := jsonx.ObjectBytes(detail, apperr.ErrInternal)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
