// Package audit 测试审计 actor 解析边界,保证共享审计能力返回精确错误语义。
package audit

import (
	"context"
	"errors"
	"testing"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
)

// TestResolveActorReturnsAuditSpecificError 确认账号摘要读取失败不会退回通用内部错误码。
func TestResolveActorReturnsAuditSpecificError(t *testing.T) {
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 2001})

	_, _, err := ResolveActor(ctx, failingIdentityReader{})
	if err == nil {
		t.Fatalf("expected actor resolve error")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrAuditActorResolveFailed.Code {
		t.Fatalf("expected audit actor resolve error, got %v", err)
	}
}

// TestActorRoleFromAccountUsesContractsRoles 确认审计角色解析遵循 contracts 稳定角色码。
func TestActorRoleFromAccountUsesContractsRoles(t *testing.T) {
	role := ActorRoleFromAccount(contracts.AccountInfo{Roles: []string{contracts.RoleSchoolAdmin}})
	if role != ActorRoleSchoolAdmin {
		t.Fatalf("expected school admin role, got %d", role)
	}
	role = ActorRoleFromAccount(contracts.AccountInfo{Roles: []string{contracts.RoleTeacher}})
	if role != ActorRoleTeacher {
		t.Fatalf("expected teacher role, got %d", role)
	}
}

type failingIdentityReader struct{}

func (failingIdentityReader) GetAccount(context.Context, int64) (contracts.AccountInfo, error) {
	return contracts.AccountInfo{}, errors.New("identity unavailable")
}
