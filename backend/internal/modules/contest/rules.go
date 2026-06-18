// contest rules 文件集中放置 M8 输入校验、状态机、限频和来源引用规则。
package contest

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"chaimir/internal/platform/auth"
	"chaimir/pkg/apperr"
)

// validateContestRequest 校验赛事管理输入和时间线。
func validateContestRequest(req ContestRequest) (ContestRequest, error) {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 255 {
		return ContestRequest{}, apperr.ErrContestInvalid
	}
	if req.Rules == nil {
		req.Rules = map[string]any{}
	}
	if !registeredContestMode(req.Mode) {
		return ContestRequest{}, apperr.ErrContestInvalid
	}
	if req.TeamMode != TeamModeSolo && req.TeamMode != TeamModeGroup {
		return ContestRequest{}, apperr.ErrContestInvalid
	}
	if req.Mode == ContestModeBattle && req.MatchMode != MatchModeRoundRobin && req.MatchMode != MatchModeELO {
		return ContestRequest{}, apperr.ErrContestInvalid
	}
	if req.Mode == ContestModeSolve {
		req.MatchMode = 0
	}
	if req.SignupStart.IsZero() || req.SignupEnd.IsZero() || req.StartAt.IsZero() || req.EndAt.IsZero() || !req.SignupStart.Before(req.SignupEnd) || req.SignupEnd.After(req.StartAt) || !req.StartAt.Before(req.EndAt) {
		return ContestRequest{}, apperr.ErrContestInvalid
	}
	if req.FreezeMinutes < 0 {
		return ContestRequest{}, apperr.ErrContestInvalid
	}
	return req, nil
}

// validateProblemRequest 校验竞赛题目引用和赛内分值。
func validateProblemRequest(req ProblemRequest, mode int16) (ProblemRequest, error) {
	req.ItemCode = strings.TrimSpace(req.ItemCode)
	req.ItemVersion = strings.TrimSpace(req.ItemVersion)
	if req.ItemCode == "" || req.ItemVersion == "" || req.Score <= 0 {
		return ProblemRequest{}, apperr.ErrContestProblemInvalid
	}
	if req.DynamicScore == nil {
		req.DynamicScore = map[string]any{}
	}
	if mode == ContestModeBattle {
		if !registeredBattleRule(req.BattleRule) {
			return ProblemRequest{}, apperr.ErrContestProblemInvalid
		}
		if err := validateBattleConfig(req.BattleConfig); err != nil {
			return ProblemRequest{}, err
		}
	} else {
		req.BattleRule = 0
		req.BattleConfig = map[string]any{}
	}
	return req, nil
}

// validateBattleConfig 校验对抗题执行所需的沙箱运行时配置。
func validateBattleConfig(cfg map[string]any) error {
	if len(cfg) == 0 {
		return apperr.ErrContestProblemInvalid
	}
	for _, key := range []string{"runtime_code", "runtime_image_version"} {
		if strings.TrimSpace(stringValue(cfg[key])) == "" {
			return apperr.ErrContestProblemInvalid
		}
	}
	if raw, ok := cfg["tool_codes"]; ok {
		items, ok := raw.([]any)
		if !ok {
			return apperr.ErrContestProblemInvalid
		}
		for _, item := range items {
			if strings.TrimSpace(stringValue(item)) == "" {
				return apperr.ErrContestProblemInvalid
			}
		}
	}
	return nil
}

// stringValue 在规则校验中读取 JSON 字符串值。
func stringValue(v any) string {
	s, _ := v.(string)
	return s
}

// validateContestTransition 校验竞赛生命周期状态流转。
func validateContestTransition(current, next int16) error {
	switch next {
	case ContestStatusSignup:
		if current == ContestStatusDraft {
			return nil
		}
	case ContestStatusRunning:
		if current == ContestStatusSignup {
			return nil
		}
	case ContestStatusEnded:
		if current == ContestStatusRunning || current == ContestStatusFrozen {
			return nil
		}
	case ContestStatusArchived:
		if current == ContestStatusEnded {
			return nil
		}
	case ContestStatusFrozen:
		if current == ContestStatusRunning {
			return nil
		}
	}
	return apperr.ErrContestStateInvalid
}

// canManageContest 校验教师作者或学校管理员对竞赛的管理权限。
func canManageContest(accountID int64, isSchoolAdmin bool, item Contest) error {
	if isSchoolAdmin || item.OrganizerID == accountID {
		return nil
	}
	return apperr.ErrForbidden
}

// validateSignupWindow 校验当前时间处于报名期。
func validateSignupWindow(item Contest, now time.Time) error {
	if item.Status != ContestStatusSignup || now.Before(item.SignupStart) || now.After(item.SignupEnd) {
		return apperr.ErrContestSignupClosed
	}
	return nil
}

// validateContestRunning 校验提交类操作处于比赛可提交状态。
func validateContestRunning(item Contest) error {
	if item.Status != ContestStatusRunning && item.Status != ContestStatusFrozen {
		return apperr.ErrContestStateInvalid
	}
	return nil
}

