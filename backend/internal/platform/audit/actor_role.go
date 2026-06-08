// Package audit 统一审计角色解析与条目构造,避免各模块各写一套 actor_role 逻辑。
package audit

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
)

const (
	// ActorRolePlatformAdmin 表示平台管理员角色。
	ActorRolePlatformAdmin = contracts.RoleNumPlatformAdmin
	// ActorRoleSchoolAdmin 表示学校管理员角色。
	ActorRoleSchoolAdmin = contracts.RoleNumSchoolAdmin
	// ActorRoleTeacher 表示教师角色。
	ActorRoleTeacher = contracts.RoleNumTeacher
	// ActorRoleStudent 表示学生角色。
	ActorRoleStudent = contracts.RoleNumStudent
	// ActorRoleSystem 表示已通过服务签名的内部系统任务。
	ActorRoleSystem int16 = 5
)

// IdentityReader 是审计角色解析依赖的最小 M1 只读契约。
type IdentityReader interface {
	GetAccount(context.Context, int64) (contracts.AccountInfo, error)
}

// ResolveActor 从服务端租户身份与 M1 账号摘要解析操作者 ID 与审计角色。
func ResolveActor(ctx context.Context, identity IdentityReader) (int64, int16, error) {
	id, ok := tenant.FromContext(ctx)
	if !ok {
		return 0, 0, apperr.ErrUnauthorized
	}
	if id.IsPlatform {
		return id.AccountID, ActorRolePlatformAdmin, nil
	}
	if identity == nil {
		return 0, 0, apperr.ErrAuditActorResolveFailed
	}
	account, err := identity.GetAccount(ctx, id.AccountID)
	if err != nil {
		return 0, 0, apperr.ErrAuditActorResolveFailed.WithCause(err)
	}
	return id.AccountID, ActorRoleFromAccount(account), nil
}

// ActorRoleFromAccount 按 M1 角色优先级选择 audit_log.actor_role。
func ActorRoleFromAccount(account contracts.AccountInfo) int16 {
	for _, code := range account.Roles {
		if code == contracts.RoleSchoolAdmin {
			return ActorRoleSchoolAdmin
		}
	}
	for _, code := range account.Roles {
		if code == contracts.RoleTeacher {
			return ActorRoleTeacher
		}
	}
	if account.BaseIdentity == 2 {
		return ActorRoleTeacher
	}
	return ActorRoleStudent
}

// BuildEntry 构造统一审计条目并完成 detail/IP/trace_id 映射。
func BuildEntry(ctx context.Context, tenantID, actorID int64, actorRole int16, action, targetType string, targetID int64, detail map[string]any) (Entry, error) {
	detailText, err := DetailString(detail)
	if err != nil {
		return Entry{}, err
	}
	req := RequestContextFrom(ctx)
	return Entry{
		TenantID:   tenantID,
		ActorID:    actorID,
		ActorRole:  actorRole,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Detail:     detailText,
		IP:         req.IP,
		TraceID:    req.TraceID,
	}, nil
}
