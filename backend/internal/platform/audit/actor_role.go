// audit 统一审计 actor_role 解析与条目构造,避免模块各自维护角色映射。
package audit

import (
	"context"
	"fmt"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/response"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
)

const (
	// ActorRolePlatformAdmin 表示平台管理员。
	ActorRolePlatformAdmin = contracts.RoleNumPlatformAdmin
	// ActorRoleSchoolAdmin 表示学校管理员。
	ActorRoleSchoolAdmin = contracts.RoleNumSchoolAdmin
	// ActorRoleTeacher 表示教师。
	ActorRoleTeacher = contracts.RoleNumTeacher
	// ActorRoleStudent 表示学生。
	ActorRoleStudent = contracts.RoleNumStudent
	// ActorRoleSystem 表示已通过服务签名的内部系统任务。
	ActorRoleSystem int16 = 5
)

// IdentityReader 是解析审计主体时所需的最小只读身份契约。
type IdentityReader interface {
	// GetAccount 读取指定账号摘要,用于确定审计角色。
	GetAccount(context.Context, int64) (contracts.AccountInfo, error)
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

// ActorRoleFromAccount 按统一优先级把账号摘要映射到审计角色。
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

// BuildEntry 构造统一审计条目,补齐 detail、ip 和 trace_id 等横切字段。
func BuildEntry(ctx context.Context, tenantID, actorID int64, actorRole int16, action, targetType string, targetID int64, detail map[string]any) (Entry, error) {
	action = strings.TrimSpace(action)
	targetType = strings.TrimSpace(targetType)
	if action == "" || targetType == "" {
		return Entry{}, fmt.Errorf("审计条目缺少 action 或 target_type")
	}
	detailText, err := DetailString(detail)
	if err != nil {
		return Entry{}, err
	}
	req := RequestContextFrom(ctx)
	if strings.TrimSpace(req.TraceID) == "" {
		req.TraceID = response.TraceFromContext(ctx)
	}
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
