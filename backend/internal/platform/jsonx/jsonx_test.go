// Package jsonx 测试 JSONB 边界序列化的统一语义。
package jsonx

import (
	"testing"

	"chaimir/pkg/apperr"
)

// TestObjectBytesUsesEmptyObjectForNil 确认 JSONB 对象空值统一保存为 {},避免各模块自定义空值语义。
func TestObjectBytesUsesEmptyObjectForNil(t *testing.T) {
	data, err := ObjectBytes(nil, apperr.ErrBadRequest)
	if err != nil {
		t.Fatalf("ObjectBytes returned error: %v", err)
	}
	if string(data) != "{}" {
		t.Fatalf("nil object should encode as {}, got %s", data)
	}
}

// TestObjectBytesWrapsMarshalError 确认非法 JSON 对象不会被静默替换为空对象。
func TestObjectBytesWrapsMarshalError(t *testing.T) {
	_, err := ObjectBytes(map[string]any{"bad": make(chan int)}, apperr.ErrBadRequest)
	if err == nil {
		t.Fatalf("expected marshal error")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrBadRequest.Code {
		t.Fatalf("expected bad request wrapper, got %v", err)
	}
}

// TestObjectMapFallsBackToEmptyObject 确认历史脏 JSONB 只在读取边界降级为空对象。
func TestObjectMapFallsBackToEmptyObject(t *testing.T) {
	got := ObjectMap([]byte(`not-json`))
	if len(got) != 0 {
		t.Fatalf("invalid JSONB should become empty object, got %#v", got)
	}
}

// TestObjectMapStrictReturnsDecodeError 确认安全配置等强校验场景不会把坏 JSON 降级为空对象。
func TestObjectMapStrictReturnsDecodeError(t *testing.T) {
	_, err := ObjectMapStrict([]byte(`not-json`))
	if err == nil {
		t.Fatalf("expected decode error")
	}
}

// TestCloneObjectCreatesIndependentJSONCopy 确认 JSON 对象深拷贝不会共享嵌套结构。
func TestCloneObjectCreatesIndependentJSONCopy(t *testing.T) {
	source := map[string]any{"nested": map[string]any{"answer": "old"}}

	clone := CloneObject(source)
	clone["nested"].(map[string]any)["answer"] = "new"

	if source["nested"].(map[string]any)["answer"] != "old" {
		t.Fatalf("clone mutation leaked into source: %#v", source)
	}
}

// TestEqualNormalizesJSONShapes 确认结构化比较统一处理可 JSON 化的 map/slice 值。
func TestEqualNormalizesJSONShapes(t *testing.T) {
	left := map[string]any{"step": "deploy", "values": []any{float64(1), "ok"}}
	right := map[string]any{"values": []any{float64(1), "ok"}, "step": "deploy"}

	if !Equal(left, right) {
		t.Fatalf("expected semantically equal JSON values")
	}
	if Equal(left, map[string]any{"step": "deploy", "values": []any{float64(2), "ok"}}) {
		t.Fatalf("expected different JSON values to be rejected")
	}
}

// TestDecodeStrictReturnsDecodeError 确认服务端持久化流程读取坏 JSON 时不会静默降级。
func TestDecodeStrictReturnsDecodeError(t *testing.T) {
	var out map[string]any
	if err := DecodeStrict([]byte(`not-json`), &out); err == nil {
		t.Fatalf("expected strict decode error")
	}
}
