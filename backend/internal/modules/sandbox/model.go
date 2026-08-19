// sandbox model 文件定义 M2 沙箱引擎内部领域模型和运行时快照。
package sandbox

import (
	"encoding/json"
	"time"

	"chaimir/internal/platform/workload"
)

// Runtime 描述可调度链运行时及其声明式适配器清单。
type Runtime struct {
	ID             int64
	Code           string
	Name           string
	Eco            string
	AdapterLevel   int16
	AdapterSpec    AdapterSpec
	CapabilityImpl string
	PluginRef      string
	SelftestStatus int16
	SelftestDetail json.RawMessage
	Status         int16
}

// RuntimeImage 描述运行时镜像版本和真实预拉取状态。
type RuntimeImage struct {
	ID            int64
	RuntimeID     int64
	ImageURL      string
	Version       string
	Status        int16
	Prepulled     bool
	PrepullStatus int16
	PrepullDetail json.RawMessage
	PrepulledAt   time.Time
	GenesisBaked  bool
	IsDefault     bool
}

// Tool 描述可挂载到沙箱的工具定义。
type Tool struct {
	ID           int64
	Code         string
	Name         string
	Kind         int16
	EcoTags      []string
	ResourceSpec ToolResourceSpec
	Status       int16
}

// CatalogRuntime 是编排目录里的运行时投影,只保留业务模块选运行时所需的字段。
// 它不是 RuntimeResponse 的子集别名:适配器清单、镜像地址、自检详情属平台运维资产,
// 这里刻意不携带,避免编排面拿到只有平台面该看的内容。
type CatalogRuntime struct {
	Code   string
	Name   string
	Eco    string
	Images []CatalogRuntimeImage
}

// CatalogRuntimeImage 是编排目录里的镜像版本投影,只有版本号与是否默认。
type CatalogRuntimeImage struct {
	Version   string
	IsDefault bool
}

// CatalogTool 是编排目录里的工具投影,只有编码、名称与类型。
type CatalogTool struct {
	Code string
	Name string
	Kind int16
}

