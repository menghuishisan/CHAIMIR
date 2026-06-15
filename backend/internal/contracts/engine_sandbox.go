// contracts 定义第 1 层沙箱引擎对外暴露的跨模块生命周期与资源统计契约。
package contracts

import "context"

const (
	// SandboxPrivateDomainJudge 表示 M3 注入隐藏测试与评分脚本的私有卷域名称。
	SandboxPrivateDomainJudge = "judge-private"
)

const (
	// SandboxPhaseAllocating 表示沙箱处于资源分配阶段。
	SandboxPhaseAllocating int16 = 1
	// SandboxPhaseReady 表示环境就绪,前端已可进入。
	SandboxPhaseReady int16 = 2
	// SandboxPhaseInitializing 表示个性化初始化仍在执行。
	SandboxPhaseInitializing int16 = 3
	// SandboxPhaseFullyReady 表示沙箱完全可用。
	SandboxPhaseFullyReady int16 = 4
)

const (
	// SandboxStatusCreating 表示沙箱创建中。
	SandboxStatusCreating int16 = 1
	// SandboxStatusRunning 表示沙箱运行中。
	SandboxStatusRunning int16 = 2
	// SandboxStatusPaused 表示沙箱已暂停。
	SandboxStatusPaused int16 = 3
	// SandboxStatusRecycling 表示沙箱回收中。
	SandboxStatusRecycling int16 = 4
	// SandboxStatusDestroyed 表示沙箱已销毁。
	SandboxStatusDestroyed int16 = 5
	// SandboxStatusFailed 表示沙箱启动或运行失败。
	SandboxStatusFailed int16 = 6
)

const (
	// SandboxToolKindBuiltin 表示平台内建工具。
	SandboxToolKindBuiltin int16 = 1
	// SandboxToolKindTerminal 表示终端类工具。
	SandboxToolKindTerminal int16 = 2
	// SandboxToolKindWebEmbed 表示 Web 嵌入类工具。
	SandboxToolKindWebEmbed int16 = 3
)

// SandboxCreateRequest 是业务模块创建沙箱时提交的最小编排请求。
type SandboxCreateRequest struct {
	TenantID                 int64    `json:"tenant_id"`
	RuntimeCode              string   `json:"runtime_code"`
	RuntimeImageVersion      string   `json:"runtime_image_version"`
	ToolCodes                []string `json:"tool_codes"`
	InitCodeRef              string   `json:"init_code_ref"`
	InitScriptRef            string   `json:"init_script_ref"`
	OwnerAccountID           int64    `json:"owner_account_id"`
	SourceRef                string   `json:"source_ref"`
	KeepAlive                bool     `json:"keep_alive"`
	SnapshotEnabled          bool     `json:"snapshot_enabled"`
	KeepAliveMinutes         int32    `json:"keep_alive_minutes"`
	SnapshotRetentionMinutes int32    `json:"snapshot_retention_minutes"`
}

// SandboxToolAccess 是沙箱内某个工具的可访问接入信息。
type SandboxToolAccess struct {
	ToolCode string `json:"tool_code"`
	Kind     int16  `json:"kind"`
	Endpoint string `json:"endpoint"`
	Status   int16  `json:"status"`
}

// SandboxResourceUsage 是单个沙箱实时用量和已申请资源摘要,用于状态查询和配额可视化。
type SandboxResourceUsage struct {
	CPUUsageMilli    int64 `json:"cpu_usage_milli"`
	MemoryUsageMiB   int64 `json:"memory_usage_mib"`
	CPURequestMilli  int64 `json:"cpu_request_milli"`
	CPULimitMilli    int64 `json:"cpu_limit_milli"`
	MemoryRequestMiB int64 `json:"memory_request_mib"`
	MemoryLimitMiB   int64 `json:"memory_limit_mib"`
	StorageBytes     int64 `json:"storage_bytes"`
}

