// M3 HTTP 请求/响应 DTO。对外雪花 ID 统一用字符串,避免前端 JSON 精度问题。
package judge

// CreateJudgerRequest 是平台管理员注册判题器的请求。
type CreateJudgerRequest struct {
	Code              string         `json:"code" binding:"required"`
	Name              string         `json:"name" binding:"required"`
	Type              int16          `json:"type" binding:"required"`
	ExecutorRef       string         `json:"executor_ref" binding:"required"`
	RuntimeRequired   bool           `json:"runtime_required"`
	DefaultTimeoutSec int32          `json:"default_timeout_sec" binding:"required"`
	ResourceSpec      map[string]any `json:"resource_spec" binding:"required"`
	Status            int16          `json:"status"`
}

// UpdateJudgerRequest 是更新判题器配置的请求。
type UpdateJudgerRequest struct {
	Name              string         `json:"name" binding:"required"`
	Type              int16          `json:"type" binding:"required"`
	ExecutorRef       string         `json:"executor_ref" binding:"required"`
	RuntimeRequired   bool           `json:"runtime_required"`
	DefaultTimeoutSec int32          `json:"default_timeout_sec" binding:"required"`
	ResourceSpec      map[string]any `json:"resource_spec" binding:"required"`
	Status            int16          `json:"status" binding:"required"`
}

// SubmitTaskRequest 是内部调用方提交判题的 HTTP 请求。
type SubmitTaskRequest struct {
	JudgerCode       string         `json:"judger_code" binding:"required"`
	ItemCode         string         `json:"item_code" binding:"required"`
	ItemVersion      string         `json:"item_version" binding:"required"`
	CodeStorageKey   string         `json:"code_storage_key" binding:"required"`
	CodeHash         string         `json:"code_hash" binding:"required"`
	SubmitterID      string         `json:"submitter_id" binding:"required"`
	SourceRef        string         `json:"source_ref" binding:"required"`
	SandboxMode      string         `json:"sandbox_mode"`
	TargetSandboxRef string         `json:"target_sandbox_ref"`
	ExtraInput       map[string]any `json:"extra_input"`
	Priority         int16          `json:"priority"`
}

// ManualScoreRequest 是教师人工评分请求。
type ManualScoreRequest struct {
	Score    int32  `json:"score"`
	MaxScore int32  `json:"max_score"`
	Comment  string `json:"comment"`
}

// RejudgeBatchRequest 是按来源批量重判请求。
type RejudgeBatchRequest struct {
	SourceRef string `json:"source_ref" binding:"required"`
}

// FingerprintSimilarityRequest 是相似度计算请求。
type FingerprintSimilarityRequest struct {
	ProblemRef     string  `json:"problem_ref" binding:"required"`
	CodeStorageKey string  `json:"code_storage_key" binding:"required"`
	Threshold      float64 `json:"threshold"`
}
