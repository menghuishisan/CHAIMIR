package sim

// 执行位置由平台按代码来源确定，上传者不能选择。
const (
	// ComputeBrowser 表示仿真在浏览器 Worker 中执行(仅平台内置包)。
	ComputeBrowser int16 = 1
	// ComputeIsolated 表示仿真在后端隔离容器中执行(扩展包与重计算仿真)。
	ComputeIsolated int16 = 2
)

const (
	// PackageStatusDraft 表示草稿态。
	PackageStatusDraft int16 = 1
	// PackageStatusReviewing 表示待平台审核。
	PackageStatusReviewing int16 = 2
	// PackageStatusPublished 表示已上架。
	PackageStatusPublished int16 = 3
	// PackageStatusArchived 表示已下架。
	PackageStatusArchived int16 = 4
	// PackageStatusRejected 表示审核退回。
	PackageStatusRejected int16 = 5
)

const (
	// AuthorPlatformBuiltIn 表示平台内置包。
	AuthorPlatformBuiltIn int16 = 1
	// AuthorTeacher 表示教师扩展包。
	AuthorTeacher int16 = 2
	// AuthorThirdParty 表示第三方组织扩展包。
	AuthorThirdParty int16 = 3
)

// builtinSimCodePrefix 是平台内置包的 code 前缀,与数据库 CHECK 约束、
// 前端 sim-sdk 的 BUILTIN_SIM_CODE_PREFIX 同源;前端据此判定一个包能否在本机 Worker 运行。
const builtinSimCodePrefix = "builtin__"

const (
	// ReviewPending 表示审核待处理。
	ReviewPending int16 = 1
	// ReviewApproved 表示审核通过。
	ReviewApproved int16 = 2
	// ReviewRejected 表示审核退回。
	ReviewRejected int16 = 3
)

const (
	// SessionAvailable 允许连接执行和追加操作，断线不改变此状态。
	SessionAvailable int16 = 1
	// SessionArchived 表示会话已归档。
	SessionArchived int16 = 2
)

const (
	// ShareActive 表示分享码有效。
	ShareActive int16 = 1
	// ShareRevoked 表示分享码已撤销。
	ShareRevoked int16 = 2
	// ShareExpired 表示分享码已过期。
	ShareExpired int16 = 3
)

const (
	validationPassed = "passed"
	validationFailed = "failed"
	// 扫描风险提示不替代人工审核，也不表示运行校验通过。
	validationWarning = "warning"
)

// 隔离执行 WebSocket 的帧类型:首帧带包自描述信息,其后只带快照。
// 与前端 `api-client/src/constants/sim.ts` 的 SIM_STREAM_FRAME 同源。
const (
	// backendStreamReady 是携带包自描述信息的首帧类型。
	backendStreamReady = "ready"
	// backendStreamSnapshot 是后续只带快照的帧类型。
	backendStreamSnapshot = "snapshot"
)

// BackendCommandKind 是隔离执行连接支持的受控命令类型。
// 与前端 `api-client/src/constants/sim.ts` 的 SIM_STREAM_COMMAND 同源,四种之外一律拒绝。
type BackendCommandKind string

const (
	// BackendCommandStep 推进一个推演时刻。
	BackendCommandStep BackendCommandKind = "step"
	// BackendCommandEvent 注入一次包内声明的交互。
	BackendCommandEvent BackendCommandKind = "event"
	// BackendCommandBack 回退一步。
	BackendCommandBack BackendCommandKind = "back"
	// BackendCommandRestart 回到初始状态。
	BackendCommandRestart BackendCommandKind = "restart"
)
