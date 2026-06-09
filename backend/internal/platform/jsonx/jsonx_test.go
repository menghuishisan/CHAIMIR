// jsonx_test 校验平台统一 JSON 边界语义,避免模块各自发明空值与解析规则。
package jsonx

import (
	"testing"

	"chaimir/pkg/apperr"
)

// TestObjectBytesUsesEmptyObjectForNil 确认 JSON 对象空值统一编码为 {}。
func TestObjectBytesUsesEmptyObjectForNil(t *testing.T) {
	data, err := ObjectBytes(nil, apperr.ErrBadRequest)
	if err != nil {
		t.Fatalf("ObjectBytes returned error: %v", err)
	}
	if string(data) != "{}" {
		t.Fatalf("nil object should encode as {}, got %s", data)
	}
}

// TestObjectBytesWrapsMarshalError 确认非法对象不会被静默替换为空对象。
func TestObjectBytesWrapsMarshalError(t *testing.T) {
	_, err := ObjectBytes(map[string]any{"bad": make(chan int)}, apperr.ErrBadRequest)
	if err == nil {
		t.Fatalf("expected marshal error")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrBadRequest.Code {
		t.Fatalf("expected bad request wrapper, got %v", err)
	}
}

// TestObjectMapFallsBackToEmptyObject 确认宽松读取在脏 JSON 下返回空对象。
func TestObjectMapFallsBackToEmptyObject(t *testing.T) {
	got := ObjectMap([]byte(`not-json`))
	if len(got) != 0 {
		t.Fatalf("invalid JSON should become empty object, got %#v", got)
	}
}

// TestObjectMapStrictReturnsDecodeError 确认强校验读取不会吞掉解析错误。
func TestObjectMapStrictReturnsDecodeError(t *testing.T) {
	_, err := ObjectMapStrict([]byte(`not-json`))
	if err == nil {
		t.Fatalf("expected decode error")
	}
}

// TestCloneObjectCreatesIndependentJSONCopy 确认深拷贝不会共享嵌套结构。
func TestCloneObjectCreatesIndependentJSONCopy(t *testing.T) {
	source := map[string]any{"nested": map[string]any{"answer": "old"}}
	clone := CloneObject(source)
	clone["nested"].(map[string]any)["answer"] = "new"
	if source["nested"].(map[string]any)["answer"] != "old" {
		t.Fatalf("clone mutation leaked into source: %#v", source)
	}
}

// TestEqualNormalizesJSONShapes 确认结构化比较基于 JSON 语义而不是 map 顺序。
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

// TestDecodeStrictReturnsDecodeError 确认严格解码在坏 JSON 下返回错误。
func TestDecodeStrictReturnsDecodeError(t *testing.T) {
	var out map[string]any
	if err := DecodeStrict([]byte(`not-json`), &out); err == nil {
		t.Fatalf("expected strict decode error")
	}
}

// TestScalarReadersNormalizeJSONValues 确认标量读取辅助使用统一宽松规则。
func TestScalarReadersNormalizeJSONValues(t *testing.T) {
	if got := StringFromAny(float64(12.5)); got != "12.5" {
		t.Fatalf("StringFromAny(float64) = %q, want 12.5", got)
	}
	if got := IntFromAny("42"); got != 42 {
		t.Fatalf("IntFromAny(string) = %d, want 42", got)
	}
	if got := Float64FromAny(int32(7)); got != 7 {
		t.Fatalf("Float64FromAny(int32) = %v, want 7", got)
	}
	if got := Int32FromAny("bad", 9); got != 9 {
		t.Fatalf("Int32FromAny(invalid) = %d, want default 9", got)
	}
}

// TestObjectAndPathReadersNormalizeJSONMaps 确认对象、数组和点路径读取逻辑集中在平台层。
func TestObjectAndPathReadersNormalizeJSONMaps(t *testing.T) {
	root := map[string]any{"a": map[string]any{"b": float64(3)}, "headers": map[string]any{"X-Test": 1}}
	if got := StringFromPath(root, "a.b"); got != "3" {
		t.Fatalf("StringFromPath() = %q, want 3", got)
	}
	if got := ObjectFromAny(root["a"]); got["b"] != float64(3) {
		t.Fatalf("ObjectFromAny() = %#v", got)
	}
	if got := StringMapFromAny(root["headers"]); got["X-Test"] != "1" {
		t.Fatalf("StringMapFromAny() = %#v", got)
	}
	if got := SliceFromAny([]any{"x"}); len(got) != 1 || got[0] != "x" {
		t.Fatalf("SliceFromAny() = %#v", got)
	}
}
