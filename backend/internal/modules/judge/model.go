// judge model 文件定义 M3 领域模型和内部快照,不依赖 HTTP 或数据库生成类型。
package judge

import (
	"encoding/json"
	"time"
)

// Judger 是平台级判题器定义。
type Judger struct {
	ID                int64
	Code              string
	Name              string
	Type              int16
	ExecutorRef       string
	RuntimeRequired   bool
	DefaultTimeoutSec int32
	ResourceSpec      JudgerResourceSpec
	SelftestStatus    int16
	Status            int16
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// JudgeTask 是一次提交的判题任务快照。
type JudgeTask struct {
	ID               int64
	TenantID         int64
	JudgerID         int64
	SourceRef        string
	SourceOwnerID    int64
	SourceCourseID   int64
	SourceScope      string
	SubmitterID      int64
	ProblemRef       string
	CodeStorageKey   string
	CodeHash         string
	InputSnapshot    JudgeInputSnapshot
	SandboxMode      int16
	TargetSandboxRef string
	Priority         int16
	Status           int16
	RetryCount       int32
	MaxRetries       int32
	LastError        string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// JudgeResultDetail 是可解释结果中的一项脱敏详情。
type JudgeResultDetail struct {
	Case          string `json:"case,omitempty"`
	Source        string `json:"source,omitempty"`
	Target        string `json:"target,omitempty"`
	Passed        bool   `json:"passed"`
	ExpectedLabel string `json:"expected_label,omitempty"`
	Actual        string `json:"actual,omitempty"`
	Hint          string `json:"hint,omitempty"`
}

// JudgeResult 是一次任务判题结果的版本化记录。
type JudgeResult struct {
	ID              int64
	TaskID          int64
	TenantID        int64
	Version         int32
	Passed          bool
	Score           int32
	MaxScore        int32
	Details         []JudgeResultDetail
	JudgeSandboxRef string
	JudgedAt        time.Time
	IsRejudge       bool
}

// JudgeTaskInfo 是 service/API 对外返回的任务摘要。
type JudgeTaskInfo struct {
	Task     JudgeTask
	Result   *JudgeResult
	Existing bool
}

// JudgeInputSnapshot 固定一次判题所需的题目版本、执行器和脱敏期望。
type JudgeInputSnapshot struct {
	ItemCode                 string         `json:"item_code"`
	ItemVersion              string         `json:"item_version"`
	TraceID                  string         `json:"trace_id"`
	JudgerCode               string         `json:"judger_code"`
	JudgerType               int16          `json:"judger_type"`
	JudgerVersion            string         `json:"judger_version"`
	SuiteRef                 string         `json:"suite_ref,omitempty"`
	SuiteArchiveName         string         `json:"suite_archive_name,omitempty"`
	VersionHash              string         `json:"version_hash"`
	RuntimeCode              string         `json:"runtime_code,omitempty"`
	RuntimeImageVersion      string         `json:"runtime_image_version,omitempty"`
	GenesisRef               string         `json:"genesis_ref,omitempty"`
	ToolCodes                []string       `json:"tool_codes,omitempty"`
	InitScriptRef            string         `json:"init_script_ref,omitempty"`
	Command                  []string       `json:"command,omitempty"`
	TimeoutSec               int32          `json:"timeout_sec"`
	MaxRetries               int32          `json:"max_retries"`
	MaxScore                 int32          `json:"max_score"`
	Expectation              map[string]any `json:"expectation,omitempty"`
	ExtraInput               map[string]any `json:"extra_input,omitempty"`
	Rejudge                  bool           `json:"rejudge,omitempty"`
	SanitizedCodeArchiveName string         `json:"sanitized_code_archive_name,omitempty"`
	SanitizedCodeArchiveRef  string         `json:"sanitized_code_archive_ref,omitempty"`
}

// SubmissionFingerprint 是 M3 生成的代码查重特征。
type SubmissionFingerprint struct {
	ID          int64
	TenantID    int64
	SourceRef   string
	ProblemRef  string
	SubmitterID int64
	CodeHash    string
	SimVector   map[string]float64
	CreatedAt   time.Time
}

// JudgeEventOutbox 是待可靠发布的终态事件。
type JudgeEventOutbox struct {
	ID            int64
	TenantID      int64
	TaskID        int64
	Subject       string
	Payload       json.RawMessage
	Status        int16
	RetryCount    int32
	NextAttemptAt time.Time
	LastError     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ProgressMessage 描述 WebSocket 推送给调用方的用户向进度。
type ProgressMessage struct {
	TaskID  int64  `json:"task_id"`
	Status  string `json:"status"`
	Stage   string `json:"stage"`
	Message string `json:"message"`
	TraceID string `json:"trace_id,omitempty"`
}
