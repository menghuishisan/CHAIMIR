// M2 沙箱领域模型:放置模块内部纯值对象与编排参数,不包含 HTTP DTO 或 sqlc 行类型。
package sandbox

import "time"

// ToolDefinition 是创建沙箱时参与生态适配判断和端点生成的工具定义。
type ToolDefinition struct {
	ID       int64
	Code     string
	Name     string
	Kind     int16
	Port     int32
	EcoTags  string
	ImageURL string
	Spec     ToolResourceSpec
}

// RuntimeDefinition 是运行时编排所需的最小配置。
type RuntimeDefinition struct {
	ID             int64
	Code           string
	Eco            string
	CapabilityImpl string
	AdapterSpec    RuntimeAdapterSpec
}

// RuntimeImageDefinition 是编排时选中的运行时镜像。
type RuntimeImageDefinition struct {
	ID           int64
	ImageURL     string
	Version      string
	GenesisBaked bool
}

// RuntimeConfigSnapshot 是运行时管理、自检与沙箱编排使用的平台级运行时投影。
type RuntimeConfigSnapshot struct {
	ID             int64
	Code           string
	Name           string
	Eco            string
	AdapterLevel   int16
	AdapterSpec    []byte
	CapabilityImpl string
	PluginRef      string
	SelftestStatus int16
	SelftestDetail []byte
	Status         int16
}

// RuntimeImageSnapshot 是运行时镜像登记、预拉取和编排门禁使用的投影。
type RuntimeImageSnapshot struct {
	ID            int64
	RuntimeID     int64
	ImageURL      string
	Version       string
	Prepulled     bool
	PrepullStatus int16
	PrepullDetail []byte
	PrepulledAt   time.Time
	GenesisBaked  bool
	IsDefault     bool
}

// ToolConfigSnapshot 是工具定义的控制面投影。
type ToolConfigSnapshot struct {
	ID           int64
	Code         string
	Name         string
	Kind         int16
	ImageURL     string
	Port         int32
	EcoTags      string
	ResourceSpec []byte
	Status       int16
}

// SandboxToolAccessSnapshot 是沙箱已挂载工具的访问端点投影。
type SandboxToolAccessSnapshot struct {
	ID             int64
	TenantID       int64
	SandboxID      int64
	ToolID         int64
	AccessEndpoint string
	Status         int16
	ToolCode       string
	ToolName       string
	ToolKind       int16
}

// TenantQuotaSnapshot 是租户沙箱配额投影。
type TenantQuotaSnapshot struct {
	TenantID                int64
	MaxConcurrentSandbox    int32
	MaxCPU                  int32
	MaxMemoryMB             int32
	IdleTimeoutMin          int32
	MaxLifetimeMin          int32
	MaxKeepaliveMin         int32
	MaxSnapshotRetentionMin int32
}

// ActiveSandboxResourceSnapshot 是活跃沙箱资源统计查询的领域投影。
type ActiveSandboxResourceSnapshot struct {
	SandboxID          int64
	RuntimeAdapterSpec []byte
	Tool               *ToolConfigSnapshot
}

// SandboxCreateRecord 是创建控制面沙箱记录所需的持久化输入。
type SandboxCreateRecord struct {
	ID               int64
	TenantID         int64
	RuntimeID        int64
	ImageID          int64
	Namespace        string
	SourceRef        string
	OwnerAccountID   int64
	KeepAlive        bool
	SnapshotEnabled  bool
	CodeStorageKey   string
	InitScriptRef    string
	KeepAliveUntil   time.Time
	SnapshotExpireAt time.Time
	ExpireAt         time.Time
}

// SandboxToolCreateRecord 是创建沙箱工具挂载记录所需的持久化输入。
type SandboxToolCreateRecord struct {
	ID             int64
	TenantID       int64
	SandboxID      int64
	ToolID         int64
	AccessEndpoint string
	Status         int16
}

// SandboxCreateSpec 是 service 传给编排器的沙箱创建规格。
type SandboxCreateSpec struct {
	SandboxID                int64
	TenantID                 int64
	Namespace                string
	Runtime                  RuntimeDefinition
	Image                    RuntimeImageDefinition
	Tools                    []ToolDefinition
	InitCodeRef              string
	InitScriptRef            string
	OwnerAccountID           int64
	SourceRef                string
	KeepAlive                bool
	KeepAliveMinutes         int32
	SnapshotEnabled          bool
	SnapshotRetentionMinutes int32
	CodeStorageKey           string
}

// ImagePrepullSpec 是镜像预拉取 DaemonSet 的编排输入。
type ImagePrepullSpec struct {
	RuntimeImageID int64
	RuntimeID      int64
	ImageURL       string
}

// ImagePrepullStatus 描述预拉取 DaemonSet 在目标节点上的真实进度。
type ImagePrepullStatus struct {
	DaemonSet    string
	DesiredNodes int32
	ReadyNodes   int32
	FailedNodes  int32
	Failure      string
	Completed    bool
}

