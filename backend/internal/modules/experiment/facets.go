// experiment facets 文件组装实验、报告与实例列表的状态分段计数(接口总览「分段计数」)。
package experiment

import "chaimir/internal/platform/pagex"

// statusCount 是按状态分组的一行。
type statusCount struct {
	Status int16
	Count  int64
}

// statusFacets 把按状态分组的行组装成 status 一组分段计数。
func statusFacets(rows []statusCount) pagex.Facets {
	group := pagex.FacetGroup{}
	for _, row := range rows {
		group.Add(row.Status, row.Count)
	}
	return pagex.Facets{"status": group}
}
