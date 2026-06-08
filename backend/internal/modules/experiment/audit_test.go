// M7 审计测试:确保实验模块统一写审计且角色来自服务端身份。
package experiment

import (
	"context"
	"testing"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
)

// TestWriteAuditRequiresConfiguredWriter 确认未注入审计 writer 时显式失败。
func TestWriteAuditRequiresConfiguredWriter(t *testing.T) {
	svc := &Service{}
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 2001})

	err := svc.writeAudit(ctx, 1001, auditActionExperimentCreate, auditTargetExperiment, 3001, map[string]any{"name": "exp"})
	if err == nil {
		t.Fatalf("expected missing auditor to fail")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrExperimentAuditFailed.Code {
		t.Fatalf("expected experiment audit error, got %v", err)
	}
}

// TestWriteAuditRecordsActorRoleFromIdentity 确认实验审计记录写入正确 actor_role。
func TestWriteAuditRecordsActorRoleFromIdentity(t *testing.T) {
	writer := &captureExperimentAuditWriter{}
	svc := &Service{
		auditor:  writer,
		identity: &experimentAuditIdentity{account: contracts.AccountInfo{AccountID: 2001, TenantID: 1001, BaseIdentity: 2, Roles: []string{"teacher"}}},
	}
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 2001})

	if err := svc.writeAudit(ctx, 1001, auditActionExperimentCreate, auditTargetExperiment, 3001, map[string]any{"name": "exp"}); err != nil {
		t.Fatalf("writeAudit returned error: %v", err)
	}
	if len(writer.entries) != 1 {
		t.Fatalf("expected one audit entry, got %d", len(writer.entries))
	}
	if writer.entries[0].ActorRole != 3 {
		t.Fatalf("expected teacher actor role, got %d", writer.entries[0].ActorRole)
	}
}

type captureExperimentAuditWriter struct {
	entries []audit.Entry
}

func (w *captureExperimentAuditWriter) Write(_ context.Context, entry audit.Entry) error {
	w.entries = append(w.entries, entry)
	return nil
}

type experimentAuditIdentity struct {
	account contracts.AccountInfo
}

func (f *experimentAuditIdentity) GetAccount(context.Context, int64) (contracts.AccountInfo, error) {
	return f.account, nil
}

func (f *experimentAuditIdentity) BatchGetAccounts(context.Context, []int64) ([]contracts.AccountInfo, error) {
	return nil, nil
}

func (f *experimentAuditIdentity) HasRole(_ context.Context, _ int64, role string) (bool, error) {
	for _, actual := range f.account.Roles {
		if actual == role {
			return true, nil
		}
	}
	return false, nil
}
