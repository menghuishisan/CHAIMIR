// sim facets 文件把状态与审核结论的分组计数转成接口用的字符串取值(与 status/result 参数同名同值)。
package sim

import "chaimir/internal/platform/pagex"

// namedFacetGroup 把数字状态的计数换成对外的字符串取值;遇到不认识的状态即报数据异常,不静默丢弃。
func namedFacetGroup(counts map[int16]int64, name func(int16) (string, error)) (pagex.FacetGroup, error) {
	group := pagex.FacetGroup{}
	for value, count := range counts {
		text, err := name(value)
		if err != nil {
			return nil, err
		}
		group.Add(text, count)
	}
	return group, nil
}
