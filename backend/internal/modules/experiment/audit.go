// M7 审计写入:统一经 platform/audit 写入 M1 audit_log。
package experiment

import (
	"context"

	"chaimir/internal/platform/audit"
	"chaimir/pkg/apperr"
)

const (
	auditActionExperimentCreate    = "experiment.definition.create"
	auditActionExperimentUpdate    = "experiment.definition.update"
	auditActionExperimentPublish   = "experiment.definition.publish"
	auditActionExperimentUnpublish = "experiment.definition.unpublish"
	auditActionInstanceFinish      = "experiment.instance.finish"
	auditActionInstanceRecycle     = "experiment.instance.recycle"
	auditTargetExperiment          = "experiment.definition"
	auditTargetInstance            = "experiment.instance"
)

// writeAudit 记录成功业务操作审计。
func (s *Service) writeAudit(ctx context.Context, tenantID int64, action, targetType string, targetID int64, detail map[string]any) error {
	if s.auditor == nil {
		return apperr.ErrExperimentAuditFailed
	}
	actorID, actorRole, err := audit.ResolveActor(ctx, s.identity)
	if err != nil {
		return apperr.ErrExperimentAuditFailed.WithCause(err)
	}
	entry, err := audit.BuildEntry(ctx, tenantID, actorID, actorRole, action, targetType, targetID, detail)
	if err != nil {
		return apperr.ErrExperimentAuditFailed.WithCause(err)
	}
	if err := s.auditor.Write(ctx, entry); err != nil {
		return apperr.ErrExperimentAuditFailed.WithCause(err)
	}
	return nil
}
