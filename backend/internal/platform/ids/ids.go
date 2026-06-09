// ids 统一雪花 ID 在后端与外部字符串之间的边界处理。
package ids

import (
	"strconv"
	"strings"
)

// Format 把 int64 雪花 ID 转成字符串,避免前端 Number 精度损失。
func Format(id int64) string {
	return strconv.FormatInt(id, 10)
}

// Parse 把外部字符串 ID 解析为正整数,非法、空值和非正数均返回 false。
func Parse(v string) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// ParseOrZero 解析可选 ID,非法或空值按未传入处理为 0。
func ParseOrZero(v string) int64 {
	id, _ := Parse(v)
	return id
}
