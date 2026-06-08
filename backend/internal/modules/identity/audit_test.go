// M1 审计辅助测试。
package identity

import (
	"context"
	"encoding/json"
	"testing"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/tenant"
)

// TestBuildAuditEntryUsesTenantIdentityAndRequestIP 确认审计条目从服务端身份与请求上下文生成。
func TestBuildAuditEntryUsesTenantIdentityAndRequestIP(t *testing.T) {
	ctx := tenant.WithContext(context.Background(), tenant.Identity{
		TenantID:  1001,
		AccountID: 2001,
	})
	ctx = audit.WithRequestContext(ctx, audit.RequestContext{IP: "192.0.2.10", TraceID: "trace-001"})

	entry, err := buildAuditEntry(ctx, RoleSchoolAdmin, AuditActionAccountCreate, AuditTargetAccount, 3001, map[string]any{
		"base_identity": BaseIdentityStudent,
	})
	if err != nil {
		t.Fatalf("build audit entry: %v", err)
	}
	if entry.TenantID != 1001 || entry.ActorID != 2001 || entry.ActorRole != RoleSchoolAdmin {
		t.Fatalf("unexpected actor context: %+v", entry)
	}
	if entry.Action != AuditActionAccountCreate || entry.TargetType != AuditTargetAccount || entry.TargetID != 3001 {
		t.Fatalf("unexpected target context: %+v", entry)
	}
	if entry.IP != "192.0.2.10" {
		t.Fatalf("expected request IP in audit entry, got %q", entry.IP)
	}
	if entry.TraceID != "trace-001" {
		t.Fatalf("expected trace id in audit entry, got %q", entry.TraceID)
	}
	var detail map[string]any
	if err := json.Unmarshal([]byte(entry.Detail), &detail); err != nil {
		t.Fatalf("detail should be JSON: %v", err)
	}
	if detail["base_identity"].(float64) != float64(BaseIdentityStudent) {
		t.Fatalf("unexpected audit detail: %s", entry.Detail)
	}
}

// TestBuildAuditLogParamsPreservesTraceID 确认审计落库参数不会丢失 trace_id。
func TestBuildAuditLogParamsPreservesTraceID(t *testing.T) {
	params := buildAuditLogParams(7001, audit.Entry{
		TenantID:   1001,
		ActorID:    2001,
		ActorRole:  RoleSchoolAdmin,
		Action:     AuditActionAccountCreate,
		TargetType: AuditTargetAccount,
		TargetID:   3001,
		Detail:     `{"field":"name"}`,
		IP:         "192.0.2.10",
		TraceID:    "trace-001",
	})
	if params.ID != 7001 || params.TenantID.Int64 != 1001 || !params.TenantID.Valid {
		t.Fatalf("unexpected identity params: %+v", params)
	}
	if params.TraceID.String != "trace-001" || !params.TraceID.Valid {
		t.Fatalf("expected trace id to be preserved, got %+v", params.TraceID)
	}
}

// TestAuditRoleResolutionUsesPlatformAudit 确认 M1 也复用平台统一 actor_role 解析规则。
func TestAuditRoleResolutionUsesPlatformAudit(t *testing.T) {
	role := audit.ActorRoleFromAccount(contracts.AccountInfo{
		BaseIdentity: BaseIdentityTeacher,
		Roles:        []string{contracts.RoleCode(RoleTeacher), contracts.RoleCode(RoleSchoolAdmin)},
	})
	if role != RoleSchoolAdmin {
		t.Fatalf("expected school admin audit role, got %d", role)
	}
}

// TestBuildAccountAuditEntryDoesNotRequireTenantContext 确认登录前流程可按已校验账号显式构造审计。
func TestBuildAccountAuditEntryDoesNotRequireTenantContext(t *testing.T) {
	ctx := audit.WithRequestContext(context.Background(), audit.RequestContext{IP: "192.0.2.10", TraceID: "trace-002"})
	entry, err := buildAccountAuditEntry(ctx, 1001, 2001, RoleTeacher, AuditActionAuthLogin, AuditTargetAuthSession, 3001, nil)
	if err != nil {
		t.Fatalf("build account audit entry: %v", err)
	}
	if entry.TenantID != 1001 || entry.ActorID != 2001 || entry.ActorRole != RoleTeacher {
		t.Fatalf("unexpected explicit actor context: %+v", entry)
	}
	if entry.IP != "192.0.2.10" || entry.TraceID != "trace-002" {
		t.Fatalf("expected request metadata, got %+v", entry)
	}
}

// TestBuildPlatformAuditEntryUsesNullTenant 确认平台级审计不伪造租户 ID。
func TestBuildPlatformAuditEntryUsesNullTenant(t *testing.T) {
	ctx := audit.WithRequestContext(context.Background(), audit.RequestContext{IP: "192.0.2.10", TraceID: "trace-003"})
	entry, err := buildPlatformAuditEntry(ctx, 9001, AuditActionTenantApprove, AuditTargetApplication, 3001, nil)
	if err != nil {
		t.Fatalf("build platform audit entry: %v", err)
	}
	params := buildAuditLogParams(7001, entry)
	if params.TenantID.Valid {
		t.Fatalf("platform audit tenant_id must be NULL, got %+v", params.TenantID)
	}
	if params.ActorRole != RolePlatformAdmin {
		t.Fatalf("expected platform admin actor role, got %d", params.ActorRole)
	}
}

// TestAuditEntryIsPlatformScoped 确认平台级审计按 tenant_id=0 识别,不能误归入学校租户。
func TestAuditEntryIsPlatformScoped(t *testing.T) {
	if !auditEntryIsPlatformScoped(audit.Entry{TenantID: 0}) {
		t.Fatalf("tenant_id=0 should be platform scoped")
	}
	if auditEntryIsPlatformScoped(audit.Entry{TenantID: 1001}) {
		t.Fatalf("tenant audit should not be platform scoped")
	}
}
