// experiment model 文件定义 M7 领域模型和组件编排快照。
package experiment

import (
	"time"

	"chaimir/internal/platform/ids"
)

// Experiment 是实验定义领域模型,组件正本以锁版本引用外部引擎能力。
type Experiment struct {
	ID              int64
	TenantID        int64
	CourseID        int64
	AuthorID        int64
	TemplateRef     string
	TemplateVersion string
	Name            string
	Description     string
	Components      ComponentConfig
	CollabMode      int16
	GroupConfig     GroupConfig
	RequireReport   bool
	WizardStep      int16
	Status          int16
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ComponentConfig 描述一个实验可自由组合的引擎组件列表。
type ComponentConfig struct {
	Envs        []EnvComponent        `json:"envs"`
	Sims        []SimComponent        `json:"sims"`
	Checkpoints []CheckpointComponent `json:"checkpoints"`
	Stages      []StageConfig         `json:"stages"`
}

// EnvComponent 描述 M2 运行时和工具入口配置。
type EnvComponent struct {
	ID                       string   `json:"id"`
	RuntimeCode              string   `json:"runtime_code"`
	RuntimeImageVersion      string   `json:"runtime_image_version"`
	Tools                    []string `json:"tools"`
	InitCodeRef              string   `json:"init_code_ref"`
	InitScriptRef            string   `json:"init_script_ref"`
	KeepAlive                bool     `json:"keep_alive"`
	SnapshotEnabled          bool     `json:"snapshot_enabled"`
	KeepAliveMinutes         int32    `json:"keep_alive_minutes"`
	SnapshotRetentionMinutes int32    `json:"snapshot_retention_minutes"`
}

// SimComponent 描述 M4 仿真包锁版本和启动参数。
type SimComponent struct {
	ID          string         `json:"id"`
	PackageCode string         `json:"package_code"`
	Version     string         `json:"version"`
	Seed        int64          `json:"seed"`
	Params      map[string]any `json:"params"`
}

// CheckpointComponent 描述 M3 判题检查点及其分值。
type CheckpointComponent struct {
	ID          string         `json:"id"`
	JudgerCode  string         `json:"judger"`
	ItemCode    string         `json:"item_code"`
	ItemVersion string         `json:"item_version"`
	Score       float64        `json:"score"`
	Mode        string         `json:"mode"`
	EnvID       string         `json:"env_id"`
	SimID       string         `json:"sim_id"`
	ExtraInput  map[string]any `json:"extra_input"`
}

// StageConfig 描述实验阶段、解锁规则和后续仿真参数注入规则。
type StageConfig struct {
	Stage           int32            `json:"stage"`
	Title           string           `json:"title"`
	Description     string           `json:"description"`
	Components      StageComponents  `json:"components"`
	UnlockCondition *UnlockCondition `json:"unlock_condition"`
	ParamBindings   []ParamBinding   `json:"param_bindings"`
}

// StageComponents 保存某阶段要激活的 M2/M4 组件 ID。
type StageComponents struct {
	Envs []string `json:"envs"`
	Sims []string `json:"sims"`
}

// UnlockCondition 描述阶段是否依赖检查点或手动激活。
type UnlockCondition struct {
	Type         string  `json:"type"`
	CheckpointID string  `json:"checkpoint_id"`
	MinScore     float64 `json:"min_score"`
}

// ParamBinding 描述从检查点输出或常量注入到后续仿真 init_params 的规则。
type ParamBinding struct {
	TargetComponent string `json:"target_component"`
	TargetParam     string `json:"target_param"`
	SourceType      string `json:"source_type"`
	SourceRef       string `json:"source_ref"`
	SourcePath      string `json:"source_path"`
	ConstantValue   any    `json:"constant_value"`
}

// GroupConfig 描述小组大小和角色定义。
type GroupConfig struct {
	Size  int      `json:"size"`
	Roles []string `json:"roles"`
}

// ExperimentInstance 是一次做实验的业务实例。
type ExperimentInstance struct {
	ID             int64
	TenantID       int64
	ExperimentID   int64
	OwnerAccountID int64
	GroupID        int64
	SourceRef      string
	SandboxRefs    []SandboxRef
	SimSessionRefs []SimSessionRef
	Status         int16
	Score          float64
	StartedAt      time.Time
	FinishedAt     time.Time
	LastActiveAt   time.Time
}

// SandboxRef 保存 M7 对 M2 沙箱资源的稳定引用和工具接入摘要。
type SandboxRef struct {
	ComponentID string           `json:"component_id"`
	Stage       int32            `json:"stage"`
	SandboxID   ids.ID           `json:"sandbox_id"`
	RuntimeCode string           `json:"runtime_code"`
	Tools       []SandboxToolDTO `json:"tools"`
}

// SimSessionRef 保存 M7 对 M4 仿真会话的稳定引用。
type SimSessionRef struct {
	ComponentID string `json:"component_id"`
	Stage       int32  `json:"stage"`
	SessionID   ids.ID `json:"session_id"`
	PackageCode string `json:"package_code"`
	Version     string `json:"version"`
	BundleRef   string `json:"bundle_ref"`
}

// ExperimentGroup 是多人协作小组。
type ExperimentGroup struct {
	ID           int64
	TenantID     int64
	ExperimentID int64
	Name         string
	CreatedAt    time.Time
	Members      []GroupMember
}

// GroupMember 是小组成员和角色绑定。
type GroupMember struct {
	ID        int64
	TenantID  int64
	GroupID   int64
	StudentID int64
	Role      string
	CreatedAt time.Time
}

// CheckpointResult 是一次实例检查点判分结果。
type CheckpointResult struct {
	ID            int64
	TenantID      int64
	InstanceID    int64
	CheckpointID  string
	JudgeTaskRef  string
	Passed        bool
	Score         float64
	DetailRef     string
	BindingOutput map[string]any
	JudgedAt      time.Time
}

// ExperimentReport 是学生提交的实验报告及教师批改结果。
type ExperimentReport struct {
	ID          int64
	TenantID    int64
	InstanceID  int64
	StudentID   int64
	ContentRef  string
	ManualScore float64
	Comment     string
	Status      int16
	SubmittedAt time.Time
}

// ExperimentStatsSnapshot 是 repo 返回给只读契约的统计投影。
type ExperimentStatsSnapshot struct {
	ExperimentCount     int64
	ActiveInstanceCount int64
}

// ExperimentScoreOutbox 是实验得分事件的生产者 outbox 记录。
type ExperimentScoreOutbox struct {
	ID           int64
	TenantID     int64
	ExperimentID int64
	InstanceID   int64
	StudentID    int64
	Score        float64
	TraceID      string
	ScoredAt     time.Time
	Status       int16
	RetryCount   int32
	LastError    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
