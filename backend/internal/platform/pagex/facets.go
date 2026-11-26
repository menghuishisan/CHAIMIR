package pagex

import "fmt"

// FacetGroup 以筛选值的字符串形式为键，记录该值的条数。
type FacetGroup map[string]int64

// Add 把一个取值的条数累加进分组;取值按接口约定转成字符串。
func (g FacetGroup) Add(value any, count int64) {
	g[fmt.Sprint(value)] += count
}

// Facets 是一个列表的全部分段计数,键是分段的筛选参数名。
type Facets map[string]FacetGroup

// Map 转成响应信封使用的映射。
func (f Facets) Map() map[string]map[string]int64 {
	out := make(map[string]map[string]int64, len(f))
	for key, group := range f {
		out[key] = map[string]int64(group)
	}
	return out
}
