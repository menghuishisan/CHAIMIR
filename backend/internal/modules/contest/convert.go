// M8 转换层:处理领域 DTO、contracts DTO 与 HTTP 输出结构之间的纯转换。
package contest

import (
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
)

// ladderRanksBytes 序列化最终榜单快照。
func ladderRanksBytes(v []LadderRankDTO) ([]byte, error) {
	if v == nil {
		v = []LadderRankDTO{}
	}
	return jsonx.AnyBytes(v, apperr.ErrContestInvalid)
}

// ladderRanksValue 解析最终榜单快照。
func ladderRanksValue(data []byte) []LadderRankDTO {
	return jsonx.Decode(data, []LadderRankDTO{})
}
