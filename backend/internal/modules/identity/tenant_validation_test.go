// M1 租户与配置枚举校验测试。
package identity

import (
	"testing"

	"chaimir/pkg/apperr"
)

// TestValidateTenantStatusRejectsUnknownStatus 确认平台租户状态更新不能写入未知状态。
func TestValidateTenantStatusRejectsUnknownStatus(t *testing.T) {
	if err := validateTenantStatus(99); err != apperr.ErrTenantStatusInvalid {
		t.Fatalf("expected bad request for invalid tenant status, got %v", err)
	}
}

// TestValidateAuthModeRejectsUnknownMode 确认学校认证方式只能使用文档定义枚举。
func TestValidateAuthModeRejectsUnknownMode(t *testing.T) {
	if err := validateAuthMode(99); err != apperr.ErrTenantAuthModeInvalid {
		t.Fatalf("expected bad request for invalid auth mode, got %v", err)
	}
}

// TestValidateSchoolTypeRejectsUnknownType 确认入驻申请学校类型只能使用文档定义枚举。
func TestValidateSchoolTypeRejectsUnknownType(t *testing.T) {
	if err := validateSchoolType(99); err != apperr.ErrSchoolTypeInvalid {
		t.Fatalf("expected bad request for invalid school type, got %v", err)
	}
}
