package sandbox

import (
	"context"

	"chaimir/internal/platform/audit"
	"chaimir/pkg/apperr"
)

func (s *Service) writeAudit(ctx context.Context, tenantID, actorID int64, actorRole int16, action, targetType string, targetID int64, detail map[string]any) error {
	entry, err := audit.BuildEntry(ctx, tenantID, actorID, actorRole, action, targetType, targetID, detail)
	if err != nil {
		return apperr.ErrSandboxAuditFailed.WithCause(err)
	}
	if err := s.audit.Write(ctx, entry); err != nil {
		return apperr.ErrSandboxAuditFailed.WithCause(err)
	}
	return nil
}

func (s *Service) writeAuditFromContext(ctx context.Context, tenantID int64, action, targetType string, targetID int64, detail map[string]any) error {
	actorID, actorRole, err := audit.ResolveActor(ctx, s.identity)
	if err != nil {
		return apperr.ErrSandboxAuditFailed.WithCause(err)
	}
	return s.writeAudit(ctx, tenantID, actorID, actorRole, action, targetType, targetID, detail)
}

// writeSystemAudit 记录后台任务或服务签名任务产生的系统动作,避免依赖 HTTP 用户上下文。
func (s *Service) writeSystemAudit(ctx context.Context, tenantID int64, action, targetType string, targetID int64, detail map[string]any) error {
	return s.writeAudit(ctx, tenantID, 0, audit.ActorRoleSystem, action, targetType, targetID, detail)
}
