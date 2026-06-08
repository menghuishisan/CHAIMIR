// M1 租户状态测试:确保所有认证入口统一遵守租户停用/到期规则。
package identity

import (
	"testing"

	"chaimir/internal/modules/identity/internal/sqlcgen"
	"chaimir/pkg/apperr"
)

// TestEnsureTenantLoginAllowedRejectsDisabledTenant 确认停用租户不能继续登录。
func TestEnsureTenantLoginAllowedRejectsDisabledTenant(t *testing.T) {
	err := ensureTenantLoginAllowed(TenantDisabled)
	if err != apperr.ErrTenantDisabled {
		t.Fatalf("expected disabled tenant login to fail, got %v", err)
	}
}

// TestEnsureTenantLoginAllowedAcceptsActiveTenant 确认正常租户允许认证流程继续。
func TestEnsureTenantLoginAllowedAcceptsActiveTenant(t *testing.T) {
	if err := ensureTenantLoginAllowed(TenantActive); err != nil {
		t.Fatalf("expected active tenant to be allowed, got %v", err)
	}
}

// TestSelectedTenantRequiresActiveStatus 确认手机号多租户选择结果携带并校验租户状态。
func TestSelectedTenantRequiresActiveStatus(t *testing.T) {
	selected := sqlcgen.FindAccountsByPhoneAllTenantsRow{
		AccountID:    1001,
		TenantID:     2001,
		TenantStatus: TenantDisabled,
	}

	if err := ensureSelectedTenantLoginAllowed(selected); err != apperr.ErrTenantDisabled {
		t.Fatalf("expected selected disabled tenant to fail, got %v", err)
	}
}
