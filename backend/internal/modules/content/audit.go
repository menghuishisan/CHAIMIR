// content audit 文件封装 M5 审计 action 和共享 audit_log 写入。
package content

import (
	"context"

	"chaimir/internal/platform/audit"
	"chaimir/pkg/apperr"
)

// writeAudit 写入 M1 共享 audit_log,禁止 M5 自建审计表。
func (s *Service) writeAudit(ctx context.Context, tenantID, actorID int64, actorRole int16, action, targetType string, targetID int64, detail map[string]any) error {
	entry, err := audit.BuildEntry(ctx, tenantID, actorID, actorRole, action, targetType, targetID, detail)
	if err != nil {
		return apperr.ErrContentInvalid.WithCause(err)
	}
	if err := s.audit.Write(ctx, entry); err != nil {
		return apperr.ErrContentInvalid.WithCause(err)
	}
	return nil
}
