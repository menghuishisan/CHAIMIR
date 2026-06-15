// contracts 定义第 1 层评测引擎对外暴露的判题与查重契约。
package contracts

import "context"

const (
	// JudgeSandboxModeFresh 表示使用全新判题沙箱执行。
	JudgeSandboxModeFresh = "fresh"
	// JudgeSandboxModeReuse 表示复用业务现场沙箱只读状态执行断言。
	JudgeSandboxModeReuse = "reuse"
)

const (
	// JudgeTaskStatusQueued 表示任务已入队等待执行。
	JudgeTaskStatusQueued int16 = 1
	// JudgeTaskStatusRunning 表示任务执行中。
	JudgeTaskStatusRunning int16 = 2
	// JudgeTaskStatusDone 表示任务已完成。
	JudgeTaskStatusDone int16 = 3
	// JudgeTaskStatusFailed 表示任务系统性失败终态。
	JudgeTaskStatusFailed int16 = 4
	// JudgeTaskStatusCanceled 表示任务已取消。
	JudgeTaskStatusCanceled int16 = 5
)

// JudgeSubmitRequest 是业务模块提交判题任务时使用的稳定契约。
type JudgeSubmitRequest struct {
	TenantID         int64          `json:"tenant_id"`
	JudgerCode       string         `json:"judger_code"`
	ItemCode         string         `json:"item_code"`
	ItemVersion      string         `json:"item_version"`
	CodeStorageKey   string         `json:"code_storage_key"`
	CodeHash         string         `json:"code_hash"`
	SubmitterID      int64          `json:"submitter_id"`
	SourceRef        string         `json:"source_ref"`
	SandboxMode      string         `json:"sandbox_mode"`
	TargetSandboxRef string         `json:"target_sandbox_ref"`
	ExtraInput       map[string]any `json:"extra_input"`
	Priority         int16          `json:"priority"`
}

// JudgeResultDetail 是评测结果中的单条可解释详情。
type JudgeResultDetail struct {
	Case          string `json:"case"`
	Passed        bool   `json:"passed"`
	ExpectedLabel string `json:"expected_label"`
	Actual        string `json:"actual"`
	Hint          string `json:"hint"`
}

// JudgeTaskResult 是跨模块回写业务结果所需的判题结果快照。
type JudgeTaskResult struct {
	Passed      bool                `json:"passed"`
	Score       int32               `json:"score"`
	MaxScore    int32               `json:"max_score"`
	Details     []JudgeResultDetail `json:"details"`
	SnapshotRef string              `json:"snapshot_ref"`
}

// JudgeTaskInfo 是评测引擎向调用方暴露的任务摘要。
type JudgeTaskInfo struct {
	TaskID      int64           `json:"task_id"`
	TenantID    int64           `json:"tenant_id"`
	SourceRef   string          `json:"source_ref"`
	SubmitterID int64           `json:"submitter_id"`
	Status      int16           `json:"status"`
	Result      JudgeTaskResult `json:"result"`
}

// FingerprintSimilarityRequest 是查重服务的相似度比对请求。
type FingerprintSimilarityRequest struct {
	TenantID         int64   `json:"tenant_id"`
	ProblemRef       string  `json:"problem_ref"`
	CodeStorageKey   string  `json:"code_storage_key"`
	CodeHash         string  `json:"code_hash"`
	ExcludeSourceRef string  `json:"exclude_source_ref"`
	Threshold        float64 `json:"threshold"`
}

// FingerprintMatch 是查重返回的一条相似提交命中。
type FingerprintMatch struct {
	SourceRef   string  `json:"source_ref"`
	SubmitterID int64   `json:"submitter_id"`
	Score       float64 `json:"score"`
	CodeHash    string  `json:"code_hash"`
}

// JudgeService 是 M3 评测引擎对 M6/M7/M8 暴露的标准判题契约。
type JudgeService interface {
	// SubmitJudgeTask 创建判题任务并返回初始任务摘要。
	SubmitJudgeTask(ctx context.Context, req JudgeSubmitRequest) (JudgeTaskInfo, error)
	// GetJudgeTask 读取任务状态与结果摘要。
	GetJudgeTask(ctx context.Context, tenantID, taskID int64) (JudgeTaskInfo, error)
	// CancelJudgeTask 取消仍在排队中的判题任务,供业务流程补偿或撤回使用。
	CancelJudgeTask(ctx context.Context, tenantID, taskID int64) error
	// Rejudge 按原输入快照重新判题,用于申诉或判题器修复后的回溯。
	Rejudge(ctx context.Context, tenantID, taskID int64) (JudgeTaskInfo, error)
	// RejudgeBySourceRef 按来源标识批量重判任务,供题目修复后的整链路回溯使用。
	RejudgeBySourceRef(ctx context.Context, tenantID int64, sourceRef string) error
}

// FingerprintService 是 M3 对 M8 防作弊场景暴露的查重能力契约。
type FingerprintService interface {
	// FindExactMatch 按题目与代码哈希查找完全相同的提交。
	FindExactMatch(ctx context.Context, tenantID int64, problemRef, codeHash string) ([]FingerprintMatch, error)
	// FindSimilarity 读取对象并生成特征向量,返回超过阈值的相似提交。
	FindSimilarity(ctx context.Context, req FingerprintSimilarityRequest) ([]FingerprintMatch, error)
}
