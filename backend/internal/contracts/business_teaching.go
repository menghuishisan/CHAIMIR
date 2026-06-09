// contracts 定义第 2 层教学模块对聚合层与横切能力开放的只读契约。
package contracts

import "context"

// TeachingCourseGrade 是 M6 输出给 M11 的单课程成绩摘要。
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

// TeachingStats 是 M6 输出给 M9 的教学统计摘要。
type TeachingStats struct {
	TenantID            int64
	CourseCount         int64
	ActiveCourseCount   int64
	LearningDurationSec int64
}

// TeachingReadService 是 M6 对 M9/M11 开放的只读教学契约。
type TeachingReadService interface {
	// ListCourseGrades 读取单课程成绩,供 M11 GPA 聚合与审核流程使用。
	ListCourseGrades(ctx context.Context, tenantID, courseID int64) ([]TeachingCourseGrade, error)
	// ListStudentGrades 读取某学生的单课程成绩集合,供 M11 学期与累计 GPA 聚合。
	ListStudentGrades(ctx context.Context, tenantID, studentID int64) ([]TeachingCourseGrade, error)
	// Stats 读取租户级教学统计,供 M9 学校看板聚合。
	Stats(ctx context.Context, tenantID int64) (TeachingStats, error)
}
