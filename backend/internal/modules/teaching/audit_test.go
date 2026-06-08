// M6 审计测试:确保教学高敏感操作不会绕过统一审计,且 actor_role 来自服务端身份。
package teaching

import (
	"context"
	"testing"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
)

// TestWriteAuditRequiresConfiguredWriter 确认未注入审计 writer 时显式失败,避免静默绕过 audit_log。
func TestWriteAuditRequiresConfiguredWriter(t *testing.T) {
	svc := &Service{}
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 2001})

	err := svc.writeAudit(ctx, 1001, auditActionCourseCreate, auditTargetCourse, 3001, map[string]any{"name": "course"})
	if err == nil {
		t.Fatalf("expected missing auditor to fail")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrTeachingAuditFailed.Code {
		t.Fatalf("expected teaching audit error, got %v", err)
	}
}

// TestWriteAuditRecordsActorRoleFromIdentity 确认 M6 审计角色来自 M1 返回的服务端角色。
func TestWriteAuditRecordsActorRoleFromIdentity(t *testing.T) {
	writer := &captureTeachingAuditWriter{}
	svc := &Service{
		auditor:  writer,
		identity: &teachingAuditIdentity{account: contracts.AccountInfo{AccountID: 2001, TenantID: 1001, BaseIdentity: 2, Roles: []string{"teacher"}}},
	}
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 2001})

	if err := svc.writeAudit(ctx, 1001, auditActionCourseCreate, auditTargetCourse, 3001, map[string]any{"name": "course"}); err != nil {
		t.Fatalf("writeAudit returned error: %v", err)
	}
	if len(writer.entries) != 1 {
		t.Fatalf("expected one audit entry, got %d", len(writer.entries))
	}
	if writer.entries[0].ActorRole != 3 {
		t.Fatalf("expected teacher actor role, got %d", writer.entries[0].ActorRole)
	}
}

type captureTeachingAuditWriter struct {
	entries []audit.Entry
}

func (w *captureTeachingAuditWriter) Write(_ context.Context, entry audit.Entry) error {
	w.entries = append(w.entries, entry)
	return nil
}

type teachingAuditIdentity struct {
	account contracts.AccountInfo
}

func (f *teachingAuditIdentity) GetAccount(context.Context, int64) (contracts.AccountInfo, error) {
	return f.account, nil
}

func (f *teachingAuditIdentity) BatchGetAccounts(context.Context, []int64) ([]contracts.AccountInfo, error) {
	return nil, nil
}

func (f *teachingAuditIdentity) HasRole(_ context.Context, _ int64, role string) (bool, error) {
	for _, actual := range f.account.Roles {
		if actual == role {
			return true, nil
		}
	}
	return false, nil
}
