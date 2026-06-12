// chainassert 包提供链上断言的通用判定能力,供 M3 判题和 M8 漏洞预验证复用。
package chainassert

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
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
	case "contains":
		passed = strings.Contains(fmt.Sprint(actualValue), fmt.Sprint(assertion.Value))
	case "exists":
		_, passed = actual[field]
	}
	return Result{Case: assertion.Label, Passed: passed, ExpectedLabel: assertion.ExpectedLabel, Actual: ShortJSON(actual), Hint: assertion.Hint}
}

// ShortJSON 返回脱敏短文本,避免把完整状态或期望结构传到前端。
func ShortJSON(v any) string {
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
