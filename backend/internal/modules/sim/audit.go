// M4 审计辅助:集中定义仿真模块动作码,并经 platform/audit.Writer 写入 M1 audit_log。
package sim

import (
	"context"

	"chaimir/internal/platform/audit"
	"chaimir/pkg/apperr"
)

const (
	auditTargetPackage = "sim.package"
	auditTargetReview  = "sim.review"
	auditTargetSession = "sim.session"
	auditTargetShare   = "sim.share"

	auditActionPackageSubmit  = "sim.package.submit"
	auditActionPackageUpdate  = "sim.package.update"
	auditActionReviewApprove  = "sim.review.approve"
	auditActionReviewReject   = "sim.review.reject"
	auditActionSessionCreate  = "sim.session.create"
	auditActionSessionArchive = "sim.session.archive"
	auditActionShareCreate    = "sim.share.create"
)

// writeAudit 写关键操作审计;缺少审计 writer 时显式失败,避免绕过 audit_log。
func (s *Service) writeAudit(ctx context.Context, tenantID int64, action, targetType string, targetID int64, detail map[string]any) error {
	if s.auditor == nil {
		return apperr.ErrSimAuditFailed
	}
	actorID, actorRole, err := audit.ResolveActor(ctx, s.identity)
	if err != nil {
		return apperr.ErrSimAuditFailed.WithCause(err)
	}
	entry, err := audit.BuildEntry(ctx, tenantID, actorID, actorRole, action, targetType, targetID, detail)
	if err != nil {
		return apperr.ErrSimAuditFailed.WithCause(err)
	}
	if err := s.auditor.Write(ctx, entry); err != nil {
		return apperr.ErrSimAuditFailed.WithCause(err)
	}
	return nil
}
