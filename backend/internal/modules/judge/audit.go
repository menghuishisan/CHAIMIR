// judge audit 文件封装 M3 审计 action 和共享 audit_log 写入。
package judge

import (
	"context"

	"chaimir/internal/platform/audit"
	"chaimir/pkg/apperr"
)

// writeAudit 写入 M1 共享 audit_log,禁止 M3 自建审计表。
func (s *Service) writeAudit(ctx context.Context, tenantID, actorID int64, actorRole int16, action, targetType string, targetID int64, detail map[string]any) error {
	if s.audit == nil {
		return apperr.ErrJudgeAuditFailed
	}
	entry, err := audit.BuildEntry(ctx, tenantID, actorID, actorRole, action, targetType, targetID, detail)
	if err != nil {
		return apperr.ErrJudgeAuditFailed.WithCause(err)
	}
	if err := s.audit.Write(ctx, entry); err != nil {
		return apperr.ErrJudgeAuditFailed.WithCause(err)
	}
	return nil
}

// writeAuditFromContext 使用统一审计 actor 解析能力记录平台或租户用户操作。
func (s *Service) writeAuditFromContext(ctx context.Context, tenantID int64, action, targetType string, targetID int64, detail map[string]any) error {
	actorID, actorRole, err := audit.ResolveActor(ctx, s.identity)
	if err != nil {
		return apperr.ErrJudgeAuditFailed.WithCause(err)
	}
	return s.writeAudit(ctx, tenantID, actorID, actorRole, action, targetType, targetID, detail)
}

// writeSystemAudit 记录内部服务和 worker 触发的系统动作,不依赖 HTTP 用户上下文。
func (s *Service) writeSystemAudit(ctx context.Context, tenantID int64, action, targetType string, targetID int64, detail map[string]any) error {
	return s.writeAudit(ctx, tenantID, 0, audit.ActorRoleSystem, action, targetType, targetID, detail)
}
