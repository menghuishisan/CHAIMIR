// M3 审计辅助:集中定义评测模块动作码,并经 platform/audit.Writer 写入 M1 audit_log。
package judge

import (
	"context"

	"chaimir/internal/platform/audit"
	"chaimir/pkg/apperr"
)

const (
	auditTargetJudger = "judge.judger"
	auditTargetTask   = "judge.task"
	auditTargetResult = "judge.result"

	auditActionJudgerCreate   = "judge.judger.create"
	auditActionJudgerUpdate   = "judge.judger.update"
	auditActionJudgerSelftest = "judge.judger.selftest"
	auditActionTaskSubmit     = "judge.task.submit"
	auditActionTaskComplete   = "judge.task.complete"
	auditActionTaskFailed     = "judge.task.failed"
	auditActionTaskCancel     = "judge.task.cancel"
	auditActionTaskRejudge    = "judge.task.rejudge"
	auditActionManualScore    = "judge.result.manual_score"
)

// writeAudit 写关键操作审计;缺少审计 writer 时显式失败,避免绕过 audit_log。
func (s *Service) writeAudit(ctx context.Context, tenantID int64, action, targetType string, targetID int64, detail map[string]any) error {
	if s.auditor == nil {
		return apperr.ErrJudgeAuditFail
	}
	actorID, actorRole, err := audit.ResolveActor(ctx, s.identity)
	if err != nil {
		return apperr.ErrJudgeAuditFail.WithCause(err)
	}
	entry, err := audit.BuildEntry(ctx, tenantID, actorID, actorRole, action, targetType, targetID, detail)
	if err != nil {
		return apperr.ErrJudgeAuditFail.WithCause(err)
	}
	if err := s.auditor.Write(ctx, entry); err != nil {
		return apperr.ErrJudgeAuditFail.WithCause(err)
	}
	return nil
}
