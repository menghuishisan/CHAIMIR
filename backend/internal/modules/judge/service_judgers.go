// judge service_judgers 文件实现内置后端判题策略和快照脱敏规则。
package judge

import (
	"context"
	"encoding/json"
	"maps"
	"reflect"
	"sort"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/intx"
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
	"chaimir/pkg/chainassert"
	pkgcrypto "chaimir/pkg/crypto"
	"chaimir/pkg/privacy"
	"chaimir/pkg/question"
)

// executeJudgerStrategy 执行不需要容器命令的后端内置判题策略。
func (s *Service) executeJudgerStrategy(ctx context.Context, task JudgeTask, sandboxID int64) (JudgeExecutionResult, bool, error) {
	switch task.InputSnapshot.JudgerType {
	case JudgerTypeTheoryQuestion:
		result, err := judgeTheoryQuestion(task)
		return result, true, err
	case JudgerTypeOnchainAssert:
		result, err := s.judgeOnchainAssertions(ctx, task, sandboxID)
		return result, true, err
	case JudgerTypeFlag:
		result, err := s.judgeFlag(ctx, task, sandboxID)
		return result, true, err
	case JudgerTypeSimCheckpoint:
		result, err := judgeSimCheckpoint(task)
		return result, true, err
	default:
		return JudgeExecutionResult{}, false, nil
	}
}

// judgeOnchainAssertions 按 M5 提供的 chain_steps/assertions 通过 M2 链能力比对现场状态。
func (s *Service) judgeOnchainAssertions(ctx context.Context, task JudgeTask, sandboxID int64) (JudgeExecutionResult, error) {
	steps, err := onchainSteps(task.SandboxMode, task.accessProfile, task.InputSnapshot.Expectation)
	if err != nil {
		return JudgeExecutionResult{}, err
	}
	assertions := jsonx.SliceFromAny(task.InputSnapshot.Expectation["assertions"])
	if len(assertions) == 0 {
		return JudgeExecutionResult{}, apperr.ErrJudgerConfigInvalid
	}
	runtimeInstance, err := judgeRuntimeInstance(task)
	if err != nil {
		return JudgeExecutionResult{}, err
	}
	details := make([]JudgeResultDetail, 0, len(assertions))
	actions := make([]contracts.JudgeReplayAction, 0, len(steps)+len(assertions))
	passed := true
	hasOutcome := false
	for _, raw := range steps {
		action, err := s.runChainStep(ctx, task, sandboxID, runtimeInstance, jsonx.ObjectFromAny(raw))
		if err != nil {
			return JudgeExecutionResult{}, err
		}
		actions = append(actions, *action)
	}
	for _, raw := range assertions {
		assertion := jsonx.ObjectFromAny(raw)
		parsedAssertion, err := chainassert.FromMap(assertion)
		if err != nil {
			return JudgeExecutionResult{}, apperr.ErrJudgerConfigInvalid.WithCause(err)
		}
		instance, err := contracts.ChainRuntimeInstance(assertion, runtimeInstance)
		if err != nil {
			return JudgeExecutionResult{}, apperr.ErrJudgerConfigInvalid.WithCause(err)
		}
		actual, err := s.sandbox.ChainQuery(ctx, contracts.SandboxChainQueryRequest{TenantID: task.TenantID, SandboxID: sandboxID, SourceRef: sandboxSourceRef(task), RuntimeInstance: instance, Target: parsedAssertion.Target})
		if err != nil {
			return JudgeExecutionResult{}, apperr.ErrJudgeWorkerFailed.WithCause(err)
		}
		result, err := chainassert.Check(parsedAssertion, actual)
		if err != nil {
			return JudgeExecutionResult{}, apperr.ErrJudgeWorkerFailed.WithCause(err)
		}
		var outcome contracts.JudgeOutcome
		if assertion["outcome"] == true {
			value, ok := actual[parsedAssertion.Field].(string)
			outcome = contracts.JudgeOutcome(value)
			if !ok || !outcome.Valid() {
				return JudgeExecutionResult{}, apperr.ErrJudgeWorkerFailed
			}
			hasOutcome = true
		}
		actions = append(actions, contracts.JudgeReplayAction{Op: "query", RuntimeInstance: instance, Payload: map[string]any{"target": parsedAssertion.Target}, Output: sanitizeReplayMap(actual)})
		if !result.Passed {
			passed = false
		}
		details = append(details, JudgeResultDetail{
			Case:          result.Case,
			Passed:        result.Passed,
			ExpectedLabel: result.ExpectedLabel,
			Actual:        result.Actual,
			Hint:          result.Hint,
			Outcome:       outcome,
		})
	}
	if hasOutcome && !passed {
		return JudgeExecutionResult{}, apperr.ErrJudgeWorkerFailed
	}
	for i := range actions {
		sequence, ok := intx.Int32(i + 1)
		if !ok {
			return JudgeExecutionResult{}, apperr.ErrJudgerConfigInvalid
		}
		actions[i].Seq = sequence
	}
	score := int32(0)
	if passed {
		score = task.InputSnapshot.MaxScore
	}
	return JudgeExecutionResult{Passed: passed, Score: score, MaxScore: task.InputSnapshot.MaxScore, Details: details, Replay: contracts.JudgeReplayTrace{Actions: actions}}, nil
}

