// contest rules_submission 统一解题提交和对抗参战请求的闭合 JSON 解码边界。
package contest

import (
	"encoding/json"
	"fmt"
)

// UnmarshalJSON 使解题提交请求严格遵循已发布的闭合字段契约。
func (req *SubmitRequest) UnmarshalJSON(data []byte) error {
	type wire SubmitRequest
	if err := rejectUnknownContestRequestFields(data, "content_ref", "sandbox_ref"); err != nil {
		return err
	}
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*req = SubmitRequest(decoded)
	return nil
}

// UnmarshalJSON 拒绝对抗参战请求中未公开的对象引用或附加字段。
func (req *BattleEntryRequest) UnmarshalJSON(data []byte) error {
	type wire BattleEntryRequest
	if err := rejectUnknownContestRequestFields(data, "problem_id", "sandbox_id", "role"); err != nil {
		return err
	}
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*req = BattleEntryRequest(decoded)
	return nil
}

// rejectUnknownContestRequestFields 校验这些具备对象归属安全边界的请求没有额外 JSON 字段。
func rejectUnknownContestRequestFields(data []byte, allowed ...string) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for name := range fields {
		known := false
		for _, field := range allowed {
			if name == field {
				known = true
				break
			}
		}
		if !known {
			return fmt.Errorf("请求包含未声明字段 %q", name)
		}
	}
	return nil
}
