// sandbox enum 文件定义 M2 沙箱引擎请求、私有状态和技术事件使用的枚举常量。
package sandbox

import "chaimir/internal/contracts"

const (
	// BuiltinExecCapability 表示由 M2 通过运行时声明的受控命令执行 L2 链能力。
	BuiltinExecCapability = "sandbox-exec"
)

const (
	// SandboxPhaseAllocating 表示沙箱处于资源分配阶段。
	SandboxPhaseAllocating = contracts.SandboxPhaseAllocating
	// SandboxPhaseReady 表示环境就绪,前端已可进入。
	SandboxPhaseReady = contracts.SandboxPhaseReady
	// SandboxPhaseInitializing 表示个性化初始化仍在执行。
	SandboxPhaseInitializing = contracts.SandboxPhaseInitializing
	// SandboxPhaseFullyReady 表示沙箱完全可用。
	SandboxPhaseFullyReady = contracts.SandboxPhaseFullyReady
)

const (
	// SandboxStatusCreating 表示沙箱创建中。
	SandboxStatusCreating = contracts.SandboxStatusCreating
	// SandboxStatusRunning 表示沙箱运行中。
	SandboxStatusRunning = contracts.SandboxStatusRunning
	// SandboxStatusPaused 表示沙箱已暂停。
	SandboxStatusPaused = contracts.SandboxStatusPaused
	// SandboxStatusRecycling 表示沙箱回收中。
	SandboxStatusRecycling = contracts.SandboxStatusRecycling
	// SandboxStatusDestroyed 表示沙箱已销毁。
	SandboxStatusDestroyed = contracts.SandboxStatusDestroyed
	// SandboxStatusFailed 表示沙箱启动或运行失败。
	SandboxStatusFailed = contracts.SandboxStatusFailed
	// SandboxStatusReady 表示沙箱环境已就绪但尚未发生学生操作。
	SandboxStatusReady = contracts.SandboxStatusReady
	// SandboxStatusIdle 表示沙箱已运行但超过空闲计时阈值,等待回收或恢复操作。
	SandboxStatusIdle = contracts.SandboxStatusIdle
)

const (
	// RecycleOutboxStatusPending 表示回收事件待发布。
	RecycleOutboxStatusPending int16 = 1
	// RecycleOutboxStatusPublishing 表示回收事件已被 worker 领取并正在发布。
	RecycleOutboxStatusPublishing int16 = 2
	// RecycleOutboxStatusPublished 表示回收事件已发布成功。
	RecycleOutboxStatusPublished int16 = 3
	// RecycleOutboxStatusFailed 表示回收事件发布失败,等待重试。
	RecycleOutboxStatusFailed int16 = 4
)

const (
	// SandboxToolKindBuiltin 表示平台内建工具。
	SandboxToolKindBuiltin = contracts.SandboxToolKindBuiltin
	// SandboxToolKindTerminal 表示终端类工具。
	SandboxToolKindTerminal = contracts.SandboxToolKindTerminal
	// SandboxToolKindWebEmbed 表示 Web 嵌入类工具。
	SandboxToolKindWebEmbed = contracts.SandboxToolKindWebEmbed
)

const (
	// RuntimeStatusAvailable 表示运行时已通过自检并可调度。
	RuntimeStatusAvailable int16 = 1
	// RuntimeStatusOnboarding 表示运行时仍处于接入中。
	RuntimeStatusOnboarding int16 = 2
	// RuntimeStatusDisabled 表示运行时已停用。
	RuntimeStatusDisabled int16 = 3
)

const (
	// RuntimeSelftestPending 表示运行时待自检。
	RuntimeSelftestPending int16 = 1
	// RuntimeSelftestPassed 表示运行时自检通过。
	RuntimeSelftestPassed int16 = 2
	// RuntimeSelftestFailed 表示运行时自检失败。
	RuntimeSelftestFailed int16 = 3
)

const (
	// RuntimeImageStatusAvailable 表示运行时镜像可用于新沙箱。
	RuntimeImageStatusAvailable int16 = 1
	// RuntimeImageStatusDisabled 表示运行时镜像已停用,不得再用于新沙箱。
	RuntimeImageStatusDisabled int16 = 2
)

const (
	// ImagePrepullPending 表示镜像尚未预拉取。
	ImagePrepullPending int16 = 1
	// ImagePrepullSucceeded 表示镜像已经在目标节点真实预拉取完成。
	ImagePrepullSucceeded int16 = 2
	// ImagePrepullFailed 表示镜像预拉取失败。
	ImagePrepullFailed int16 = 3
	// ImagePrepullRunning 表示镜像预拉取仍在进行。
	ImagePrepullRunning int16 = 4
)