// onchainSteps 只允许独立判题或已核验的私有对局执行程序，学生现场只读。
func onchainSteps(mode int16, profile contracts.SandboxAccessProfile, expectation map[string]any) ([]any, error) {
	hasOutcome, err := validateOutcomeAssertions(expectation)
	if err != nil {
		return nil, err
	}
	if hasOutcome && (mode != JudgeSandboxModeReuse || profile != contracts.SandboxAccessContestBattle) {
		return nil, apperr.ErrJudgerConfigInvalid
	}
	raw, exists := expectation["chain_steps"]
	var steps []any
	if exists {
		var ok bool
		steps, ok = raw.([]any)
		if !ok {
			return nil, apperr.ErrJudgerConfigInvalid
		}
	}
	if mode != JudgeSandboxModeReuse {
		return steps, nil
	}
	if profile != contracts.SandboxAccessContestBattle {
		if len(steps) > 0 {
			return nil, apperr.ErrJudgerConfigInvalid
		}
		return steps, nil
	}
	hasExecution := false
	for _, raw := range steps {
		switch jsonx.StringField(jsonx.ObjectFromAny(raw), "op") {
		case "deploy", "tx":
			hasExecution = true
		case "query", "reset":
		default:
			return nil, apperr.ErrJudgerConfigInvalid
		}
	}
	if hasExecution {
		return steps, nil
	}
	return nil, apperr.ErrJudgerConfigInvalid
}

// validateOutcomeAssertions 在执行前确定唯一结果来源，避免先改变链状态才发现配置冲突。
func validateOutcomeAssertions(expectation map[string]any) (bool, error) {
	hasOutcome := false
	for _, raw := range jsonx.SliceFromAny(expectation["assertions"]) {
		assertion := jsonx.ObjectFromAny(raw)
		marker, exists := assertion["outcome"]
		if !exists {
			continue
		}
		enabled, ok := marker.(bool)
		if !ok || !enabled {
			return false, apperr.ErrJudgerConfigInvalid
		}
		parsed, err := chainassert.FromMap(assertion)
		if err != nil || hasOutcome || parsed.Op != chainassert.OperationExists || parsed.Field == "" {
			return false, apperr.ErrJudgerConfigInvalid.WithCause(err)
		}
		hasOutcome = true
	}
	return hasOutcome, nil
}

// validateOutcomeRequirement 将 M8 规则绑定到锁定题目的唯一结果来源。
func validateOutcomeRequirement(typ, mode int16, profile contracts.SandboxAccessProfile, hasOutcome, required bool) error {
	if required && (typ != JudgerTypeOnchainAssert || mode != JudgeSandboxModeReuse || profile != contracts.SandboxAccessContestBattle) {
		return apperr.ErrJudgerConfigInvalid
	}
	if hasOutcome && profile != contracts.SandboxAccessContestBattle || profile == contracts.SandboxAccessContestBattle && hasOutcome != required {
		return apperr.ErrJudgerConfigInvalid
	}
	return nil
}

// judgeRuntimeInstance 使用本次环境冻结的工作区实例，现场模式不借用平台判题器的实例名。
func judgeRuntimeInstance(task JudgeTask) (string, error) {
	instance := strings.TrimSpace(task.InputSnapshot.CompositionSnapshot.Spec.WorkspaceRuntimeInstance)
	if task.SandboxMode == JudgeSandboxModeReuse {
		instance = task.runtimeInstance
	}
	if instance == "" {
		return "", apperr.ErrJudgerConfigInvalid
	}
	return instance, nil
}

