// 竞赛审计经 M1 写入；学生写操作沿用主办学校事务并保留真实身份。
package contest

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
)

const (
	auditTargetContest         = "contest"
	auditTargetContestTeam     = "contest_team"
	auditTargetSolveSubmission = "solve_submission"
	auditTargetBattleEntry     = "battle_entry"
	auditTargetBattleMatch     = "battle_match"
	auditTargetCheatRecord     = "cheat_record"
	auditTargetVulnSource      = "vuln_source"
	auditTargetVulnProblem     = "vuln_problem"
)

// writeAudit 统一补齐请求上下文，并保留审计写入失败。
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

// writeStudentAudit 必须在主办学校事务内调用，操作者学校只从服务端身份取得。
func (s *Service) writeStudentAudit(ctx context.Context, organizerTenantID int64, actor tenant.Identity, action, targetType string, targetID int64, detail map[string]any) error {
	if detail == nil {
		detail = make(map[string]any)
	}
	detail["actor_tenant_id"] = actor.TenantID
	return s.writeAudit(ctx, organizerTenantID, actor.AccountID, contracts.RoleNumStudent, action, targetType, targetID, detail)
}
