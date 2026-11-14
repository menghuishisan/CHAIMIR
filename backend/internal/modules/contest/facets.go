// contest facets 文件组装漏洞题工坊的两组分段计数(接口总览「分段计数」)。
package contest

import "chaimir/internal/platform/pagex"

// vulnFacetRow 是按状态与预验证结果分组的一行。
type vulnFacetRow struct {
	Status            int16
	PrevalidateStatus int16
	Count             int64
}

// vulnFacets 组装两组分段:每组不套用本组筛选,但套用另一组的筛选(0 表示不筛)。
func vulnFacets(rows []vulnFacetRow, status, prevalidateStatus int16) pagex.Facets {
	facets := pagex.Facets{"status": pagex.FacetGroup{}, "prevalidate_status": pagex.FacetGroup{}}
	for _, row := range rows {
		if prevalidateStatus == 0 || row.PrevalidateStatus == prevalidateStatus {
			facets["status"].Add(row.Status, row.Count)
		}
		if status == 0 || row.Status == status {
			facets["prevalidate_status"].Add(row.PrevalidateStatus, row.Count)
		}
	}
	return facets
}
