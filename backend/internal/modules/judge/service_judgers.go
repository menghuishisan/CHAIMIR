// M3 判题器策略:按 judger.type 分发 J1-J6 的执行与结果判定逻辑。
package judge

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
)

// executeJudgerStrategy 按判题器类型执行对应策略。
func (s *Service) executeJudgerStrategy(ctx context.Context, sandboxID int64, taskSnapshot map[string]any, judgerType int16, spec JudgerResourceSpec, defaultTimeout int32) (JudgeExecutionResult, error) {
	expectation := jsonx.ObjectFromAny(taskSnapshot["expectation"])
	extraInput := jsonx.ObjectFromAny(taskSnapshot["extra_input"])
	maxScore := jsonx.Int32FromAny(taskSnapshot["max_score"], 100)
	switch judgerType {
	case JudgerTypeTestcase, JudgerTypeStaticScan:
		return s.runJudgeCommand(ctx, sandboxID, spec, defaultTimeout)
	case JudgerTypeOnchainAssert:
		return s.judgeOnchainAssertions(ctx, sandboxID, expectation, maxScore)
	case JudgerTypeFlag:
		return judgeFlag(expectation, extraInput, maxScore)
	case JudgerTypeSimCheckpoint:
		return judgeSimCheckpoint(expectation, extraInput, maxScore)
	default:
		return JudgeExecutionResult{}, apperr.ErrJudgerInvalid
	}
}

// judgeOnchainAssertions 执行链上步骤并按 expectation.assertions 校验结果。
func (s *Service) judgeOnchainAssertions(ctx context.Context, sandboxID int64, expectation map[string]any, maxScore int32) (JudgeExecutionResult, error) {
	// 第一步执行可选链上步骤,结果按 name 保存供断言引用。
	results := map[string]any{}
	for _, step := range jsonx.SliceFromAny(expectation["chain_steps"]) {
		item := jsonx.ObjectFromAny(step)
		name := assertionString(item["name"])
		action := assertionString(item["action"])
		result, err := s.runChainStep(ctx, sandboxID, action, item)
		if err != nil {
			return JudgeExecutionResult{}, err
		}
		if name != "" {
			results[name] = result
		}
	}
	// 第二步逐条断言链上结果,任一失败即判定不通过但仍返回可解释详情。
	assertions := jsonx.SliceFromAny(expectation["assertions"])
	if len(assertions) == 0 {
		return JudgeExecutionResult{}, apperr.ErrJudgerInvalid
	}
	details := []map[string]any{}
	passed := true
	for _, raw := range assertions {
		assertion := jsonx.ObjectFromAny(raw)
		source := assertionString(assertion["source"])
		target := assertionString(assertion["target"])
		op := assertionString(assertion["op"])
		sourceValues := jsonx.ObjectFromAny(results[source])
		actual, exists := sourceValues[target]
		err := checkAssertionValue(sourceValues, target, op, assertion["expected"])
		item := map[string]any{
			"source": source, "target": target, "op": op, "passed": err == nil,
			"expected_label": expectedLabel(assertion, target, op),
			"actual":         actual,
		}
		if op == "exists" && !exists {
			item["actual"] = nil
		}
		if err != nil {
			if errorsIsJudgerInvalid(err) {
				return JudgeExecutionResult{}, err
			}
			passed = false
			item["hint"] = "链上状态不符合要求"
		}
		details = append(details, item)
	}
	score := int32(0)
	if passed {
		score = maxScore
	}
	return JudgeExecutionResult{Passed: passed, Score: score, MaxScore: maxScore, Details: details}, nil
}

// runChainStep 调用 M2 链上 contract 执行单个步骤。
func (s *Service) runChainStep(ctx context.Context, sandboxID int64, action string, step map[string]any) (map[string]any, error) {
	switch action {
	case "deploy":
		return s.sandbox.ChainDeploy(ctx, sandboxID, jsonx.ObjectFromAny(step["payload"]))
	case "tx":
		return s.sandbox.ChainSendTx(ctx, sandboxID, jsonx.ObjectFromAny(step["payload"]))
	case "query":
		return s.sandbox.ChainQuery(ctx, sandboxID, assertionString(step["target"]))
	case "reset":
		if err := s.sandbox.ChainReset(ctx, sandboxID); err != nil {
			return nil, err
		}
		return map[string]any{"reset": true}, nil
	default:
		return nil, apperr.ErrJudgerInvalid
	}
}

// judgeFlag 按 expectation 中的 flag_hash 或 flag_value 校验提交值。
func judgeFlag(expectation, extraInput map[string]any, maxScore int32) (JudgeExecutionResult, error) {
	key := assertionString(expectation["flag_input_key"])
	if key == "" {
		key = "flag"
	}
	submitted := assertionString(extraInput[key])
	if submitted == "" {
		return JudgeExecutionResult{}, apperr.ErrJudgeTaskInvalid
	}
	expectedHash := assertionString(expectation["flag_hash"])
	passed := false
	if expectedHash != "" {
		if !isSHA256Hex(expectedHash) {
			return JudgeExecutionResult{}, apperr.ErrJudgerInvalid
		}
		sum := sha256.Sum256([]byte(submitted))
		passed = strings.EqualFold(hex.EncodeToString(sum[:]), expectedHash)
	} else {
		return JudgeExecutionResult{}, apperr.ErrJudgerInvalid
	}
	score := int32(0)
	if passed {
		score = maxScore
	}
	return JudgeExecutionResult{
		Passed: passed, Score: score, MaxScore: maxScore,
		Details: []map[string]any{{"case": "flag", "passed": passed}},
	}, nil
}

