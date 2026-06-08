// M3 枚举常量:集中定义判题器类型、状态、沙箱模式与自检状态。
package judge

const (
	// JudgerTypeTestcase 至 JudgerTypeManual 定义 M3 支持的判题器类型。
	JudgerTypeTestcase      int16 = 1
	JudgerTypeOnchainAssert int16 = 2
	JudgerTypeFlag          int16 = 3
	JudgerTypeStaticScan    int16 = 4
	JudgerTypeSimCheckpoint int16 = 5
	JudgerTypeManual        int16 = 6

	// JudgerSelftestPending 至 JudgerSelftestFailed 定义判题器自检状态。
	JudgerSelftestPending int16 = 1
	JudgerSelftestPassed  int16 = 2
	JudgerSelftestFailed  int16 = 3

	// JudgerStatusAvailable 至 JudgerStatusDisabled 定义判题器接入状态。
	JudgerStatusAvailable  int16 = 1
	JudgerStatusOnboarding int16 = 2
	JudgerStatusDisabled   int16 = 3

	// JudgeTaskQueued 至 JudgeTaskFailed 定义判题任务生命周期状态。
	JudgeTaskQueued    int16 = 1
	JudgeTaskJudging   int16 = 2
	JudgeTaskDone      int16 = 3
	JudgeTaskTimeout   int16 = 4
	JudgeTaskError     int16 = 5
	JudgeTaskCancelled int16 = 6
	JudgeTaskFailed    int16 = 7

	// SandboxModeFresh 和 SandboxModeReuse 定义 M3 使用 M2 沙箱的方式。
	SandboxModeFresh int16 = 1
	SandboxModeReuse int16 = 2
)

const (
	// SandboxModeFreshText 和 SandboxModeReuseText 是 contracts/API 层使用的文本值。
	SandboxModeFreshText = "fresh"
	SandboxModeReuseText = "reuse"
)