// contestSourceRef 生成竞赛级来源引用,用于结束归档级联回收。
func contestSourceRef(contestID int64, now time.Time) string {
	return fmt.Sprintf("contest:%04d:contest:%d", now.Year(), contestID)
}

// submissionSourceRef 生成解题提交来源引用。
func submissionSourceRef(id int64, now time.Time) string {
	return fmt.Sprintf("contest:%04d:submission:%d", now.Year(), id)
}

// battleSourceRef 生成对抗对局来源引用。
func battleSourceRef(id int64, now time.Time) string {
	return fmt.Sprintf("contest:%04d:battle:%d", now.Year(), id)
}

// isSHA256Hex 校验参赛提交的内容哈希格式,与 M3 判题契约保持一致。
func isSHA256Hex(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

// validContestSourceRef 校验事件来源确属 M8。
func validContestSourceRef(sourceRef string) bool {
	return auth.ValidSourceRef(sourceRef) && strings.HasPrefix(strings.TrimSpace(sourceRef), "contest:")
}

// validateTeamName 校验队伍名称。
func validateTeamName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 128 {
		return "", apperr.ErrContestTeamInvalid
	}
	return name, nil
}

// validateBattleEntryRequest 校验参战物角色和对象引用。
func validateBattleEntryRequest(req BattleEntryRequest) (BattleEntryRequest, error) {
	req.ArtifactRef = strings.TrimSpace(req.ArtifactRef)
	req.CodeHash = strings.TrimSpace(req.CodeHash)
	if req.ProblemID <= 0 || req.ArtifactRef == "" || len(req.ArtifactRef) > 255 || !isSHA256Hex(req.CodeHash) {
		return BattleEntryRequest{}, apperr.ErrContestBattleEntryInvalid
	}
	if req.Role != BattleRoleStrategy && req.Role != BattleRoleDefense && req.Role != BattleRoleAttack {
		return BattleEntryRequest{}, apperr.ErrContestBattleEntryInvalid
	}
	return req, nil
}

// validateCheatRequest 校验防作弊处理输入。
func validateCheatRequest(req CheatRecordRequest) (CheatRecordRequest, error) {
	if req.TeamID <= 0 || (req.Type != CheatTypeSimilarity && req.Type != CheatTypeBehavior && req.Type != CheatTypeEnvironment) || (req.Action != CheatActionWarn && req.Action != CheatActionPenalty && req.Action != CheatActionDisqualify) {
		return CheatRecordRequest{}, apperr.ErrContestCheatInvalid
	}
	if req.Evidence == nil {
		req.Evidence = map[string]any{}
	}
	if req.Action == CheatActionPenalty && float64FromMap(req.Evidence, "penalty_score", 0) <= 0 {
		return CheatRecordRequest{}, apperr.ErrContestCheatInvalid
	}
	return req, nil
}

// float64FromMap 从结构化 evidence 读取数值。
func float64FromMap(m map[string]any, key string, defaultValue float64) float64 {
	switch v := m[key].(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return defaultValue
	}
}

// validateVulnProblemInput 校验漏洞题草稿输入。
func validateVulnProblemInput(req ImportVulnProblemRequest) (ImportVulnProblemRequest, error) {
	req.Title = strings.TrimSpace(req.Title)
	req.ExternalRef = strings.TrimSpace(req.ExternalRef)
	if req.Title == "" || len(req.Title) > 255 || (req.Level != VulnLevelA && req.Level != VulnLevelB && req.Level != VulnLevelC) || (req.RuntimeMode != VulnRuntimeIsolated && req.RuntimeMode != VulnRuntimeForked) {
		return ImportVulnProblemRequest{}, apperr.ErrContestVulnProblemInvalid
	}
	if len(req.DraftBody) == 0 {
		return ImportVulnProblemRequest{}, apperr.ErrContestVulnProblemInvalid
	}
	return req, nil
}

// validatePrevalidateRequest 校验漏洞预验证运行时参数。
func validatePrevalidateRequest(req PrevalidateRequest) (PrevalidateRequest, error) {
	req.RuntimeCode = strings.TrimSpace(req.RuntimeCode)
	req.RuntimeImageVersion = strings.TrimSpace(req.RuntimeImageVersion)
	req.InitCodeRef = strings.TrimSpace(req.InitCodeRef)
	req.InitScriptRef = strings.TrimSpace(req.InitScriptRef)
	if req.RuntimeCode == "" || req.RuntimeImageVersion == "" {
		return PrevalidateRequest{}, apperr.ErrContestVulnProblemInvalid
	}
	outTools := make([]string, 0, len(req.ToolCodes))
	for _, code := range req.ToolCodes {
		code = strings.TrimSpace(code)
		if code != "" {
			outTools = append(outTools, code)
		}
	}
	req.ToolCodes = outTools
	return req, nil
}

// stableContestCode 为漏洞题固化生成稳定内容 code。
func stableContestCode(problem VulnProblem) string {
	return fmt.Sprintf("VULN-%d", problem.ID)
}
