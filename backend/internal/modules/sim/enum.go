// sim enum 文件定义 M4 内部状态常量与对外稳定协议字符串枚举:
// 前者与数据库 smallint 一一对应,后者是 HTTP/WS 契约上不可随意改动的取值(前端 constants/sim.ts 与之同源)。
package sim

// 执行位置按代码来源派生,不是作者可选项(见 docs/04-仿真可视化引擎/02-架构设计.md §8):
// 平台内置包是随版本交付的平台代码,在浏览器 Worker 内运行;教师与第三方扩展包是外部提交的
// 可执行代码,一律在后端隔离容器内运行,浏览器只渲染容器回传的纯数据教学帧。
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
	// SessionCreating 表示会话创建中。
	SessionCreating int16 = 1
	// SessionRunning 表示会话进行中。
	SessionRunning int16 = 2
	// SessionIdle 表示会话空闲。
	SessionIdle int16 = 3
	// SessionCompleted 表示会话已完成。
	SessionCompleted int16 = 4
	// SessionArchived 表示会话已归档。
	SessionArchived int16 = 5
	// SessionFailed 表示会话失败。
	SessionFailed int16 = 6
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