// snapshotExpectationForJudger 清洗并固化判题期望,避免答案正本进入 M3 input_snapshot。
func snapshotExpectationForJudger(req contracts.JudgeSubmitRequest, problemRef string, judgerType int16, expectation map[string]any) (map[string]any, error) {
	out := jsonx.CloneObject(expectation)
	if judgerType != JudgerTypeFlag {
		if containsSensitiveExpectationMaterial(out) {
			return nil, apperr.ErrJudgerInvalid
		}
		return out, nil
	}
	if secret := assertionString(out["flag_hmac_secret"]); secret != "" {
		seed := assertionString(out["flag_hmac_seed"])
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(fmt.Sprintf("%d:%d:%s:%s", req.TenantID, req.SubmitterID, problemRef, seed)))
		expected := hex.EncodeToString(mac.Sum(nil))
		sum := sha256.Sum256([]byte(expected))
		out["flag_hash"] = hex.EncodeToString(sum[:])
		out["flag_mode"] = "hmac_sha256"
		delete(out, "flag_hmac_secret")
		return out, nil
	}
	if value := assertionString(out["flag_value"]); value != "" {
		sum := sha256.Sum256([]byte(value))
		out["flag_hash"] = hex.EncodeToString(sum[:])
		out["flag_mode"] = "sha256"
		delete(out, "flag_value")
		return out, nil
	}
	if assertionString(out["flag_hash"]) != "" {
		if !isSHA256Hex(assertionString(out["flag_hash"])) {
			return nil, apperr.ErrJudgerInvalid
		}
		delete(out, "flag_value")
		delete(out, "flag_hmac_secret")
		return out, nil
	}
	return nil, apperr.ErrJudgerInvalid
}

// isSHA256Hex 校验 flag_hash 使用 64 位十六进制 SHA-256 表达。
func isSHA256Hex(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

// containsSensitiveExpectationMaterial 拒绝把答案正本、密钥或完整套件源码保存进 M3 快照。
func containsSensitiveExpectationMaterial(v any) bool {
	switch item := v.(type) {
	case map[string]any:
		for key, value := range item {
			normalized := strings.ToLower(strings.ReplaceAll(key, "_", ""))
			for _, marker := range []string{"answer", "secret", "suitesource", "testsource", "privatekey", "mnemonic", "solution"} {
				if strings.Contains(normalized, marker) {
					return true
				}
			}
			if containsSensitiveExpectationMaterial(value) {
				return true
			}
		}
	case []any:
		for _, value := range item {
			if containsSensitiveExpectationMaterial(value) {
				return true
			}
		}
	}
	return false
}

// judgeSimCheckpoint 对比上层提交的仿真检查点快照与 M5 期望检查点。
func judgeSimCheckpoint(expectation, extraInput map[string]any, maxScore int32) (JudgeExecutionResult, error) {
	expected := expectation["checkpoint"]
	actual := extraInput["checkpoint"]
	if expected == nil || actual == nil {
		return JudgeExecutionResult{}, apperr.ErrJudgeTaskInvalid
	}
	passed := jsonx.Equal(actual, expected)
	score := int32(0)
	if passed {
		score = maxScore
	}
	return JudgeExecutionResult{
		Passed: passed, Score: score, MaxScore: maxScore,
		Details: []map[string]any{{
			"case":           "sim_checkpoint",
			"passed":         passed,
			"expected_label": checkpointExpectedLabel(expectation),
			"actual":         actual,
		}},
	}, nil
}

// expectedLabel 生成链上断言结果中的期望摘要,避免泄露完整标准答案结构。
func expectedLabel(assertion map[string]any, target, op string) string {
	if label := assertionString(assertion["label"]); label != "" {
		return label
	}
	if label := assertionString(assertion["expected_label"]); label != "" {
		return label
	}
	if target != "" && op != "" {
		return target + " " + op
	}
	return "链上断言应满足"
}

// checkpointExpectedLabel 生成仿真检查点结果中的期望摘要。
func checkpointExpectedLabel(expectation map[string]any) string {
	if label := assertionString(expectation["checkpoint_label"]); label != "" {
		return label
	}
	if label := assertionString(expectation["expected_label"]); label != "" {
		return label
	}
	return "仿真检查点应匹配"
}

// checkAssertionValue 对链上步骤结果执行单条断言。
func checkAssertionValue(source map[string]any, target, op string, expected any) error {
	actual, ok := source[target]
	if op == "exists" {
		if ok && actual != nil {
			return nil
		}
		return apperr.ErrJudgeTaskRunFail
	}
	if !ok {
		return apperr.ErrJudgeTaskRunFail
	}
	switch op {
	case "eq":
		if fmt.Sprint(actual) == fmt.Sprint(expected) {
			return nil
		}
	case "ne":
		if fmt.Sprint(actual) != fmt.Sprint(expected) {
			return nil
		}
	case "contains":
		if containsValue(actual, expected) {
			return nil
		}
	default:
		return apperr.ErrJudgerInvalid
	}
	return apperr.ErrJudgeTaskRunFail
}

// errorsIsJudgerInvalid 判断断言失败是否属于判题器配置错误。
func errorsIsJudgerInvalid(err error) bool {
	if ae, ok := apperr.As(err); ok {
		return ae.Code == apperr.ErrJudgerInvalid.Code
	}
	return false
}

// containsValue 判断字符串或数组是否包含期望值。
func containsValue(actual, expected any) bool {
	want := fmt.Sprint(expected)
	if strings.Contains(fmt.Sprint(actual), want) {
		return true
	}
	for _, item := range jsonx.SliceFromAny(actual) {
		if fmt.Sprint(item) == want {
			return true
		}
	}
	return false
}

// assertionString 把判题期望或执行结果值转为可比较字符串,用于断言配置的宽松匹配。
func assertionString(v any) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}
