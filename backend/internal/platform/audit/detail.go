// audit 统一审计详情的 JSON 序列化边界,保证 detail 写入语义稳定。
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
