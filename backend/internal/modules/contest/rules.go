package contest

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/net/http/httpguts"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/netx"
	"chaimir/pkg/apperr"
)

var errVulnForkInvalid = apperr.New(apperr.CodeContestVulnPrevalidateFailed, "请配置完整历史状态、固定链身份及工作区私有连接后重试")

func validateContestRequest(req ContestRequest) (ContestRequest, error) {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || utf8.RuneCountInString(req.Name) > 255 {
		return ContestRequest{}, apperr.ErrContestInvalid
	}
	if req.Mode != ContestModeSolve && req.Mode != ContestModeBattle {
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

func validateProblemRequest(req ProblemRequest, mode int16) (ProblemRequest, error) {
	req.ItemCode = strings.TrimSpace(req.ItemCode)
	req.ItemVersion = strings.TrimSpace(req.ItemVersion)
	if req.ItemCode == "" || req.ItemVersion == "" || req.Score <= 0 || req.Seq <= 0 {
		return ProblemRequest{}, apperr.ErrContestProblemInvalid
	}
	if req.DynamicScore != nil {
		if mode != ContestModeSolve || req.DynamicScore.MinScore <= 0 || req.DynamicScore.MinScore > req.Score || req.DynamicScore.DecayPerSolve <= 0 {
			return ProblemRequest{}, apperr.ErrContestProblemInvalid
		}
	}
	if mode == ContestModeBattle {
		if req.BattleRule != BattleRuleAttackDefense && req.BattleRule != BattleRuleGame {
			return ProblemRequest{}, apperr.ErrContestProblemInvalid
		}
	} else if req.BattleRule != 0 {
		return ProblemRequest{}, apperr.ErrContestProblemInvalid
	}
	return req, nil
}

func contestContentTypeAllowed(mode, contentType int16) bool {
	switch mode {
	case ContestModeSolve:
		return contentType == contentTypeContestProblem || contentType == contentTypeTheoryQuestion
	case ContestModeBattle:
		return contentType == contentTypeContestProblem
	default:
		return false
	}
}

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
		if current == ContestStatusRunning || current == ContestStatusFrozen || current == ContestStatusEnded {
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

// validateCheatReviewContestStatus 校验赛事是否已有参赛阶段数据且尚未归档。
func validateCheatReviewContestStatus(status int16) error {
	if status == ContestStatusDraft || status == ContestStatusArchived {
		return apperr.ErrContestStateInvalid
	}
	return nil
}

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
		freezeStart := time.Unix(item.EndAt.Unix()-int64(item.FreezeMinutes)*60, int64(item.EndAt.Nanosecond()))
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

func canManageContest(accountID int64, isSchoolAdmin bool, item Contest) error {
	if isSchoolAdmin || item.OrganizerID == accountID {
		return nil
	}
	return apperr.ErrForbidden
}

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

func submissionSourceRef(id int64, now time.Time) string {
	return fmt.Sprintf("contest:%04d:submission:%s", now.Year(), ids.Format(id))
}

func battleSourceRef(id int64, now time.Time) string {
	return fmt.Sprintf("contest:%04d:battle:%s", now.Year(), ids.Format(id))
}

func validContestSourceRef(sourceRef string) bool {
	return auth.ValidSourceRef(sourceRef) && strings.HasPrefix(strings.TrimSpace(sourceRef), "contest:")
}

func validateTeamName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 128 {
		return "", apperr.ErrContestTeamInvalid
	}
	return name, nil
}

func validateBattleEntryRequest(req BattleEntryRequest) (BattleEntryRequest, error) {
	if req.ProblemID <= 0 || req.SandboxID <= 0 {
		return BattleEntryRequest{}, apperr.ErrContestBattleEntryInvalid
	}
	if req.Role != BattleRoleStrategy && req.Role != BattleRoleDefense && req.Role != BattleRoleAttack {
		return BattleEntryRequest{}, apperr.ErrContestBattleEntryInvalid
	}
	return req, nil
}

// validateContestSolveRequest 校验环境提交与答案提交的互斥契约。
func validateContestSolveRequest(req SubmitRequest, submitKey string, needsSandbox bool) error {
	if req.ContentRef == nil {
		return apperr.ErrContestSubmissionInvalid
	}
	if needsSandbox {
		if strings.TrimSpace(req.SandboxRef) == "" || len(req.ContentRef) != 0 {
			return apperr.ErrContestSubmissionInvalid
		}
		return nil
	}
	if submitKey == "" || strings.TrimSpace(req.SandboxRef) != "" || len(req.ContentRef) != 1 ||
		strings.TrimSpace(jsonx.StringValue(req.ContentRef[submitKey])) == "" {
		return apperr.ErrContestSubmissionInvalid
	}
	return nil
}

// 角色边界由题目规则唯一决定,不能让客户端选择一个撮合器永远无法配对的角色。
func validateBattleEntryRole(rule, role int16) error {
	switch rule {
	case BattleRuleAttackDefense:
		if role == BattleRoleAttack || role == BattleRoleDefense {
			return nil
		}
	case BattleRuleGame:
		if role == BattleRoleStrategy {
			return nil
		}
	}
	return apperr.ErrContestBattleEntryInvalid
}

func validateCheatRequest(req CheatRecordRequest) (CheatRecordRequest, error) {
	if req.TeamID <= 0 || (req.Type != CheatTypeSimilarity && req.Type != CheatTypeBehavior && req.Type != CheatTypeEnvironment) || (req.Action != CheatActionWarn && req.Action != CheatActionPenalty && req.Action != CheatActionDisqualify) {
		return CheatRecordRequest{}, apperr.ErrContestCheatInvalid
	}
	if req.Evidence == nil {
		req.Evidence = map[string]any{}
	}
	if req.Action == CheatActionPenalty {
		score, ok := jsonx.Float64FromAnyOK(req.Evidence["penalty_score"])
		_, text := req.Evidence["penalty_score"].(string)
		if !ok || text || score <= 0 {
			return CheatRecordRequest{}, apperr.ErrContestCheatInvalid
		}
	}
	return req, nil
}

func validateVulnProblemInput(req VulnProblemRequest) (VulnProblemRequest, error) {
	req.Title = strings.TrimSpace(req.Title)
	req.ExternalRef = strings.TrimSpace(req.ExternalRef)
	if req.Title == "" || utf8.RuneCountInString(req.Title) > 255 || utf8.RuneCountInString(req.ExternalRef) > 128 || req.SourceID < 0 || (req.Level != VulnLevelA && req.Level != VulnLevelB && req.Level != VulnLevelC) || (req.RuntimeMode != VulnRuntimeIsolated && req.RuntimeMode != VulnRuntimeForked) {
		return VulnProblemRequest{}, apperr.ErrContestVulnProblemInvalid
	}
	if !validVulnDraftShape(req.DraftBody) {
		return VulnProblemRequest{}, apperr.ErrContestVulnProblemInvalid
	}
	return req, nil
}

// validVulnDraftShape 允许字段未填完，但拒绝无法恢复到编辑表单的已知字段类型。
func validVulnDraftShape(body map[string]any) bool {
	if body == nil {
		return false
	}
	for _, key := range []string{"init_steps", "init_code_ref", "init_script_ref"} {
		if _, exists := body[key]; exists {
			return false
		}
	}
	stringsOnly := func(row map[string]any, keys ...string) bool {
		for _, key := range keys {
			if value, exists := row[key]; exists {
				if _, ok := value.(string); !ok {
					return false
				}
			}
		}
		return true
	}
	if !stringsOnly(body, "description", "contract_source") {
		return false
	}
	for _, key := range []string{"positive_steps", "assertions"} {
		value, exists := body[key]
		if !exists {
			continue
		}
		items, ok := value.([]any)
		if !ok {
			return false
		}
		for _, value := range items {
			row, ok := value.(map[string]any)
			if !ok || row == nil || !stringsOnly(row, "op", "runtime_instance", "target", "label", "field", "expected_label", "hint") {
				return false
			}
		}
	}
	// 标准 JSON 解码负责数字和组合内部字段的类型；未知正文扩展仍保留原样。
	raw, err := json.Marshal(body)
	if err != nil {
		return false
	}
	var fields struct {
		ForkBlock   *float64 `json:"fork_block"`
		JudgeConfig *struct {
			JudgerCode string   `json:"judger_code"`
			MaxScore   *float64 `json:"max_score"`
		} `json:"judge_config"`
	}
	if err := json.Unmarshal(raw, &fields); err != nil {
		return false
	}
	if value, exists := body["fork_block"]; exists && value == nil {
		return false
	}
	if value, exists := body["judge_config"]; exists {
		judge, ok := value.(map[string]any)
		if !ok || judge == nil || !stringsOnly(judge, "judger_code") {
			return false
		}
		if score, exists := judge["max_score"]; exists && score == nil {
			return false
		}
	}
	if value, exists := body["composition"]; exists {
		composition, ok := value.(map[string]any)
		if !ok || composition == nil || !stringsOnly(composition, "id", "workspace_runtime_instance") {
			return false
		}
		for _, key := range []string{"runtimes", "tools", "infra", "links", "init_assets", "init_chain_steps"} {
			if value, exists := composition[key]; exists {
				items, ok := value.([]any)
				if !ok {
					return false
				}
				for _, value := range items {
					row, ok := value.(map[string]any)
					if !ok || row == nil || !stringsOnly(row, "instance_code", "runtime_code", "image_version", "code", "source_component", "source_endpoint", "target_component", "target_endpoint", "protocol", "access_scope", "config_binding", "runtime_instance", "op", "target") {
						return false
					}
					if params, exists := row["params"]; exists {
						if value, ok := params.(map[string]any); !ok || value == nil {
							return false
						}
					}
					if key == "tools" || key == "infra" {
						selection, ok := row["selection"].(string)
						if !ok || (selection != "explicit" && selection != "auto") || strings.TrimSpace(jsonx.StringValue(row["code"])) == "" {
							return false
						}
					}
					if flag, exists := row["required_at_start"]; exists {
						if _, ok := flag.(bool); !ok {
							return false
						}
					}
				}
			}
		}
		raw, err := json.Marshal(composition)
		if err != nil {
			return false
		}
		var spec contracts.SandboxCompositionSpec
		if jsonx.DecodeStrictKnownFields(raw, &spec) != nil {
			return false
		}
		names := make(map[string]struct{}, len(spec.Runtimes))
		for _, runtime := range spec.Runtimes {
			name := strings.TrimSpace(runtime.InstanceCode)
			if name == "" {
				continue
			}
			if _, exists := names[name]; exists {
				return false
			}
			names[name] = struct{}{}
		}
	}
	return true
}

// validateVulnDraftBody 在执行前检查判题配置；草稿暂存不要求可执行。
func validateVulnDraftBody(body map[string]any) bool {
	judgeConfig, ok := body["judge_config"].(map[string]any)
	return ok && strings.TrimSpace(jsonx.StringFromAny(judgeConfig["judger_code"])) != "" && jsonx.Int32FromAny(judgeConfig["max_score"], 0) > 0
}

// validationSteps 按声明顺序读取执行项，拒绝静默丢弃畸形步骤或断言。
func validationSteps(body map[string]any, keys ...string) ([]map[string]any, error) {
	var out []map[string]any
	for _, key := range keys {
		value, exists := body[key]
		if !exists {
			continue
		}
		items, ok := value.([]any)
		if !ok {
			return nil, apperr.ErrContestVulnProblemInvalid
		}
		for _, item := range items {
			step, ok := item.(map[string]any)
			if !ok || len(step) == 0 {
				return nil, apperr.ErrContestVulnProblemInvalid
			}
			out = append(out, step)
		}
	}
	return out, nil
}

// vulnPrevalidateSpec 只从已保存草稿读取环境，执行前拒绝未完成的判题配置。
func vulnPrevalidateSpec(body map[string]any) (contracts.SandboxCompositionSpec, error) {
	if !validVulnDraftShape(body) || !validateVulnDraftBody(body) {
		return contracts.SandboxCompositionSpec{}, apperr.ErrContestVulnProblemInvalid
	}
	raw, err := json.Marshal(body["composition"])
	if err != nil {
		return contracts.SandboxCompositionSpec{}, apperr.ErrContestVulnProblemInvalid.WithCause(err)
	}
	var spec contracts.SandboxCompositionSpec
	if err := jsonx.DecodeStrictKnownFields(raw, &spec); err != nil {
		return spec, apperr.ErrContestVulnProblemInvalid.WithCause(err)
	}
	spec.ID = strings.TrimSpace(spec.ID)
	if spec.ID == "" || len(spec.Runtimes) == 0 {
		return contracts.SandboxCompositionSpec{}, apperr.ErrContestVulnProblemInvalid
	}
	for index := range spec.Runtimes {
		runtime := &spec.Runtimes[index]
		runtime.InstanceCode = strings.TrimSpace(runtime.InstanceCode)
		runtime.Code = strings.TrimSpace(runtime.Code)
		runtime.ImageVersion = strings.TrimSpace(runtime.ImageVersion)
		if runtime.InstanceCode == "" || runtime.Code == "" || runtime.ImageVersion == "" {
			return contracts.SandboxCompositionSpec{}, apperr.ErrContestVulnProblemInvalid
		}
	}
	steps, err := validationSteps(body["composition"].(map[string]any), "init_chain_steps")
	if err != nil {
		return contracts.SandboxCompositionSpec{}, err
	}
	if _, err := validationSteps(body, "positive_steps", "assertions"); err != nil {
		return contracts.SandboxCompositionSpec{}, err
	}
	for index, raw := range steps {
		spec.InitChainSteps[index].RuntimeInstance, err = contracts.ChainRuntimeInstance(raw, spec.WorkspaceRuntimeInstance)
		if err != nil {
			return contracts.SandboxCompositionSpec{}, apperr.ErrContestVulnProblemInvalid.WithCause(err)
		}
	}
	spec.AccessProfile = contracts.SandboxAccessVulnerabilityPrevalidate
	return spec, nil
}

// bindVulnForkIdentity 从已保存的私有连接提取历史身份，交由 M2 目录契约编译，不猜测运行时代码。
func bindVulnForkIdentity(spec *contracts.SandboxCompositionSpec, body map[string]any) error {
	switch body["fork_block"].(type) {
	case int, int32, int64, float32, float64, json.Number:
	default:
		return errVulnForkInvalid
	}
	block := jsonx.Int64FromAny(body["fork_block"], -1)
	if block < 0 {
		return errVulnForkInvalid
	}
	var selected *contracts.CompositionLink
	for index := range spec.Links {
		link := &spec.Links[index]
		if link.SourceComponent != spec.WorkspaceRuntimeInstance || link.ConfigBinding != "env:CHAIMIR_EVM_FORK_RPC_URL" {
			continue
		}
		if selected != nil || link.SourceEndpoint != "rpc" || link.TargetEndpoint != "rpc" || link.Protocol != "http" || !link.RequiredAtStart || link.AccessScope != "sandbox" || link.TargetComponent == spec.WorkspaceRuntimeInstance {
			return errVulnForkInvalid
		}
		selected = link
	}
	if selected == nil {
		return errVulnForkInvalid
	}
	var workspace, history *contracts.CompositionRuntimeRef
	for index := range spec.Runtimes {
		ref := &spec.Runtimes[index]
		if ref.InstanceCode == spec.WorkspaceRuntimeInstance {
			workspace = ref
		}
		if ref.InstanceCode == selected.TargetComponent {
			history = ref
		}
	}
	if workspace == nil || history == nil {
		return errVulnForkInvalid
	}
	identity := make(map[string]string, 4)
	for _, key := range []string{"block_number", "block_hash", "state_root", "chain_id"} {
		value, ok := history.Params["history_"+key].(string)
		if !ok || value == "" {
			return errVulnForkInvalid
		}
		identity[key] = value
	}
	if identity["block_number"] != strconv.FormatUint(uint64(block), 10) {
		return errVulnForkInvalid
	}
	chain, ok := new(big.Int).SetString(identity["chain_id"], 10)
	if !ok || chain.Sign() <= 0 || chain.BitLen() > 256 || chain.String() != identity["chain_id"] {
		return errVulnForkInvalid
	}
	for _, key := range []string{"block_hash", "state_root"} {
		value := identity[key]
		if len(value) != 66 || !strings.HasPrefix(value, "0x") || strings.Trim(value[2:], "0") == "" {
			return errVulnForkInvalid
		}
		for _, ch := range value[2:] {
			if !(ch >= '0' && ch <= '9' || ch >= 'a' && ch <= 'f') {
				return errVulnForkInvalid
			}
		}
	}
	if workspace.Params == nil {
		workspace.Params = make(map[string]any, 4)
	}
	for key, value := range identity {
		name := "fork_" + key
		if stored, exists := workspace.Params[name]; exists && stored != value {
			return errVulnForkInvalid
		}
		workspace.Params[name] = value
	}
	return nil
}

func validateVulnForkSnapshot(item VulnProblem) error {
	if item.CompositionSnapshot == nil || item.CompositionDigest == "" || item.CompositionSnapshot.Digest != item.CompositionDigest {
		return errVulnForkInvalid
	}
	expected, err := vulnPrevalidateSpec(item.DraftBody)
	if err != nil {
		return errVulnForkInvalid.WithCause(err)
	}
	if err := bindVulnForkIdentity(&expected, item.DraftBody); err != nil {
		return err
	}
	expected.ID = item.CompositionSnapshot.Spec.ID
	want, err := contracts.CanonicalDigest(expected)
	if err != nil {
		return errVulnForkInvalid.WithCause(err)
	}
	actualSpec := item.CompositionSnapshot.Spec
	actualSpec.Infra = nil
	for _, ref := range item.CompositionSnapshot.Spec.Infra {
		if ref.Selection != "auto" {
			actualSpec.Infra = append(actualSpec.Infra, ref)
		}
	}
	actual, err := contracts.CanonicalDigest(actualSpec)
	if err != nil || actual != want {
		return errVulnForkInvalid.WithCause(err)
	}
	digest, err := contracts.CanonicalSnapshotDigest(*item.CompositionSnapshot)
	if err != nil || digest != item.CompositionDigest {
		return errVulnForkInvalid.WithCause(err)
	}
	return nil
}

func stableContestCode(problem VulnProblem) string {
	return "VULN-" + ids.Format(problem.ID)
}

func vulnSourceFromRequest(req VulnSourceRequest, tenantID, generatedID int64) (VulnSource, error) {
	req.Name = strings.TrimSpace(req.Name)
	if req.ID <= 0 {
		req.ID = ids.ID(generatedID)
	}
	if (req.Type != VulnSourceTypeSWC && req.Type != VulnSourceTypeIntelligence && req.Type != VulnSourceTypeCVEOnchain) || req.Name == "" || utf8.RuneCountInString(req.Name) > 128 || req.Config == nil || (req.DefaultLevel != VulnLevelA && req.DefaultLevel != VulnLevelB && req.DefaultLevel != VulnLevelC) {
		return VulnSource{}, apperr.ErrContestVulnSourceInvalid
	}
	return VulnSource{ID: req.ID.Int64(), TenantID: tenantID, Type: req.Type, Name: req.Name, Config: req.Config, DefaultLevel: req.DefaultLevel, Enabled: req.Enabled}, nil
}

// parseVulnSourceConfig 统一保存与执行的类型、默认值和空白处理，不修改原配置或凭据。
func parseVulnSourceConfig(cfg map[string]any, defaultTimeout int) (vulnSourceHTTPConfig, error) {
	parsed := vulnSourceHTTPConfig{timeoutSeconds: defaultTimeout}
	for name, target := range map[string]*string{"endpoint": &parsed.endpoint, "method": &parsed.method, "cases_path": &parsed.casesPath} {
		if value, exists := cfg[name]; exists {
			text, ok := value.(string)
			if !ok {
				return vulnSourceHTTPConfig{}, apperr.ErrContestVulnSourceInvalid
			}
			*target = strings.TrimSpace(text)
		}
	}
	endpoint, err := netx.ValidatePublicHTTPURL(parsed.endpoint)
	if err != nil {
		return vulnSourceHTTPConfig{}, apperr.ErrContestVulnSourceInvalid.WithCause(err)
	}
	parsed.endpoint = endpoint
	parsed.method = strings.ToUpper(parsed.method)
	if parsed.method == "" {
		parsed.method = http.MethodGet
	}
	if parsed.method != http.MethodGet && parsed.method != http.MethodPost {
		return vulnSourceHTTPConfig{}, apperr.ErrContestVulnSourceInvalid
	}
	if value, exists := cfg["timeout_seconds"]; exists {
		if _, text := value.(string); text {
			return vulnSourceHTTPConfig{}, apperr.ErrContestVulnSourceInvalid
		}
		parsed.timeoutSeconds = jsonx.IntFromAny(value)
	}
	if parsed.timeoutSeconds < 1 || parsed.timeoutSeconds > 60 {
		return vulnSourceHTTPConfig{}, apperr.ErrContestVulnSourceInvalid
	}
	if raw, exists := cfg["headers"]; exists {
		headers, ok := raw.(map[string]any)
		if !ok {
			return vulnSourceHTTPConfig{}, apperr.ErrContestVulnSourceInvalid
		}
		parsed.headers = make(map[string]string, len(headers))
		for name, raw := range headers {
			value, ok := raw.(string)
			if !ok || !httpguts.ValidHeaderFieldName(name) || !httpguts.ValidHeaderFieldValue(value) {
				return vulnSourceHTTPConfig{}, apperr.ErrContestVulnSourceInvalid
			}
			parsed.headers[name] = value
		}
	}
	mapping, ok := cfg["mapping"].(map[string]any)
	if !ok {
		return vulnSourceHTTPConfig{}, apperr.ErrContestVulnSourceInvalid
	}
	parsed.mapping = make(map[string]string, len(mapping))
	for name, raw := range mapping {
		value, ok := raw.(string)
		if !ok {
			return vulnSourceHTTPConfig{}, apperr.ErrContestVulnSourceInvalid
		}
		parsed.mapping[name] = strings.TrimSpace(value)
	}
	for _, key := range []string{"external_ref", "title", "draft_body"} {
		if parsed.mapping[key] == "" {
			return vulnSourceHTTPConfig{}, apperr.ErrContestVulnSourceInvalid
		}
	}
	return parsed, nil
}

// selectCases 读取完整案例数组，结构错误时不返回部分案例。
func selectCases(payload any, path string) ([]map[string]any, error) {
	current := payload
	if path != "" {
		root, ok := payload.(map[string]any)
		if !ok {
			return nil, apperr.ErrContestVulnSourceJSONInvalid
		}
		current = jsonx.ValueFromPath(root, path)
	}
	items, ok := current.([]any)
	if !ok {
		return nil, apperr.ErrContestVulnSourceJSONInvalid
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			return nil, apperr.ErrContestVulnSourceJSONInvalid
		}
		out = append(out, obj)
	}
	return out, nil
}

func vulnProblemFromExternal(item map[string]any, mapping map[string]string, defaultLevel int16) (VulnProblem, error) {
	body, ok := jsonx.ValueFromPath(item, mapping["draft_body"]).(map[string]any)
	if !ok || body == nil {
		return VulnProblem{}, apperr.ErrContestVulnProblemInvalid
	}
	req := VulnProblemRequest{
		ExternalRef: strings.TrimSpace(jsonx.StringFromPath(item, mapping["external_ref"])),
		Title:       strings.TrimSpace(jsonx.StringFromPath(item, mapping["title"])),
		Level:       vulnLevelFromAny(jsonx.ValueFromPath(item, mapping["level"]), defaultLevel),
		RuntimeMode: vulnRuntimeFromAny(jsonx.ValueFromPath(item, mapping["runtime_mode"]), VulnRuntimeIsolated),
		DraftBody:   body,
	}
	req, err := validateVulnProblemInput(req)
	if err != nil {
		return VulnProblem{}, err
	}
	return VulnProblem{ExternalRef: req.ExternalRef, Title: req.Title, Level: req.Level, RuntimeMode: req.RuntimeMode, DraftBody: req.DraftBody}, nil
}

// vulnLevelFromAny 解析 A/B/C 或 1/2/3 分级。
func vulnLevelFromAny(v any, defaultValue int16) int16 {
	if v == nil {
		return defaultValue
	}
	switch strings.ToUpper(strings.TrimSpace(jsonx.StringValue(v))) {
	case "A":
		return VulnLevelA
	case "B":
		return VulnLevelB
	case "C":
		return VulnLevelC
	}
	return jsonx.Int16FromAny(v, 0)
}

// vulnRuntimeFromAny 解析 isolated/forked 或 1/2 运行时。
func vulnRuntimeFromAny(v any, defaultValue int16) int16 {
	if v == nil {
		return defaultValue
	}
	switch strings.ToLower(strings.TrimSpace(jsonx.StringValue(v))) {
	case "isolated":
		return VulnRuntimeIsolated
	case "forked":
		return VulnRuntimeForked
	}
	return jsonx.Int16FromAny(v, 0)
}
