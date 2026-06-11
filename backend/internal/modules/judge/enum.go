// judge enum 文件定义 M3 评测引擎判题器、任务、outbox 和进度状态枚举。
package judge

const (
	// JudgerTypeTestcase 表示测试用例判题。
	JudgerTypeTestcase int16 = 1
	// JudgerTypeOnchainAssert 表示链上状态断言。
	JudgerTypeOnchainAssert int16 = 2
	// JudgerTypeFlag 表示 Flag 判题。
	JudgerTypeFlag int16 = 3
	// JudgerTypeStaticScan 表示代码静态检查。
	JudgerTypeStaticScan int16 = 4
	// JudgerTypeSimCheckpoint 表示仿真检查点判题。
	JudgerTypeSimCheckpoint int16 = 5
	// JudgerTypeManual 表示人工评分。
	JudgerTypeManual int16 = 6
)

const (
	// JudgerSelftestPending 表示判题器待自检。
	JudgerSelftestPending int16 = 1
	// JudgerSelftestPassed 表示判题器自检通过。
	JudgerSelftestPassed int16 = 2
	// JudgerSelftestFailed 表示判题器自检失败。
	JudgerSelftestFailed int16 = 3
)

const (
	// JudgerStatusAvailable 表示判题器可用。
	JudgerStatusAvailable int16 = 1
	// JudgerStatusDisabled 表示判题器已停用。
	JudgerStatusDisabled int16 = 2
)

const (
	// JudgeSandboxModeFresh 表示使用新判题沙箱。
	JudgeSandboxModeFresh int16 = 1
	// JudgeSandboxModeReuse 表示复用现场沙箱只读状态。
	JudgeSandboxModeReuse int16 = 2
)

const (
	// JudgeTaskStatusQueued 表示任务已排队。
	JudgeTaskStatusQueued int16 = 1
	// JudgeTaskStatusJudging 表示任务执行中或待人工评分。
	JudgeTaskStatusJudging int16 = 2
	// JudgeTaskStatusDone 表示任务完成。
	JudgeTaskStatusDone int16 = 3
	// JudgeTaskStatusTimeout 表示任务发生超时中间态。
	JudgeTaskStatusTimeout int16 = 4
	// JudgeTaskStatusFailed 表示任务系统性失败终态。
	JudgeTaskStatusFailed int16 = 5
	// JudgeTaskStatusError 表示任务发生系统性错误中间态。
	JudgeTaskStatusError int16 = 6
	// JudgeTaskStatusCancelled 表示任务已取消。
	JudgeTaskStatusCancelled int16 = 7
)

const (
	// JudgeOutboxPending 表示终态事件待发布。
	JudgeOutboxPending int16 = 1
	// JudgeOutboxPublished 表示终态事件已发布。
	JudgeOutboxPublished int16 = 2
	// JudgeOutboxFailed 表示终态事件发布失败待重试。
	JudgeOutboxFailed int16 = 3
)

const (
	// ProgressStageQueued 表示判题任务等待执行。
	ProgressStageQueued = "等待判题"
	// ProgressStageJudging 表示判题任务正在执行。
	ProgressStageJudging = "正在判题"
	// ProgressStageDone 表示判题任务已完成。
	ProgressStageDone = "判题完成"
	// ProgressStageFailed 表示判题任务执行失败。
	ProgressStageFailed = "判题失败"
)
