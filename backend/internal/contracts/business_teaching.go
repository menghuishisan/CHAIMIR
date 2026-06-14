// contracts 定义第 2 层教学模块对聚合层与横切能力开放的只读契约。
package contracts

import "context"

// TeachingCourseGrade 是 M6 输出给 M11 的单课程成绩摘要。
type TeachingCourseGrade struct {
	TenantID      int64
	CourseID      int64
	Semester      string
	StudentID     int64
	AutoTotal     float64
	OverrideTotal *float64
	FinalTotal    float64
	IsOverridden  bool
	Credits       float64
}

// TeachingCourseInfo 是 M6 输出给 M11 的课程归属摘要,用于聚合层校验权限与学期范围。
type TeachingCourseInfo struct {
	TenantID  int64
	CourseID  int64
	TeacherID int64
	Semester  string
	Credits   float64
	Status    int16
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
	// GetCourse 读取课程归属摘要,供 M11 审核和申诉流程校验课程边界。
	GetCourse(ctx context.Context, tenantID, courseID int64) (TeachingCourseInfo, error)
	// GetCourseGrade 读取单个学生在单课程的成绩,供 M11 申诉校验成绩存在性。
	GetCourseGrade(ctx context.Context, tenantID, courseID, studentID int64) (TeachingCourseGrade, error)
	// IsCourseMember 判断学生是否属于课程,供 M11 防止跨课程申诉。
	IsCourseMember(ctx context.Context, tenantID, courseID, studentID int64) (bool, error)
	// ListCourseGrades 读取单课程成绩,供 M11 GPA 聚合与审核流程使用。
	ListCourseGrades(ctx context.Context, tenantID, courseID int64) ([]TeachingCourseGrade, error)
	// ListStudentGrades 读取某学生的单课程成绩集合,供 M11 学期与累计 GPA 聚合。
	ListStudentGrades(ctx context.Context, tenantID, studentID int64) ([]TeachingCourseGrade, error)
	// Stats 读取租户级教学统计,供 M9 学校看板聚合。
	Stats(ctx context.Context, tenantID int64) (TeachingStats, error)
}
