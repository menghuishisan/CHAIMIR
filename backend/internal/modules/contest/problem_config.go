// contest problem_config 文件定义赛题固定配置,供 HTTP、领域模型和 JSONB 持久化共用。
package contest

import (
	"chaimir/internal/contracts"
	"chaimir/pkg/apperr"
)

// DynamicScoreConfig 定义按已解出队伍数衰减的计分参数。
type DynamicScoreConfig struct {
	MinScore      int32 `json:"min_score"`
	DecayPerSolve int32 `json:"decay_per_solve"`
}

// BattleRuntimeConfig 定义对抗题启动沙箱所需的运行时参数。
type BattleRuntimeConfig struct {
	ExecutionProfile string         `json:"execution_profile"`
	EntryRoles       []int16        `json:"entry_roles"`
	ReplayProfile    map[string]any `json:"replay_profile"`
}

// Validate 校验对抗题只保存赛制参数,环境组合由 M5 题目版本提供。
func (cfg BattleRuntimeConfig) Validate() error {
	if cfg.ExecutionProfile != string(contracts.SandboxAccessContestBattle) || len(cfg.EntryRoles) == 0 || len(cfg.ReplayProfile) == 0 {
		return apperr.ErrContestProblemInvalid
	}
	return nil
}
