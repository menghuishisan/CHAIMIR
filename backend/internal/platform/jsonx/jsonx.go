// Package jsonx 统一平台 JSONB 边界处理,避免业务模块各自定义空值和错误语义。
package jsonx

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"

	"chaimir/pkg/apperr"
)

// ObjectBytes 将 JSONB 对象序列化为数据库字节;nil 对象按空对象保存。
func ObjectBytes(v map[string]any, marshalErr *apperr.Error) ([]byte, error) {
	if v == nil {
		return []byte("{}"), nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil, marshalErr.WithCause(err)
	}
	return data, nil
}

// AnyBytes 将任意 JSONB 结构序列化为数据库字节;nil 按空对象保存。
func AnyBytes(v any, marshalErr *apperr.Error) ([]byte, error) {
	if v == nil {
		return []byte("{}"), nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil, marshalErr.WithCause(err)
	}
	return data, nil
}

// ObjectMap 将 JSONB 对象字节解析为 map;展示读取遇到历史脏数据时返回空对象。
func ObjectMap(data []byte) map[string]any {
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil || out == nil {
		return map[string]any{}
	}
	return out
}

// ObjectMapStrict 将 JSONB 对象字节解析为 map;配置等强校验场景需要保留解析错误。
func ObjectMapStrict(data []byte) (map[string]any, error) {
	if len(data) == 0 {
		return map[string]any{}, nil
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	if out == nil {
		return map[string]any{}, nil
	}
	return out, nil
}

// Decode 解析 JSONB 到指定类型,读取边界失败时返回调用方提供的零值。
func Decode[T any](data []byte, zeroValue T) T {
	if len(data) == 0 {
		return zeroValue
	}
	var out T
	if err := json.Unmarshal(data, &out); err != nil {
		return zeroValue
	}
	return out
}

// DecodeStrict 解析 JSON 到调用方传入的目标;持久化流程需要把坏数据作为错误返回。
func DecodeStrict(data []byte, out any) error {
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, out)
}

// CloneObject 对 JSON 对象做深拷贝;不可序列化或空对象按空对象返回,供业务克隆边界复用。
func CloneObject(in map[string]any) map[string]any {
	data, err := json.Marshal(in)
	if err != nil {
		return map[string]any{}
	}
	return ObjectMap(data)
}

// Equal 按 JSON 结构语义比较两个值,避免各模块各自用 JSON 往返归一化。
func Equal(left, right any) bool {
	return reflect.DeepEqual(normalize(left), normalize(right))
}

// StringFromAny 把 JSON 标量转为字符串,非标量或空值返回空字符串。
func StringFromAny(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(val), 'f', -1, 32)
	case int:
		return strconv.Itoa(val)
	case int16:
		return strconv.FormatInt(int64(val), 10)
	case int32:
		return strconv.FormatInt(int64(val), 10)
	case int64:
		return strconv.FormatInt(val, 10)
	default:
		return ""
	}
}

// IntFromAny 把 JSON 数字或数字字符串转为 int,无效值返回 0。
func IntFromAny(v any) int {
	switch val := v.(type) {
	case float64:
		return int(val)
	case float32:
		return int(val)
	case int:
		return val
	case int16:
		return int(val)
	case int32:
		return int(val)
	case int64:
		return int(val)
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(val))
		if err != nil {
			return 0
		}
		return n
	default:
		return 0
	}
}

// Int32FromAny 把 JSON 数字或数字字符串转为 int32,无效值返回调用方给出的默认值。
func Int32FromAny(v any, defaultValue int32) int32 {
	if v == nil {
		return defaultValue
	}
	switch val := v.(type) {
	case int32:
		return val
	case int:
		return int32(val)
	case int16:
		return int32(val)
	case int64:
		return int32(val)
	case float64:
		return int32(val)
	case float32:
		return int32(val)
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(val), 10, 32)
		if err != nil {
			return defaultValue
		}
		return int32(n)
	default:
		return defaultValue
	}
}

// Float64FromAny 把 JSON 数字或数字字符串转为 float64,无效值返回 0。
func Float64FromAny(v any) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int16:
		return float64(val)
	case int32:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		n, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
		if err != nil {
			return 0
		}
		return n
	default:
		return 0
	}
}

// ObjectFromAny 把 JSON 值归一为对象,不匹配时返回空对象。
func ObjectFromAny(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

// SliceFromAny 把 JSON 值归一为数组,不匹配时返回空数组。
func SliceFromAny(v any) []any {
	if s, ok := v.([]any); ok {
		return s
	}
	return []any{}
}

// ValueFromPath 按点路径从 JSON 对象读取值,路径不存在时返回 nil。
func ValueFromPath(root map[string]any, path string) any {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	var current any = root
	for _, part := range strings.Split(path, ".") {
		obj, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = obj[strings.TrimSpace(part)]
	}
	return current
}

// StringFromPath 按点路径读取 JSON 标量字符串值。
func StringFromPath(root map[string]any, path string) string {
	return StringFromAny(ValueFromPath(root, path))
}

// StringMapFromAny 把 JSON 对象转为字符串映射,空键和空值会被丢弃。
func StringMapFromAny(v any) map[string]string {
	raw, ok := v.(map[string]any)
	if !ok {
		return map[string]string{}
	}
	out := make(map[string]string, len(raw))
	for key, value := range raw {
		s := strings.TrimSpace(StringFromAny(value))
		if strings.TrimSpace(key) != "" && s != "" {
			out[key] = s
		}
	}
	return out
}

// normalize 通过 JSON 往返把结构化值归一到标准 map/slice/number 表示。
func normalize(v any) any {
	data, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		return v
	}
	return out
}
