// sim audit 文件封装 M4 审计 action 和共享 audit_log 写入。
package sim

import (
	"context"

	"chaimir/internal/platform/audit"
	"chaimir/pkg/apperr"
)

// writeAudit 写入 M1 共享 audit_log,禁止 M4 自建审计表。
func (s *Service) writeAudit(ctx context.Context, tenantID, actorID int64, actorRole int16, action, targetType string, targetID int64, detail map[string]any) error {
	if s.audit == nil {
		return apperr.ErrSimSessionStateInvalid
	}
	detailText, err := audit.DetailString(detail)
	if err != nil {
		return apperr.ErrSimSessionStateInvalid.WithCause(err)
	}
	req := audit.RequestContextFrom(ctx)
	if err := s.audit.Write(ctx, audit.Entry{TenantID: tenantID, ActorID: actorID, ActorRole: actorRole, Action: action, TargetType: targetType, TargetID: targetID, Detail: detailText, IP: req.IP, TraceID: req.TraceID}); err != nil {
		return apperr.ErrSimSessionStateInvalid.WithCause(err)
	}
	return nil
}
