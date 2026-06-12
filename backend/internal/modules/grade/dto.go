// grade dto 文件定义 M11 HTTP 请求结构。
package grade

// LevelConfigRequest 是等级映射配置请求。
type LevelConfigRequest struct {
	Name         string       `json:"name"`
	Mapping      []LevelRule  `json:"mapping"`
	WarningRules WarningRules `json:"warning_rules"`
	IsDefault    bool         `json:"is_default"`
}

// SemesterRequest 是学期配置请求。
type SemesterRequest struct {
	Name      string `json:"name"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	IsCurrent bool   `json:"is_current"`
}

// ReviewRequest 是提交成绩审核请求。
type ReviewRequest struct {
	CourseID   int64  `json:"course_id,string"`
	SemesterID int64  `json:"semester_id,string,omitempty"`
	Comment    string `json:"comment"`
}

// ReviewDecisionRequest 是审核通过、驳回和解锁请求。
type ReviewDecisionRequest struct {
	SemesterID int64  `json:"semester_id,string,omitempty"`
	Comment    string `json:"comment"`
}

// AppealRequest 是学生成绩申诉请求。
type AppealRequest struct {
	CourseID int64  `json:"course_id,string"`
	Reason   string `json:"reason"`
}

// AppealDecisionRequest 是申诉处理请求。
type AppealDecisionRequest struct {
	Comment string `json:"comment"`
}

// TranscriptRequest 是成绩单生成请求。
type TranscriptRequest struct {
	StudentID  int64 `json:"student_id,string,omitempty"`
	Scope      int16 `json:"scope"`
	SemesterID int64 `json:"semester_id,string,omitempty"`
}

// LevelConfigDTO 表示等级映射配置响应。
type LevelConfigDTO struct {
	ID           int64        `json:"id,string"`
	TenantID     int64        `json:"tenant_id,string"`
	Name         string       `json:"name"`
	Mapping      []LevelRule  `json:"mapping"`
	WarningRules WarningRules `json:"warning_rules"`
	IsDefault    bool         `json:"is_default"`
	CreatedAt    string       `json:"created_at"`
	UpdatedAt    string       `json:"updated_at"`
}

// SemesterDTO 表示学期响应。
type SemesterDTO struct {
	ID        int64  `json:"id,string"`
	TenantID  int64  `json:"tenant_id,string"`
	Name      string `json:"name"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	IsCurrent bool   `json:"is_current"`
}

// ReviewDTO 表示成绩审核响应。
type ReviewDTO struct {
	ID          int64  `json:"id,string"`
	TenantID    int64  `json:"tenant_id,string"`
	CourseID    int64  `json:"course_id,string"`
	SemesterID  int64  `json:"semester_id,omitempty,string"`
	SubmitterID int64  `json:"submitter_id,string"`
	ReviewerID  int64  `json:"reviewer_id,omitempty,string"`
	Status      int16  `json:"status"`
	IsLocked    bool   `json:"is_locked"`
	Comment     string `json:"comment,omitempty"`
	SubmittedAt string `json:"submitted_at"`
	ReviewedAt  string `json:"reviewed_at,omitempty"`
}

// AppealDTO 表示成绩申诉响应。
type AppealDTO struct {
	ID            int64  `json:"id,string"`
	TenantID      int64  `json:"tenant_id,string"`
	StudentID     int64  `json:"student_id,string"`
	CourseID      int64  `json:"course_id,string"`
	Reason        string `json:"reason"`
	Status        int16  `json:"status"`
	HandlerID     int64  `json:"handler_id,omitempty,string"`
	ResultComment string `json:"result_comment,omitempty"`
	CreatedAt     string `json:"created_at"`
	HandledAt     string `json:"handled_at,omitempty"`
}

// WarningDTO 表示学业预警响应。
type WarningDTO struct {
	ID         int64          `json:"id,string"`
	TenantID   int64          `json:"tenant_id,string"`
	StudentID  int64          `json:"student_id,string"`
	SemesterID int64          `json:"semester_id,string"`
	Type       int16          `json:"type"`
	Detail     map[string]any `json:"detail"`
	Status     int16          `json:"status"`
	CreatedAt  string         `json:"created_at"`
}

// TranscriptDTO 表示成绩单元数据响应,PDFRef 仅供服务端下载授权使用。
type TranscriptDTO struct {
	ID          int64  `json:"id,string"`
	TenantID    int64  `json:"tenant_id,string"`
	StudentID   int64  `json:"student_id,string"`
	Scope       int16  `json:"scope"`
	SemesterID  int64  `json:"semester_id,omitempty,string"`
	PDFRef      string `json:"-"`
	GeneratedAt string `json:"generated_at"`
}
