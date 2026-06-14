// sandbox model 文件定义 M2 沙箱引擎内部领域模型和运行时快照。
package sandbox

import (
	"encoding/json"
	"time"
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
	ImageURL     string
	Port         int32
	EcoTags      []string
	ResourceSpec ToolResourceSpec
	Status       int16
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
}

// AdapterSpec 是 runtime.adapter_spec 的控制面可执行结构。
type AdapterSpec struct {
	WorkspaceDir       string               `json:"workspace_dir"`
	VolumeDomains      []VolumeDomainSpec   `json:"volume_domains"`
	RuntimeContainer   ContainerSpec        `json:"runtime_container"`
	InfraSidecars      []ContainerSpec      `json:"infra_sidecars"`
	Pods               []PodSpec            `json:"pods"`
	NetworkRules       []NetworkRuleSpec    `json:"network_rules"`
	InitAssets         []InitAssetSpec      `json:"init_assets"`
	DefaultToolCodes   []string             `json:"default_tool_codes"`
	Selftest           map[string]any       `json:"selftest"`
	WorkspaceOps       WorkspaceOps         `json:"workspace_ops"`
	CapabilityCommands CapabilityCommandSet `json:"capability_commands"`
}

// VolumeDomainSpec 描述沙箱卷安全域,用于区分学生工作区、运行态和私有判题数据。
type VolumeDomainSpec struct {
	Name          string `json:"name"`
	MountPath     string `json:"mount_path"`
	StudentAccess string `json:"student_access"`
	Persistence   string `json:"persistence"`
	SnapshotScope string `json:"snapshot_scope"`
}

// ContainerSpec 描述运行时或协同容器的启动、安全和探活配置。
type ContainerSpec struct {
	Name           string            `json:"name"`
	Image          string            `json:"image_url"`
	Command        []string          `json:"command"`
	Args           []string          `json:"args"`
	Env            []EnvVarSpec      `json:"env"`
	Ports          []PortSpec        `json:"ports"`
	Resources      ResourceSpec      `json:"resources"`
	ReadinessProbe ProbeSpec         `json:"readiness_probe"`
	LivenessProbe  ProbeSpec         `json:"liveness_probe"`
	Workdir        string            `json:"workdir"`
	Labels         map[string]string `json:"labels"`
}

// PodSpec 描述一个沙箱工作负载 Pod 及其容器组。
type PodSpec struct {
	Name       string            `json:"name"`
	Labels     map[string]string `json:"labels"`
	Containers []ContainerSpec   `json:"containers"`
}

// NetworkRuleSpec 描述同一沙箱内不同 Pod 之间显式允许的网络访问。
type NetworkRuleSpec struct {
	Name    string           `json:"name"`
	FromPod string           `json:"from_pod"`
	ToPod   string           `json:"to_pod"`
	Ports   []NetworkPortRef `json:"ports"`
}

// NetworkPortRef 描述网络策略放行的目标端口,优先使用容器端口名称。
type NetworkPortRef struct {
	Name string `json:"name"`
	Port int32  `json:"port"`
}

// EnvVarSpec 描述允许注入容器的非敏感字面量环境变量。
type EnvVarSpec struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// PortSpec 描述容器端口和平台代理暴露口径。
type PortSpec struct {
	Name          string `json:"name"`
	ContainerPort int32  `json:"container_port"`
	ServicePort   int32  `json:"service_port"`
	Protocol      string `json:"protocol"`
}

// ResourceSpec 描述容器 requests/limits。
type ResourceSpec struct {
	Requests map[string]string `json:"requests"`
	Limits   map[string]string `json:"limits"`
}

// ProbeSpec 描述运行时声明的探活探针。
type ProbeSpec struct {
	Type             string   `json:"type"`
	Path             string   `json:"path"`
	Port             string   `json:"port"`
	Command          []string `json:"command"`
	PeriodSeconds    int32    `json:"period_seconds"`
	FailureThreshold int32    `json:"failure_threshold"`
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
	MountWorkspace  *bool                    `json:"mount_workspace"`
	BuiltinEndpoint string                   `json:"builtin_endpoint"`
	EphemeralMounts []ToolEphemeralMountSpec `json:"ephemeral_mounts"`
	NetworkRules    []ToolNetworkRuleSpec    `json:"network_rules"`
	Workdir         string                   `json:"workdir"`
	Command         []string                 `json:"command"`
	Args            []string                 `json:"args"`
	Env             []EnvVarSpec             `json:"env"`
	Resources       ResourceSpec             `json:"resources"`
	ReadinessProbe  ProbeSpec                `json:"readiness_probe"`
}

// ToolEphemeralMountSpec 描述工具容器在只读根文件系统下需要的临时可写目录。
type ToolEphemeralMountSpec struct {
	Name      string `json:"name"`
	MountPath string `json:"mount_path"`
}

// ToolNetworkRuleSpec 描述工具 Pod 访问运行时 Pod 的最小网络放行。
type ToolNetworkRuleSpec struct {
	Name  string           `json:"name"`
	ToPod string           `json:"to_pod"`
	Ports []NetworkPortRef `json:"ports"`
}

// CreateSandboxPlan 汇总创建沙箱时 service 交给编排器的完整上下文。
type CreateSandboxPlan struct {
	Sandbox Sandbox
	Runtime Runtime
	Image   RuntimeImage
	Tools   []Tool
}

// SnapshotResult 描述一次 CSI 快照成功创建后的可恢复引用和覆盖卷域。
type SnapshotResult struct {
	Ref     string
	Domains []string
}
