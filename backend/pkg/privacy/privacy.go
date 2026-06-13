// privacy 提供跨模块可复用的用户数据脱敏函数。
package privacy

import "strings"

// MaskPhone 对中国大陆手机号做用户向掩码展示,非法长度返回空字符串避免误展示原值。
func MaskPhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if len(phone) != 11 {
		return ""
	}
	return phone[:3] + "****" + phone[7:]
}
