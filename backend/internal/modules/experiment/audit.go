package experiment

import (
	"context"

	"chaimir/internal/platform/audit"
	"chaimir/pkg/apperr"
)

const (
	auditTargetExperiment = "experiment"
	auditTargetInstance   = "experiment_instance"
	auditTargetGroup      = "experiment_group"
	auditTargetReport     = "experiment_report"
)

func (s *Service) writeAudit(ctx context.Context, tenantID, actorID int64, actorRole int16, action, targetType string, targetID int64, detail map[string]any) error {
	if s.audit == nil {
		return apperr.ErrAuditWriterMissing
	}
	entry, err := audit.BuildEntry(ctx, tenantID, actorID, actorRole, action, targetType, targetID, detail)
	if err != nil {
		return apperr.ErrInternal.WithCause(err)
	}
	return s.audit.Write(ctx, entry)
}
