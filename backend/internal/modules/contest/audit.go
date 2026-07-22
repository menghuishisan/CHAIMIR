// contest audit 文件封装 M8 审计动作,统一写入 identity 的 audit_log。
package contest

import (
	"context"

	"chaimir/internal/platform/audit"
	"chaimir/pkg/apperr"
)

const (
	// auditTargetContest 标识竞赛定义审计目标。
	auditTargetContest = "contest"
	// auditTargetContestTeam 标识参赛队伍审计目标。
	auditTargetContestTeam = "contest_team"
	// auditTargetSolveSubmission 标识解题提交审计目标。
	auditTargetSolveSubmission = "solve_submission"
	// auditTargetBattleEntry 标识参战物审计目标。
	auditTargetBattleEntry = "battle_entry"
	// auditTargetBattleMatch 标识对抗对局审计目标。
	auditTargetBattleMatch = "battle_match"
	// auditTargetCheatRecord 标识违规处理审计目标。
	auditTargetCheatRecord = "cheat_record"
	// auditTargetVulnSource 标识漏洞源审计目标。
	auditTargetVulnSource = "vuln_source"
	// auditTargetVulnProblem 标识漏洞题草稿审计目标。
	auditTargetVulnProblem = "vuln_problem"
)

// writeAudit 写入 M8 关键操作审计,详细上下文以 JSON 存入统一审计表。
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
