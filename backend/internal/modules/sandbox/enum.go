// M2 沙箱枚举常量与配置上限,与 docs/02-沙箱引擎 数据模型/状态机保持一致。
package sandbox

const (
	RuntimeSelftestPending int16 = 1
	RuntimeSelftestPassed  int16 = 2
	RuntimeSelftestFailed  int16 = 3
)

const (
	RuntimeStatusAvailable  int16 = 1
	RuntimeStatusOnboarding int16 = 2
	RuntimeStatusDisabled   int16 = 3
)

const (
	RuntimeImagePrepullPending int16 = 1
	RuntimeImagePrepullDone    int16 = 2
	RuntimeImagePrepullFailed  int16 = 3
	RuntimeImagePrepullRunning int16 = 4
)

const (
	ToolKindTerminal        int16 = 1
	ToolKindWebEmbed        int16 = 2
	ToolKindPlatformBuiltin int16 = 3
)

const (
	ToolStatusAvailable int16 = 1
	ToolStatusDisabled  int16 = 2
)

const (
	SandboxPhaseAllocating       int16 = 1
	SandboxPhaseEnvironmentReady int16 = 2
	SandboxPhaseInitializing     int16 = 3
	SandboxPhaseReady            int16 = 4
)

const (
	SandboxStatusCreating  int16 = 1
	SandboxStatusReady     int16 = 2
	SandboxStatusRunning   int16 = 3
	SandboxStatusIdle      int16 = 4
	SandboxStatusRecycling int16 = 5
	SandboxStatusDestroyed int16 = 6
	SandboxStatusError     int16 = 7
)

const (
	SandboxToolStatusReady    int16 = 1
	SandboxToolStatusStarting int16 = 2
	SandboxToolStatusFailed   int16 = 3
)

const (
	SandboxEventCreate      = "create"
	SandboxEventPhaseChange = "phase_change"
	SandboxEventRecycle     = "recycle"
	SandboxEventError       = "error"
	SandboxEventSaveFiles   = "save_files"
)
