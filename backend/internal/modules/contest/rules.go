// M8 业务规则:竞赛状态机、报名窗口、提交校验、ELO 结算与漏洞题门禁。
package contest

import (
	"errors"
	"math"
	"strings"
	"time"

	"chaimir/internal/platform/ids"
	"chaimir/pkg/apperr"
)

// validateContestRequest 校验竞赛基础字段和赛程顺序。
func validateContestRequest(req ContestRequest) error {
	if strings.TrimSpace(req.Name) == "" {
		return apperr.ErrContestInvalid
	}
	if req.Mode != ContestModeSolve && req.Mode != ContestModeBattle {
		return apperr.ErrContestInvalid
	}
	if req.TeamMode != TeamModeSolo && req.TeamMode != TeamModeTeam {
		return apperr.ErrContestInvalid
	}
	if req.Mode == ContestModeBattle && req.MatchMode != MatchModeRoundRobin && req.MatchMode != MatchModeElo {
		return apperr.ErrContestInvalid
	}
	if req.SignupStart.IsZero() || req.SignupEnd.IsZero() || req.StartAt.IsZero() || req.EndAt.IsZero() {
		return apperr.ErrContestInvalid
	}
	if !req.SignupStart.Before(req.SignupEnd) || !req.SignupEnd.Before(req.StartAt) || !req.StartAt.Before(req.EndAt) {
		return apperr.ErrContestInvalid.WithCause(errors.New("contest schedule order is invalid"))
	}
	if req.FreezeMinutes < 0 {
		return apperr.ErrContestInvalid
	}
	return nil
}

// validateContestTransition 校验竞赛生命周期流转。
func validateContestTransition(from, to int16) error {
	allowed := map[int16][]int16{
		ContestStatusDraft:   {ContestStatusSignup},
		ContestStatusSignup:  {ContestStatusRunning},
		ContestStatusRunning: {ContestStatusFrozen, ContestStatusEnded},
		ContestStatusFrozen:  {ContestStatusEnded},
		ContestStatusEnded:   {ContestStatusArchived},
	}
	for _, candidate := range allowed[from] {
		if candidate == to {
			return nil
		}
	}
	return apperr.ErrContestState
}

// validateProblemRequest 校验竞赛题目引用与分值。
func validateProblemRequest(req ContestProblemRequest) error {
	if strings.TrimSpace(req.ItemCode) == "" || strings.TrimSpace(req.ItemVersion) == "" || req.Score <= 0 || req.Seq <= 0 {
		return apperr.ErrContestProblem
	}
	return nil
}

// validateSignupWindow 确认当前处于报名状态和报名时间内。
func validateSignupWindow(contest ContestDTO, now time.Time) error {
	if contest.Status != ContestStatusSignup {
		return apperr.ErrContestSignupClosed
	}
	if now.Before(contest.SignupStart) || now.After(contest.SignupEnd) {
		return apperr.ErrContestSignupClosed
	}
	return nil
}

// validateJoinTeamRequest 校验邀请码入队请求。
func validateJoinTeamRequest(req JoinTeamRequest) error {
	if strings.TrimSpace(req.InviteCode) == "" {
		return apperr.ErrContestTeamInvalid
	}
	return nil
}

// validateSolveSubmitRequest 校验解题提交最小信息。
func validateSolveSubmitRequest(req SolveSubmitRequest) error {
	if ids.ParseOrZero(req.TeamID) == 0 || strings.TrimSpace(req.JudgerCode) == "" {
		return apperr.ErrContestSubmissionInvalid
	}
	return nil
}

// validateBattleEntryRequest 校验参战物提交边界。
func validateBattleEntryRequest(req BattleEntryRequest) error {
	if ids.ParseOrZero(req.TeamID) == 0 || strings.TrimSpace(req.ArtifactRef) == "" {
		return apperr.ErrContestBattleInvalid
	}
	if req.Role != BattleRoleStrategy && req.Role != BattleRoleDefense && req.Role != BattleRoleAttack {
		return apperr.ErrContestBattleInvalid
	}
	return nil
}

// validateBattleResult 校验对局结算输入。
func validateBattleResult(result BattleMatchResult) error {
	if result.ContestID <= 0 || result.EntryAID <= 0 || result.EntryBID <= 0 || result.EntryAID == result.EntryBID {
		return apperr.ErrContestBattleInvalid
	}
	if result.Result != BattleResultAWin && result.Result != BattleResultBWin && result.Result != BattleResultDraw {
		return apperr.ErrContestBattleInvalid
	}
	if strings.TrimSpace(result.ReplayRef) == "" {
		return apperr.ErrContestBattleInvalid
	}
	return nil
}

// eloDelta 根据胜负和当前分计算 ELO 变化。
func eloDelta(scoreA, scoreB float64, result int16) (float64, float64) {
	const k = 32.0
	expectedA := 1 / (1 + math.Pow(10, (scoreB-scoreA)/400))
	actualA := 0.5
	if result == BattleResultAWin {
		actualA = 1
	}
	if result == BattleResultBWin {
		actualA = 0
	}
	deltaA := k * (actualA - expectedA)
	return deltaA, -deltaA
}

// validateVulnSourceRequest 校验漏洞源配置请求。
func validateVulnSourceRequest(req VulnSourceRequest) error {
	if strings.TrimSpace(req.Name) == "" || req.Type <= 0 || req.DefaultLevel < VulnLevelA || req.DefaultLevel > VulnLevelC {
		return apperr.ErrContestVulnSourceInvalid
	}
	return nil
}

// validateVulnProblemImport 校验漏洞题草稿导入请求。
func validateVulnProblemImport(req VulnProblemImportRequest) error {
	if strings.TrimSpace(req.Title) == "" || req.Level < VulnLevelA || req.Level > VulnLevelC {
		return apperr.ErrContestVulnProblemInvalid
	}
	if req.RuntimeMode != VulnRuntimeIsolated && req.RuntimeMode != VulnRuntimeForked {
		return apperr.ErrContestVulnProblemInvalid
	}
	return nil
}

// validateVulnFinalizeGate 确认漏洞题已通过预验证且尚未固化。
func validateVulnFinalizeGate(problem VulnProblemDTO) error {
	if problem.Status != VulnProblemDraft {
		return apperr.ErrContestVulnProblemInvalid
	}
	if problem.PrevalidateStatus != VulnPrevalidatePassed {
		return apperr.ErrContestVulnPrevalidate
	}
	return nil
}
