// M11 DTO 定义:等级配置、学期、审核、GPA、申诉、预警与成绩单请求响应结构。
package grade

import "time"

// LevelMappingDTO 是分数段到等级和绩点的映射项。
type LevelMappingDTO struct {
	Min   float64 `json:"min"`
	Grade string  `json:"grade"`
	GPA   float64 `json:"gpa"`
}

// WarningRuleDTO 是学业预警规则配置。
type WarningRuleDTO struct {
	FailCount int     `json:"fail_count"`
	MinGPA    float64 `json:"min_gpa"`
}

// LevelConfigRequest 是等级映射配置保存请求。
type LevelConfigRequest struct {
	Name         string            `json:"name"`
	Mapping      []LevelMappingDTO `json:"mapping"`
	WarningRules WarningRuleDTO    `json:"warning_rules"`
	IsDefault    bool              `json:"is_default"`
}

// LevelConfigDTO 是等级映射配置响应。
type LevelConfigDTO struct {
	ID           string            `json:"id"`
	TenantID     string            `json:"tenant_id"`
	Name         string            `json:"name"`
	Mapping      []LevelMappingDTO `json:"mapping"`
	WarningRules WarningRuleDTO    `json:"warning_rules"`
	IsDefault    bool              `json:"is_default"`
}

// SemesterRequest 是学期配置请求。
type SemesterRequest struct {
	Name      string `json:"name"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	IsCurrent bool   `json:"is_current"`
}

// SemesterDTO 是学期响应。
type SemesterDTO struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Name      string    `json:"name"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	IsCurrent bool      `json:"is_current"`
}

// ReviewCreateRequest 是教师提交课程成绩审核请求。
type ReviewCreateRequest struct {
	CourseID string `json:"course_id"`
}

// ReviewDecisionRequest 是管理员审核处理请求。
type ReviewDecisionRequest struct {
	Comment    string `json:"comment"`
	SemesterID string `json:"semester_id"`
}

// ReviewDTO 是成绩审核记录响应。
type ReviewDTO struct {
	ID          string     `json:"id"`
	TenantID    string     `json:"tenant_id"`
	CourseID    string     `json:"course_id"`
	SemesterID  string     `json:"semester_id,omitempty"`
	SubmitterID string     `json:"submitter_id"`
	ReviewerID  string     `json:"reviewer_id,omitempty"`
	Status      int16      `json:"status"`
	IsLocked    bool       `json:"is_locked"`
	Comment     string     `json:"comment,omitempty"`
	SubmittedAt time.Time  `json:"submitted_at"`
	ReviewedAt  *time.Time `json:"reviewed_at,omitempty"`
}

// RecomputeRequest 是 GPA 重算请求。
type RecomputeRequest struct {
	CourseID   string `json:"course_id"`
	SemesterID string `json:"semester_id"`
}

// SemesterGradeUpsert 是学期聚合写入参数。
type SemesterGradeUpsert struct {
	ID            int64
	StudentID     int64
	SemesterID    int64
	TotalCredits  float64
	GPA           float64
	CumulativeGPA float64
}

// SemesterGradeDTO 是学生学期 GPA 聚合结果。
type SemesterGradeDTO struct {
	ID            string    `json:"id"`
	TenantID      string    `json:"tenant_id"`
	StudentID     string    `json:"student_id"`
	SemesterID    string    `json:"semester_id"`
	TotalCredits  float64   `json:"total_credits"`
	GPA           float64   `json:"gpa"`
	CumulativeGPA float64   `json:"cumulative_gpa"`
	ComputedAt    time.Time `json:"computed_at"`
}

// StudentGradesDTO 是学生成绩详情响应。
type StudentGradesDTO struct {
	StudentID string             `json:"student_id"`
	Semester  string             `json:"semester_id,omitempty"`
	Courses   []CourseGradeDTO   `json:"courses"`
	GPA       []SemesterGradeDTO `json:"gpa"`
}

// CourseGradeDTO 是从 M6 只读取得的单课程成绩展示对象。
type CourseGradeDTO struct {
	CourseID   string  `json:"course_id"`
	StudentID  string  `json:"student_id"`
	FinalTotal float64 `json:"final_total"`
	Credits    float64 `json:"credits"`
	Grade      string  `json:"grade,omitempty"`
	GPA        float64 `json:"gpa"`
}

// AppealCreateRequest 是学生提交申诉请求。
type AppealCreateRequest struct {
	CourseID string `json:"course_id"`
	Reason   string `json:"reason"`
}

// AppealHandleRequest 是申诉处理请求。
type AppealHandleRequest struct {
	ResultComment string `json:"result_comment"`
}

// AppealDTO 是申诉记录响应。
type AppealDTO struct {
	ID            string     `json:"id"`
	TenantID      string     `json:"tenant_id"`
	StudentID     string     `json:"student_id"`
	CourseID      string     `json:"course_id"`
	Reason        string     `json:"reason"`
	Status        int16      `json:"status"`
	HandlerID     string     `json:"handler_id,omitempty"`
	ResultComment string     `json:"result_comment,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	HandledAt     *time.Time `json:"handled_at,omitempty"`
}

// WarningScanRequest 是学业预警扫描请求。
type WarningScanRequest struct {
	SemesterID string `json:"semester_id"`
}

// WarningCreate 是写入学业预警的参数。
type WarningCreate struct {
	StudentID  int64
	SemesterID int64
	Type       int16
	Detail     map[string]any
}

// WarningDTO 是学业预警响应。
type WarningDTO struct {
	ID         string         `json:"id"`
	TenantID   string         `json:"tenant_id"`
	StudentID  string         `json:"student_id"`
	SemesterID string         `json:"semester_id"`
	Type       int16          `json:"type"`
	Detail     map[string]any `json:"detail"`
	Status     int16          `json:"status"`
	CreatedAt  time.Time      `json:"created_at"`
}

// TranscriptRequest 是生成成绩单请求。
type TranscriptRequest struct {
	StudentID  string `json:"student_id"`
	Scope      int16  `json:"scope"`
	SemesterID string `json:"semester_id,omitempty"`
}

// TranscriptBatchRequest 是批量生成成绩单请求。
type TranscriptBatchRequest struct {
	StudentIDs []string `json:"student_ids"`
	Scope      int16    `json:"scope"`
	SemesterID string   `json:"semester_id,omitempty"`
}

// TranscriptDTO 是成绩单记录响应。
type TranscriptDTO struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	StudentID   string    `json:"student_id"`
	Scope       int16     `json:"scope"`
	SemesterID  string    `json:"semester_id,omitempty"`
	PDFRef      string    `json:"-"`
	GeneratedAt time.Time `json:"generated_at"`
}
