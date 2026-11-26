// Package textx 限制用户可见文本的长度。
package textx

// TruncateRunes 保留最多 limit 个 Unicode 码点，供展示层决定是否追加省略号。
func TruncateRunes(text string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit])
}
