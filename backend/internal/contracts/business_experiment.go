// contracts 定义第 2 层实验模块对聚合层开放的只读统计与得分契约。
package contracts

import "context"

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
	// GetInstanceScore 读取单个实验实例的最终得分。
	GetInstanceScore(ctx context.Context, tenantID, instanceID int64) (ExperimentScoreSnapshot, error)
	// Stats 按过滤条件读取实验统计,供 M9 学校看板聚合。
	Stats(ctx context.Context, query ExperimentStatsQuery) (ExperimentStats, error)
}
