// teaching audit 文件封装 M6 审计 action 和共享 audit_log 写入。
package teaching

import (
	"context"

	"chaimir/internal/platform/audit"
	"chaimir/pkg/apperr"
)

// writeAudit 写入 M1 共享 audit_log,禁止 M6 自建审计表。
func (s *Service) writeAudit(ctx context.Context, tenantID, actorID int64, actorRole int16, action, targetType string, targetID int64, detail map[string]any) error {
	entry, err := audit.BuildEntry(ctx, tenantID, actorID, actorRole, action, targetType, targetID, detail)
	if err != nil {
		return apperr.ErrTeachingCourseInvalid.WithCause(err)
	}
	if err := s.audit.Write(ctx, entry); err != nil {
		return apperr.ErrTeachingCourseInvalid.WithCause(err)
	}
	return nil
}
