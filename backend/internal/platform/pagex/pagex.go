// pagex 统一分页参数默认值和上限,供 API、service 与 repo 共用。
package pagex

import (
	"math"

	"chaimir/internal/platform/intx"
)

const (
	defaultPage = 1
	defaultSize = 20
	maxSize     = 100
	// maxPage 保证任意规范化 page/size 计算出的 SQL int32 offset 不会溢出。
	maxPage = math.MaxInt32 / maxSize
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
	if page > maxPage {
		page = maxPage
	}
	return page, size
}

// MaximumSize 返回平台统一单页上限,供内部批处理复用同一分页口径。
func MaximumSize() int { return maxSize }

// Int32 将规范化分页参数安全收窄为 sqlc 使用的 int32 类型。
func Int32(page, size int) (int32, int32) {
	page, size = Normalize(page, size)
	page32, pageOK := intx.Int32(page)
	size32, sizeOK := intx.Int32(size)
	if !pageOK || !sizeOK {
		return 0, 0
	}
	return page32, size32
}

// LimitOffset 返回已规范化的 SQL LIMIT 和 OFFSET，避免每个仓储层重复整数乘法和收窄。
func LimitOffset(page, size int) (int32, int32) {
	page, size = Normalize(page, size)
	offset := (page - 1) * size
	limit32, limitOK := intx.Int32(size)
	offset32, offsetOK := intx.Int32(offset)
	if !limitOK || !offsetOK {
		return 0, 0
	}
	return limit32, offset32
}
