// judge service_judgers 文件实现内置后端判题策略和快照脱敏规则。
package judge

import (
	"context"
	"reflect"
	"sort"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
	"chaimir/pkg/chainassert"
	pkgcrypto "chaimir/pkg/crypto"
	"chaimir/pkg/privacy"
)

// executeJudgerStrategy 执行不需要容器命令的后端内置判题策略。
func (s *Service) executeJudgerStrategy(ctx context.Context, task JudgeTask, sandboxID int64) (JudgeExecutionResult, bool, error) {
	switch task.InputSnapshot.JudgerType {
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
	steps := sliceValue(task.InputSnapshot.Expectation["chain_steps"])
	for _, step := range steps {
		if err := s.runChainStep(ctx, task, sandboxID, mapAny(step)); err != nil {
			return JudgeExecutionResult{}, err
		}
	}
	assertions := sliceValue(task.InputSnapshot.Expectation["assertions"])
	if len(assertions) == 0 {
		return JudgeExecutionResult{}, apperr.ErrJudgerConfigInvalid
	}
	details := make([]JudgeResultDetail, 0, len(assertions))
	passed := true
	for _, raw := range assertions {
		assertion := mapAny(raw)
		actual, err := s.sandbox.ChainQuery(ctx, contracts.SandboxChainQueryRequest{TenantID: task.TenantID, SandboxID: sandboxID, SourceRef: task.SourceRef, Target: stringValue(assertion["target"])})
		if err != nil {
			return JudgeExecutionResult{}, apperr.ErrJudgeWorkerFailed.WithCause(err)
		}
		result := chainassert.Check(chainassert.FromMap(assertion), actual)
		if !result.Passed {
			passed = false
		}
		details = append(details, JudgeResultDetail{
			Case:          result.Case,
			Passed:        result.Passed,
			ExpectedLabel: result.ExpectedLabel,
			Actual:        result.Actual,
			Hint:          result.Hint,
		})
	}
	score := int32(0)
	if passed {
		score = task.InputSnapshot.MaxScore
	}
	return JudgeExecutionResult{Passed: passed, Score: score, MaxScore: task.InputSnapshot.MaxScore, Details: details}, nil
}

// runChainStep 执行 deploy/tx/reset/query 预处理步骤。
func (s *Service) runChainStep(ctx context.Context, task JudgeTask, sandboxID int64, step map[string]any) error {
	switch stringValue(step["op"]) {
	case "deploy":
		_, err := s.sandbox.ChainDeploy(ctx, contracts.SandboxChainDeployRequest{TenantID: task.TenantID, SandboxID: sandboxID, SourceRef: task.SourceRef, Payload: mapAny(step["payload"])})
		return err
	case "tx":
		_, err := s.sandbox.ChainSendTx(ctx, contracts.SandboxChainTxRequest{TenantID: task.TenantID, SandboxID: sandboxID, SourceRef: task.SourceRef, Payload: mapAny(step["payload"])})
		return err
	case "reset":
		return s.sandbox.ChainReset(ctx, contracts.SandboxChainResetRequest{TenantID: task.TenantID, SandboxID: sandboxID, SourceRef: task.SourceRef})
	case "query", "":
		return nil
	default:
		return apperr.ErrJudgerConfigInvalid
	}
}

// judgeFlag 仅用快照中的 HMAC 摘要比对提交值或链上查询值,不保存明文 flag 或 HMAC secret。
func (s *Service) judgeFlag(ctx context.Context, task JudgeTask, sandboxID int64) (JudgeExecutionResult, error) {
	hash := stringValue(task.InputSnapshot.Expectation["flag_hash"])
	inputKey := stringValue(task.InputSnapshot.Expectation["flag_input_key"])
	if hash == "" || inputKey == "" {
		return JudgeExecutionResult{}, apperr.ErrJudgerConfigInvalid
	}
	submittedHash := stringValue(task.InputSnapshot.ExtraInput["submitted_flag_hash"])
	if target := stringValue(task.InputSnapshot.Expectation["flag_chain_target"]); target != "" {
		actual, err := s.sandbox.ChainQuery(ctx, contracts.SandboxChainQueryRequest{TenantID: task.TenantID, SandboxID: sandboxID, SourceRef: task.SourceRef, Target: target})
		if err != nil {
			return JudgeExecutionResult{}, apperr.ErrJudgeWorkerFailed.WithCause(err)
		}
		submittedHash = ""
		submitted := stringValue(actual[stringValue(task.InputSnapshot.Expectation["flag_chain_field"])])
		if submitted != "" {
			var err error
			submittedHash, err = pkgcrypto.HMACSHA256Hex(s.hmacKey, strings.TrimSpace(submitted))
			if err != nil {
				return JudgeExecutionResult{}, apperr.ErrJudgerConfigInvalid.WithCause(err)
			}
		}
		if submittedHash == "" {
			submittedHash, err = pkgcrypto.HMACSHA256Hex(s.hmacKey, chainassert.ShortJSON(actual))
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
	passed := reflect.DeepEqual(expected, actual)
	score := int32(0)
	if passed {
		score = task.InputSnapshot.MaxScore
	}
	return JudgeExecutionResult{
		Passed:   passed,
		Score:    score,
		MaxScore: task.InputSnapshot.MaxScore,
		Details:  []JudgeResultDetail{{Case: "仿真检查点", Passed: passed, ExpectedLabel: stringValue(task.InputSnapshot.Expectation["expected_label"]), Actual: chainassert.ShortJSON(actual)}},
	}, nil
}

// snapshotExpectationForJudger 生成可复现但不泄露答案的快照期望。
func (s *Service) snapshotExpectationForJudger(typ int16, expectation map[string]any, _ map[string]any) (map[string]any, error) {
	out := map[string]any{}
	for k, v := range expectation {
		out[k] = v
	}
	switch typ {
	case JudgerTypeFlag:
		hash := stringValue(out["flag_hash"])
		if !isSHA256Hex(hash) {
			return nil, apperr.ErrJudgerConfigInvalid
		}
		inputKey := stringValue(out["flag_input_key"])
		if inputKey == "" {
			return nil, apperr.ErrJudgerConfigInvalid
		}
		safe := map[string]any{"flag_hash": hash, "flag_input_key": inputKey}
		if target := stringValue(out["flag_chain_target"]); target != "" {
			safe["flag_chain_target"] = target
			safe["flag_chain_field"] = stringValue(out["flag_chain_field"])
		}
		return safe, nil
	case JudgerTypeOnchainAssert:
		if len(sliceValue(out["assertions"])) == 0 {
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
		if containsSensitiveExpectationMaterial(out) {
			return nil, apperr.ErrJudgerConfigInvalid
		}
		if !hasDeterministicExpectation(out) {
			return nil, apperr.ErrJudgerConfigInvalid
		}
		return out, nil
	}
	return out, nil
}

// executionExpectationForJudger 只在 worker 执行期恢复 J2/J5 所需的 M5 全量配置,不写回数据库快照。
func (s *Service) executionExpectationForJudger(typ int16, spec contracts.ContentJudgeSpec, snapshot map[string]any) (map[string]any, error) {
	switch typ {
	case JudgerTypeOnchainAssert:
		out := cloneExpectationMap(spec.Expectation)
		if len(sliceValue(out["assertions"])) == 0 {
			return nil, apperr.ErrJudgerConfigInvalid
		}
		if !hasDeterministicExpectation(out) {
			return nil, apperr.ErrJudgerConfigInvalid
		}
		return out, nil
	case JudgerTypeSimCheckpoint:
		out := cloneExpectationMap(spec.Expectation)
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
	assertions := sliceValue(expectation["assertions"])
	out := map[string]any{
		"assertion_count": len(assertions),
	}
	if steps := sliceValue(expectation["chain_steps"]); len(steps) > 0 {
		out["chain_step_count"] = len(steps)
	}
	labels := make([]string, 0, len(assertions))
	targets := make([]string, 0, len(assertions))
	operators := make([]string, 0, len(assertions))
	for _, raw := range assertions {
		assertion := mapAny(raw)
		if label := firstNonEmptyString(assertion["expected_label"], assertion["label"], assertion["target"]); label != "" {
			labels = append(labels, label)
		}
		if target := stringValue(assertion["target"]); target != "" {
			targets = append(targets, target)
		}
		if op := stringValue(assertion["op"]); op != "" {
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
	if label := stringValue(expectation["expected_label"]); label != "" {
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

// cloneExpectationMap 复制 M5 配置顶层 map,避免运行期修改污染契约返回值。
func cloneExpectationMap(expectation map[string]any) map[string]any {
	out := make(map[string]any, len(expectation))
	for k, v := range expectation {
		out[k] = v
	}
	return out
}

// firstNonEmptyString 选择第一个非空摘要文本。
func firstNonEmptyString(values ...any) string {
	for _, value := range values {
		if text := stringValue(value); text != "" {
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
func containsSensitiveExpectationMaterial(v any) bool {
	raw := strings.ToLower(chainassert.ShortJSON(v))
	return privacy.ContainsResultSensitiveText(raw)
}

// mapAny 读取 map[string]any。
func mapAny(v any) map[string]any {
	return jsonx.ObjectFromAny(v)
}

// sliceValue 读取 []any。
func sliceValue(v any) []any {
	return jsonx.SliceFromAny(v)
}

// stringValue 读取字符串值。
func stringValue(v any) string {
	return strings.TrimSpace(jsonx.StringFromAny(v))
}

// flagActualText 返回用户向 flag 判题实际状态。
func flagActualText(passed bool) string {
	if passed {
		return "凭证正确"
	}
	return "凭证不正确"
}
