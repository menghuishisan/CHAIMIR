// Package jsonx 统一平台 JSONB 边界处理,避免业务模块各自定义空值和错误语义。
package jsonx

import (
	"encoding/json"
	"reflect"

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

// ObjectMap 将 JSONB 对象字节解析为 map;历史脏数据只在展示边界降级为空对象。
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
