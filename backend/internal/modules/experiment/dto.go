// experiment dto 文件定义 M7 HTTP 请求和响应结构,不承载业务逻辑。
package experiment

import "time"

// ExperimentRequest 是创建或更新实验定义的请求。
type ExperimentRequest struct {
	CourseID        int64           `json:"course_id"`
	TemplateRef     string          `json:"template_ref"`
	TemplateVersion string          `json:"template_version"`
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	Components      ComponentConfig `json:"components"`
	CollabMode      int16           `json:"collab_mode"`
	GroupConfig     GroupConfig     `json:"group_config"`
	RequireReport   bool            `json:"require_report"`
	WizardStep      int16           `json:"wizard_step"`
}

// ExperimentDTO 是实验定义的用户向输出。
type ExperimentDTO struct {
	ID              int64           `json:"id,string"`
	CourseID        int64           `json:"course_id,string,omitempty"`
	AuthorID        int64           `json:"author_id,string"`
	TemplateRef     string          `json:"template_ref,omitempty"`
	TemplateVersion string          `json:"template_version,omitempty"`
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	Components      ComponentConfig `json:"components"`
	CollabMode      int16           `json:"collab_mode"`
	GroupConfig     GroupConfig     `json:"group_config"`
	RequireReport   bool            `json:"require_report"`
	WizardStep      int16           `json:"wizard_step"`
	Status          int16           `json:"status"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// ValidationIssueDTO 是发布前校验问题。
type ValidationIssueDTO struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}

// ValidationResultDTO 是发布前校验结果。
type ValidationResultDTO struct {
	OK     bool                 `json:"ok"`
	Issues []ValidationIssueDTO `json:"issues"`
}

// CreateInstanceRequest 是发起实验实例的请求。
type CreateInstanceRequest struct {
	GroupID int64 `json:"group_id"`
}

// InstanceDTO 是实验实例输出。
type InstanceDTO struct {
	ID             int64           `json:"instance_id,string"`
	ExperimentID   int64           `json:"experiment_id,string"`
	OwnerAccountID int64           `json:"owner_account_id,string"`
	GroupID        int64           `json:"group_id,string,omitempty"`
	SourceRef      string          `json:"source_ref"`
	Sandboxes      []SandboxRef     `json:"sandboxes"`
	Sims           []SimSessionRef  `json:"sims"`
	Status         int16           `json:"status"`
	Score          float64         `json:"score"`
	StartedAt      time.Time       `json:"started_at"`
	FinishedAt     time.Time       `json:"finished_at,omitempty"`
	LastActiveAt   time.Time       `json:"last_active_at"`
	Checkpoints    []CheckpointDTO `json:"checkpoints,omitempty"`
}

// SandboxToolDTO 是工作台展示的沙箱工具入口。
type SandboxToolDTO struct {
	Code     string `json:"code"`
	Kind     int16  `json:"kind"`
	Endpoint string `json:"endpoint"`
	Status   int16  `json:"status"`
}

// CheckpointDTO 是工作台展示的检查点状态。
type CheckpointDTO struct {
	ID           string  `json:"id"`
	JudgeTaskRef string  `json:"judge_task_ref,omitempty"`
	Passed       bool    `json:"passed"`
	Score        float64 `json:"score"`
	DetailRef    string  `json:"detail_ref,omitempty"`
}

// ProgressDTO 返回给前端的 M10 topic 订阅元信息。
type ProgressDTO struct {
	Topic   string `json:"topic"`
	Channel string `json:"channel"`
}

// JudgeCheckpointRequest 是触发检查点判分时可选传入的代码快照。
type JudgeCheckpointRequest struct {
	CodeStorageKey string         `json:"code_storage_key"`
	CodeHash       string         `json:"code_hash"`
	ExtraInput     map[string]any `json:"extra_input"`
}

// SubmitReportRequest 是提交实验报告的请求。
type SubmitReportRequest struct {
	ContentRef string `json:"content_ref"`
}

// GradeReportRequest 是教师批改实验报告的请求。
type GradeReportRequest struct {
	ManualScore float64 `json:"manual_score"`
	Comment     string  `json:"comment"`
}

// ReportDTO 是实验报告输出。
type ReportDTO struct {
	ID          int64     `json:"id,string"`
	InstanceID  int64     `json:"instance_id,string"`
	StudentID   int64     `json:"student_id,string"`
	ContentRef  string    `json:"content_ref"`
	ManualScore float64   `json:"manual_score"`
	Comment     string    `json:"comment,omitempty"`
	Status      int16     `json:"status"`
	SubmittedAt time.Time `json:"submitted_at"`
}

// CreateGroupRequest 是创建协作小组的请求。
type CreateGroupRequest struct {
	Name string `json:"name"`
}

// UpsertGroupMemberRequest 是加入或调整小组成员角色的请求。
type UpsertGroupMemberRequest struct {
	StudentID int64  `json:"student_id"`
	Role      string `json:"role"`
}

// GroupDTO 是协作小组输出。
type GroupDTO struct {
	ID           int64            `json:"id,string"`
	ExperimentID int64            `json:"experiment_id,string"`
	Name         string           `json:"name"`
	Members      []GroupMemberDTO `json:"members"`
	CreatedAt    time.Time        `json:"created_at"`
}

// GroupMemberDTO 是协作小组成员输出。
type GroupMemberDTO struct {
	ID        int64     `json:"id,string"`
	GroupID   int64     `json:"group_id,string"`
	StudentID int64     `json:"student_id,string"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}
