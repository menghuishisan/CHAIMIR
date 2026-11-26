// contracts 定义第 1 层评测引擎对外暴露的判题与查重契约。
package contracts

import "context"

const (
	// JudgerCodeTheoryQuestion 是 M5 与 M3 共用的内置理论题策略标识。
	JudgerCodeTheoryQuestion = "theory-question"

	// JudgeSandboxModeFresh 表示使用全新判题沙箱执行。
	JudgeSandboxModeFresh = "fresh"
	// JudgeSandboxModeReuse 使用既有沙箱；学生现场只读，私有对局执行锁定程序。
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

// JudgeInputContext 用于提交前的能力校验，不授予沙箱权限。
type JudgeInputContext struct {
	SandboxMode     string
	AccessProfile   SandboxAccessProfile
	RequiresOutcome bool
}

// JudgeInputRequirements 由 M3 根据锁定判题配置决定提交方式。
type JudgeInputRequirements struct {
	SandboxMode  string `json:"sandbox_mode"`
	RequiresCode bool   `json:"requires_code"`
}

// JudgeSubmitRequest 是业务模块提交判题任务时使用的稳定契约。
type JudgeSubmitRequest struct {
	TenantID int64 `json:"tenant_id"`
	// JudgerCode 为空时由 M3 使用 M5 锁定题目版本的 judge_config.judger_code;业务模块不得硬编码题目判题器别名。
	JudgerCode  string `json:"judger_code"`
	ItemCode    string `json:"item_code"`
	ItemVersion string `json:"item_version"`
	// CodeStorageKey/CodeHash 仅代码类判题器必填;Flag、仿真检查点、人工评分和 reuse 链上断言必须留空。
	CodeStorageKey    string `json:"code_storage_key"`
	CodeHash          string `json:"code_hash"`
	SubmitterTenantID int64  `json:"submitter_tenant_id"`
	SubmitterID       int64  `json:"submitter_id"`
	SourceRef         string `json:"source_ref"`
	SourceOwnerID     int64  `json:"source_owner_id"`
	SourceCourseID    int64  `json:"source_course_id"`
	SourceScope       string `json:"source_scope"`
	SandboxMode       string `json:"sandbox_mode"`
	TargetSandboxRef  string `json:"target_sandbox_ref"`
	// SandboxSourceRef 是 reuse 模式目标沙箱真实来源;任务 SourceRef 仍保持本次判题事件唯一。
	SandboxSourceRef         string         `json:"sandbox_source_ref"`
	SandboxCompositionDigest string         `json:"sandbox_composition_digest,omitempty"`
	RequiresOutcome          bool           `json:"-"`
	ExtraInput               map[string]any `json:"extra_input"`
	Priority                 int16          `json:"priority"`
}

// JudgeOutcome 是锁定对局程序产生的机器结果，与展示摘要无关。
type JudgeOutcome string

const (
	JudgeOutcomeAWin JudgeOutcome = "a_win"
	JudgeOutcomeBWin JudgeOutcome = "b_win"
	JudgeOutcomeDraw JudgeOutcome = "draw"
)

// Valid 拒绝未声明、别名和未知结果。
func (outcome JudgeOutcome) Valid() bool {
	return outcome == JudgeOutcomeAWin || outcome == JudgeOutcomeBWin || outcome == JudgeOutcomeDraw
}

// JudgeResultDetail 是评测结果中的单条可解释详情。
type JudgeResultDetail struct {
	Case          string       `json:"case"`
	Passed        bool         `json:"passed"`
	ExpectedLabel string       `json:"expected_label,omitempty"`
	Actual        string       `json:"actual"`
	Hint          string       `json:"hint,omitempty"`
	Outcome       JudgeOutcome `json:"outcome,omitempty"`
}

// JudgeReplayAction 按执行顺序记录链上操作，查询同样保留目标与脱敏结果。
type JudgeReplayAction struct {
	Seq             int32          `json:"seq"`
	RuntimeInstance string         `json:"runtime_instance"`
	Op              string         `json:"op"`
	Payload         map[string]any `json:"payload,omitempty"`
	Output          map[string]any `json:"output,omitempty"`
}

// JudgeReplayTrace 是 M3 从 M2 链能力调用中收集的可复现交易序列。
type JudgeReplayTrace struct {
	Actions []JudgeReplayAction `json:"actions"`
}

// JudgeTaskResult 是跨模块回写业务结果所需的判题结果快照。
type JudgeTaskResult struct {
	Passed    bool                `json:"passed"`
	Score     int32               `json:"score"`
	MaxScore  int32               `json:"max_score"`
	Details   []JudgeResultDetail `json:"details"`
	ResultRef string              `json:"result_ref"`
	Replay    JudgeReplayTrace    `json:"replay"`
}

// JudgeTaskInfo 是评测引擎向调用方暴露的任务摘要。
type JudgeTaskInfo struct {
	TaskID            int64           `json:"task_id"`
	TenantID          int64           `json:"tenant_id"`
	SubmitterTenantID int64           `json:"submitter_tenant_id"`
	SourceRef         string          `json:"source_ref"`
	SubmitterID       int64           `json:"submitter_id"`
	Status            int16           `json:"status"`
	Result            JudgeTaskResult `json:"result"`
}

// FingerprintSimilarityRequest 是查重服务的相似度比对请求。
type FingerprintSimilarityRequest struct {
	TenantID   int64  `json:"tenant_id"`
	ProblemRef string `json:"problem_ref"`
	// SourceRefs 由调用方按业务权限确定；空集合不匹配任何提交。
	SourceRefs       []string `json:"source_refs"`
	CodeHash         string   `json:"code_hash"`
	ExcludeSourceRef string   `json:"exclude_source_ref"`
	Threshold        float64  `json:"threshold"`
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
	// SubmitJudgeTask 创建判题任务并返回初始任务摘要。JudgerCode 为空时必须使用 M5 锁定题目版本的 judge_config.judger_code,业务模块不得硬编码题目判题器别名。
	SubmitJudgeTask(ctx context.Context, req JudgeSubmitRequest) (JudgeTaskInfo, error)
	// GetJudgeInputRequirements 校验锁定题目的输入要求；执行权限仍以 M2 真实沙箱为准。
	GetJudgeInputRequirements(ctx context.Context, tenantID int64, itemCode, itemVersion string, input JudgeInputContext) (JudgeInputRequirements, error)
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
	// FindSimilarity 在已授权来源集合内选择已存指纹基准和候选，返回达到阈值的提交。
	FindSimilarity(ctx context.Context, req FingerprintSimilarityRequest) ([]FingerprintMatch, error)
}
