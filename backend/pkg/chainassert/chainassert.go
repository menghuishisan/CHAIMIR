// chainassert 包提供链上断言的通用判定能力,供 M3 判题和 M8 漏洞预验证复用。
package chainassert

import (
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"slices"
	"strings"

	"chaimir/pkg/privacy"
)

const (
	shortJSONRuneLimit = 256
	redactedValue      = "[已脱敏]"
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

// FromMap 将 JSON map 解析为断言配置。
func FromMap(raw map[string]any) Assertion {
	return Assertion{
		Label:         stringValue(raw["label"]),
		Target:        stringValue(raw["target"]),
		Field:         stringValue(raw["field"]),
		Op:            stringValue(raw["op"]),
		Value:         raw["value"],
		ExpectedLabel: stringValue(raw["expected_label"]),
		Hint:          stringValue(raw["hint"]),
	}
}

// Validate 校验链上断言的唯一字段集合和可执行比较符。
func Validate(raw map[string]any) bool {
	if len(raw) != 6 || !hasOnlyKeys(raw, "label", "target", "field", "op", "value", "expected_label") {
		return false
	}
	assertion := FromMap(raw)
	if assertion.Label == "" || assertion.Target == "" || assertion.Field == "" || assertion.ExpectedLabel == "" || raw["value"] == nil {
		return false
	}
	return slices.Contains([]string{"eq", "ne", "gt", "gte", "lt", "lte", "contains"}, assertion.Op)
}

// Check 对单条链上查询结果执行断言。
func Check(assertion Assertion, actual map[string]any) Result {
	field := assertion.Field
	if field == "" {
		field = assertion.Target
	}
	actualValue := actual[field]
	passed := false
	switch assertion.Op {
	case "eq", "":
		passed = reflect.DeepEqual(actualValue, assertion.Value)
	case "ne":
		passed = !reflect.DeepEqual(actualValue, assertion.Value)
	case "gt", "gte", "lt", "lte":
		passed = compareNumbers(actualValue, assertion.Value, assertion.Op)
	case "contains":
		passed = strings.Contains(fmt.Sprint(actualValue), fmt.Sprint(assertion.Value))
	case "exists":
		_, passed = actual[field]
	}
	return Result{Case: assertion.Label, Passed: passed, ExpectedLabel: assertion.ExpectedLabel, Actual: ShortJSON(actual), Hint: assertion.Hint}
}

// compareNumbers 使用有理数比较避免大整数经 float64 再次丢失精度。
func compareNumbers(actual, expected any, op string) bool {
	left, leftOK := new(big.Rat).SetString(strings.TrimSpace(fmt.Sprint(actual)))
	right, rightOK := new(big.Rat).SetString(strings.TrimSpace(fmt.Sprint(expected)))
	if !leftOK || !rightOK {
		return false
	}
	cmp := left.Cmp(right)
	switch op {
	case "gt":
		return cmp > 0
	case "gte":
		return cmp >= 0
	case "lt":
		return cmp < 0
	case "lte":
		return cmp <= 0
	default:
		return false
	}
}

// hasOnlyKeys 判断断言对象没有未声明字段。
func hasOnlyKeys(value map[string]any, allowed ...string) bool {
	for key := range value {
		if !slices.Contains(allowed, key) {
			return false
		}
	}
	return true
}

// ShortJSON 返回脱敏短文本,避免把完整状态或期望结构传到前端。
func ShortJSON(v any) string {
	raw, err := json.Marshal(redactSensitiveValues(v))
	if err != nil {
		return ""
	}
	text := string(raw)
	runes := []rune(text)
	if len(runes) > shortJSONRuneLimit {
		return string(runes[:shortJSONRuneLimit])
	}
	return text
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
