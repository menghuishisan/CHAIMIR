// contest rules 文件集中放置 M8 输入校验、状态机、限频和来源引用规则。
package contest

import (
	"fmt"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
	pkgcrypto "chaimir/pkg/crypto"
)

var (
	// contestModeRegistry 注册平台内置和后续扩展的竞赛类型。
	contestModeRegistry = map[int16]string{ContestModeSolve: "solve", ContestModeBattle: "battle"}
	// battleRuleRegistry 注册平台内置和后续扩展的对局规则。
	battleRuleRegistry = map[int16]string{BattleRuleAttackDefense: "attack-defense", BattleRuleGame: "game"}
)

// registeredContestMode 判断竞赛类型是否已注册。
func registeredContestMode(mode int16) bool {
	_, ok := contestModeRegistry[mode]
	return ok
}

// registeredBattleRule 判断对局规则是否已注册。
func registeredBattleRule(rule int16) bool {
	_, ok := battleRuleRegistry[rule]
	return ok
}

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
	if req.DynamicScore != nil {
		if req.DynamicScore.MinScore <= 0 || req.DynamicScore.MinScore > req.Score || req.DynamicScore.DecayPerSolve <= 0 {
			return ProblemRequest{}, apperr.ErrContestProblemInvalid
		}
	}
	if mode == ContestModeBattle {
		if !registeredBattleRule(req.BattleRule) {
			return ProblemRequest{}, apperr.ErrContestProblemInvalid
		}
		battleConfig, err := validateBattleConfig(req.BattleConfig)
		if err != nil {
			return ProblemRequest{}, err
		}
		req.BattleConfig = battleConfig
	} else {
		if req.BattleRule != 0 || req.BattleConfig != nil {
			return ProblemRequest{}, apperr.ErrContestProblemInvalid
		}
		req.BattleRule = 0
		req.BattleConfig = nil
	}
	return req, nil
}

// validateBattleConfig 校验并规范化对抗题执行所需的沙箱运行时配置。
func validateBattleConfig(cfg *BattleRuntimeConfig) (*BattleRuntimeConfig, error) {
	if cfg == nil {
		return nil, apperr.ErrContestProblemInvalid
	}
	normalized := BattleRuntimeConfig{
		ExecutionProfile: strings.TrimSpace(cfg.ExecutionProfile),
		EntryRoles:       append([]int16(nil), cfg.EntryRoles...),
		ReplayProfile:    cfg.ReplayProfile,
	}
	if err := normalized.Validate(); err != nil {
		return nil, apperr.ErrContestProblemInvalid
	}
	return &normalized, nil
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

// validateContestTransitionWindow 在状态图基础上叠加竞赛时间窗口约束。
func validateContestTransitionWindow(item Contest, next int16, now time.Time) error {
	if err := validateContestTransition(item.Status, next); err != nil {
		return err
	}
	switch next {
	case ContestStatusSignup:
		if now.After(item.SignupEnd) {
			return apperr.ErrContestStateInvalid
		}
	case ContestStatusRunning:
		if now.Before(item.StartAt) || !now.Before(item.EndAt) {
			return apperr.ErrContestStateInvalid
		}
	case ContestStatusFrozen:
		if item.FreezeMinutes <= 0 {
			return apperr.ErrContestStateInvalid
		}
		freezeStart := item.EndAt.Add(-time.Duration(item.FreezeMinutes) * time.Minute)
		if now.Before(freezeStart) || !now.Before(item.EndAt) {
			return apperr.ErrContestStateInvalid
		}
	case ContestStatusEnded:
		if now.Before(item.EndAt) {
			return apperr.ErrContestStateInvalid
		}
	}
	return nil
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
	return fmt.Sprintf("contest:%04d:contest:%s", now.Year(), ids.Format(contestID))
}

// submissionSourceRef 生成解题提交来源引用。
func submissionSourceRef(id int64, now time.Time) string {
	return fmt.Sprintf("contest:%04d:submission:%s", now.Year(), ids.Format(id))
}

// battleSourceRef 生成对抗对局来源引用。
func battleSourceRef(id int64, now time.Time) string {
	return fmt.Sprintf("contest:%04d:battle:%s", now.Year(), ids.Format(id))
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
	if req.ProblemID <= 0 || req.ArtifactRef == "" || len(req.ArtifactRef) > 255 || !pkgcrypto.ValidSHA256Hex(req.CodeHash) {
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
	if !validateVulnDraftBody(req.DraftBody) {
		return ImportVulnProblemRequest{}, apperr.ErrContestVulnProblemInvalid
	}
	return req, nil
}

// validateVulnDraftBody 确保漏洞草稿固化时具备完整判题配置,不允许导入后才发现无法判题。
func validateVulnDraftBody(body map[string]any) bool {
	judgeConfig, ok := body["judge_config"].(map[string]any)
	return ok && strings.TrimSpace(jsonx.StringFromAny(judgeConfig["judger_code"])) != "" && jsonx.Int32FromAny(judgeConfig["max_score"], 0) > 0
}

// validatePrevalidateRequest 校验漏洞预验证运行时参数。
func validatePrevalidateRequest(req PrevalidateRequest) (PrevalidateRequest, error) {
	req.InitCodeRef = strings.TrimSpace(req.InitCodeRef)
	req.InitScriptRef = strings.TrimSpace(req.InitScriptRef)
	req.Composition.ID = strings.TrimSpace(req.Composition.ID)
	if req.Composition.ID == "" || len(req.Composition.Runtimes) == 0 {
		return PrevalidateRequest{}, apperr.ErrContestVulnProblemInvalid
	}
	for index := range req.Composition.Runtimes {
		runtime := &req.Composition.Runtimes[index]
		runtime.InstanceCode = strings.TrimSpace(runtime.InstanceCode)
		runtime.Code = strings.TrimSpace(runtime.Code)
		runtime.ImageVersion = strings.TrimSpace(runtime.ImageVersion)
		if runtime.InstanceCode == "" || runtime.Code == "" || runtime.ImageVersion == "" {
			return PrevalidateRequest{}, apperr.ErrContestVulnProblemInvalid
		}
	}
	req.Composition.AccessProfile = contracts.SandboxAccessVulnerabilityPrevalidate
	return req, nil
}

// stableContestCode 为漏洞题固化生成稳定内容 code。
func stableContestCode(problem VulnProblem) string {
	return "VULN-" + ids.Format(problem.ID)
}
