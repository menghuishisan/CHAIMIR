// Package storage 测试对象存储基础能力的安全边界。
package storage

import "testing"

// TestObjectKeyRejectsUnsafeSegments 确认对象 key 由安全段组成,避免跨租户/跨资源命名空间混淆。
func TestObjectKeyRejectsUnsafeSegments(t *testing.T) {
	cases := [][]string{
		{"sandbox", "code", "../escape"},
		{"sandbox", "code", "a/b"},
		{"sandbox", "code", ""},
		{"sandbox", "code", "."},
		{"sandbox", "code", ".."},
	}
	for _, tc := range cases {
		if key, err := ObjectKey(1, tc[0], tc[1], tc[2]); err == nil {
			t.Fatalf("unsafe object key segment should fail, got %q", key)
		}
	}
}

// TestObjectKeyBuildsTenantScopedKey 确认安全段会生成统一租户前缀对象 key。
func TestObjectKeyBuildsTenantScopedKey(t *testing.T) {
	key, err := ObjectKey(42, "sandbox", "code", "1001")
	if err != nil {
		t.Fatalf("object key: %v", err)
	}
	if key != "42/sandbox/code/1001" {
		t.Fatalf("unexpected object key: %q", key)
	}
}
