// contracts 定义第 2 层实验模块对聚合层开放的只读统计与得分契约。
package contracts

import "context"

// ExperimentCloneRequest 的来源由已锁定课程产生，不接受外部请求直接指定。
type ExperimentCloneRequest struct {
	SourceTenantID      int64
	SourceCourseID      int64
	SourceTeacherID     int64
	TargetCourseID      int64
	LessonExperimentIDs []int64
}

// ExperimentClonePlan 由 M7 持有目标保存和对象补偿，只向 M6 返回实验编号映射。
type ExperimentClonePlan struct {
	IDs     map[int64]int64
	Save    func(context.Context) error
	Discard func(context.Context) error
}

// ExperimentCloneService 仅供 M6 克隆已授权课程；保存复用目标普通事务。
type ExperimentCloneService interface {
	PrepareExperimentClone(ctx context.Context, req ExperimentCloneRequest) (ExperimentClonePlan, error)
}

// ExperimentStats 是 M7 输出给 M9 的实验统计摘要。
type ExperimentStats struct {
	TenantID            int64 `json:"tenant_id"`
	CourseID            int64 `json:"course_id"`
	ExperimentCount     int64 `json:"experiment_count"`
	ActiveInstanceCount int64 `json:"active_instance_count"`
}

// ExperimentStatsQuery 是实验统计读取时使用的只读过滤条件。
type ExperimentStatsQuery struct {
	TenantID int64 `json:"tenant_id"`
	CourseID int64 `json:"course_id"`
}

// ExperimentScoreSnapshot 是 M7 输出给上层流程的实例得分快照。
type ExperimentScoreSnapshot struct {
	TenantID     int64   `json:"tenant_id"`
	ExperimentID int64   `json:"experiment_id"`
	InstanceID   int64   `json:"instance_id"`
	StudentID    int64   `json:"student_id"`
	Score        float64 `json:"score"`
}

// ExperimentReadService 是 M7 对聚合层和受控内部流程开放的只读契约。
type ExperimentReadService interface {
	// GetInstanceScore 读取单个实验实例的最终得分。内部 HTTP 入口必须使用服务签名上下文中的 tenant_id,不得信任查询参数覆盖租户边界。
	GetInstanceScore(ctx context.Context, tenantID, instanceID int64) (ExperimentScoreSnapshot, error)
	// Stats 按过滤条件读取实验统计,供 M9 学校看板聚合。内部 HTTP 入口必须使用服务签名上下文中的 tenant_id,不得信任查询参数覆盖租户边界。
	Stats(ctx context.Context, query ExperimentStatsQuery) (ExperimentStats, error)
}
