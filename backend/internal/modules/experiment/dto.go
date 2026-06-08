// M7 DTO 定义:隔离 HTTP 请求/响应、服务内部编排结构与跨模块引用。
package experiment

import "time"

// ExperimentComponents 是实验组件编排 JSONB 的强类型视图。
type ExperimentComponents struct {
	Envs        []EnvComponent        `json:"envs"`
	Sims        []SimComponent        `json:"sims"`
	Checkpoints []CheckpointComponent `json:"checkpoints"`
}

// EnvComponent 描述一个 M2 沙箱环境组件。
type EnvComponent struct {
	RuntimeCode              string   `json:"runtime_code"`
	ToolCodes                []string `json:"tools"`
	InitCodeRef              string   `json:"init_code_ref"`
	InitScriptRef            string   `json:"init_script_ref"`
	KeepAlive                bool     `json:"keep_alive"`
	KeepAliveMinutes         int32    `json:"keep_alive_minutes"`
	SnapshotEnabled          bool     `json:"snapshot_enabled"`
	SnapshotRetentionMinutes int32    `json:"snapshot_retention_minutes"`
}

// SimComponent 描述一个 M4 仿真会话组件。
type SimComponent struct {
	PackageCode string         `json:"package_code"`
	Version     string         `json:"version"`
	Seed        int64          `json:"seed"`
	Params      map[string]any `json:"params"`
}

// CheckpointComponent 描述一个 M3 检查点判题组件。
type CheckpointComponent struct {
	ID          string         `json:"id"`
	JudgerCode  string         `json:"judger"`
	ItemCode    string         `json:"item_code"`
	ItemVersion string         `json:"item_version"`
	Score       float64        `json:"score"`
	ExtraInput  map[string]any `json:"extra_input"`
}

// ExperimentRequest 是创建和更新实验定义的请求。
type ExperimentRequest struct {
	CourseID        string               `json:"course_id"`
	TemplateRef     string               `json:"template_ref"`
	TemplateVersion string               `json:"template_version"`
	Name            string               `json:"name"`
	Description     string               `json:"description"`
	Components      ExperimentComponents `json:"components"`
	CollabMode      int16                `json:"collab_mode"`
	GroupConfig     map[string]any       `json:"group_config"`
	RequireReport   bool                 `json:"require_report"`
	WizardStep      int16                `json:"wizard_step"`
}

// ExperimentDTO 是实验定义响应。
type ExperimentDTO struct {
	ID              string               `json:"id"`
	TenantID        string               `json:"tenant_id,omitempty"`
	CourseID        string               `json:"course_id,omitempty"`
	AuthorID        string               `json:"author_id,omitempty"`
	TemplateRef     string               `json:"template_ref,omitempty"`
	TemplateVersion string               `json:"template_version,omitempty"`
	Name            string               `json:"name"`
	Description     string               `json:"description"`
	Components      ExperimentComponents `json:"components"`
	CollabMode      int16                `json:"collab_mode"`
	GroupConfig     map[string]any       `json:"group_config"`
	RequireReport   bool                 `json:"require_report"`
	WizardStep      int16                `json:"wizard_step"`
	Status          int16                `json:"status"`
	CreatedAt       time.Time            `json:"created_at,omitempty"`
	UpdatedAt       time.Time            `json:"updated_at,omitempty"`
}

// ValidationIssue 是发布前校验的单条问题。
type ValidationIssue struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}

// ValidationResult 是发布前校验响应。
type ValidationResult struct {
	OK     bool              `json:"ok"`
	Issues []ValidationIssue `json:"issues"`
}

// StartInstanceRequest 是学生发起实例请求。
type StartInstanceRequest struct {
	GroupID string `json:"group_id"`
}

// SandboxRef 是 M7 持久化的 M2 沙箱引用摘要。
type SandboxRef struct {
	ID      int64                  `json:"id"`
	Ref     string                 `json:"ref"`
	Tools   []SandboxToolAccessDTO `json:"tools"`
	Runtime string                 `json:"runtime_code,omitempty"`
	Meta    map[string]any         `json:"meta,omitempty"`
}

