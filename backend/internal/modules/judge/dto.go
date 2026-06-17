// judge dto 文件定义 M3 HTTP 请求和响应结构,不承载业务编排逻辑。
package judge

import "encoding/json"

// JudgerRequest 是平台管理员创建或更新判题器配置的请求。
type JudgerRequest struct {
	Code              string          `json:"code"`
	Name              string          `json:"name"`
	Type              int16           `json:"type"`
	ExecutorRef       string          `json:"executor_ref"`
	RuntimeRequired   bool            `json:"runtime_required"`
	DefaultTimeoutSec int32           `json:"default_timeout_sec"`
	ResourceSpec      json.RawMessage `json:"resource_spec"`
	Status            int16           `json:"status"`
}

// SubmitTaskRequest 是内部服务提交判题任务的请求。
type SubmitTaskRequest struct {
	JudgerCode       string         `json:"judger_code"`
	ItemCode         string         `json:"item_code"`
	ItemVersion      string         `json:"item_version"`
	CodeStorageKey   string         `json:"code_storage_key"`
	CodeHash         string         `json:"code_hash"`
	SubmitterID      int64          `json:"submitter_id"`
	SourceOwnerID    int64          `json:"source_owner_id"`
	SourceCourseID   int64          `json:"source_course_id"`
	SourceScope      string         `json:"source_scope"`
	SandboxMode      string         `json:"sandbox_mode"`
	TargetSandboxRef string         `json:"target_sandbox_ref"`
	ExtraInput       map[string]any `json:"extra_input"`
	Priority         int16          `json:"priority"`
}

// ManualScoreRequest 是教师录入人工评分的请求。
type ManualScoreRequest struct {
	Score    int32  `json:"score"`
	MaxScore int32  `json:"max_score"`
	Passed   bool   `json:"passed"`
	Comment  string `json:"comment"`
}

// RejudgeBatchRequest 是按来源批量重判的请求。
type RejudgeBatchRequest struct {
	SourceRef string `json:"source_ref"`
}

// FingerprintSimilarityRequest 是相似度查重请求。
type FingerprintSimilarityRequest struct {
	ProblemRef       string  `json:"problem_ref"`
	CodeStorageKey   string  `json:"code_storage_key"`
	CodeHash         string  `json:"code_hash"`
	ExcludeSourceRef string  `json:"exclude_source_ref"`
	Threshold        float64 `json:"threshold"`
}