const (
	// ToolStatusAvailable 表示工具可调度。
	ToolStatusAvailable int16 = 1
	// ToolStatusDisabled 表示工具已停用。
	ToolStatusDisabled int16 = 2
)

const (
	// SandboxToolStatusReady 表示沙箱工具已就绪。
	SandboxToolStatusReady int16 = 1
	// SandboxToolStatusStarting 表示沙箱工具启动中。
	SandboxToolStatusStarting int16 = 2
	// SandboxToolStatusFailed 表示沙箱工具启动失败。
	SandboxToolStatusFailed int16 = 3
)

const (
	// EventTypeCreate 表示沙箱创建技术事件。
	EventTypeCreate = "create"
	// EventTypePhaseChange 表示沙箱阶段变化技术事件。
	EventTypePhaseChange = "phase_change"
	// EventTypeFileSave 表示沙箱文件持久化技术事件。
	EventTypeFileSave = "file_save"
	// EventTypeExec 表示沙箱命令执行技术事件。
	EventTypeExec = "exec"
	// EventTypeRecycle 表示沙箱回收技术事件。
	EventTypeRecycle = "recycle"
	// EventTypeError 表示沙箱错误技术事件。
	EventTypeError = "error"
)

const (
	// SandboxProgressStageAllocating 表示用户向进度处于环境分配阶段。
	SandboxProgressStageAllocating = "环境准备中"
	// SandboxProgressStageReady 表示用户向进度处于可进入阶段。
	SandboxProgressStageReady = "环境就绪"
	// SandboxProgressStageInitializing 表示用户向进度处于个性化初始化阶段。
	SandboxProgressStageInitializing = "初始化中"
	// SandboxProgressStageFailed 表示用户向进度处于准备失败阶段。
	SandboxProgressStageFailed = "准备失败"
	// SandboxProgressStageRecycling 表示用户向进度处于释放阶段。
	SandboxProgressStageRecycling = "环境释放中"
)

const (
	// InitAssetApplyPhaseInit 表示初始化资产在沙箱个性化初始化阶段开始时执行。
	InitAssetApplyPhaseInit = "init"
	// InitAssetApplyPhasePersonalization 表示初始化资产在文档定义的个性化阶段执行。
	InitAssetApplyPhasePersonalization = "personalization"
)

const (
	// WorkspacePlaceholderRoot 表示工作区根目录模板变量。
	WorkspacePlaceholderRoot = "{{workspace}}"
	// WorkspacePlaceholderPath 表示工作区内相对路径拼接后的目标模板变量。
	WorkspacePlaceholderPath = "{{path}}"
	// WorkspacePlaceholderScript 表示初始化脚本临时路径模板变量。
	WorkspacePlaceholderScript = "{{script}}"
)

const (
	// VolumeDomainWorkspace 表示学生代码工作区安全域。
	VolumeDomainWorkspace = "workspace"
	// VolumeDomainPublicAssets 表示公开只读素材安全域。
	VolumeDomainPublicAssets = "public-assets"
	// VolumeDomainRuntimeState 表示链节点账本和运行态安全域。
	VolumeDomainRuntimeState = "runtime-state"
	// VolumeDomainJudgePrivate 表示隐藏测试、答案和评分脚本私有域。
	VolumeDomainJudgePrivate = "judge-private"
)

const (
	// VolumeAccessNone 表示学生不可访问该卷域。
	VolumeAccessNone = "none"
	// VolumeAccessReadOnly 表示学生只读访问该卷域。
	VolumeAccessReadOnly = "read_only"
	// VolumeAccessReadWrite 表示学生可读写该卷域。
	VolumeAccessReadWrite = "read_write"
)

const (
	// VolumePersistenceMinioCode 表示仅工作区代码由 MinIO 持久化。
	VolumePersistenceMinioCode = "minio_code"
	// VolumePersistenceEphemeral 表示卷域默认随沙箱销毁。
	VolumePersistenceEphemeral = "ephemeral"
	// VolumePersistenceSnapshot 表示卷域只允许通过受控快照保留。
	VolumePersistenceSnapshot = "snapshot"
)

const (
	// VolumeSnapshotNever 表示该卷域永不进入学生沙箱快照。
	VolumeSnapshotNever = "never"
	// VolumeSnapshotAlways 表示该卷域属于默认快照范围。
	VolumeSnapshotAlways = "always"
	// VolumeSnapshotEnabled 表示仅显式开启快照时纳入。
	VolumeSnapshotEnabled = "snapshot_enabled"
)
