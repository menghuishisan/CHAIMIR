// M4 枚举定义:集中维护仿真包、审核、会话、分享等状态常量。
package sim

const (
	// ComputeFrontend 表示仿真在前端 Worker 中运行。
	ComputeFrontend int16 = 1
	// ComputeBackend 表示仿真由 M4 后端计算适配器运行。
	ComputeBackend int16 = 2
)

const (
	// AuthorTypeBuiltin 表示平台内置仿真包。
	AuthorTypeBuiltin int16 = 1
	// AuthorTypeTeacher 表示教师提交的仿真包。
	AuthorTypeTeacher int16 = 2
	// AuthorTypeOrg 表示第三方组织提交的仿真包。
	AuthorTypeOrg int16 = 3
)

const (
	// PackageStatusDraft 表示仿真包草稿态。
	PackageStatusDraft int16 = 1
	// PackageStatusReviewing 表示仿真包审核中。
	PackageStatusReviewing int16 = 2
	// PackageStatusPublished 表示仿真包已上架。
	PackageStatusPublished int16 = 3
	// PackageStatusArchived 表示仿真包已下架。
	PackageStatusArchived int16 = 4
	// PackageStatusRejected 表示仿真包被退回。
	PackageStatusRejected int16 = 5
)

const (
	// ReviewResultPending 表示审核记录待处理。
	ReviewResultPending int16 = 1
	// ReviewResultApproved 表示审核通过。
	ReviewResultApproved int16 = 2
	// ReviewResultRejected 表示审核退回。
	ReviewResultRejected int16 = 3
)

const (
	// SessionStatusCreating 表示会话创建中。
	SessionStatusCreating int16 = 1
	// SessionStatusRunning 表示会话运行中。
	SessionStatusRunning int16 = 2
	// SessionStatusIdle 表示会话空闲。
	SessionStatusIdle int16 = 3
	// SessionStatusCompleted 表示会话已完成。
	SessionStatusCompleted int16 = 4
	// SessionStatusArchived 表示会话已归档。
	SessionStatusArchived int16 = 5
	// SessionStatusFailed 表示会话失败。
	SessionStatusFailed int16 = 6
)

const (
	// ShareStatusActive 表示分享码有效。
	ShareStatusActive int16 = 1
	// ShareStatusRevoked 表示分享码已撤销。
	ShareStatusRevoked int16 = 2
	// ShareStatusExpired 表示分享码已过期。
	ShareStatusExpired int16 = 3
)
