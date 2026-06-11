// judge judgers 文件实现内置后端判题策略和快照脱敏规则。
package judge

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/pkg/apperr"
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
		ok := checkAssertionValue(assertion, actual)
		if !ok {
			passed = false
		}
		details = append(details, JudgeResultDetail{
			Case:          stringValue(assertion["label"]),
			Passed:        ok,
			ExpectedLabel: stringValue(assertion["expected_label"]),
			Actual:        shortJSON(actual),
			Hint:          stringValue(assertion["hint"]),
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

// judgeFlag 仅用快照中的 hash 比对提交值或链上查询值,不保存明文 flag 或 HMAC secret。
func (s *Service) judgeFlag(ctx context.Context, task JudgeTask, sandboxID int64) (JudgeExecutionResult, error) {
	hash := stringValue(task.InputSnapshot.Expectation["flag_hash"])
	inputKey := stringValue(task.InputSnapshot.Expectation["flag_input_key"])
	if hash == "" || inputKey == "" {
		return JudgeExecutionResult{}, apperr.ErrJudgerConfigInvalid
	}
	submitted := stringValue(task.InputSnapshot.ExtraInput[inputKey])
	if target := stringValue(task.InputSnapshot.Expectation["flag_chain_target"]); target != "" {
		actual, err := s.sandbox.ChainQuery(ctx, contracts.SandboxChainQueryRequest{TenantID: task.TenantID, SandboxID: sandboxID, SourceRef: task.SourceRef, Target: target})
		if err != nil {
			return JudgeExecutionResult{}, apperr.ErrJudgeWorkerFailed.WithCause(err)
		}
		submitted = stringValue(actual[stringValue(task.InputSnapshot.Expectation["flag_chain_field"])])
		if submitted == "" {
			submitted = shortJSON(actual)
		}
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(submitted)))
	passed := hex.EncodeToString(sum[:]) == hash
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
		Details:  []JudgeResultDetail{{Case: "仿真检查点", Passed: passed, ExpectedLabel: stringValue(task.InputSnapshot.Expectation["expected_label"]), Actual: shortJSON(actual)}},
	}, nil
}

// snapshotExpectationForJudger 生成可复现但不泄露答案的快照期望。
func snapshotExpectationForJudger(typ int16, expectation map[string]any, extra map[string]any) (map[string]any, error) {
	out := map[string]any{}
	for k, v := range expectation {
		out[k] = v
	}
	switch typ {
	case JudgerTypeFlag:
		hash := stringValue(out["flag_hash"])
		if hash == "" {
			if value := stringValue(out["flag_value"]); value != "" {
				sum := sha256.Sum256([]byte(value))
				hash = hex.EncodeToString(sum[:])
			}
		}
		if hash == "" {
			secret := stringValue(out["flag_hmac_secret"])
			seed := stringValue(extra["flag_seed"])
			if seed == "" {
				seed = stringValue(out["flag_seed"])
			}
			if secret != "" && seed != "" {
				mac := hmac.New(sha256.New, []byte(secret))
				mac.Write([]byte(seed))
				expected := hex.EncodeToString(mac.Sum(nil))
				sum := sha256.Sum256([]byte(expected))
				hash = hex.EncodeToString(sum[:])
			}
		}
		if !isSHA256Hex(hash) {
			return nil, apperr.ErrJudgerConfigInvalid
		}
		inputKey := stringValue(out["flag_input_key"])
		if inputKey == "" {
			inputKey = "flag"
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
		return out, nil
	case JudgerTypeSimCheckpoint:
		if _, ok := out["checkpoint"]; !ok {
			return nil, apperr.ErrJudgerConfigInvalid
		}
		if !hasDeterministicExpectation(out) {
			return nil, apperr.ErrJudgerConfigInvalid
		}
		return out, nil
	default:
		if containsSensitiveExpectationMaterial(out) {
			return nil, apperr.ErrJudgerConfigInvalid
		}
		if !hasDeterministicExpectation(out) {
			return nil, apperr.ErrJudgerConfigInvalid
		}
		return out, nil
	}
	_ = extra
	return out, nil
}

// containsSensitiveExpectationMaterial 保守拒绝命令型判题期望中携带敏感明文。
func containsSensitiveExpectationMaterial(v any) bool {
	raw := strings.ToLower(shortJSON(v))
	for _, key := range []string{"flag_value", "flag_hmac_secret", "private_key", "answer_source", "suite_source"} {
		if strings.Contains(raw, key) {
			return true
		}
	}
	return false
}

// checkAssertionValue 执行基础断言操作。
func checkAssertionValue(assertion map[string]any, actual map[string]any) bool {
	target := stringValue(assertion["field"])
	if target == "" {
		target = stringValue(assertion["target"])
	}
	actualValue := actual[target]
	expected := assertion["value"]
	switch stringValue(assertion["op"]) {
	case "eq", "":
		return reflect.DeepEqual(actualValue, expected)
	case "ne":
		return !reflect.DeepEqual(actualValue, expected)
	case "contains":
		return strings.Contains(fmt.Sprint(actualValue), fmt.Sprint(expected))
	case "exists":
		_, ok := actual[target]
		return ok
	default:
		return false
	}
}

// mapAny 读取 map[string]any。
func mapAny(v any) map[string]any {
	if out, ok := v.(map[string]any); ok {
		return out
	}
	return map[string]any{}
}

// sliceValue 读取 []any。
func sliceValue(v any) []any {
	if out, ok := v.([]any); ok {
		return out
	}
	return nil
}

// stringValue 读取字符串值。
func stringValue(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	default:
		return strings.TrimSpace(fmt.Sprint(x))
	}
}

// shortJSON 返回脱敏短文本,避免把完整结构回传给前端。
func shortJSON(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	text := string(raw)
	if len(text) > 256 {
		return text[:256]
	}
	return text
}

// flagActualText 返回用户向 flag 判题实际状态。
func flagActualText(passed bool) string {
	if passed {
		return "凭证正确"
	}
	return "凭证不正确"
}