// SandboxInfo 是跨模块传递的沙箱摘要,不暴露 K8s 内部对象细节。
type SandboxInfo struct {
	SandboxID           int64                `json:"sandbox_id"`
	TenantID            int64                `json:"tenant_id"`
	Namespace           string               `json:"namespace"`
	SourceRef           string               `json:"source_ref"`
	OwnerAccountID      int64                `json:"owner_account_id"`
	RuntimeCode         string               `json:"runtime_code"`
	RuntimeImageVersion string               `json:"runtime_image_version"`
	Phase               int16                `json:"phase"`
	Status              int16                `json:"status"`
	ToolAccess          []SandboxToolAccess  `json:"tool_access"`
	ResourceUsage       SandboxResourceUsage `json:"resource_usage"`
}

// SandboxFileWriteRequest 是内部服务写入沙箱工作区文件的请求。
type SandboxFileWriteRequest struct {
	TenantID      int64  `json:"tenant_id"`
	SandboxID     int64  `json:"sandbox_id"`
	SourceRef     string `json:"source_ref"`
	RelativePath  string `json:"relative_path"`
	ContentBase64 string `json:"content_base64"`
}

// SandboxPrivateArchiveInjectRequest 是内部判题服务注入隐藏套件归档的请求。
type SandboxPrivateArchiveInjectRequest struct {
	TenantID      int64  `json:"tenant_id"`
	SandboxID     int64  `json:"sandbox_id"`
	SourceRef     string `json:"source_ref"`
	Domain        string `json:"domain"`
	ArchiveName   string `json:"archive_name"`
	ContentBase64 string `json:"content_base64"`
}

// SandboxSaveRequest 是内部服务请求立即保存工作区的来源绑定请求。
type SandboxSaveRequest struct {
	TenantID  int64  `json:"tenant_id"`
	SandboxID int64  `json:"sandbox_id"`
	SourceRef string `json:"source_ref"`
}

// SandboxExecRequest 是受控执行沙箱命令的内部请求。
type SandboxExecRequest struct {
	TenantID   int64    `json:"tenant_id"`
	SandboxID  int64    `json:"sandbox_id"`
	SourceRef  string   `json:"source_ref"`
	Command    []string `json:"command"`
	Stdin      []byte   `json:"stdin"`
	TimeoutSec int32    `json:"timeout_sec"`
}

// SandboxExecResult 是沙箱命令执行结果,仅保留调用方判定所需输出。
type SandboxExecResult struct {
	Stdout []byte `json:"stdout"`
	Stderr []byte `json:"stderr"`
}

// SandboxRecycleRequest 是按来源级联回收沙箱的内部请求。
type SandboxRecycleRequest struct {
	TenantID  int64  `json:"tenant_id"`
	SourceRef string `json:"source_ref"`
	Reason    string `json:"reason"`
}

// SandboxControlRequest 是暂停、恢复和销毁单个沙箱的内部控制请求。
type SandboxControlRequest struct {
	TenantID  int64  `json:"tenant_id"`
	SandboxID int64  `json:"sandbox_id"`
	SourceRef string `json:"source_ref"`
}

// SandboxChainDeployRequest 是统一链部署能力的内部请求。
type SandboxChainDeployRequest struct {
	TenantID  int64          `json:"tenant_id"`
	SandboxID int64          `json:"sandbox_id"`
	SourceRef string         `json:"source_ref"`
	Payload   map[string]any `json:"payload"`
}

// SandboxChainTxRequest 是统一链交易能力的内部请求。
type SandboxChainTxRequest struct {
	TenantID  int64          `json:"tenant_id"`
	SandboxID int64          `json:"sandbox_id"`
	SourceRef string         `json:"source_ref"`
	Payload   map[string]any `json:"payload"`
}

// SandboxChainQueryRequest 是统一链查询能力的内部请求。
type SandboxChainQueryRequest struct {
	TenantID  int64  `json:"tenant_id"`
	SandboxID int64  `json:"sandbox_id"`
	SourceRef string `json:"source_ref"`
	Target    string `json:"target"`
}