// SandboxToolAccessDTO 是工作台可展示的工具入口。
type SandboxToolAccessDTO struct {
	Code     string `json:"code"`
	Kind     int16  `json:"kind"`
	Endpoint string `json:"endpoint"`
	Status   int16  `json:"status"`
}

// SimSessionRef 是 M7 持久化的 M4 仿真会话引用摘要。
type SimSessionRef struct {
	ID          int64  `json:"id"`
	Ref         string `json:"ref"`
	PackageCode string `json:"package_code"`
	Version     string `json:"version"`
	BundleRef   string `json:"bundle_url"`
}

// ExperimentInstanceDTO 是实验实例响应。
type ExperimentInstanceDTO struct {
	ID             string          `json:"instance_id"`
	TenantID       string          `json:"tenant_id,omitempty"`
	ExperimentID   string          `json:"experiment_id"`
	OwnerAccountID string          `json:"owner_account_id,omitempty"`
	GroupID        string          `json:"group_id,omitempty"`
	SourceRef      string          `json:"source_ref,omitempty"`
	Sandboxes      []SandboxRef    `json:"sandboxes"`
	Sims           []SimSessionRef `json:"sims"`
	Status         int16           `json:"status"`
	Score          *float64        `json:"score,omitempty"`
	StartedAt      time.Time       `json:"started_at,omitempty"`
	FinishedAt     time.Time       `json:"finished_at,omitempty"`
	LastActiveAt   time.Time       `json:"last_active_at,omitempty"`
}

// CheckpointResultDTO 是某实例检查点判题结果。
type CheckpointResultDTO struct {
	ID           string  `json:"id,omitempty"`
	TenantID     int64   `json:"tenant_id,omitempty"`
	InstanceID   int64   `json:"instance_id,omitempty"`
	CheckpointID string  `json:"checkpoint_id"`
	JudgeTaskRef string  `json:"judge_task_ref,omitempty"`
	Passed       bool    `json:"passed"`
	Score        float64 `json:"score"`
	DetailRef    string  `json:"detail_ref,omitempty"`
}

// ReportRequest 是学生提交实验报告的请求。
type ReportRequest struct {
	ContentRef string `json:"content_ref"`
}

// ReportGradeRequest 是教师批改报告的请求。
type ReportGradeRequest struct {
	Score   float64 `json:"score"`
	Comment string  `json:"comment"`
}

// ReportDTO 是实验报告响应。
type ReportDTO struct {
	ID          string     `json:"id"`
	InstanceID  string     `json:"instance_id"`
	StudentID   string     `json:"student_id"`
	ContentRef  string     `json:"content_ref"`
	ManualScore *float64   `json:"manual_score,omitempty"`
	Comment     string     `json:"comment,omitempty"`
	Status      int16      `json:"status"`
	SubmittedAt *time.Time `json:"submitted_at,omitempty"`
}

// GroupRequest 是创建实验小组的请求。
type GroupRequest struct {
	Name string `json:"name"`
}

// GroupMemberRequest 是添加或更新小组成员角色的请求。
type GroupMemberRequest struct {
	StudentID string `json:"student_id"`
	Role      string `json:"role"`
}

// GroupDTO 是协作小组响应。
type GroupDTO struct {
	ID           string           `json:"id"`
	ExperimentID string           `json:"experiment_id"`
	Name         string           `json:"name"`
	Members      []GroupMemberDTO `json:"members"`
}

// GroupMemberDTO 是小组成员响应。
type GroupMemberDTO struct {
	ID        string `json:"id"`
	GroupID   string `json:"group_id"`
	StudentID string `json:"student_id"`
	Role      string `json:"role"`
}

// StatsDTO 是 M7 给内部看板使用的统计响应。
type StatsDTO struct {
	TenantID            string `json:"tenant_id"`
	CourseID            string `json:"course_id,omitempty"`
	ExperimentCount     int64  `json:"experiment_count"`
	ActiveInstanceCount int64  `json:"active_instance_count"`
}

// ScorePart 是参与实验总分汇总的一项分值。
type ScorePart struct {
	Score float64
}

// PendingCheckpoint 是已提交判题但等待事件回写的检查点定位信息。
type PendingCheckpoint struct {
	TenantID     int64
	InstanceID   int64
	CheckpointID string
	SourceRef    string
}
