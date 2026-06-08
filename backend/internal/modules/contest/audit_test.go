// M8 审计测试:确保竞赛模块不会静默绕过审计,并正确记录角色。
package contest

import (
	"context"
	"testing"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
)

// TestWriteAuditRequiresConfiguredWriter 确认竞赛审计缺少 writer 时显式失败。
func TestWriteAuditRequiresConfiguredWriter(t *testing.T) {
	svc := &Service{}
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 2001})

	err := svc.writeAudit(ctx, 1001, auditActionContestCreate, auditTargetContest, 3001, map[string]any{"name": "contest"})
	if err == nil {
		t.Fatalf("expected missing auditor to fail")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrContestAuditFailed.Code {
		t.Fatalf("expected contest audit error, got %v", err)
	}
}

// TestWriteAuditRecordsActorRoleFromIdentity 确认竞赛审计记录写入正确 actor_role。
func TestWriteAuditRecordsActorRoleFromIdentity(t *testing.T) {
	writer := &captureContestAuditWriter{}
	svc := &Service{
		auditor:  writer,
		identity: &contestAuditIdentity{account: contracts.AccountInfo{AccountID: 2001, TenantID: 1001, BaseIdentity: 2, Roles: []string{"teacher"}}},
	}
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 2001})

	if err := svc.writeAudit(ctx, 1001, auditActionContestCreate, auditTargetContest, 3001, map[string]any{"name": "contest"}); err != nil {
		t.Fatalf("writeAudit returned error: %v", err)
	}
	if len(writer.entries) != 1 {
		t.Fatalf("expected one audit entry, got %d", len(writer.entries))
	}
	if writer.entries[0].ActorRole != 3 {
		t.Fatalf("expected teacher actor role, got %d", writer.entries[0].ActorRole)
	}
}

type captureContestAuditWriter struct {
	entries []audit.Entry
}

func (w *captureContestAuditWriter) Write(_ context.Context, entry audit.Entry) error {
	w.entries = append(w.entries, entry)
	return nil
}

type contestAuditIdentity struct {
	account contracts.AccountInfo
}

func (f *contestAuditIdentity) GetAccount(context.Context, int64) (contracts.AccountInfo, error) {
	return f.account, nil
}

func (f *contestAuditIdentity) BatchGetAccounts(context.Context, []int64) ([]contracts.AccountInfo, error) {
	return nil, nil
}

func (f *contestAuditIdentity) HasRole(_ context.Context, _ int64, role string) (bool, error) {
	for _, actual := range f.account.Roles {
		if actual == role {
			return true, nil
		}
	}
	return false, nil
}
