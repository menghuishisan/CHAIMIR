// ids 统一雪花 ID 在后端内部 int64 与公开字符串契约之间的边界处理。
package ids

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// ID 是公开 HTTP/WS JSON 使用的雪花 ID 类型，始终编码为十进制字符串。
type ID int64

// Format 把 int64 雪花 ID 转成字符串,避免前端 Number 精度损失。
func Format(id int64) string {
	return strconv.FormatInt(id, 10)
}

// Parse 把外部规范十进制字符串 ID 解析为正整数，拒绝空白、符号、前导零和非正数。
func Parse(v string) (int64, bool) {
	if v == "" || v[0] < '1' || v[0] > '9' {
		return 0, false
	}
	for _, char := range v {
		if char < '0' || char > '9' {
			return 0, false
		}
	}
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// FromInt64 把通过业务校验的内部 ID 转为公开边界类型。
func FromInt64(value int64) (ID, error) {
	if value <= 0 {
		return 0, fmt.Errorf("雪花 ID 必须为正整数")
	}
	return ID(value), nil
}

// Int64 返回供模块 service 与数据库层使用的内部表示。
func (id ID) Int64() int64 {
	return int64(id)
}

// String 返回规范十进制字符串。
func (id ID) String() string {
	return Format(int64(id))
}

// MarshalJSON 把公开 ID 编码为字符串，避免浏览器 Number 精度丢失。
func (id ID) MarshalJSON() ([]byte, error) {
	if id <= 0 {
		return nil, fmt.Errorf("雪花 ID 必须为正整数")
	}
	return json.Marshal(id.String())
}

// UnmarshalJSON 只接受规范十进制 JSON 字符串，不兼容 JSON number。
func (id *ID) UnmarshalJSON(data []byte) error {
	if id == nil {
		return fmt.Errorf("雪花 ID 接收目标不能为空")
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("雪花 ID 必须使用 JSON 字符串: %w", err)
	}
	parsed, ok := Parse(value)
	if !ok {
		return fmt.Errorf("雪花 ID 必须为规范十进制正整数字符串")
	}
	*id = ID(parsed)
	return nil
}

// MarshalText 为查询参数和结构化日志提供规范十进制表示。
func (id ID) MarshalText() ([]byte, error) {
	if id <= 0 {
		return nil, fmt.Errorf("雪花 ID 必须为正整数")
	}
	return []byte(id.String()), nil
}

// UnmarshalText 严格解析查询参数中的规范十进制 ID。
func (id *ID) UnmarshalText(data []byte) error {
	if id == nil {
		return fmt.Errorf("雪花 ID 接收目标不能为空")
	}
	parsed, ok := Parse(string(data))
	if !ok {
		return fmt.Errorf("雪花 ID 必须为规范十进制正整数字符串")
	}
	*id = ID(parsed)
	return nil
}
