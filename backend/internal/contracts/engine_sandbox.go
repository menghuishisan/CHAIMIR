// contracts 定义第 1 层沙箱引擎对外暴露的跨模块生命周期与资源统计契约。
package contracts

import "context"

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

// SandboxToolAccess 是沙箱内某个工具的可访问接入信息。
type SandboxToolAccess struct {
	ToolCode string
	Kind     int16
	Endpoint string
	Status   int16
}

// SandboxInfo 是跨模块传递的沙箱摘要,不暴露 K8s 内部对象细节。
type SandboxInfo struct {
	SandboxID           int64
	TenantID            int64
	Namespace           string
	SourceRef           string
	OwnerAccountID      int64
	RuntimeCode         string
	RuntimeImageVersion string
	Phase               int16
	Status              int16
	ToolAccess          []SandboxToolAccess
}

// SandboxFileWriteRequest 是内部服务写入沙箱工作区文件的请求。
type SandboxFileWriteRequest struct {
	TenantID      int64
	SandboxID     int64
	RelativePath  string
	ContentBase64 string
}

// SandboxExecRequest 是受控执行沙箱命令的内部请求。
type SandboxExecRequest struct {
	TenantID   int64
	SandboxID  int64
	Command    []string
	Stdin      []byte
	TimeoutSec int32
}

// SandboxExecResult 是沙箱命令执行结果,仅保留调用方判定所需输出。
type SandboxExecResult struct {
	Stdout []byte
	Stderr []byte
}

// SandboxRecycleRequest 是按来源级联回收沙箱的内部请求。
type SandboxRecycleRequest struct {
	TenantID  int64
	SourceRef string
	Reason    string
}

// SandboxChainDeployRequest 是统一链部署能力的内部请求。
type SandboxChainDeployRequest struct {
	TenantID  int64
	SandboxID int64
	Payload   map[string]any
}

// SandboxChainTxRequest 是统一链交易能力的内部请求。
type SandboxChainTxRequest struct {
	TenantID  int64
	SandboxID int64
	Payload   map[string]any
}

// SandboxChainQueryRequest 是统一链查询能力的内部请求。
type SandboxChainQueryRequest struct {
	TenantID  int64
	SandboxID int64
	Target    string
}

// SandboxQuotaStats 是 M2 提供给 M9 学校看板的资源统计摘要。
type SandboxQuotaStats struct {
	TenantID                int64
	ActiveSandboxCount      int64
	MaxConcurrentSandbox    int32
	MaxCPU                  int32
	MaxMemoryMB             int32
	IdleTimeoutMin          int32
	MaxLifetimeMin          int32
	MaxKeepaliveMin         int32
	MaxSnapshotRetentionMin int32
}

// SandboxService 是 M2 沙箱引擎对 M3/M7/M8/M9 暴露的标准能力契约。
type SandboxService interface {
	// CreateSandbox 创建沙箱并返回控制面摘要,实际启动过程异步推进。
	CreateSandbox(ctx context.Context, req SandboxCreateRequest) (SandboxInfo, error)
	// GetSandbox 查询单个沙箱当前状态与工具接入信息。
	GetSandbox(ctx context.Context, tenantID, sandboxID int64) (SandboxInfo, error)
	// PauseSandbox 暂停单个沙箱,供实验实例进入已暂停状态时调用。
	PauseSandbox(ctx context.Context, tenantID, sandboxID int64) error
	// ResumeSandbox 恢复单个沙箱,供实验实例从已暂停状态继续运行。
	ResumeSandbox(ctx context.Context, tenantID, sandboxID int64) error
	// DestroySandbox 主动销毁单个沙箱,供显式关闭实例或补偿清理使用。
	DestroySandbox(ctx context.Context, tenantID, sandboxID int64) error
	// RecycleBySourceRef 按来源标识级联回收沙箱,用于实验/竞赛结束收尾。
	RecycleBySourceRef(ctx context.Context, req SandboxRecycleRequest) error
	// PutSandboxFile 把提交代码、脚本或判题输入写入沙箱工作区。
	PutSandboxFile(ctx context.Context, req SandboxFileWriteRequest) error
	// SaveSandboxFiles 立即持久化当前工作区,返回保存后的代码引用与哈希。
	SaveSandboxFiles(ctx context.Context, tenantID, sandboxID int64) (string, string, error)
	// ExecSandboxCommand 在沙箱内执行受限命令,供判题 worker 运行套件。
	ExecSandboxCommand(ctx context.Context, req SandboxExecRequest) (SandboxExecResult, error)
	// ChainDeploy 调用统一链部署能力。
	ChainDeploy(ctx context.Context, req SandboxChainDeployRequest) (map[string]any, error)
	// ChainSendTx 调用统一链交易能力。
	ChainSendTx(ctx context.Context, req SandboxChainTxRequest) (map[string]any, error)
	// ChainQuery 调用统一链查询能力。
	ChainQuery(ctx context.Context, req SandboxChainQueryRequest) (map[string]any, error)
	// ChainReset 调用统一链重置能力。
	ChainReset(ctx context.Context, tenantID, sandboxID int64) error
	// Stats 返回租户级沙箱资源统计,供 M9 学校看板聚合。
	Stats(ctx context.Context, tenantID int64) (SandboxQuotaStats, error)
}
