// M1 API 权限辅助测试。
package identity

import (
	"context"
	"os"
	"strings"
	"testing"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
)

// TestAuditAccessAllowsPlatformAdminWithoutRoleLookup 确认平台管理员查询审计时不进入学校角色查询。
func TestAuditAccessAllowsPlatformAdminWithoutRoleLookup(t *testing.T) {
	ctx := tenant.WithContext(context.Background(), tenant.Identity{
		AccountID:  9001,
		IsPlatform: true,
	})
	called := false

	if err := authorizeAuditAccess(ctx, func(_ context.Context, _ int64, _ string) (bool, error) {
		called = true
		return false, nil
	}); err != nil {
		t.Fatalf("platform admin should be allowed: %v", err)
	}
	if called {
		t.Fatalf("platform admin audit access must not query school roles")
	}
}

// TestAuditAccessRequiresSchoolAdminRole 确认租户内审计查询仍要求学校管理员角色。
func TestAuditAccessRequiresSchoolAdminRole(t *testing.T) {
	ctx := tenant.WithContext(context.Background(), tenant.Identity{
		TenantID:  1001,
		AccountID: 2001,
	})

	err := authorizeAuditAccess(ctx, func(_ context.Context, accountID int64, role string) (bool, error) {
		if accountID != 2001 || role != contracts.RoleCode(RoleSchoolAdmin) {
			t.Fatalf("unexpected role lookup account=%d role=%s", accountID, role)
		}
		return false, nil
	})
	if err != apperr.ErrForbidden {
		t.Fatalf("expected forbidden for non-admin tenant account, got %v", err)
	}
}

// TestOrgReadAccessRejectsStudent 确认组织架构只读接口不向学生开放。
func TestOrgReadAccessRejectsStudent(t *testing.T) {
	ctx := tenant.WithContext(context.Background(), tenant.Identity{
		TenantID:  1001,
		AccountID: 2001,
	})

	err := authorizeOrgReadAccess(ctx, func(_ context.Context, accountID int64, role string) (bool, error) {
		if accountID != 2001 {
			t.Fatalf("unexpected role lookup account=%d", accountID)
		}
		return role == contracts.RoleCode(RoleStudent), nil
	})
	if err != apperr.ErrForbidden {
		t.Fatalf("expected forbidden for student org read, got %v", err)
	}
}

// TestOrgReadAccessAllowsTeacher 确认教师可按权限矩阵只读组织架构。
func TestOrgReadAccessAllowsTeacher(t *testing.T) {
	ctx := tenant.WithContext(context.Background(), tenant.Identity{
		TenantID:  1001,
		AccountID: 2001,
	})

	err := authorizeOrgReadAccess(ctx, func(_ context.Context, _ int64, role string) (bool, error) {
		return role == contracts.RoleCode(RoleTeacher), nil
	})
	if err != nil {
		t.Fatalf("teacher org read should be allowed: %v", err)
	}
}

// TestIdentityAPIDoesNotDefineDuplicateRoleGuard 防止 M1 API 保留与 platform/auth 重复的通用角色中间件。
func TestIdentityAPIDoesNotDefineDuplicateRoleGuard(t *testing.T) {
	data, err := os.ReadFile("api.go")
	if err != nil {
		t.Fatalf("read api.go: %v", err)
	}
	if strings.Contains(string(data), "func (a *API) requireRole") {
		t.Fatalf("identity API must reuse platform/auth role guards instead of defining requireRole")
	}
}
