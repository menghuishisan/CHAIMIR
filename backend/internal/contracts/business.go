// 第2层 业务模块对高层聚合开放的只读/受控写契约。
// M6 teaching 暴露单课程成绩能力给 M11,但不反向依赖 M11。
package contracts

import "context"

// TeachingCourseGrade 是 M6 单课程成绩摘要。
type TeachingCourseGrade struct {
	TenantID      int64
	CourseID      int64
	StudentID     int64
	AutoTotal     float64
	OverrideTotal *float64
	FinalTotal    float64
	IsOverridden  bool
	Credits       float64
}

// TeachingStats 是 M6 给 M9 看板的教学统计摘要。
type TeachingStats struct {
	TenantID            int64
	CourseCount         int64
	ActiveCourseCount   int64
	LearningDurationSec int64
}

// TeachingService 是 M6 对上层聚合开放的契约。
type TeachingService interface {
	// ListCourseGrades 读取单课程成绩,供 M11 跨课程聚合。
	ListCourseGrades(ctx context.Context, tenantID, courseID int64) ([]TeachingCourseGrade, error)
	// ListStudentGrades 读取学生跨课程成绩,供 M11 学生 GPA 和预警聚合。
	ListStudentGrades(ctx context.Context, tenantID, studentID int64) ([]TeachingCourseGrade, error)
	// Stats 读取教学统计,供 M9 看板聚合。
	Stats(ctx context.Context, tenantID int64) (TeachingStats, error)
}

// ExperimentStats 是 M7 给 M9 看板的实验统计摘要。
type ExperimentStats struct {
	TenantID            int64
	CourseID            int64
	ExperimentCount     int64
	ActiveInstanceCount int64
}

// ExperimentService 是 M7 对上层聚合开放的只读契约。
type ExperimentService interface {
	// Stats 读取实验统计,供 M9 看板聚合。
	Stats(ctx context.Context, tenantID, courseID int64) (ExperimentStats, error)
}

// ContestStats 是 M8 给 M9 看板的竞赛统计摘要。
type ContestStats struct {
	TenantID           int64
	ContestCount       int64
	ActiveContestCount int64
	TeamCount          int64
}

// ContestAchievement 是 M8 给 M11 只读展示的竞赛成就摘要。
type ContestAchievement struct {
	TenantID  int64
	StudentID int64
	ContestID int64
	TeamID    int64
	Score     float64
	Rank      int32
}

// ContestService 是 M8 对上层聚合开放的只读契约。
type ContestService interface {
	// Stats 读取竞赛统计,供 M9 看板聚合。
	Stats(ctx context.Context, tenantID int64) (ContestStats, error)
	// ListStudentAchievements 读取学生竞赛成就,供 M11 独立展示且不计入 GPA。
	ListStudentAchievements(ctx context.Context, tenantID, studentID int64) ([]ContestAchievement, error)
}
