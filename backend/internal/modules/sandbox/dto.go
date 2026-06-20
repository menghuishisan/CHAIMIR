// sandbox dto 文件定义 M2 HTTP 请求和响应结构,不承载业务编排逻辑。
package sandbox

import (
	"encoding/json"

	"chaimir/internal/contracts"
)

// RuntimeRequest 是平台管理员注册或更新运行时的请求。
type RuntimeRequest struct {
	Code           string          `json:"code"`
	Name           string          `json:"name"`
	Eco            string          `json:"eco"`
	AdapterLevel   int16           `json:"adapter_level"`
	AdapterSpec    json.RawMessage `json:"adapter_spec"`
	CapabilityImpl string          `json:"capability_impl"`
	PluginRef      string          `json:"plugin_ref"`
	Status         int16           `json:"status"`
}

// RuntimeImageRequest 是平台管理员登记运行时镜像版本的请求。
type RuntimeImageRequest struct {
	ImageURL     string `json:"image_url"`
	Version      string `json:"version"`
	Digest       string `json:"digest"`
	GenesisBaked bool   `json:"genesis_baked"`
	IsDefault    bool   `json:"is_default"`
}

// ToolRequest 是平台管理员注册或更新沙箱工具的请求。
type ToolRequest struct {
	Code         string          `json:"code"`
	Name         string          `json:"name"`
	Kind         int16           `json:"kind"`
	EcoTags      []string        `json:"eco_tags"`
	ResourceSpec json.RawMessage `json:"resource_spec"`
	Status       int16           `json:"status"`
}

// CreateSandboxRequest 是内部 HTTP 创建沙箱请求。
type CreateSandboxRequest struct {
	TenantID                 int64    `json:"tenant_id"`
	RuntimeCode              string   `json:"runtime_code"`
	RuntimeImageVersion      string   `json:"runtime_image_version"`
	Tools                    []string `json:"tools"`
	InitCodeRef              string   `json:"init_code_ref"`
	InitScriptRef            string   `json:"init_script_ref"`
	OwnerAccountID           int64    `json:"owner_account_id"`
	SourceRef                string   `json:"source_ref"`
	KeepAlive                bool     `json:"keep_alive"`
	SnapshotEnabled          bool     `json:"snapshot_enabled"`
	KeepAliveMinutes         int32    `json:"keep_alive_minutes"`
	SnapshotRetentionMinutes int32    `json:"snapshot_retention_minutes"`
}

// RecycleRequest 是内部 HTTP 来源级联回收请求。
type RecycleRequest struct {
	TenantID  int64  `json:"tenant_id"`
	SourceRef string `json:"source_ref"`
	Reason    string `json:"reason"`
}

// FileWriteRequest 是用户或内部服务写入工作区文件的请求。
type FileWriteRequest struct {
	TenantID      int64  `json:"tenant_id"`
	RelativePath  string `json:"relative_path"`
	ContentBase64 string `json:"content_base64"`
}

// ChainRequest 是链部署或交易的统一请求体。
type ChainRequest struct {
	TenantID int64          `json:"tenant_id"`
	Payload  map[string]any `json:"payload"`
}

// QuotaRequest 是配额调整请求。
type QuotaRequest struct {
	TenantID                int64 `json:"tenant_id"`
	MaxConcurrentSandbox    int32 `json:"max_concurrent_sandbox"`
	MaxCPU                  int32 `json:"max_cpu"`
	MaxMemoryMB             int32 `json:"max_memory_mb"`
	IdleTimeoutMin          int32 `json:"idle_timeout_min"`
	MaxLifetimeMin          int32 `json:"max_lifetime_min"`
	MaxKeepaliveMin         int32 `json:"max_keepalive_min"`
	MaxSnapshotRetentionMin int32 `json:"max_snapshot_retention_min"`
}

// PrepullResponse 描述镜像预拉取状态响应。
type PrepullResponse struct {
	ImageID       int64  `json:"image_id"`
	PrepullStatus int16  `json:"prepull_status"`
	DesiredNodes  int32  `json:"desired_nodes"`
	ReadyNodes    int32  `json:"ready_nodes"`
	DaemonSet     string `json:"daemonset"`
}

// RuntimeSelftestResponse 描述运行时接入即测的当前结果。
type RuntimeSelftestResponse struct {
	RuntimeID      int64           `json:"runtime_id"`
	SelftestStatus int16           `json:"selftest_status"`
	RuntimeStatus  int16           `json:"runtime_status"`
	Detail         json.RawMessage `json:"detail"`
}

// SandboxResponse 描述用户侧可见的沙箱状态,不暴露 Kubernetes Namespace 等内部资源名。
type SandboxResponse struct {
	SandboxID           int64                          `json:"sandbox_id"`
	TenantID            int64                          `json:"tenant_id"`
	SourceRef           string                         `json:"source_ref"`
	OwnerAccountID      int64                          `json:"owner_account_id"`
	RuntimeCode         string                         `json:"runtime_code"`
	RuntimeImageVersion string                         `json:"runtime_image_version"`
	Phase               int16                          `json:"phase"`
	Status              int16                          `json:"status"`
	ToolAccess          []contracts.SandboxToolAccess  `json:"tool_access"`
	ResourceUsage       contracts.SandboxResourceUsage `json:"resource_usage"`
}

// FileSaveResponse 描述立即保存工作区后的对象引用和内容哈希。
type FileSaveResponse struct {
	CodeStorageKey string `json:"code_storage_key"`
	CodeHash       string `json:"code_hash"`
}

// FileReadResponse 描述工作区文件读取响应。
type FileReadResponse struct {
	RelativePath  string `json:"relative_path"`
	ContentBase64 string `json:"content_base64"`
	ContentSHA256 string `json:"content_sha256"`
	ContentSize   int64  `json:"content_size"`
}

// FileEntryResponse 描述工作区目录列表中的单个安全条目。
type FileEntryResponse struct {
	Name         string `json:"name"`
	RelativePath string `json:"relative_path"`
	IsDir        bool   `json:"is_dir"`
	Size         int64  `json:"size"`
}

// FileListResponse 描述工作区目录列表响应。
type FileListResponse struct {
	RelativePath string              `json:"relative_path"`
	Entries      []FileEntryResponse `json:"entries"`
}

// ProgressMessage 描述 WebSocket 推送给前端的用户向沙箱进度。
type ProgressMessage struct {
	Phase   int16  `json:"phase"`
	Status  int16  `json:"status"`
	Stage   string `json:"stage"`
	Message string `json:"message"`
	TraceID string `json:"trace_id,omitempty"`
}