// Sandbox 描述单个沙箱实例的内部运行态快照。
type Sandbox struct {
	ID                int64
	TenantID          int64
	RuntimeID         int64
	ImageID           int64
	Namespace         string
	SourceRef         string
	OwnerAccountID    int64
	Phase             int16
	Status            int16
	KeepAlive         bool
	SnapshotEnabled   bool
	CodeStorageKey    string
	CodeHash          string
	InitCodeRef       string
	InitScriptRef     string
	SnapshotRef       string
	SnapshotDomains   []string
	SnapshotCreatedAt time.Time
	SnapshotExpireAt  time.Time
	KeepAliveUntil    time.Time
	LastActiveAt      time.Time
	ExpireAt          time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// SandboxTool 描述沙箱内已挂载工具的访问端点。
type SandboxTool struct {
	ID             int64
	TenantID       int64
	SandboxID      int64
	ToolID         int64
	ToolCode       string
	Kind           int16
	ResourceSpec   ToolResourceSpec
	AccessEndpoint string
	Status         int16
}

// TenantQuota 描述租户级沙箱资源配额。
type TenantQuota struct {
	TenantID                int64
	MaxConcurrentSandbox    int32
	MaxCPU                  int32
	MaxMemoryMB             int32
	IdleTimeoutMin          int32
	MaxLifetimeMin          int32
	MaxKeepaliveMin         int32
	MaxSnapshotRetentionMin int32
}

// SandboxRecycleOutbox 是沙箱回收事件的生产者 outbox 记录。
type SandboxRecycleOutbox struct {
	ID             int64
	TenantID       int64
	SandboxID      int64
	SourceRef      string
	OwnerAccountID int64
	Reason         string
	TraceID        string
	RecycledAt     time.Time
	Status         int16
	RetryCount     int32
	LastError      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	LeaseToken     string
	LeaseUntil     time.Time
}

// CreateSandboxInputModel 是 service 传入规则层的本模块创建校验模型。
type CreateSandboxInputModel struct {
	TenantID                 int64
	RuntimeCode              string
	RuntimeImageVersion      string
	ToolCodes                []string
	InitCodeRef              string
	InitScriptRef            string
	OwnerAccountID           int64
	SourceRef                string
	KeepAlive                bool
	SnapshotEnabled          bool
	KeepAliveMinutes         int32
	SnapshotRetentionMinutes int32
	PrivateSidecars          []workload.ComponentSpec
}

// AdapterSpec 是 runtime.adapter_spec 的控制面可执行结构。
type AdapterSpec struct {
	WorkspaceDir       string                     `json:"workspace_dir"`
	VolumeDomains      []VolumeDomainSpec         `json:"volume_domains"`
	RuntimeContainer   workload.ComponentSpec     `json:"runtime_container"`
	InfraSidecars      []workload.ComponentSpec   `json:"infra_sidecars"`
	Pods               []workload.PodSpec         `json:"pods"`
	Services           []workload.ServiceSpec     `json:"services"`
	Routes             []workload.RouteSpec       `json:"routes"`
	NetworkRules       []workload.NetworkRuleSpec `json:"network_rules"`
	InitAssets         []InitAssetSpec            `json:"init_assets"`
	DefaultToolCodes   []string                   `json:"default_tool_codes"`
	Selftest           map[string]any             `json:"selftest"`
	WorkspaceOps       WorkspaceOps               `json:"workspace_ops"`
	CapabilityCommands CapabilityCommandSet       `json:"capability_commands"`
}

// VolumeDomainSpec 描述沙箱卷安全域,用于区分学生工作区、运行态和私有判题数据。
type VolumeDomainSpec struct {
	Name          string `json:"name"`
	MountPath     string `json:"mount_path"`
	StudentAccess string `json:"student_access"`
	Persistence   string `json:"persistence"`
	SnapshotScope string `json:"snapshot_scope"`
}

// InitAssetSpec 描述个性化阶段注入的已审核资产。
type InitAssetSpec struct {
	Name       string `json:"name"`
	SourceRef  string `json:"source_ref"`
	ApplyPhase string `json:"apply_phase"`
}

// WorkspaceOps 描述运行时镜像内已审核的工作区操作命令模板。
type WorkspaceOps struct {
	ReadFile  []string `json:"read_file"`
	WriteFile []string `json:"write_file"`
	ListFiles []string `json:"list_files"`
	PackTar   []string `json:"pack_tar"`
	UnpackTar []string `json:"unpack_tar"`
	RunScript []string `json:"run_script"`
	Terminal  []string `json:"terminal"`
	Selftest  []string `json:"selftest"`
}

// CapabilityCommandSet 是 L2 标准链能力的受控命令清单,由运行时镜像内 helper 执行。
type CapabilityCommandSet struct {
	Deploy CapabilityCommandSpec `json:"deploy"`
	Tx     CapabilityCommandSpec `json:"tx"`
	Query  CapabilityCommandSpec `json:"query"`
	Reset  CapabilityCommandSpec `json:"reset"`
}

// CapabilityCommandSpec 描述单个链能力动作的命令和超时,输入输出均为 JSON。
type CapabilityCommandSpec struct {
	Command        []string `json:"command"`
	TimeoutSeconds int32    `json:"timeout_seconds"`
}

// ToolResourceSpec 是 tool.resource_spec 的控制面可执行结构。
type ToolResourceSpec struct {
	BuiltinEndpoint string                     `json:"builtin_endpoint"`
	Components      []workload.ComponentSpec   `json:"components"`
	Services        []workload.ServiceSpec     `json:"services"`
	Routes          []workload.RouteSpec       `json:"routes"`
	NetworkRules    []workload.NetworkRuleSpec `json:"network_rules"`
	CommandPolicy   CommandToolPolicy          `json:"command_policy"`
	PrepullCommand  []string                   `json:"prepull_command"`
}

// PrepullImageSpec 描述预拉取闭环中单个镜像的真实拉取与最小自检命令。
type PrepullImageSpec struct {
	ImageURL        string
	Command         []string
	Hold            bool
	EphemeralMounts []workload.EphemeralMountSpec
}

// CommandToolPolicy 描述命令工具允许执行的完整 argv 白名单和超时边界。
type CommandToolPolicy struct {
	AllowedArgv           [][]string `json:"allowed_argv"`
	DefaultTimeoutSeconds int32      `json:"default_timeout_seconds"`
	MaxTimeoutSeconds     int32      `json:"max_timeout_seconds"`
}

// CreateSandboxPlan 汇总创建沙箱时 service 交给编排器的完整上下文。
type CreateSandboxPlan struct {
	Sandbox         Sandbox
	Runtime         Runtime
	Image           RuntimeImage
	Tools           []Tool
	PrivateSidecars []workload.ComponentSpec
}

// SnapshotResult 描述一次 CSI 快照成功创建后的可恢复引用和覆盖卷域。
type SnapshotResult struct {
	Ref     string
	Domains []string
}
