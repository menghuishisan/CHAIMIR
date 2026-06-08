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
	expectation := mapValue(taskSnapshot, "expectation")
	extraInput := mapValue(taskSnapshot, "extra_input")
	maxScore := int32Value(taskSnapshot["max_score"], 100)
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
	for _, step := range sliceValue(expectation["chain_steps"]) {
		item := mapAny(step)
		name := stringValue(item["name"])
		action := stringValue(item["action"])
		result, err := s.runChainStep(ctx, sandboxID, action, item)
		if err != nil {
			return JudgeExecutionResult{}, err
		}
		if name != "" {
			results[name] = result
		}
	}
	// 第二步逐条断言链上结果,任一失败即判定不通过但仍返回可解释详情。
	assertions := sliceValue(expectation["assertions"])
	if len(assertions) == 0 {
		return JudgeExecutionResult{}, apperr.ErrJudgerInvalid
	}
	details := []map[string]any{}
	passed := true
	for _, raw := range assertions {
		assertion := mapAny(raw)
		source := stringValue(assertion["source"])
		target := stringValue(assertion["target"])
		op := stringValue(assertion["op"])
		sourceValues := mapAny(results[source])
		actual, exists := sourceValues[target]
		err := checkAssertionValue(sourceValues, target, op, assertion["expected"])
		item := map[string]any{
			"source": source, "target": target, "op": op, "passed": err == nil,
			"expected": assertion["expected"],
			"actual":   actual,
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
		return s.sandbox.ChainDeploy(ctx, sandboxID, mapValue(step, "payload"))
	case "tx":
		return s.sandbox.ChainSendTx(ctx, sandboxID, mapValue(step, "payload"))
	case "query":
		return s.sandbox.ChainQuery(ctx, sandboxID, stringValue(step["target"]))
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
	key := stringValue(expectation["flag_input_key"])
	if key == "" {
		key = "flag"
	}
	submitted := stringValue(extraInput[key])
	if submitted == "" {
		return JudgeExecutionResult{}, apperr.ErrJudgeTaskInvalid
	}
	expectedHash := stringValue(expectation["flag_hash"])
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
	if secret := stringValue(out["flag_hmac_secret"]); secret != "" {
		seed := stringValue(out["flag_hmac_seed"])
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(fmt.Sprintf("%d:%d:%s:%s", req.TenantID, req.SubmitterID, problemRef, seed)))
		expected := hex.EncodeToString(mac.Sum(nil))
		sum := sha256.Sum256([]byte(expected))
		out["flag_hash"] = hex.EncodeToString(sum[:])
		out["flag_mode"] = "hmac_sha256"
		delete(out, "flag_hmac_secret")
		return out, nil
	}
	if value := stringValue(out["flag_value"]); value != "" {
		sum := sha256.Sum256([]byte(value))
		out["flag_hash"] = hex.EncodeToString(sum[:])
		out["flag_mode"] = "sha256"
		delete(out, "flag_value")
		return out, nil
	}
	if stringValue(out["flag_hash"]) != "" {
		if !isSHA256Hex(stringValue(out["flag_hash"])) {
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
			"case":     "sim_checkpoint",
			"passed":   passed,
			"expected": expected,
			"actual":   actual,
		}},
	}, nil
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
	for _, item := range sliceValue(actual) {
		if fmt.Sprint(item) == want {
			return true
		}
	}
	return false
}

// mapValue 从 map 中取子对象,不存在时返回空对象。
func mapValue(source map[string]any, key string) map[string]any {
	return mapAny(source[key])
}

// mapAny 把 any 归一为 map[string]any。
func mapAny(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

// sliceValue 把 any 归一为 []any。
func sliceValue(v any) []any {
	if s, ok := v.([]any); ok {
		return s
	}
	return []any{}
}

// stringValue 把 any 转成去空白字符串。
func stringValue(v any) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

// int32Value 把 JSON 数字转 int32,无效时返回默认值。
func int32Value(v any, defaultValue int32) int32 {
	switch n := v.(type) {
	case int32:
		return n
	case int:
		return int32(n)
	case int64:
		return int32(n)
	case float64:
		return int32(n)
	default:
		return defaultValue
	}
}
