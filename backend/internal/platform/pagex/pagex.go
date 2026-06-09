// pagex 统一分页参数默认值和上限,供 API、service 与 repo 共用。
package pagex

const (
	defaultPage = 1
	defaultSize = 20
	maxSize     = 100
)

// Normalize 将分页参数归一为平台统一规则:默认第一页、默认 20 条、最多 100 条。
func Normalize(page, size int) (int, int) {
	if page <= 0 {
		page = defaultPage
	}
	if size <= 0 {
		size = defaultSize
	}
	if size > maxSize {
		size = maxSize
	}
	return page, size
}
