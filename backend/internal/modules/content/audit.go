// content audit 文件封装 M5 审计 action 和共享 audit_log 写入。
package content

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
)

// auditActorSnapshot 仅由已鉴权入口构造，不替代业务归属校验。
type auditActorSnapshot struct {
	tenantID  int64
	accountID int64
	role      int16
}

// writeAudit 写入 M1 共享 audit_log,禁止 M5 自建审计表。
func (s *Service) writeAudit(ctx context.Context, tenantID, actorID int64, actorRole int16, action, targetType string, targetID int64, detail map[string]any) error {
	entry, err := audit.BuildEntry(ctx, tenantID, actorID, actorRole, action, targetType, targetID, detail)
	if err != nil {
		return apperr.ErrAuditWriteFailed.WithCause(err)
	}
	if err := s.audit.Write(ctx, entry); err != nil {
		return apperr.ErrAuditWriteFailed.WithCause(err)
	}
	return nil
}

// resolveAuditActor 在路由鉴权后读取一次身份摘要，供本次操作的审计归属使用。
func resolveAuditActor(ctx context.Context, identity audit.IdentityReader) (auditActorSnapshot, error) {
	id, ok := tenant.FromContext(ctx)
	if !ok || id.TenantID <= 0 || id.AccountID <= 0 || id.IsSystem || id.IsPlatform || identity == nil {
		return auditActorSnapshot{}, apperr.ErrAuditActorResolveFailed
	}
	account, err := identity.GetAccount(ctx, id.AccountID)
	if err != nil {
		return auditActorSnapshot{}, apperr.ErrAuditActorResolveFailed.WithCause(err)
	}
	if account.AccountID != id.AccountID || account.TenantID != id.TenantID {
		return auditActorSnapshot{}, apperr.ErrAuditActorResolveFailed
	}
	role := audit.ActorRoleFromAccount(account)
	if role != contracts.RoleNumTeacher && role != contracts.RoleNumSchoolAdmin {
		return auditActorSnapshot{}, apperr.ErrForbidden
	}
	return auditActorSnapshot{tenantID: id.TenantID, accountID: account.AccountID, role: role}, nil
}

// validate 防止主体快照与当前请求身份错配，不重复查询 M1。
func (actor auditActorSnapshot) validate(identity tenant.Identity) error {
	if identity.TenantID <= 0 || identity.IsSystem || identity.IsPlatform || actor.tenantID != identity.TenantID || actor.accountID <= 0 || actor.accountID != identity.AccountID || (actor.role != contracts.RoleNumTeacher && actor.role != contracts.RoleNumSchoolAdmin) {
		return apperr.ErrAuditActorResolveFailed
	}
	return nil
}
