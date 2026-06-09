// M6 领域模型:定义教学 service 编排所需的最小业务投影。
package teaching

import "time"

// CourseAccessSnapshot 承载课程权限、状态机和克隆命名需要的最小课程信息。
type CourseAccessSnapshot struct {
	ID         int64
	TeacherID  int64
	Name       string
	Visibility int16
	Status     int16
}

// ChapterLocation 标识章节所属课程,用于课时和目录权限校验。
type ChapterLocation struct {
	ID       int64
	CourseID int64
}

// LessonContentSnapshot 承载课时详情响应和章节归属校验需要的内容投影。
type LessonContentSnapshot struct {
	ID          int64
	ChapterID   int64
	Title       string
	ContentType int16
	ContentRef  map[string]any
	Sort        int32
}

// AssignmentPolicySnapshot 承载作业发布状态、迟交策略和提交次数限制。
type AssignmentPolicySnapshot struct {
	ID          int64
	CourseID    int64
	Title       string
	ChapterID   int64
	DueAt       time.Time
	MaxAttempts int32
	LatePolicy  int16
	LatePenalty map[string]any
	Status      int16
}

// AssignmentItemSnapshot 承载作业题目引用、分值和判题器选择。
type AssignmentItemSnapshot struct {
	ID          int64
	ItemCode    string
	ItemVersion string
	Score       int32
	Seq         int32
	GradingMode int16
	JudgerCode  string
}

// SubmissionScoreSnapshot 承载提交归属校验、评分计算和反馈响应需要的提交信息。
type SubmissionScoreSnapshot struct {
	ID           int64
	TenantID     int64
	AssignmentID int64
	StudentID    int64
	AttemptNo    int32
	ContentRef   map[string]any
	JudgeTaskRef string
	AutoScore    *int32
	ManualScore  *int32
	FinalScore   *int32
	Comment      string
	IsLate       bool
	Status       int16
	SubmittedAt  time.Time
}

// SubmissionJudgeOutboxSnapshot 承载 service 派发 M3 判题任务需要的 outbox 信息。
type SubmissionJudgeOutboxSnapshot struct {
	ID             int64
	TenantID       int64
	SubmissionID   int64
	AssignmentID   int64
	StudentID      int64
	ItemCode       string
	ItemVersion    string
	JudgerCode     string
	CodeStorageKey string
	CodeHash       string
	ExtraInput     map[string]any
	SourceRef      string
}

// CourseGradeSnapshot 承载成绩响应和审计 old/new 快照需要的成绩信息。
type CourseGradeSnapshot struct {
	ID            int64
	TenantID      int64
	CourseID      int64
	StudentID     int64
	Credits       float64
	AutoTotal     float64
	OverrideTotal *float64
	FinalTotal    float64
	IsOverridden  bool
}

// ProgressSnapshot 承载学习进度统计需要的最小状态和时长。
type ProgressSnapshot struct {
	Status      int16
	DurationSec int32
}

// AssignmentScoreSnapshot 承载成绩计算需要的最新作业得分。
type AssignmentScoreSnapshot struct {
	AssignmentID int64
	StudentID    int64
	FinalScore   *int32
}

// SubmissionJudgeOutboxCreate 承载提交事务内创建判题 outbox 所需数据。
type SubmissionJudgeOutboxCreate struct {
	ItemCode       string
	ItemVersion    string
	JudgerCode     string
	CodeStorageKey string
	CodeHash       string
	ExtraInput     []byte
	SourceRef      string
}
