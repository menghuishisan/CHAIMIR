// experiment audit 文件封装 M7 审计动作,统一写入 identity 的 audit_log。
package experiment

import (
	"context"

	"chaimir/internal/platform/audit"
	"chaimir/pkg/apperr"
	"chaimir/pkg/response"
)

const (
	// auditTargetExperiment 标识实验定义审计目标。
	auditTargetExperiment = "experiment"
	// auditTargetInstance 标识实验实例审计目标。
	auditTargetInstance = "experiment_instance"
	// auditTargetGroup 标识实验小组审计目标。
	auditTargetGroup = "experiment_group"
	// auditTargetReport 标识实验报告审计目标。
	auditTargetReport = "experiment_report"
)

// writeAudit 写入 M7 关键操作审计,详细上下文以 JSON 存入统一审计表。
func (s *Service) writeAudit(ctx context.Context, tenantID, actorID int64, actorRole int16, action, targetType string, targetID int64, detail map[string]any) error {
	if s.audit == nil {
		return apperr.ErrAuditWriterMissing
	}
	if detail == nil {
		detail = map[string]any{}
	}
	detailText, err := audit.DetailString(detail)
	if err != nil {
		return apperr.ErrInternal.WithCause(err)
	}
	req := audit.RequestContextFrom(ctx)
	traceID := req.TraceID
	if traceID == "" {
		traceID = response.TraceFromContext(ctx)
	}
	return s.audit.Write(ctx, audit.Entry{TenantID: tenantID, ActorID: actorID, ActorRole: actorRole, Action: action, TargetType: targetType, TargetID: targetID, Detail: detailText, IP: req.IP, TraceID: traceID})
}
