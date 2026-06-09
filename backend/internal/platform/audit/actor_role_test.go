// audit_test 校验统一审计角色解析边界,保证共享审计能力返回精确错误语义。
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

// TestResolveActorReturnsSystemRoleForSignedService 确认内部服务签名上下文会统一解析为系统任务审计角色。
func TestResolveActorReturnsSystemRoleForSignedService(t *testing.T) {
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, IsSystem: true})

	actorID, actorRole, err := ResolveActor(ctx, nil)
	if err != nil {
		t.Fatalf("resolve actor: %v", err)
	}
	if actorID != 0 || actorRole != ActorRoleSystem {
		t.Fatalf("unexpected system actor identity: id=%d role=%d", actorID, actorRole)
	}
}

type failingIdentityReader struct{}

// GetAccount 返回测试用失败,用于验证错误语义。
func (failingIdentityReader) GetAccount(context.Context, int64) (contracts.AccountInfo, error) {
	return contracts.AccountInfo{}, errors.New("identity unavailable")
}
