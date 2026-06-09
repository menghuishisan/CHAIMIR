// M3 领域模型:定义 service/worker 使用的判题器、任务、结果与 outbox 投影。
package judge

// JudgerSnapshot 是判题器平台配置的业务投影,供 service/worker 判定策略和生成快照。
type JudgerSnapshot struct {
	ID                int64
	Code              string
	Name              string
	Type              int16
	ExecutorRef       string
	RuntimeRequired   bool
	DefaultTimeoutSec int32
	ResourceSpec      []byte
	SelftestStatus    int16
	SelftestDetail    []byte
	Status            int16
	UpdatedAtText     string
}

// JudgeTaskSnapshot 是判题任务的业务投影,承载队列、状态机和审计所需字段。
type JudgeTaskSnapshot struct {
	ID               int64
	TenantID         int64
	JudgerID         int64
	SourceRef        string
	SubmitterID      int64
	ProblemRef       string
	CodeStorageKey   string
	CodeHash         string
	InputSnapshot    []byte
	SandboxMode      int16
	TargetSandboxRef string
	Priority         int16
	Status           int16
	RetryCount       int32
	MaxRetries       int32
	CreatedAtUnixMs  int64
}

// JudgeTaskCreate 是创建判题任务和提交指纹时 service 传入 repo 的持久化意图。
type JudgeTaskCreate struct {
	TaskID           int64
	FingerprintID    int64
	TenantID         int64
	JudgerID         int64
	SourceRef        string
	SubmitterID      int64
	ProblemRef       string
	CodeStorageKey   string
	CodeHash         string
	InputSnapshot    []byte
	SandboxMode      int16
	TargetSandboxRef string
	Priority         int16
	Status           int16
	MaxRetries       int32
	SimVector        []byte
}

// JudgeResultCreate 是写入 judge_result 时 worker/service 传入 repo 的结果投影。
type JudgeResultCreate struct {
	TaskID          int64
	TenantID        int64
	Passed          bool
	Score           int32
	MaxScore        int32
	Details         []byte
	JudgeSandboxRef string
	IsRejudge       bool
}

// SubmissionFingerprintSnapshot 是查重输出使用的提交指纹投影。
type SubmissionFingerprintSnapshot struct {
	ID          int64
	SourceRef   string
	ProblemRef  string
	SubmitterID int64
	CodeHash    string
	SimVector   []byte
	CreatedAt   any
}

// JudgeOutboxSnapshot 是终态事件 outbox 的发布投影。
type JudgeOutboxSnapshot struct {
	ID        int64
	TenantID  int64
	TaskID    int64
	Subject   string
	Payload   []byte
	LastError string
}

// JudgeOutboxCreate 是在终态事务内创建 outbox 时传入 repo 的事件投影。
type JudgeOutboxCreate struct {
	ID       int64
	TenantID int64
	TaskID   int64
	Subject  string
	Payload  []byte
}