// SnapshotSpec 描述沙箱 PVC 快照请求。
type SnapshotSpec struct {
	SandboxID int64
	TenantID  int64
	Namespace string
	ExpiresAt time.Time
}

// SnapshotResult 是 CSI VolumeSnapshot 创建结果。
type SnapshotResult struct {
	Ref       string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// RuntimeAdapterSpec 是 runtime.adapter_spec 的执行期结构。
type RuntimeAdapterSpec struct {
	WorkspaceDir     string              `json:"workspace_dir"`
	RuntimeContainer ContainerSpec       `json:"runtime_container"`
	InfraSidecars    []ContainerSpec     `json:"infra_sidecars"`
	DefaultToolCodes []string            `json:"default_tool_codes"`
	Selftest         RuntimeSelftestSpec `json:"selftest"`
}

// RuntimeSelftestSpec 描述接入即测的样例输入。
type RuntimeSelftestSpec struct {
	DeployPayload map[string]any `json:"deploy_payload"`
	QueryTarget   string         `json:"query_target"`
	TxPayload     map[string]any `json:"tx_payload"`
}

// ToolResourceSpec 是 tool.resource_spec 的执行期结构。
type ToolResourceSpec struct {
	MountWorkspace *bool        `json:"mount_workspace"`
	Workdir        string       `json:"workdir"`
	Command        []string     `json:"command"`
	Args           []string     `json:"args"`
	Env            []EnvVarSpec `json:"env"`
	Resources      ResourceSpec `json:"resources"`
	ReadinessProbe ProbeSpec    `json:"readiness_probe"`
	LivenessProbe  ProbeSpec    `json:"liveness_probe"`
}

// ContainerSpec 描述 runtime/tool sidecar 的容器启动信息。
type ContainerSpec struct {
	Name           string       `json:"name"`
	ImageURL       string       `json:"image_url"`
	Command        []string     `json:"command"`
	Args           []string     `json:"args"`
	Env            []EnvVarSpec `json:"env"`
	Ports          []PortSpec   `json:"ports"`
	Resources      ResourceSpec `json:"resources"`
	ReadinessProbe ProbeSpec    `json:"readiness_probe"`
	LivenessProbe  ProbeSpec    `json:"liveness_probe"`
	Workdir        string       `json:"workdir"`
	MountWorkspace *bool        `json:"mount_workspace"`
}

// EnvVarSpec 是声明式环境变量。
type EnvVarSpec struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// PortSpec 描述容器端口及其 Service 暴露方式。
type PortSpec struct {
	Name          string `json:"name"`
	ContainerPort int32  `json:"container_port"`
	ServicePort   int32  `json:"service_port"`
	Protocol      string `json:"protocol"`
}

// ResourceSpec 描述容器 requests/limits。
type ResourceSpec struct {
	Requests ResourcePair `json:"requests"`
	Limits   ResourcePair `json:"limits"`
}

// ResourcePair 是一组 CPU/内存资源声明。
type ResourcePair struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
}

// ProbeSpec 是运行时与工具共用的探针定义。
type ProbeSpec struct {
	Type             string   `json:"type"`
	Path             string   `json:"path"`
	Port             string   `json:"port"`
	Command          []string `json:"command"`
	PeriodSeconds    int32    `json:"period_seconds"`
	FailureThreshold int32    `json:"failure_threshold"`
}

// SandboxProgressEvent 是对前端 progress WS 推送的载荷。
type SandboxProgressEvent struct {
	SandboxID int64  `json:"sandbox_id"`
	Phase     int16  `json:"phase"`
	Stage     string `json:"stage"`
	Message   string `json:"message"`
	Status    int16  `json:"status"`
}

// SandboxFileEntry 是文件列表响应项。
type SandboxFileEntry struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	IsDir     bool      `json:"is_dir"`
	Size      int64     `json:"size"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SandboxFilePayload 表示文件读写结果。
type SandboxFilePayload struct {
	Path     string             `json:"path"`
	IsDir    bool               `json:"is_dir"`
	Content  string             `json:"content,omitempty"`
	Encoding string             `json:"encoding,omitempty"`
	Entries  []SandboxFileEntry `json:"entries,omitempty"`
}

// SandboxToolEndpoint 描述某个工具在数据面的内部 Service 目标。
type SandboxToolEndpoint struct {
	ToolCode    string
	ServiceName string
	ServicePort int32
}

// SandboxRuntimeBinding 记录一个沙箱在数据面的关键定位信息。
type SandboxRuntimeBinding struct {
	Namespace    string
	WorkspaceDir string
	PodName      string
	Container    string
	ServiceName  string
	PortByName   map[string]int32
}

// SandboxLifecycleSnapshot 是沙箱生命周期、交互鉴权与回收闭环使用的领域投影。
type SandboxLifecycleSnapshot struct {
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
	InitScriptRef     string
	SnapshotRef       string
	SnapshotCreatedAt time.Time
	SnapshotExpireAt  time.Time
	KeepAliveUntil    time.Time
	LastActiveAt      time.Time
	ExpireAt          time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
