// M2 沙箱 HTTP 请求/响应 DTO。
// 对外雪花 ID 统一用字符串,避免前端 JSON 精度问题。
package sandbox

// CreateRuntimeRequest 是平台管理员注册运行时的请求。
type CreateRuntimeRequest struct {
	Code           string         `json:"code" binding:"required"`
	Name           string         `json:"name" binding:"required"`
	Eco            string         `json:"eco" binding:"required"`
	AdapterLevel   int16          `json:"adapter_level" binding:"required"`
	AdapterSpec    map[string]any `json:"adapter_spec" binding:"required"`
	CapabilityImpl string         `json:"capability_impl"`
	PluginRef      string         `json:"plugin_ref"`
}

// CreateRuntimeImageRequest 是登记运行时镜像的请求。
type CreateRuntimeImageRequest struct {
	ImageURL     string `json:"image_url" binding:"required"`
	Version      string `json:"version" binding:"required"`
	Digest       string `json:"digest" binding:"required"`
	GenesisBaked bool   `json:"genesis_baked"`
	IsDefault    bool   `json:"is_default"`
}

// CreateToolRequest 是平台注册工具定义的请求。
type CreateToolRequest struct {
	Code         string         `json:"code" binding:"required"`
	Name         string         `json:"name" binding:"required"`
	Kind         int16          `json:"kind" binding:"required"`
	ImageURL     string         `json:"image_url"`
	Digest       string         `json:"digest"`
	Port         int32          `json:"port"`
	EcoTags      string         `json:"eco_tags" binding:"required"`
	ResourceSpec map[string]any `json:"resource_spec"`
}

// CreateSandboxRequest 是内部调用方创建沙箱的 HTTP 请求。
type CreateSandboxRequest struct {
	RuntimeCode              string   `json:"runtime_code" binding:"required"`
	RuntimeImageVersion      string   `json:"runtime_image_version"`
	Tools                    []string `json:"tools" binding:"required"`
	InitCodeRef              string   `json:"init_code_ref"`
	InitScriptRef            string   `json:"init_script_ref"`
	OwnerAccountID           string   `json:"owner_account_id" binding:"required"`
	SourceRef                string   `json:"source_ref" binding:"required"`
	KeepAlive                bool     `json:"keep_alive"`
	KeepAliveMinutes         int32    `json:"keep_alive_minutes"`
	SnapshotEnabled          bool     `json:"snapshot_enabled"`
	SnapshotRetentionMinutes int32    `json:"snapshot_retention_minutes"`
}

// SandboxView 是沙箱状态响应。
type SandboxView struct {
	ID                  string            `json:"id"`
	Namespace           string            `json:"namespace"`
	RuntimeID           string            `json:"runtime_id"`
	ImageID             string            `json:"image_id"`
	RuntimeImageVersion string            `json:"runtime_image_version"`
	SourceRef           string            `json:"source_ref"`
	OwnerAccountID      string            `json:"owner_account_id"`
	Phase               int16             `json:"phase"`
	Status              int16             `json:"status"`
	Tools               []SandboxToolView `json:"tools"`
}

// SandboxToolView 是沙箱工具接入响应。
type SandboxToolView struct {
	ToolCode string `json:"tool_code"`
	Name     string `json:"name"`
	Kind     int16  `json:"kind"`
	Endpoint string `json:"endpoint"`
	Status   int16  `json:"status"`
}

// RecycleSandboxRequest 是按来源级联回收请求。
type RecycleSandboxRequest struct {
	SourceRef string `json:"source_ref" binding:"required"`
	Reason    string `json:"reason" binding:"required"`
}

// QuotaRequest 是租户配额调整请求。
type QuotaRequest struct {
	MaxConcurrentSandbox    int32 `json:"max_concurrent_sandbox" binding:"required"`
	MaxCPU                  int32 `json:"max_cpu" binding:"required"`
	MaxMemoryMB             int32 `json:"max_memory_mb" binding:"required"`
	IdleTimeoutMin          int32 `json:"idle_timeout_min" binding:"required"`
	MaxLifetimeMin          int32 `json:"max_lifetime_min" binding:"required"`
	MaxKeepaliveMin         int32 `json:"max_keepalive_min" binding:"required"`
	MaxSnapshotRetentionMin int32 `json:"max_snapshot_retention_min" binding:"required"`
}

// UpdateRuntimeRequest 是更新运行时配置的请求。
type UpdateRuntimeRequest struct {
	Name           string         `json:"name" binding:"required"`
	Eco            string         `json:"eco" binding:"required"`
	AdapterLevel   int16          `json:"adapter_level" binding:"required"`
	AdapterSpec    map[string]any `json:"adapter_spec" binding:"required"`
	CapabilityImpl string         `json:"capability_impl"`
	PluginRef      string         `json:"plugin_ref"`
	Status         int16          `json:"status" binding:"required"`
}

// FileWriteRequest 是沙箱文件写入请求。
type FileWriteRequest struct {
	Content  string `json:"content" binding:"required"`
	Encoding string `json:"encoding"`
}
