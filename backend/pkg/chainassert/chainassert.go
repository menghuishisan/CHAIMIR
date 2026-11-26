// Package chainassert 判定链上状态并生成脱敏摘要。
package chainassert

import (
	"bytes"
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
	if _, exists := raw["value"]; !exists && op != OperationExists {
		return Assertion{}, fmt.Errorf("链上比较断言必须提供 value")
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

// Check 判定链上查询结果；缺失字段不通过，显式 null 仍是存在的值。
func Check(assertion Assertion, actual map[string]any) (Result, error) {
	field := assertion.Field
	if field == "" {
		field = assertion.Target
	}
	actualValue, exists := actual[field]
	passed := false
	switch assertion.Op {
	case OperationEq, "":
		passed = exists && reflect.DeepEqual(actualValue, assertion.Value)
	case OperationNe:
		passed = exists && !reflect.DeepEqual(actualValue, assertion.Value)
	case OperationContains:
		passed = exists && strings.Contains(fmt.Sprint(actualValue), fmt.Sprint(assertion.Value))
	case OperationExists:
		passed = exists
	default:
		return Result{}, fmt.Errorf("链上断言 op 不受支持: %s", assertion.Op)
	}
	actualSummary, err := ShortJSON(actual)
	if err != nil {
		return Result{}, err
	}
	label := assertion.Label
	if label == "" {
		label = "链上状态检查"
	}
	return Result{Case: label, Passed: passed, ExpectedLabel: assertion.ExpectedLabel, Actual: actualSummary, Hint: assertion.Hint}, nil
}

// ShortJSON 返回脱敏短文本,避免把完整状态或期望结构传到前端。
func ShortJSON(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("序列化链上状态失败: %w", err)
	}
	// 统一成 JSON 形状，具体 Go 类型的字段同样经过脱敏。
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return "", fmt.Errorf("解析链上状态失败: %w", err)
	}
	raw, err = json.Marshal(redactSensitiveValues(value))
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
			if !privacy.IsResultSensitiveKey(key) {
				out[key] = redactSensitiveValues(value)
			}
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, value := range x {
			out[i] = redactSensitiveValues(value)
		}
		return out
	case string:
		if privacy.ContainsResultSensitiveText(x) {
			return redactedValue
		}
		return x
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
