// Package jsonx 统一 JSON 编码、解码与标量转换。
package jsonx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"reflect"
	"strconv"
	"strings"

	"chaimir/internal/platform/intx"
	"chaimir/pkg/apperr"
)

// RawMessage 返回输入 JSON 字节的隔离副本,供模块边界安全传递原始 JSON。
func RawMessage(data []byte) json.RawMessage {
	if len(data) == 0 {
		return nil
	}
	out := make([]byte, len(data))
	copy(out, data)
	return json.RawMessage(out)
}

// EncodeLineBytes 将结构化输入编码为一行 JSON,用于受控命令 stdin。
func EncodeLineBytes(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(true)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ObjectBytes 将 JSON 对象序列化为数据库字节;nil 对象统一按空对象保存。
func ObjectBytes(v map[string]any, marshalErr *apperr.Error) ([]byte, error) {
	if v == nil {
		return []byte("{}"), nil
	}
	return AnyBytes(v, marshalErr)
}

// AnyBytes 将任意 JSON 结构序列化为数据库字节;nil 统一按空对象保存。
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

// ObjectMapStrict 将 JSON 对象字节解析为 map;配置等强校验场景保留解析错误。
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

// DecodeStrict 解析 JSON 到指定目标;强校验读取时直接返回错误。
func DecodeStrict(data []byte, out any) error {
	if len(data) == 0 {
		return nil
	}
	return decodeStrict(data, out, false)
}

// DecodeStrictKnownFields 解析 JSON 到指定目标,并拒绝结构体中未声明的字段。
func DecodeStrictKnownFields(data []byte, out any) error {
	if len(data) == 0 {
		return nil
	}
	return decodeStrict(data, out, true)
}

// Valid 判断输入是否为合法 JSON,用于只需要结构合法性、不需要落地解析的边界校验。
func Valid(data []byte) bool {
	return json.Valid(data)
}

// CloneObjectStrict 对 JSON 对象做深拷贝;不可序列化时显式返回错误。
func CloneObjectStrict(in map[string]any) (map[string]any, error) {
	if in == nil {
		return map[string]any{}, nil
	}
	data, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	return ObjectMapStrict(data)
}

// Equal 按 JSON 结构语义比较两个值,避免 map 顺序和具体实现影响比较结果。
func Equal(left, right any) bool {
	return reflect.DeepEqual(normalize(left), normalize(right))
}

// StringFromAny 把 JSON 标量转换为字符串表示,无效值返回空字符串。
func StringFromAny(v any) string {
	if text, ok := stringScalar(v); ok {
		return text
	}
	return ""
}

// StringValue 只接受 JSON 字符串标量,其他类型一律返回空字符串。
func StringValue(v any) string {
	text, _ := v.(string)
	return text
}

// StringField 按键读取 JSON 对象中的字符串标量,键缺失或类型不符返回空字符串。
func StringField(obj map[string]any, key string) string {
	return StringValue(obj[key])
}

// IntFromAny 把 JSON 数字或数字字符串转换为 int,无效值返回 0。
func IntFromAny(v any) int {
	if n, ok := int64Scalar(v); ok && (strconv.IntSize == 64 || (n >= math.MinInt32 && n <= math.MaxInt32)) {
		return int(n)
	}
	return 0
}

// Int32FromAny 把 JSON 数字或数字字符串转换为 int32,无效值返回默认值。
func Int32FromAny(v any, defaultValue int32) int32 {
	if n, ok := int64Scalar(v); ok {
		converted, fits := intx.Int64ToInt32(n)
		if !fits {
			return defaultValue
		}
		return converted
	}
	return defaultValue
}

// Int16FromAny 把 JSON 数字或数字字符串转换为 int16,无效值返回默认值。
func Int16FromAny(v any, defaultValue int16) int16 {
	if n, ok := int64Scalar(v); ok {
		converted, fits := intx.Int64ToInt16(n)
		if !fits {
			return defaultValue
		}
		return converted
	}
	return defaultValue
}

// Int64FromAny 把 JSON 数字或数字字符串转换为 int64,无效值返回默认值。
func Int64FromAny(v any, defaultValue int64) int64 {
	if n, ok := int64Scalar(v); ok {
		return n
	}
	return defaultValue
}

// Float64FromAnyOK 把 JSON 数字或数字字符串转换为 float64,并返回是否成功。
func Float64FromAnyOK(v any) (float64, bool) {
	return float64Scalar(v)
}

// stringScalar 把平台允许的 JSON 标量规整为字符串,集中维护数字到文本的格式规则。
func stringScalar(v any) (string, bool) {
	switch val := v.(type) {
	case string:
		return val, true
	case int:
		return strconv.Itoa(val), true
	case int16:
		return strconv.FormatInt(int64(val), 10), true
	case int32:
		return strconv.FormatInt(int64(val), 10), true
	case int64:
		return strconv.FormatInt(val, 10), true
	case float32:
		return strconv.FormatFloat(float64(val), 'f', -1, 32), true
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64), true
	case json.Number:
		return val.String(), true
	default:
		return "", false
	}
}

func int64Scalar(v any) (int64, bool) {
	switch val := v.(type) {
	case int:
		return int64(val), true
	case int16:
		return int64(val), true
	case int32:
		return int64(val), true
	case int64:
		return val, true
	case float32:
		return int64Scalar(float64(val))
	case float64:
		// 2^63 恰可由 float64 表示，但超出了 int64 上界。
		if math.Trunc(val) != val || val < -0x1p63 || val >= 0x1p63 {
			return 0, false
		}
		return int64(val), true
	case json.Number:
		return int64Scalar(val.String())
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64)
		return n, err == nil
	default:
		return 0, false
	}
}

// float64Scalar 把平台允许的 JSON 数字或数字字符串转换为 float64,并返回是否成功。
func float64Scalar(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, isFiniteFloat64(val)
	case float32:
		converted := float64(val)
		return converted, isFiniteFloat64(converted)
	case int:
		return float64(val), true
	case int16:
		return float64(val), true
	case int32:
		return float64(val), true
	case int64:
		return float64(val), true
	case json.Number:
		parsed, err := val.Float64()
		return parsed, err == nil && isFiniteFloat64(parsed)
	case string:
		n, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
		return n, err == nil && isFiniteFloat64(n)
	default:
		return 0, false
	}
}

// isFiniteFloat64 判断浮点值是否可安全参与范围比较和持久化。
func isFiniteFloat64(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
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
		part = strings.TrimSpace(part)
		if part == "" {
			return nil
		}
		obj, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = obj[part]
	}
	return current
}

// StringFromPath 按点路径读取 JSON 标量字符串。
func StringFromPath(root map[string]any, path string) string {
	return StringFromAny(ValueFromPath(root, path))
}

// StringMapFromAny 把 JSON 对象转换为字符串映射,空键和空值会被丢弃。
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

// decodeStrict 使用标准 decoder 统一强校验 JSON,拒绝未知字段可选,并拒绝尾随非空 JSON token。
func decodeStrict(data []byte, out any, knownFields bool) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	if knownFields {
		dec.DisallowUnknownFields()
	}
	if err := dec.Decode(out); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("JSON 包含尾随内容")
		}
		return err
	}
	return nil
}
