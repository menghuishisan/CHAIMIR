// experiment dto 文件定义 M7 HTTP 请求和响应结构,不承载业务逻辑。
package experiment

import (
	"time"

	"chaimir/internal/platform/ids"
)

// ExperimentRequest 是创建或更新实验定义的请求。
type ExperimentRequest struct {
	CourseID        ids.ID          `json:"course_id"`
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
	ID              ids.ID          `json:"id"`
	CourseID        ids.ID          `json:"course_id,omitempty"`
	AuthorID        ids.ID          `json:"author_id"`
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

// StudentExperimentDTO 是学生可发现实验视图，不包含初始化脚本、答案和判题配置。
type StudentExperimentDTO struct {
	ID            ids.ID                 `json:"id"`
	CourseID      ids.ID                 `json:"course_id,omitempty"`
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	Components    StudentComponentConfig `json:"components"`
	CollabMode    int16                  `json:"collab_mode"`
	GroupConfig   GroupConfig            `json:"group_config"`
	RequireReport bool                   `json:"require_report"`
	Status        int16                  `json:"status"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

// StudentComponentConfig 只暴露学生理解实验流程所需的组件摘要。
type StudentComponentConfig struct {
	Envs        []StudentEnvComponent        `json:"envs"`
	Sims        []StudentSimComponent        `json:"sims"`
	Checkpoints []StudentCheckpointComponent `json:"checkpoints"`
	Stages      []StudentStageConfig         `json:"stages"`
}

// StudentEnvComponent 描述学生可见的运行环境和工具。
type StudentEnvComponent struct {
	ID          string   `json:"id"`
	RuntimeCode string   `json:"runtime_code"`
	Tools       []string `json:"tools"`
}

// StudentSimComponent 描述学生可见的仿真包锁定版本。
type StudentSimComponent struct {
	ID          string `json:"id"`
	PackageCode string `json:"package_code"`
	Version     string `json:"version"`
}

// StudentCheckpointComponent 描述检查点标识和分值，不泄露判题实现。
type StudentCheckpointComponent struct {
	ID    string  `json:"id"`
	Score float64 `json:"score"`
	Mode  string  `json:"mode,omitempty"`
}

// StudentStageConfig 描述学生可见的阶段流程，不暴露参数注入规则。
type StudentStageConfig struct {
	Stage           int32            `json:"stage"`
	Title           string           `json:"title"`
	Description     string           `json:"description"`
	Components      StageComponents  `json:"components"`
	UnlockCondition *UnlockCondition `json:"unlock_condition,omitempty"`
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
	GroupID ids.ID `json:"group_id"`
}

// InstanceDTO 是实验实例输出。
type InstanceDTO struct {
	ID             ids.ID          `json:"instance_id"`
	ExperimentID   ids.ID          `json:"experiment_id"`
	OwnerAccountID ids.ID          `json:"owner_account_id"`
	GroupID        ids.ID          `json:"group_id,omitempty"`
	SourceRef      string          `json:"source_ref"`
	Sandboxes      []SandboxRef    `json:"sandboxes"`
	Sims           []SimSessionRef `json:"sims"`
	Status         int16           `json:"status"`
	Score          float64         `json:"score"`
	StartedAt      time.Time       `json:"started_at"`
	FinishedAt     time.Time       `json:"finished_at,omitempty"`
	LastActiveAt   time.Time       `json:"last_active_at"`
	Checkpoints    []CheckpointDTO `json:"checkpoints,omitempty"`
	Stages         []StageDTO      `json:"stages,omitempty"`
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
	ID            string         `json:"id"`
	JudgeTaskRef  string         `json:"judge_task_ref,omitempty"`
	Passed        bool           `json:"passed"`
	Score         float64        `json:"score"`
	DetailRef     string         `json:"detail_ref,omitempty"`
	BindingOutput map[string]any `json:"binding_output,omitempty"`
}

// StageDTO 是工作台展示阶段解锁、激活和完成状态的输出。
type StageDTO struct {
	Stage           int32            `json:"stage"`
	Title           string           `json:"title"`
	Description     string           `json:"description,omitempty"`
	Status          string           `json:"status"`
	Components      StageComponents  `json:"components"`
	UnlockCondition *UnlockCondition `json:"unlock_condition,omitempty"`
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
	BindingOutput  map[string]any `json:"binding_output"`
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
	ID          ids.ID    `json:"id"`
	InstanceID  ids.ID    `json:"instance_id"`
	StudentID   ids.ID    `json:"student_id"`
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
	StudentID ids.ID `json:"student_id"`
	Role      string `json:"role"`
}

// GroupDTO 是协作小组输出。
type GroupDTO struct {
	ID             ids.ID           `json:"id"`
	ExperimentID   ids.ID           `json:"experiment_id"`
	Name           string           `json:"name"`
	Members        []GroupMemberDTO `json:"members"`
	SharedInstance *InstanceDTO     `json:"shared_instance,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
}

// GroupMemberDTO 是协作小组成员输出。
type GroupMemberDTO struct {
	ID        ids.ID    `json:"id"`
	GroupID   ids.ID    `json:"group_id"`
	StudentID ids.ID    `json:"student_id"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}