// runChainStep 执行 deploy/tx/reset/query 预处理步骤。
func (s *Service) runChainStep(ctx context.Context, task JudgeTask, sandboxID int64, runtimeInstance string, step map[string]any) (*contracts.JudgeReplayAction, error) {
	runtimeInstance, err := contracts.ChainRuntimeInstance(step, runtimeInstance)
	if err != nil {
		return nil, apperr.ErrJudgerConfigInvalid.WithCause(err)
	}
	op := strings.TrimSpace(jsonx.StringFromAny(step["op"]))
	payload := sanitizeReplayMap(jsonx.ObjectFromAny(step["payload"]))
	switch op {
	case "deploy":
		output, err := s.sandbox.ChainDeploy(ctx, contracts.SandboxChainDeployRequest{TenantID: task.TenantID, SandboxID: sandboxID, SourceRef: sandboxSourceRef(task), RuntimeInstance: runtimeInstance, Payload: jsonx.ObjectFromAny(step["payload"])})
		if err != nil {
			return nil, err
		}
		return &contracts.JudgeReplayAction{Op: op, RuntimeInstance: runtimeInstance, Payload: payload, Output: sanitizeReplayMap(output)}, nil
	case "tx":
		output, err := s.sandbox.ChainSendTx(ctx, contracts.SandboxChainTxRequest{TenantID: task.TenantID, SandboxID: sandboxID, SourceRef: sandboxSourceRef(task), RuntimeInstance: runtimeInstance, Payload: jsonx.ObjectFromAny(step["payload"])})
		if err != nil {
			return nil, err
		}
		return &contracts.JudgeReplayAction{Op: op, RuntimeInstance: runtimeInstance, Payload: payload, Output: sanitizeReplayMap(output)}, nil
	case "reset":
		if err := s.sandbox.ChainReset(ctx, contracts.SandboxChainResetRequest{TenantID: task.TenantID, SandboxID: sandboxID, SourceRef: sandboxSourceRef(task), RuntimeInstance: runtimeInstance}); err != nil {
			return nil, err
		}
		return &contracts.JudgeReplayAction{Op: op, RuntimeInstance: runtimeInstance}, nil
	case "query":
		target := strings.TrimSpace(jsonx.StringField(step, "target"))
		if target == "" {
			return nil, apperr.ErrJudgerConfigInvalid
		}
		output, err := s.sandbox.ChainQuery(ctx, contracts.SandboxChainQueryRequest{TenantID: task.TenantID, SandboxID: sandboxID, SourceRef: sandboxSourceRef(task), RuntimeInstance: runtimeInstance, Target: target})
		if err != nil {
			return nil, err
		}
		return &contracts.JudgeReplayAction{Op: op, RuntimeInstance: runtimeInstance, Payload: map[string]any{"target": target}, Output: sanitizeReplayMap(output)}, nil
	default:
		return nil, apperr.ErrJudgerConfigInvalid
	}
}

// judgeFlag 仅用快照中的 HMAC 摘要比对提交值或链上查询值,不保存明文 flag 或 HMAC secret。
func (s *Service) judgeFlag(ctx context.Context, task JudgeTask, sandboxID int64) (JudgeExecutionResult, error) {
	hash := strings.TrimSpace(jsonx.StringFromAny(task.InputSnapshot.Expectation["flag_hash"]))
	inputKey := strings.TrimSpace(jsonx.StringFromAny(task.InputSnapshot.Expectation["flag_input_key"]))
	if hash == "" || inputKey == "" {
		return JudgeExecutionResult{}, apperr.ErrJudgerConfigInvalid
	}
	submittedHash := strings.TrimSpace(jsonx.StringFromAny(task.InputSnapshot.ExtraInput["submitted_flag_hash"]))
	if target := strings.TrimSpace(jsonx.StringFromAny(task.InputSnapshot.Expectation["flag_chain_target"])); target != "" {
		runtimeInstance, err := judgeRuntimeInstance(task)
		if err != nil {
			return JudgeExecutionResult{}, err
		}
		actual, err := s.sandbox.ChainQuery(ctx, contracts.SandboxChainQueryRequest{TenantID: task.TenantID, SandboxID: sandboxID, SourceRef: sandboxSourceRef(task), RuntimeInstance: runtimeInstance, Target: target})
		if err != nil {
			return JudgeExecutionResult{}, apperr.ErrJudgeWorkerFailed.WithCause(err)
		}
		submittedHash = ""
		field := strings.TrimSpace(jsonx.StringFromAny(task.InputSnapshot.Expectation["flag_chain_field"]))
		submitted := strings.TrimSpace(jsonx.StringFromAny(actual[field]))
		if submitted != "" {
			var err error
			submittedHash, err = pkgcrypto.HMACSHA256Hex(s.hmacKey, strings.TrimSpace(submitted))
			if err != nil {
				return JudgeExecutionResult{}, apperr.ErrJudgerConfigInvalid.WithCause(err)
			}
		}
		if submittedHash == "" {
			actualSummary, summaryErr := chainassert.ShortJSON(actual)
			if summaryErr != nil {
				return JudgeExecutionResult{}, apperr.ErrJudgeWorkerFailed.WithCause(summaryErr)
			}
			submittedHash, err = pkgcrypto.HMACSHA256Hex(s.hmacKey, actualSummary)
			if err != nil {
				return JudgeExecutionResult{}, apperr.ErrJudgerConfigInvalid.WithCause(err)
			}
		}
	}
	if submittedHash == "" {
		return JudgeExecutionResult{}, apperr.ErrJudgerConfigInvalid
	}
	passed := pkgcrypto.EqualHexHMAC(submittedHash, hash)
	score := int32(0)
	if passed {
		score = task.InputSnapshot.MaxScore
	}
	return JudgeExecutionResult{
		Passed:   passed,
		Score:    score,
		MaxScore: task.InputSnapshot.MaxScore,
		Details:  []JudgeResultDetail{{Case: "Flag 校验", Passed: passed, ExpectedLabel: "应提交正确凭证", Actual: flagActualText(passed), Hint: ""}},
	}, nil
}

