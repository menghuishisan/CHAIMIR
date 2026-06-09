// M5 审计测试:确认题库模块审计失败使用本模块错误码并保留原始原因。
package content

import (
	"context"
	"errors"
	"testing"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
)

// TestWriteAuditRequiresModuleSpecificError 确认缺少审计 writer 时不会退回通用内部错误码。
func TestWriteAuditRequiresModuleSpecificError(t *testing.T) {
	svc := &Service{}
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 2001})

	err := svc.writeAudit(ctx, 1001, auditActionItemCreate, auditTargetItem, 3001, map[string]any{"code": "p1"})

	ae, ok := apperr.As(err)
	if !ok {
		t.Fatalf("expected app error, got %T %v", err, err)
	}
	if ae.Code != apperr.ErrContentAuditFailed.Code {
		t.Fatalf("expected content audit code %s, got %s", apperr.ErrContentAuditFailed.Code, ae.Code)
	}
	if ae.Code == apperr.ErrInternal.Code {
		t.Fatalf("content audit failure must not use generic internal error")
	}
}

// TestWriteAuditWrapsWriterFailure 确认 audit_log 写入失败会映射为 M5 审计错误并保留 cause。
func TestWriteAuditWrapsWriterFailure(t *testing.T) {
	cause := errors.New("audit store unavailable")
	svc := &Service{
		auditor: failingContentAuditWriter{err: cause},
		identity: contentAuditIdentity{account: contracts.AccountInfo{
			AccountID: 2001,
			TenantID:  1001,
			Roles:     []string{contracts.RoleTeacher},
		}},
	}
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 2001})

	err := svc.writeAudit(ctx, 1001, auditActionItemCreate, auditTargetItem, 3001, map[string]any{"code": "p1"})

	ae, ok := apperr.As(err)
	if !ok {
		t.Fatalf("expected app error, got %T %v", err, err)
	}
	if ae.Code != apperr.ErrContentAuditFailed.Code {
		t.Fatalf("expected content audit code %s, got %s", apperr.ErrContentAuditFailed.Code, ae.Code)
	}
	if !errors.Is(ae, cause) {
		t.Fatalf("audit failure should preserve writer cause")
	}
}

type failingContentAuditWriter struct {
	err error
}

func (w failingContentAuditWriter) Write(context.Context, audit.Entry) error {
	return w.err
}

type contentAuditIdentity struct {
	account contracts.AccountInfo
	err     error
}

func (i contentAuditIdentity) GetAccount(context.Context, int64) (contracts.AccountInfo, error) {
	if i.err != nil {
		return contracts.AccountInfo{}, i.err
	}
	return i.account, nil
}

func (i contentAuditIdentity) BatchGetAccounts(context.Context, []int64) ([]contracts.AccountInfo, error) {
	return nil, nil
}

func (i contentAuditIdentity) HasRole(context.Context, int64, string) (bool, error) {
	return false, nil
}
