// 账号导入校验测试。
package identity

import (
	"strings"
	"testing"
)

// TestValidateImportRowRejectsUnknownTargetType 确认导入目标类型必须显式合法。
func TestValidateImportRowRejectsUnknownTargetType(t *testing.T) {
	msg := validateImportRowBasic(99, ImportRowInput{
		Phone: "13800138000",
		Name:  "张三",
		No:    "S001",
		OrgID: "1",
	}, map[string]bool{})
	if !strings.Contains(msg, "导入类型") {
		t.Fatalf("expected target type error, got %q", msg)
	}
}

// TestValidateImportRowRejectsDuplicatePhoneInFile 确认导入预览阶段拦截文件内重复手机号。
func TestValidateImportRowRejectsDuplicatePhoneInFile(t *testing.T) {
	seen := map[string]bool{"phone:13800138000": true}

	msg := validateImportRowBasic(ImportTargetStudent, ImportRowInput{
		Phone: "13800138000",
		Name:  "李四",
		No:    "S002",
		OrgID: "bad-org",
	}, seen)
	if msg != "文件内手机号重复" {
		t.Fatalf("expected duplicate phone error, got %q", msg)
	}
}
