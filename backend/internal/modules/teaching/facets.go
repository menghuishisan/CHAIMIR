// teaching facets 文件组装课程列表与批改台的分段计数(接口总览「分段计数」)。
package teaching

import "chaimir/internal/platform/pagex"

// submissionFacetRow 是批改台按状态与是否迟交分组的一行。
type submissionFacetRow struct {
	Status int16
	IsLate bool
	Count  int64
}

// submissionFacets 组装批改台的分段计数:状态一组、是否迟交一组,都是该作业在当前视角下的全量。
func submissionFacets(rows []submissionFacetRow) pagex.Facets {
	facets := pagex.Facets{"status": pagex.FacetGroup{}, "is_late": pagex.FacetGroup{}}
	for _, row := range rows {
		facets["status"].Add(row.Status, row.Count)
		facets["is_late"].Add(row.IsLate, row.Count)
	}
	return facets
}
