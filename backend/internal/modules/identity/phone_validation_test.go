// M1 手机号格式校验测试。
package identity

import (
	"strings"
	"testing"
)

// TestValidCNPhoneAcceptsMainlandMobile 确认国内高校主登录手机号按中国大陆手机号格式校验。
func TestValidCNPhoneAcceptsMainlandMobile(t *testing.T) {
	if !validCNPhone("13800138000") {
		t.Fatalf("expected mainland mobile phone to be valid")
	}
}

// TestValidCNPhoneRejectsInvalidPrefix 确认 11 位但非手机号号段不能通过。
func TestValidCNPhoneRejectsInvalidPrefix(t *testing.T) {
	if validCNPhone("12800138000") {
		t.Fatalf("expected invalid mobile prefix to be rejected")
	}
}

// TestValidateImportRowRejectsInvalidPhonePrefix 确认导入校验不会只按长度判断手机号。
func TestValidateImportRowRejectsInvalidPhonePrefix(t *testing.T) {
	msg := validateImportRowBasic(ImportTargetStudent, ImportRowInput{
		Phone: "12800138000",
		Name:  "张三",
		No:    "S001",
		OrgID: "1",
	}, map[string]bool{})
	if !strings.Contains(msg, "手机号格式") {
		t.Fatalf("expected phone format error, got %q", msg)
	}
}
