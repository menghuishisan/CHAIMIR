// M8 审计写入:统一经 platform/audit 写入 M1 audit_log。
package contest

import (
	"context"

	"chaimir/internal/platform/audit"
	"chaimir/pkg/apperr"
)

const (
	auditActionContestCreate  = "contest.create"
	auditActionContestUpdate  = "contest.update"
	auditActionContestPublish = "contest.publish"
	auditActionContestStart   = "contest.start"
	auditActionContestEnd     = "contest.end"
	auditActionContestArchive = "contest.archive"
	auditActionCheatRecord    = "contest.cheat.record"
	auditTargetContest        = "contest"
)

// writeAudit 记录成功业务操作审计。
func (s *Service) writeAudit(ctx context.Context, tenantID int64, action, targetType string, targetID int64, detail map[string]any) error {
	if s.auditor == nil {
		return apperr.ErrContestAuditFailed
	}
	actorID, actorRole, err := audit.ResolveActor(ctx, s.identity)
	if err != nil {
		return apperr.ErrContestAuditFailed.WithCause(err)
	}
	entry, err := audit.BuildEntry(ctx, tenantID, actorID, actorRole, action, targetType, targetID, detail)
	if err != nil {
		return apperr.ErrContestAuditFailed.WithCause(err)
	}
	if err := s.auditor.Write(ctx, entry); err != nil {
		return apperr.ErrContestAuditFailed.WithCause(err)
	}
	return nil
}