// judgeSimCheckpoint 对比上层提交的仿真检查点快照。
func judgeSimCheckpoint(task JudgeTask) (JudgeExecutionResult, error) {
	expected := task.InputSnapshot.Expectation["checkpoint"]
	actual := task.InputSnapshot.ExtraInput["checkpoint"]
	actualSummary, err := chainassert.ShortJSON(actual)
	if err != nil {
		return JudgeExecutionResult{}, apperr.ErrJudgerConfigInvalid.WithCause(err)
	}
	passed := reflect.DeepEqual(expected, actual)
	score := int32(0)
	if passed {
		score = task.InputSnapshot.MaxScore
	}
	return JudgeExecutionResult{
		Passed:   passed,
		Score:    score,
		MaxScore: task.InputSnapshot.MaxScore,
		Details:  []JudgeResultDetail{{Case: "仿真检查点", Passed: passed, ExpectedLabel: strings.TrimSpace(jsonx.StringFromAny(task.InputSnapshot.Expectation["expected_label"])), Actual: actualSummary}},
	}, nil
}

// snapshotExpectationForJudger 按题型冻结私有判定依据；该快照不得用于用户响应。
func snapshotExpectationForJudger(typ int16, expectation map[string]any) (map[string]any, error) {
	out := map[string]any{}
	maps.Copy(out, expectation)
	switch typ {
	case JudgerTypeTheoryQuestion:
		definition, err := question.Decode(out, true)
		if err != nil || !definition.Automatic() {
			return nil, apperr.ErrJudgerConfigInvalid.WithCause(err)
		}
		snapshot := definition.Face()
		snapshot["answer"] = out["answer"]
		return snapshot, nil
	case JudgerTypeFlag:
		hash := strings.TrimSpace(jsonx.StringFromAny(out["flag_hash"]))
		if !pkgcrypto.ValidSHA256Hex(hash) {
			return nil, apperr.ErrJudgerConfigInvalid
		}
		inputKey := strings.TrimSpace(jsonx.StringFromAny(out["flag_input_key"]))
		if inputKey == "" {
			return nil, apperr.ErrJudgerConfigInvalid
		}
		safe := map[string]any{"flag_hash": hash, "flag_input_key": inputKey}
		if target := strings.TrimSpace(jsonx.StringFromAny(out["flag_chain_target"])); target != "" {
			safe["flag_chain_target"] = target
			safe["flag_chain_field"] = strings.TrimSpace(jsonx.StringFromAny(out["flag_chain_field"]))
		}
		return safe, nil
	case JudgerTypeOnchainAssert:
		if _, err := validateOutcomeAssertions(out); err != nil {
			return nil, err
		}
		if len(jsonx.SliceFromAny(out["assertions"])) == 0 {
			return nil, apperr.ErrJudgerConfigInvalid
		}
		if !hasDeterministicExpectation(out) {
			return nil, apperr.ErrJudgerConfigInvalid
		}
		return summarizeOnchainExpectation(out), nil
	case JudgerTypeSimCheckpoint:
		if _, ok := out["checkpoint"]; !ok {
			return nil, apperr.ErrJudgerConfigInvalid
		}
		if !hasDeterministicExpectation(out) {
			return nil, apperr.ErrJudgerConfigInvalid
		}
		return summarizeSimCheckpointExpectation(out), nil
	default:
		containsSensitive, err := containsSensitiveExpectationMaterial(out)
		if err != nil {
			return nil, apperr.ErrJudgerConfigInvalid.WithCause(err)
		}
		if containsSensitive {
			return nil, apperr.ErrJudgerConfigInvalid
		}
		if !hasDeterministicExpectation(out) {
			return nil, apperr.ErrJudgerConfigInvalid
		}
		return out, nil
	}
}

