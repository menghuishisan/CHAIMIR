// grade model 文件定义 M11 成绩中心领域模型和响应视图。
package grade

import (
	"time"

	"chaimir/internal/platform/ids"
)

// LevelRule 是分数到等级和绩点的映射规则。
type LevelRule struct {
	Min   float64 `json:"min"`
	Grade string  `json:"grade"`
	GPA   float64 `json:"gpa"`
}

// WarningRules 是学业预警阈值配置。
type WarningRules struct {
	FailCount int     `json:"fail_count"`
	MinGPA    float64 `json:"min_gpa"`
}

// CourseGradeInput 是 GPA 计算所需的单课程成绩输入。
type CourseGradeInput struct {
	CourseID   ids.ID  `json:"course_id"`
	StudentID  ids.ID  `json:"student_id"`
	FinalTotal float64 `json:"final_total"`
	Credits    float64 `json:"credits"`
}

// GradeSummaryDTO 是学生成绩聚合响应。
type GradeSummaryDTO struct {
	StudentID     ids.ID             `json:"student_id"`
	SemesterID    ids.ID             `json:"semester_id,omitempty"`
	TotalCredits  float64            `json:"total_credits"`
	GPA           float64            `json:"gpa"`
	CumulativeGPA float64            `json:"cumulative_gpa"`
	CourseGrades  []CourseGradeInput `json:"course_grades"`
	ComputedAt    time.Time          `json:"computed_at"`
}

// GradeLockOutbox 是成绩锁定事件的生产者 outbox 记录。
type GradeLockOutbox struct {
	ID         int64
	TenantID   int64
	ReviewID   int64
	CourseID   int64
	Locked     bool
	Reason     string
	TraceID    string
	Status     int16
	RetryCount int32
	LastError  string
	CreatedAt  string
	UpdatedAt  string
}
