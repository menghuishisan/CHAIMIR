// M2 审计规则测试:确保跨模块审计写入遵循 M1 audit_log 的角色语义。
package sandbox

import (
	"testing"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
)

// TestAuditRoleFromAccountPrefersMostPrivilegedRole 确认审计角色来自 identity contracts 返回的服务端角色。
func TestAuditRoleFromAccountPrefersMostPrivilegedRole(t *testing.T) {
	role := audit.ActorRoleFromAccount(contracts.AccountInfo{Roles: []string{"student", "school_admin"}})
	if role != audit.ActorRoleSchoolAdmin {
		t.Fatalf("expected school admin audit role, got %d", role)
	}
}

// TestAuditRoleFromAccountDefaultsToBaseIdentity 确认账号未带附加角色时使用服务端基础身份。
func TestAuditRoleFromAccountDefaultsToBaseIdentity(t *testing.T) {
	role := audit.ActorRoleFromAccount(contracts.AccountInfo{BaseIdentity: 2})
	if role != audit.ActorRoleTeacher {
		t.Fatalf("expected teacher audit role from base identity, got %d", role)
	}
}
