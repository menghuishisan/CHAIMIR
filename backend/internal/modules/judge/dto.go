// judge dto 文件定义 M3 HTTP 请求和响应结构,不承载业务编排逻辑。
package judge

import (
	"encoding/json"

	"chaimir/internal/platform/ids"
)

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

// JudgerDTO 是平台管理员读取和维护判题器时使用的公开响应。
type JudgerDTO struct {
	ID                ids.ID          `json:"id"`
	Code              string          `json:"code"`
	Name              string          `json:"name"`
	Type              int16           `json:"type"`
	ExecutorRef       string          `json:"executor_ref"`
	RuntimeRequired   bool            `json:"runtime_required"`
	DefaultTimeoutSec int32           `json:"default_timeout_sec"`
	ResourceSpec      json.RawMessage `json:"resource_spec"`
	SelftestStatus    int16           `json:"selftest_status"`
	Status            int16           `json:"status"`
}

// JudgerCatalogResponse 是判题器目录接口的输出,只含业务模块选判题方式所需字段。
type JudgerCatalogResponse struct {
	Judgers []CatalogJudgerResponse `json:"judgers"`
}

// CatalogJudgerResponse 是目录里的单个判题方式,不含资源清单与执行引用。
type CatalogJudgerResponse struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Type int16  `json:"type"`
}

// SubmitTaskRequest 是内部服务提交判题任务的请求。
type SubmitTaskRequest struct {
	JudgerCode       string         `json:"judger_code"`
	ItemCode         string         `json:"item_code"`
	ItemVersion      string         `json:"item_version"`
	CodeStorageKey   string         `json:"code_storage_key"`
	CodeHash         string         `json:"code_hash"`
	SubmitterID      ids.ID         `json:"submitter_id"`
	SourceOwnerID    ids.ID         `json:"source_owner_id"`
	SourceCourseID   ids.ID         `json:"source_course_id"`
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

// JudgeTaskDTO 是教师查询、重判和人工评分返回的任务摘要。
type JudgeTaskDTO struct {
	TaskID      ids.ID              `json:"task_id"`
	TenantID    ids.ID              `json:"tenant_id"`
	SourceRef   string              `json:"source_ref"`
	SubmitterID ids.ID              `json:"submitter_id"`
	Status      string              `json:"status"`
	Existing    bool                `json:"existing"`
	Result      *JudgeTaskResultDTO `json:"result,omitempty"`
}

// JudgeTaskResultDTO 是判题任务最新版本的可公开结果。
type JudgeTaskResultDTO struct {
	Passed    bool                   `json:"passed"`
	Score     int32                  `json:"score"`
	MaxScore  int32                  `json:"max_score"`
	Version   int32                  `json:"version"`
	IsRejudge bool                   `json:"is_rejudge"`
	Details   []JudgeResultDetailDTO `json:"details"`
	ResultRef string                 `json:"result_ref"`
}

// JudgeResultDetailDTO 是单条脱敏的判题可解释详情。
type JudgeResultDetailDTO struct {
	Case          string `json:"case,omitempty"`
	Source        string `json:"source,omitempty"`
	Target        string `json:"target,omitempty"`
	Passed        bool   `json:"passed"`
	ExpectedLabel string `json:"expected_label,omitempty"`
	Actual        string `json:"actual,omitempty"`
	Hint          string `json:"hint,omitempty"`
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
