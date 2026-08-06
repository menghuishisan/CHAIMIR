// textx 是文本展示边界的最小工具包:只处理"给人看的文本要有长度上限"这一件事。
//
// 为什么单独成包:同一段按 rune 截断的逻辑此前在 pkg/chainassert(链上断言摘要)、
// identity(租户开通失败原因入库)与 sim(隔离预览失败原因入报告)各写了一遍。
// 三处都在同一件事上:文本来自不可信或不可控来源,长度不可信,入库或下发前必须截断。
// 按 rune 而不是 byte 截断是硬要求 —— 中文按字节切会切出半个字符,展示成乱码。
//
// 它放 pkg 而不是 internal/platform:调用方之一 pkg/chainassert 是 pkg 层,
// 若把工具放 internal/platform 会让 pkg 反向依赖 internal,层级就倒了。
package textx

// TruncateRunes 把文本截断到最多 limit 个 rune;limit 非正数时返回空串。
// 不追加省略号:调用方可能把结果写进结构化字段或再拼接,省略号该由展示层决定。
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
