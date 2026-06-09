// storage_test 校验对象存储 key 生成的租户边界与安全段规则。
package storage

import "testing"

// TestObjectKeyRejectsUnsafeSegments 确认对象 key 由安全段组成,避免命名空间逃逸。
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

// TestTenantQuotaAllowUpload 确认统一文件服务会在上传前校验租户文件配额。
func TestTenantQuotaAllowUpload(t *testing.T) {
	quota := TenantQuota{
		MaxFiles:  2,
		MaxBytes:  100,
		UsedFiles: 1,
		UsedBytes: 40,
	}
	if err := quota.AllowUpload(1, 50); err != nil {
		t.Fatalf("expected upload within quota to pass: %v", err)
	}
}

// TestTenantQuotaRejectsFileCountOverflow 确认租户文件数超过上限会被统一拒绝。
func TestTenantQuotaRejectsFileCountOverflow(t *testing.T) {
	quota := TenantQuota{
		MaxFiles:  2,
		MaxBytes:  100,
		UsedFiles: 2,
		UsedBytes: 40,
	}
	if err := quota.AllowUpload(1, 10); err == nil {
		t.Fatalf("expected file count overflow to fail")
	}
}

// TestTenantQuotaRejectsByteOverflow 确认租户文件字节数超过上限会被统一拒绝。
func TestTenantQuotaRejectsByteOverflow(t *testing.T) {
	quota := TenantQuota{
		MaxFiles:  2,
		MaxBytes:  100,
		UsedFiles: 1,
		UsedBytes: 90,
	}
	if err := quota.AllowUpload(1, 11); err == nil {
		t.Fatalf("expected byte overflow to fail")
	}
}
