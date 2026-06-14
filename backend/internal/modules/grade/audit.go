// grade audit 文件封装 M11 审计 action 和共享 audit_log 写入。
package grade

import (
	"context"

	"chaimir/internal/platform/audit"
	"chaimir/pkg/apperr"
)

const (
	auditTargetGradeReview = "grade.review"
	auditTargetAppeal      = "grade.appeal"
	auditTargetTranscript  = "grade.transcript"
)

// writeAudit 写入 M1 共享 audit_log,禁止 M11 自建审计表或用日志替代审计。
func (s *Service) writeAudit(ctx context.Context, tenantID, actorID int64, actorRole int16, action, targetType string, targetID int64, detail map[string]any) error {
	entry, err := audit.BuildEntry(ctx, tenantID, actorID, actorRole, action, targetType, targetID, detail)
	if err != nil {
		return apperr.ErrGradeAuditFailed.WithCause(err)
	}
	if err := s.audit.Write(ctx, entry); err != nil {
		return apperr.ErrGradeAuditFailed.WithCause(err)
	}
	return nil
}
