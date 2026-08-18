// chainassert 包提供链上断言的通用判定能力,供 M3 判题和 M8 漏洞预验证复用。
package chainassert

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"chaimir/pkg/privacy"
	"chaimir/pkg/textx"
)

const (
	shortJSONRuneLimit = 256
	redactedValue      = "[已脱敏]"
)

const (
	// OperationEq 表示实际值必须等于期望值。
	OperationEq = "eq"
	// OperationNe 表示实际值必须不等于期望值。
	OperationNe = "ne"
	// OperationContains 表示实际文本必须包含期望文本。
	OperationContains = "contains"
	// OperationExists 表示目标字段必须存在。
	OperationExists = "exists"
)

// Assertion 是链上状态断言配置。
type Assertion struct {
	Label         string
	Target        string
	Field         string
	Op            string
	Value         any
	ExpectedLabel string
	Hint          string
}

// Result 是单条断言的脱敏判定结果。
type Result struct {
	Case          string
	Passed        bool
	ExpectedLabel string
	Actual        string
	Hint          string
}

// FromMap 严格解析链上断言协议字段,拒绝非字符串字段、空目标和未知操作符。
func FromMap(raw map[string]any) (Assertion, error) {
	label, err := optionalString(raw, "label")
	if err != nil {
		return Assertion{}, err
	}
	target, err := optionalString(raw, "target")
	if err != nil {
		return Assertion{}, err
	}
	if target == "" {
		return Assertion{}, fmt.Errorf("链上断言 target 必须是非空字符串")
	}
	field, err := optionalString(raw, "field")
	if err != nil {
		return Assertion{}, err
	}
	op, err := optionalString(raw, "op")
	if err != nil {
		return Assertion{}, err
	}
	if op == "" {
		op = OperationEq
	}
	switch op {
	case OperationEq, OperationNe, OperationContains, OperationExists:
	default:
		return Assertion{}, fmt.Errorf("链上断言 op 不受支持: %s", op)
	}
	expectedLabel, err := optionalString(raw, "expected_label")
	if err != nil {
		return Assertion{}, err
	}
	hint, err := optionalString(raw, "hint")
	if err != nil {
		return Assertion{}, err
	}
	return Assertion{Label: label, Target: target, Field: field, Op: op, Value: raw["value"], ExpectedLabel: expectedLabel, Hint: hint}, nil
}

// Check 对单条链上查询结果执行断言。
func Check(assertion Assertion, actual map[string]any) (Result, error) {
	field := assertion.Field
	if field == "" {
		field = assertion.Target
	}
	actualValue := actual[field]
	passed := false
	switch assertion.Op {
	case OperationEq, "":
		passed = reflect.DeepEqual(actualValue, assertion.Value)
	case OperationNe:
		passed = !reflect.DeepEqual(actualValue, assertion.Value)
	case OperationContains:
		passed = strings.Contains(fmt.Sprint(actualValue), fmt.Sprint(assertion.Value))
	case OperationExists:
		_, passed = actual[field]
	default:
		return Result{}, fmt.Errorf("链上断言 op 不受支持: %s", assertion.Op)
	}
	actualSummary, err := ShortJSON(actual)
	if err != nil {
		return Result{}, err
	}
	return Result{Case: assertion.Label, Passed: passed, ExpectedLabel: assertion.ExpectedLabel, Actual: actualSummary, Hint: assertion.Hint}, nil
}

// ShortJSON 返回脱敏短文本,避免把完整状态或期望结构传到前端。
func ShortJSON(v any) (string, error) {
	raw, err := json.Marshal(redactSensitiveValues(v))
	if err != nil {
		return "", fmt.Errorf("序列化链上断言摘要失败: %w", err)
	}
	return textx.TruncateRunes(string(raw), shortJSONRuneLimit), nil
}

// redactSensitiveValues 在链上断言摘要序列化前按字段名递归脱敏,避免截断文本仍泄露密钥或 flag。
func redactSensitiveValues(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for key, value := range x {
			if privacy.IsResultSensitiveKey(key) {
				out[key] = redactedValue
				continue
			}
			out[key] = redactSensitiveValues(value)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, value := range x {
			out[i] = redactSensitiveValues(value)
		}
		return out
	default:
		return v
	}
}

// optionalString 读取可选协议字符串;字段存在但类型错误时显式失败。
func optionalString(raw map[string]any, key string) (string, error) {
	value, exists := raw[key]
	if !exists || value == nil {
		return "", nil
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("链上断言 %s 必须是字符串", key)
	}
	return strings.TrimSpace(text), nil
}
