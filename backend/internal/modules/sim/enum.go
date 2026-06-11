// sim enum 文件定义 M4 内部状态常量,与数据库 smallint 枚举一一对应。
package sim

const (
	// ComputeFrontend 表示仿真默认在前端 Worker 中执行。
	ComputeFrontend int16 = 1
	// ComputeBackend 表示仿真由 M4 后端计算适配器执行。
	ComputeBackend int16 = 2
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
