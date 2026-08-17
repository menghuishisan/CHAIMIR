// contest problem_config 文件定义赛题固定配置,供 HTTP、领域模型和 JSONB 持久化共用。
package contest

// DynamicScoreConfig 定义按已解出队伍数衰减的计分参数。
type DynamicScoreConfig struct {
	MinScore      int32 `json:"min_score"`
	DecayPerSolve int32 `json:"decay_per_solve"`
}

// BattleRuntimeConfig 定义对抗题启动沙箱所需的运行时参数。
type BattleRuntimeConfig struct {
	RuntimeCode         string   `json:"runtime_code"`
	RuntimeImageVersion string   `json:"runtime_image_version"`
	ToolCodes           []string `json:"tool_codes,omitempty"`
}
