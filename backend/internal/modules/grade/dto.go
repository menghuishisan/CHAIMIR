// grade dto 文件定义 M11 HTTP 请求结构。
package grade

import "chaimir/internal/platform/ids"

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
	CourseID   ids.ID `json:"course_id"`
	SemesterID ids.ID `json:"semester_id,omitempty"`
	Comment    string `json:"comment"`
}

// ReviewDecisionRequest 是审核通过、驳回和解锁请求。
type ReviewDecisionRequest struct {
	SemesterID ids.ID `json:"semester_id,omitempty"`
	Comment    string `json:"comment"`
}

// AppealRequest 是学生成绩申诉请求。
type AppealRequest struct {
	CourseID ids.ID `json:"course_id"`
	Reason   string `json:"reason"`
}

// AppealDecisionRequest 是申诉处理请求。
type AppealDecisionRequest struct {
	Comment string `json:"comment"`
}

// TranscriptRequest 是成绩单生成请求。
type TranscriptRequest struct {
	StudentID  ids.ID `json:"student_id,omitempty"`
	Scope      int16  `json:"scope"`
	SemesterID ids.ID `json:"semester_id,omitempty"`
}

// TranscriptBatchRequest 是批量成绩单生成请求。
type TranscriptBatchRequest struct {
	StudentIDs []ids.ID `json:"student_ids"`
	Scope      int16    `json:"scope"`
	SemesterID ids.ID   `json:"semester_id,omitempty"`
}

// RecomputeRequest 是学生 GPA 重算请求。
type RecomputeRequest struct {
	SemesterID ids.ID `json:"semester_id"`
}

// WarningScanRequest 是学业预警扫描请求。
type WarningScanRequest struct {
	StudentID  ids.ID `json:"student_id,omitempty"`
	SemesterID ids.ID `json:"semester_id,omitempty"`
}

// WarningScanResultDTO 表示一次学业预警扫描结果。
type WarningScanResultDTO struct {
	Scanned int `json:"scanned"`
	Created int `json:"created"`
}

// LevelConfigDTO 表示等级映射配置响应。
type LevelConfigDTO struct {
	ID           ids.ID       `json:"id"`
	TenantID     ids.ID       `json:"tenant_id"`
	Name         string       `json:"name"`
	Mapping      []LevelRule  `json:"mapping"`
	WarningRules WarningRules `json:"warning_rules"`
	IsDefault    bool         `json:"is_default"`
	CreatedAt    string       `json:"created_at"`
	UpdatedAt    string       `json:"updated_at"`
}

// SemesterDTO 表示学期响应。
type SemesterDTO struct {
	ID        ids.ID `json:"id"`
	TenantID  ids.ID `json:"tenant_id"`
	Name      string `json:"name"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	IsCurrent bool   `json:"is_current"`
}

// ReviewDTO 表示成绩审核响应。
type ReviewDTO struct {
	ID          ids.ID `json:"id"`
	TenantID    ids.ID `json:"tenant_id"`
	CourseID    ids.ID `json:"course_id"`
	SemesterID  ids.ID `json:"semester_id,omitempty"`
	SubmitterID ids.ID `json:"submitter_id"`
	ReviewerID  ids.ID `json:"reviewer_id,omitempty"`
	Status      int16  `json:"status"`
	IsLocked    bool   `json:"is_locked"`
	Comment     string `json:"comment,omitempty"`
	SubmittedAt string `json:"submitted_at"`
	ReviewedAt  string `json:"reviewed_at,omitempty"`
}

// AppealDTO 表示成绩申诉响应。
type AppealDTO struct {
	ID            ids.ID `json:"id"`
	TenantID      ids.ID `json:"tenant_id"`
	StudentID     ids.ID `json:"student_id"`
	CourseID      ids.ID `json:"course_id"`
	Reason        string `json:"reason"`
	Status        int16  `json:"status"`
	HandlerID     ids.ID `json:"handler_id,omitempty"`
	ResultComment string `json:"result_comment,omitempty"`
	CreatedAt     string `json:"created_at"`
	HandledAt     string `json:"handled_at,omitempty"`
}

// WarningDTO 表示学业预警响应。
type WarningDTO struct {
	ID         ids.ID         `json:"id"`
	TenantID   ids.ID         `json:"tenant_id"`
	StudentID  ids.ID         `json:"student_id"`
	SemesterID ids.ID         `json:"semester_id"`
	Type       int16          `json:"type"`
	Detail     map[string]any `json:"detail"`
	Status     int16          `json:"status"`
	CreatedAt  string         `json:"created_at"`
}

// TranscriptDTO 表示成绩单元数据响应,PDFRef 仅供服务端下载授权使用。
type TranscriptDTO struct {
	ID          ids.ID `json:"id"`
	TenantID    ids.ID `json:"tenant_id"`
	StudentID   ids.ID `json:"student_id"`
	Scope       int16  `json:"scope"`
	SemesterID  ids.ID `json:"semester_id,omitempty"`
	PDFRef      string `json:"-"`
	GeneratedAt string `json:"generated_at"`
}

// TranscriptDownloadGrantDTO 表示成绩单短时下载授权响应。
type TranscriptDownloadGrantDTO struct {
	Token      string        `json:"token"`
	Transcript TranscriptDTO `json:"transcript"`
	ExpiresAt  string        `json:"expires_at"`
}