// SandboxChainResetRequest 是统一链重置能力的内部请求。
type SandboxChainResetRequest struct {
	TenantID  int64  `json:"tenant_id"`
	SandboxID int64  `json:"sandbox_id"`
	SourceRef string `json:"source_ref"`
}

// SandboxQuotaStats 是 M2 提供给 M9 学校看板的资源统计摘要。
type SandboxQuotaStats struct {
	TenantID                int64 `json:"tenant_id"`
	ActiveSandboxCount      int64 `json:"active_sandbox_count"`
	MaxConcurrentSandbox    int32 `json:"max_concurrent_sandbox"`
	MaxCPU                  int32 `json:"max_cpu"`
	MaxMemoryMB             int32 `json:"max_memory_mb"`
	IdleTimeoutMin          int32 `json:"idle_timeout_min"`
	MaxLifetimeMin          int32 `json:"max_lifetime_min"`
	MaxKeepaliveMin         int32 `json:"max_keepalive_min"`
	MaxSnapshotRetentionMin int32 `json:"max_snapshot_retention_min"`
}

// SandboxService 是 M2 沙箱引擎对 M3/M7/M8/M9 暴露的标准能力契约。
type SandboxService interface {
	// CreateSandbox 创建沙箱并返回控制面摘要,实际启动过程异步推进。
	CreateSandbox(ctx context.Context, req SandboxCreateRequest) (SandboxInfo, error)
	// GetSandbox 查询单个沙箱当前状态与工具接入信息。
	GetSandbox(ctx context.Context, tenantID, sandboxID int64) (SandboxInfo, error)
	// PauseSandbox 暂停单个沙箱,供实验实例进入已暂停状态时调用。
	PauseSandbox(ctx context.Context, req SandboxControlRequest) error
	// ResumeSandbox 恢复单个沙箱,供实验实例从已暂停状态继续运行。
	ResumeSandbox(ctx context.Context, req SandboxControlRequest) error
	// DestroySandbox 主动销毁单个沙箱,供显式关闭实例或补偿清理使用。
	DestroySandbox(ctx context.Context, req SandboxControlRequest) error
	// RecycleBySourceRef 按来源标识级联回收沙箱,用于实验/竞赛结束收尾。
	RecycleBySourceRef(ctx context.Context, req SandboxRecycleRequest) error
	// PutSandboxFile 把提交代码或公开脚本写入沙箱工作区,不得用于隐藏测试或答案。
	PutSandboxFile(ctx context.Context, req SandboxFileWriteRequest) error
	// PutSandboxPrivateArchive 把隐藏测试、答案或评分脚本安全解包到私有判题域。
	PutSandboxPrivateArchive(ctx context.Context, req SandboxPrivateArchiveInjectRequest) error
	// SaveSandboxFiles 立即持久化当前工作区,返回保存后的代码引用与哈希。
	SaveSandboxFiles(ctx context.Context, req SandboxSaveRequest) (string, string, error)
	// ExecSandboxCommand 在沙箱内执行受限命令,供判题 worker 运行套件。
	ExecSandboxCommand(ctx context.Context, req SandboxExecRequest) (SandboxExecResult, error)
	// ChainDeploy 调用统一链部署能力。
	ChainDeploy(ctx context.Context, req SandboxChainDeployRequest) (map[string]any, error)
	// ChainSendTx 调用统一链交易能力。
	ChainSendTx(ctx context.Context, req SandboxChainTxRequest) (map[string]any, error)
	// ChainQuery 调用统一链查询能力。
	ChainQuery(ctx context.Context, req SandboxChainQueryRequest) (map[string]any, error)
	// ChainReset 调用统一链重置能力。
	ChainReset(ctx context.Context, req SandboxChainResetRequest) error
	// Stats 返回租户级沙箱资源统计,供 M9 学校看板聚合。
	Stats(ctx context.Context, tenantID int64) (SandboxQuotaStats, error)
}
