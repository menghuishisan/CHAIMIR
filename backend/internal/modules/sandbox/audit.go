// sandbox audit 文件封装 M2 审计动作,统一写入 identity 的 audit_log。
package sandbox

import (
	"context"
	"encoding/json"

	"chaimir/internal/platform/audit"
	"chaimir/pkg/apperr"
)

// writeAudit 写入沙箱关键操作审计,不得自建审计表。
func (s *Service) writeAudit(ctx context.Context, tenantID, actorID int64, actorRole int16, action, targetType string, targetID int64, detail map[string]any) error {
	if detail == nil {
		detail = map[string]any{}
	}
	raw, err := json.Marshal(detail)
	if err != nil {
		return apperr.ErrSandboxAuditFailed.WithCause(err)
	}
	req := audit.RequestContextFrom(ctx)
	if err := s.audit.Write(ctx, audit.Entry{
		TenantID:   tenantID,
		ActorID:    actorID,
		ActorRole:  actorRole,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Detail:     string(raw),
		IP:         req.IP,
		TraceID:    req.TraceID,
	}); err != nil {
		return apperr.ErrSandboxAuditFailed.WithCause(err)
	}
	return nil
}
