// Package storage 的对象引用测试覆盖 minio://bucket/key 统一解析规则。
package storage

import "testing"

// TestParseObjectRefAcceptsMinIORefs 确认对象存储引用统一解析出 bucket 与 key。
func TestParseObjectRefAcceptsMinIORefs(t *testing.T) {
	ref, err := ParseObjectRef("minio://chaimir-code/submissions/1.tgz")
	if err != nil {
		t.Fatalf("parse object ref: %v", err)
	}
	if ref.Bucket != "chaimir-code" || ref.Key != "submissions/1.tgz" {
		t.Fatalf("unexpected ref: %#v", ref)
	}
}

// TestParseObjectRefRejectsMalformedRefs 防止模块各自接受不完整对象引用。
func TestParseObjectRefRejectsMalformedRefs(t *testing.T) {
	for _, raw := range []string{"", "s3://bucket/key", "minio://bucket", "minio:///key", "minio://bucket/", "minio://bucket/../escape", "minio://bucket/a//b"} {
		if _, err := ParseObjectRef(raw); err == nil {
			t.Fatalf("expected malformed ref to fail: %q", raw)
		}
	}
}
