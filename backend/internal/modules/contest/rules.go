// contest rules 文件集中放置 M8 输入校验、状态机、限频和来源引用规则。
package contest

import (
	"fmt"
	"strings"
	"time"

	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
	"chaimir/pkg/chainassert"
	"chaimir/pkg/crypto"
)

// validateContestRequest 校验赛事管理输入和时间线。
func validateContestRequest(req ContestRequest) (ContestRequest, error) {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 255 {
		return ContestRequest{}, apperr.ErrContestInvalid
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
		if req.DynamicScore.MinScore <= 0 || req.DynamicScore.DecayPerSolve <= 0 || req.DynamicScore.MinScore > req.Score {
			return ProblemRequest{}, apperr.ErrContestProblemInvalid
		}
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
		req.BattleConfig = nil
	}
	return req, nil
}

// validateBattleConfig 校验对抗题执行所需的沙箱运行时配置。
func validateBattleConfig(cfg *BattleRuntimeConfig) error {
	if cfg == nil {
		return apperr.ErrContestProblemInvalid
	}
	cfg.RuntimeCode = strings.TrimSpace(cfg.RuntimeCode)
	cfg.RuntimeImageVersion = strings.TrimSpace(cfg.RuntimeImageVersion)
	if cfg.RuntimeCode == "" || cfg.RuntimeImageVersion == "" {
		return apperr.ErrContestProblemInvalid
	}
	for i, code := range cfg.ToolCodes {
		cfg.ToolCodes[i] = strings.TrimSpace(code)
		if cfg.ToolCodes[i] == "" {
			return apperr.ErrContestProblemInvalid
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
	if req.ProblemID <= 0 || req.ArtifactRef == "" || len(req.ArtifactRef) > 255 || !crypto.ValidSHA256Hex(req.CodeHash) {
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
	if !jsonx.HasOnlyKeys(req.Evidence, "review_note", "source_refs", "penalty_score") {
		return CheatRecordRequest{}, apperr.ErrContestCheatInvalid
	}
	reviewNote := strings.TrimSpace(stringValue(req.Evidence["review_note"]))
	sourceRefs, sourceRefsOK := normalizedStringSlice(req.Evidence["source_refs"], true)
	if reviewNote == "" || len([]rune(reviewNote)) > 2000 || !sourceRefsOK {
		return CheatRecordRequest{}, apperr.ErrContestCheatInvalid
	}
	evidence := map[string]any{"review_note": reviewNote, "source_refs": sourceRefs}
	if req.Action == CheatActionPenalty {
		penalty, ok := jsonx.Float64FromNumberOK(req.Evidence["penalty_score"])
		if !ok || penalty <= 0 {
			return CheatRecordRequest{}, apperr.ErrContestCheatInvalid
		}
		evidence["penalty_score"] = penalty
	} else if _, exists := req.Evidence["penalty_score"]; exists {
		return CheatRecordRequest{}, apperr.ErrContestCheatInvalid
	}
	req.Evidence = evidence
	return req, nil
}

// validateVulnProblemInput 校验漏洞题草稿输入。
func validateVulnProblemInput(req ImportVulnProblemRequest) (ImportVulnProblemRequest, error) {
	req.Title = strings.TrimSpace(req.Title)
	req.ExternalRef = strings.TrimSpace(req.ExternalRef)
	if req.Title == "" || len(req.Title) > 255 || (req.Level != VulnLevelA && req.Level != VulnLevelB && req.Level != VulnLevelC) || (req.RuntimeMode != VulnRuntimeIsolated && req.RuntimeMode != VulnRuntimeForked) {
		return ImportVulnProblemRequest{}, apperr.ErrContestVulnProblemInvalid
	}
	if !validateVulnDraftBody(req.DraftBody) {
		return ImportVulnProblemRequest{}, apperr.ErrContestVulnProblemInvalid
	}
	return req, nil
}

// validateVulnDraftBody 校验漏洞草稿唯一结构和预验证所需步骤。
func validateVulnDraftBody(body map[string]any) bool {
	if !jsonx.HasOnlyKeys(body, "statement", "judge_config", "init_contracts", "init_steps", "positive_steps", "assertions", "ad_config") || strings.TrimSpace(stringValue(body["statement"])) == "" {
		return false
	}
	if _, ok := normalizedStringSlice(body["init_contracts"], true); !ok || !validateVulnJudgeConfig(mapAny(body["judge_config"])) {
		return false
	}
	if !validateVulnSteps(body["init_steps"], false) || !validateVulnSteps(body["positive_steps"], true) || !validateVulnAssertions(body["assertions"]) {
		return false
	}
	if raw, exists := body["ad_config"]; exists && raw != nil {
		config := mapAny(raw)
		tools, ok := normalizedStringSlice(config["tool_codes"], true)
		if !jsonx.HasOnlyKeys(config, "runtime_code", "runtime_image_version", "tool_codes") || strings.TrimSpace(stringValue(config["runtime_code"])) == "" || strings.TrimSpace(stringValue(config["runtime_image_version"])) == "" || !ok {
			return false
		}
		config["tool_codes"] = tools
	}
	return true
}

// validateVulnJudgeConfig 校验漏洞草稿在固化前使用的判题器配置。
func validateVulnJudgeConfig(config map[string]any) bool {
	if !jsonx.HasOnlyKeys(config, "judger_code", "suite_ref", "max_score") || strings.TrimSpace(stringValue(config["judger_code"])) == "" {
		return false
	}
	if suiteRef, exists := config["suite_ref"]; exists && suiteRef != nil {
		if _, ok := suiteRef.(string); !ok {
			return false
		}
	}
	maxScore, ok := jsonx.Int32FromNumberOK(config["max_score"])
	return ok && maxScore > 0
}

// validateVulnSteps 校验链操作集合；正向步骤至少包含一条操作。
func validateVulnSteps(raw any, required bool) bool {
	items, ok := mapSlice(raw)
	if !ok || (required && len(items) == 0) {
		return false
	}
	for _, step := range items {
		if !jsonx.HasOnlyKeys(step, "op", "payload") {
			return false
		}
		op := strings.ToLower(strings.TrimSpace(stringValue(step["op"])))
		if op != "deploy" && op != "tx" && op != "query" && op != "reset" {
			return false
		}
		payload := mapAny(step["payload"])
		if (op == "deploy" || op == "tx") && len(payload) == 0 {
			return false
		}
		if op == "query" && strings.TrimSpace(stringValue(payload["target"])) == "" {
			return false
		}
	}
	return true
}

// validateVulnAssertions 校验链上断言的完整字段和比较方式。
func validateVulnAssertions(raw any) bool {
	items, ok := mapSlice(raw)
	if !ok || len(items) == 0 {
		return false
	}
	for _, assertion := range items {
		if !chainassert.Validate(assertion) {
			return false
		}
	}
	return true
}

// validateSubmitContent 校验学生补充答案的唯一结构。
func validateSubmitContent(content map[string]any) bool {
	if !jsonx.HasOnlyKeys(content, "answer") {
		return false
	}
	_, ok := content["answer"].(string)
	return ok
}

// normalizedStringSlice 读取并清理 JSON 字符串数组。
func normalizedStringSlice(value any, allowEmpty bool) ([]string, bool) {
	var raw []string
	switch typed := value.(type) {
	case []string:
		raw = typed
	case []any:
		raw = make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				return nil, false
			}
			raw = append(raw, text)
		}
	default:
		return nil, false
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" {
			return nil, false
		}
		out = append(out, item)
	}
	return out, allowEmpty || len(out) > 0
}

// mapSlice 读取请求或内部构造的对象数组。
func mapSlice(value any) ([]map[string]any, bool) {
	switch typed := value.(type) {
	case []map[string]any:
		return typed, true
	case []any:
		out := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			object, ok := item.(map[string]any)
			if !ok {
				return nil, false
			}
			out = append(out, object)
		}
		return out, true
	default:
		return nil, false
	}
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