// executionExpectationForJudger 只在 worker 执行期恢复 J2/J5 所需的 M5 全量配置,不写回数据库快照。
func executionExpectationForJudger(typ int16, expectation, snapshot map[string]any) (map[string]any, error) {
	switch typ {
	case JudgerTypeOnchainAssert:
		out := maps.Clone(expectation)
		if len(jsonx.SliceFromAny(out["assertions"])) == 0 {
			return nil, apperr.ErrJudgerConfigInvalid
		}
		if !hasDeterministicExpectation(out) {
			return nil, apperr.ErrJudgerConfigInvalid
		}
		return out, nil
	case JudgerTypeSimCheckpoint:
		out := maps.Clone(expectation)
		if _, ok := out["checkpoint"]; !ok {
			return nil, apperr.ErrJudgerConfigInvalid
		}
		if !hasDeterministicExpectation(out) {
			return nil, apperr.ErrJudgerConfigInvalid
		}
		return out, nil
	default:
		return snapshot, nil
	}
}

// summarizeOnchainExpectation 保留链上断言的审计摘要,剥离步骤 payload 和断言标准值。
func summarizeOnchainExpectation(expectation map[string]any) map[string]any {
	assertions := jsonx.SliceFromAny(expectation["assertions"])
	out := map[string]any{
		"assertion_count": len(assertions),
	}
	if steps := jsonx.SliceFromAny(expectation["chain_steps"]); len(steps) > 0 {
		out["chain_step_count"] = len(steps)
	}
	labels := make([]string, 0, len(assertions))
	targets := make([]string, 0, len(assertions))
	operators := make([]string, 0, len(assertions))
	for _, raw := range assertions {
		assertion := jsonx.ObjectFromAny(raw)
		if label := firstNonEmptyString(assertion["expected_label"], assertion["label"], assertion["target"]); label != "" {
			labels = append(labels, label)
		}
		if target := strings.TrimSpace(jsonx.StringFromAny(assertion["target"])); target != "" {
			targets = append(targets, target)
		}
		if op := strings.TrimSpace(jsonx.StringFromAny(assertion["op"])); op != "" {
			operators = append(operators, op)
		}
	}
	if len(labels) > 0 {
		out["expected_labels"] = labels
	}
	if targets = uniqueSortedStrings(targets); len(targets) > 0 {
		out["targets"] = targets
	}
	if operators = uniqueSortedStrings(operators); len(operators) > 0 {
		out["operators"] = operators
	}
	return out
}

// summarizeSimCheckpointExpectation 只保存检查点标签和形态摘要,不保存标准检查点结构。
func summarizeSimCheckpointExpectation(expectation map[string]any) map[string]any {
	out := map[string]any{}
	if label := strings.TrimSpace(jsonx.StringFromAny(expectation["expected_label"])); label != "" {
		out["expected_label"] = label
	}
	checkpoint := expectation["checkpoint"]
	switch typed := checkpoint.(type) {
	case map[string]any:
		out["checkpoint_kind"] = "object"
		out["checkpoint_field_count"] = len(typed)
	case []any:
		out["checkpoint_kind"] = "array"
		out["checkpoint_item_count"] = len(typed)
	default:
		out["checkpoint_kind"] = "scalar"
	}
	return out
}

// firstNonEmptyString 选择第一个非空摘要文本。
func firstNonEmptyString(values ...any) string {
	for _, value := range values {
		if text := strings.TrimSpace(jsonx.StringFromAny(value)); text != "" {
			return text
		}
	}
	return ""
}

// uniqueSortedStrings 返回去重后的稳定字符串集合,保证快照摘要可重复。
func uniqueSortedStrings(values []string) []string {
	set := map[string]struct{}{}
	for _, value := range values {
		if value == "" {
			continue
		}
		set[value] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

// containsSensitiveExpectationMaterial 保守拒绝命令型判题期望中携带敏感明文。
func containsSensitiveExpectationMaterial(v any) (bool, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return false, err
	}
	return privacy.ContainsResultSensitiveText(string(raw)), nil
}

// flagActualText 返回用户向 flag 判题实际状态。
func flagActualText(passed bool) string {
	if passed {
		return "凭证正确"
	}
	return "凭证不正确"
}
