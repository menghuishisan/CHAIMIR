// contracts 定义第 2 层实验模块对聚合层开放的只读统计与得分契约。
package contracts

import "context"

// ExperimentStats 是 M7 输出给 M9 的实验统计摘要。
type ExperimentStats struct {
	TenantID            int64
	CourseID            int64
	ExperimentCount     int64
	ActiveInstanceCount int64
}

// ExperimentStatsQuery 是实验统计读取时使用的只读过滤条件。
type ExperimentStatsQuery struct {
	TenantID int64
	CourseID int64
}

// ExperimentScoreSnapshot 是 M7 输出给上层流程的实例得分快照。
type ExperimentScoreSnapshot struct {
	TenantID     int64
	ExperimentID int64
	InstanceID   int64
	StudentID    int64
	Score        float64
}

// ExperimentReadService 是 M7 对聚合层和受控内部流程开放的只读契约。
type ExperimentReadService interface {
	// GetInstanceScore 读取单个实验实例的最终得分。
	GetInstanceScore(ctx context.Context, tenantID, instanceID int64) (ExperimentScoreSnapshot, error)
	// Stats 按过滤条件读取实验统计,供 M9 学校看板聚合。
	Stats(ctx context.Context, query ExperimentStatsQuery) (ExperimentStats, error)
}
