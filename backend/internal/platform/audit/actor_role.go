package audit

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
)

const (
	// ActorRoleSystem 表示已通过服务签名的内部系统任务。
	ActorRoleSystem int16 = 5
)

// IdentityReader 是解析审计主体时所需的最小只读身份契约。
type IdentityReader interface {
	// GetAccount 读取指定账号摘要,用于确定审计角色。
	GetAccount(context.Context, int64, int64) (contracts.AccountInfo, error)
}

// ResolveActor 从服务端身份上下文与账号摘要解析操作者 ID 和审计角色。
func ResolveActor(ctx context.Context, identity IdentityReader) (int64, int16, error) {
	id, ok := tenant.FromContext(ctx)
	if !ok {
		return 0, 0, apperr.ErrUnauthorized
	}
	if id.IsSystem {
		return 0, ActorRoleSystem, nil
	}
	if id.IsPlatform {
		return id.AccountID, contracts.RoleNumPlatformAdmin, nil
	}
	if identity == nil {
		return 0, 0, apperr.ErrAuditActorResolveFailed
	}
	account, err := identity.GetAccount(ctx, id.TenantID, id.AccountID)
	if err != nil {
		return 0, 0, apperr.ErrAuditActorResolveFailed.WithCause(err)
	}
	return id.AccountID, ActorRoleFromAccount(account), nil
}

// ActorRoleFromAccount 按统一优先级把账号摘要映射到审计角色。
func ActorRoleFromAccount(account contracts.AccountInfo) int16 {
	for _, code := range account.Roles {
		if code == contracts.RoleSchoolAdmin {
			return contracts.RoleNumSchoolAdmin
		}
	}
	for _, code := range account.Roles {
		if code == contracts.RoleTeacher {
			return contracts.RoleNumTeacher
		}
	}
	if account.BaseIdentity == 2 {
		return contracts.RoleNumTeacher
	}
	return contracts.RoleNumStudent
}
