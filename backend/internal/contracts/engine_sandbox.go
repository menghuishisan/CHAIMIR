// contracts 定义第 1 层沙箱引擎对外暴露的跨模块生命周期与资源统计契约。
package contracts

import (
	"context"
	"io"
)

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
	// SandboxStatusReady 表示沙箱环境已就绪但尚未发生学生操作。
	SandboxStatusReady int16 = 7
	// SandboxStatusIdle 表示沙箱已运行但超过空闲计时阈值,等待回收或恢复操作。
	SandboxStatusIdle int16 = 8
)

const (
	// SandboxToolKindBuiltin 表示平台内建工具。
	SandboxToolKindBuiltin int16 = 1
	// SandboxToolKindTerminal 表示终端类工具。
	SandboxToolKindTerminal int16 = 2
	// SandboxToolKindWebEmbed 表示 Web 嵌入类工具。
	SandboxToolKindWebEmbed int16 = 3
	// SandboxToolKindCommand 表示受控命令类工具。
	SandboxToolKindCommand int16 = 4
	// SandboxToolKindInfra 表示仅参与沙箱编排的基础设施组件。
	SandboxToolKindInfra int16 = 5
)

// SandboxCreateRequest 是业务模块创建沙箱时提交的最小编排请求。
// 调用方只能通过本契约使用 M2,不得 import sandbox 模块内部包。
// SourceRef 必须使用全局四段规范,实验实例必须传 experiment:<year>:instance:<id>,不得使用 exp 等短前缀别名。
// keep_alive 与 snapshot_enabled 默认必须为 false;只有调用方业务明确要求保活或快照时才能开启。
// 开启 keep_alive 必须同时提交正数 keep_alive_minutes;开启 snapshot_enabled 必须同时提交正数 snapshot_retention_minutes。
// 未开启对应能力时分钟数字段必须为 0,不得用隐式默认值绕过 M2 配额校验。
type SandboxCreateRequest struct {
	TenantID                 int64                      `json:"tenant_id"`
	CompositionSnapshot      SandboxCompositionSnapshot `json:"composition_snapshot"`
	InitCodeRef              string                     `json:"init_code_ref"`
	InitScriptRef            string                     `json:"init_script_ref"`
	OwnerAccountID           int64                      `json:"owner_account_id"`
	AuthorizedAccountIDs     []int64                    `json:"authorized_account_ids,omitempty"`
	SourceRef                string                     `json:"source_ref"`
	ScopeRef                 string                     `json:"scope_ref"`
	KeepAlive                bool                       `json:"keep_alive"`
	SnapshotEnabled          bool                       `json:"snapshot_enabled"`
	KeepAliveMinutes         int32                      `json:"keep_alive_minutes"`
	SnapshotRetentionMinutes int32                      `json:"snapshot_retention_minutes"`
	// PrivateSidecars 仅供服务端内部判题等场景挂载非学生可访问执行容器。
	PrivateSidecars []SandboxPrivateSidecarSpec `json:"private_sidecars,omitempty"`
}

// SandboxEnvVarSpec 描述内部私有执行容器允许注入的非敏感字面量环境变量。
type SandboxEnvVarSpec struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// SandboxResourceSpec 描述内部私有执行容器 requests/limits。
type SandboxResourceSpec struct {
	Requests map[string]string `json:"requests"`
	Limits   map[string]string `json:"limits"`
}

// SandboxEphemeralMountSpec 描述内部私有执行容器只读根文件系统下的临时可写目录。
type SandboxEphemeralMountSpec struct {
	Name      string `json:"name"`
	MountPath string `json:"mount_path"`
}

// SandboxPrivateSidecarSpec 是 M3 判题等服务端内部场景传给 M2 的非学生执行容器声明。
type SandboxPrivateSidecarSpec struct {
	Name                   string                      `json:"name"`
	ImageURL               string                      `json:"image_url"`
	Command                []string                    `json:"command"`
	Args                   []string                    `json:"args"`
	Env                    []SandboxEnvVarSpec         `json:"env"`
	Resources              SandboxResourceSpec         `json:"resources"`
	Workdir                string                      `json:"workdir"`
	ReadOnlyRootFilesystem *bool                       `json:"read_only_root_filesystem"`
	Labels                 map[string]string           `json:"labels"`
	MountWorkspace         *bool                       `json:"mount_workspace"`
	MountDomains           []string                    `json:"mount_domains,omitempty"`
	EphemeralMounts        []SandboxEphemeralMountSpec `json:"ephemeral_mounts"`
}

// SandboxToolAccess 是沙箱内某个工具的可访问接入信息。
type SandboxToolAccess struct {
	ToolCode string `json:"tool_code"`
	Kind     int16  `json:"kind"`
	Endpoint string `json:"endpoint"`
	Status   int16  `json:"status"`
}

// SandboxCapabilities 是后端按实际运行时和已注册实现计算出的前端工作台能力。
type SandboxCapabilities struct {
	FileWorkspace   bool     `json:"file_workspace"`
	Terminal        bool     `json:"terminal"`
	CommandTools    bool     `json:"command_tools"`
	ChainOperations []string `json:"chain_operations"`
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

// SandboxInfo 是跨模块传递给服务端调用方的沙箱摘要。
// Namespace 仅允许调用方用于服务端内部资源归属校验和补偿排障,不得透传给前端工作台响应。
type SandboxInfo struct {
	SandboxID           int64                `json:"sandbox_id"`
	TenantID            int64                `json:"tenant_id"`
	Namespace           string               `json:"namespace"`
	SourceRef           string               `json:"source_ref"`
	ScopeRef            string               `json:"scope_ref"`
	CompositionDigest   string               `json:"composition_digest"`
	OwnerAccountID      int64                `json:"owner_account_id"`
	RuntimeCode         string               `json:"runtime_code"`
	RuntimeImageVersion string               `json:"runtime_image_version"`
	RuntimeInstances    []string             `json:"runtime_instances"`
	Phase               int16                `json:"phase"`
	Status              int16                `json:"status"`
	ToolAccess          []SandboxToolAccess  `json:"tool_access"`
	Capabilities        SandboxCapabilities  `json:"capabilities"`
	ResourceUsage       SandboxResourceUsage `json:"resource_usage"`
	WorkspaceRevision   int64                `json:"workspace_revision"`
}

// SandboxAccessPrincipal 是 M8 等业务网关经自身授权表核验后传给 M2 的受控访问主体。
// M2 仍会校验目标租户、沙箱和来源一致性；该主体不会被普通用户 HTTP 路由接受。
type SandboxAccessPrincipal struct {
	AuthorizationID       int64  `json:"authorization_id"`
	AuthorizationRevision int64  `json:"authorization_revision"`
	SubjectTenantID       int64  `json:"subject_tenant_id"`
	SubjectAccountID      int64  `json:"subject_account_id"`
	SourceRef             string `json:"source_ref"`
}

// SandboxPrincipalRequest 绑定受控主体、组织租户沙箱和不可伪造的来源引用。
type SandboxPrincipalRequest struct {
	TenantID  int64                  `json:"tenant_id"`
	SandboxID int64                  `json:"sandbox_id"`
	SourceRef string                 `json:"source_ref"`
	Principal SandboxAccessPrincipal `json:"principal"`
}

// SandboxWorkspaceFileRead 是跨模块读取工作区文件的稳定响应。
type SandboxWorkspaceFileRead struct {
	RelativePath      string `json:"relative_path"`
	ContentBase64     string `json:"content_base64"`
	ContentSHA256     string `json:"content_sha256"`
	ContentSize       int64  `json:"content_size"`
	WorkspaceRevision int64  `json:"workspace_revision"`
}

// SandboxWorkspaceFileEntry 是跨模块列目录的单个安全条目。
type SandboxWorkspaceFileEntry struct {
	Name         string `json:"name"`
	RelativePath string `json:"relative_path"`
	IsDir        bool   `json:"is_dir"`
	Size         int64  `json:"size"`
}

// SandboxWorkspaceFileList 是跨模块列目录响应。
type SandboxWorkspaceFileList struct {
	RelativePath string                      `json:"relative_path"`
	Entries      []SandboxWorkspaceFileEntry `json:"entries"`
}

// SandboxWorkspaceFileWrite 是经 M8 授权后写入一个公开工作区文件的请求。
type SandboxWorkspaceFileWrite struct {
	Access           SandboxPrincipalRequest `json:"access"`
	RelativePath     string                  `json:"relative_path"`
	ContentBase64    string                  `json:"content_base64"`
	ExpectedRevision int64                   `json:"expected_revision"`
}

// SandboxWorkspaceSave 是工作区立即持久化后的稳定返回值。
type SandboxWorkspaceSave struct {
	CodeStorageKey    string `json:"code_storage_key"`
	CodeHash          string `json:"code_hash"`
	WorkspaceRevision int64  `json:"workspace_revision"`
}

// SandboxCommandToolRequest 是 M8 代理命令工具时的受控 argv 请求。
type SandboxCommandToolRequest struct {
	Access      SandboxPrincipalRequest `json:"access"`
	ToolCode    string                  `json:"tool_code"`
	Command     []string                `json:"command"`
	StdinBase64 string                  `json:"stdin_base64"`
	TimeoutSec  int32                   `json:"timeout_sec"`
}

// SandboxCommandToolResult 是命令工具执行结果。
type SandboxCommandToolResult struct {
	StdoutBase64 string `json:"stdout_base64"`
	StderrBase64 string `json:"stderr_base64"`
	ExitCode     int    `json:"exit_code"`
}

// SandboxTerminalTarget 是已授权终端连接可进入的受控目标,不包含 Kubernetes 凭据。
type SandboxTerminalTarget struct {
	TenantID  int64    `json:"tenant_id"`
	SandboxID int64    `json:"sandbox_id"`
	Namespace string   `json:"namespace"`
	Container string   `json:"container"`
	Command   []string `json:"command"`
}

// SandboxProgressMessage 是受控进度订阅的初始用户向状态。
type SandboxProgressMessage struct {
	Phase   int16  `json:"phase"`
	Status  int16  `json:"status"`
	Stage   string `json:"stage"`
	Message string `json:"message"`
	TraceID string `json:"trace_id,omitempty"`
}

// SandboxToolProxyTarget 是 M8 浏览器工具代理所需的内部 Service 目标。
type SandboxToolProxyTarget struct {
	TargetURL string `json:"target_url"`
}

// SandboxFileWriteRequest 是内部服务写入沙箱工作区文件的请求。
type SandboxFileWriteRequest struct {
	TenantID         int64  `json:"tenant_id"`
	SandboxID        int64  `json:"sandbox_id"`
	SourceRef        string `json:"source_ref"`
	RelativePath     string `json:"relative_path"`
	ContentBase64    string `json:"content_base64"`
	ExpectedRevision int64  `json:"expected_revision"`
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

// SandboxArchiveRestoreRequest 是内部服务把已授权对象归档恢复到工作区子目录的请求。
// ExpectedHash 非空时 M2 必须按对象实际内容计算 SHA-256 后逐字匹配,调用方不得只信任提交时声明。
type SandboxArchiveRestoreRequest struct {
	TenantID     int64  `json:"tenant_id"`
	SandboxID    int64  `json:"sandbox_id"`
	SourceRef    string `json:"source_ref"`
	ObjectRef    string `json:"object_ref"`
	ExpectedHash string `json:"expected_hash"`
	TargetDir    string `json:"target_dir"`
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
	Container  string   `json:"container,omitempty"`
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
	ScopeRef  string `json:"scope_ref"`
	SourceRef string `json:"source_ref"`
	Reason    string `json:"reason"`
}

// SandboxControlRequest 是暂停、恢复和销毁单个沙箱的内部控制请求。
type SandboxControlRequest struct {
	TenantID  int64  `json:"tenant_id"`
	SandboxID int64  `json:"sandbox_id"`
	SourceRef string `json:"source_ref"`
}

// SandboxAuthorizedAccountsRequest 更新已创建沙箱的共享账号集合。
type SandboxAuthorizedAccountsRequest struct {
	TenantID             int64   `json:"tenant_id"`
	SandboxID            int64   `json:"sandbox_id"`
	SourceRef            string  `json:"source_ref"`
	AuthorizedAccountIDs []int64 `json:"authorized_account_ids"`
}

// SandboxChainDeployRequest 是统一链部署能力的内部请求。
type SandboxChainDeployRequest struct {
	TenantID        int64          `json:"tenant_id"`
	SandboxID       int64          `json:"sandbox_id"`
	SourceRef       string         `json:"source_ref"`
	RuntimeInstance string         `json:"runtime_instance"`
	Payload         map[string]any `json:"payload"`
}

// SandboxChainTxRequest 是统一链交易能力的内部请求。
type SandboxChainTxRequest struct {
	TenantID        int64          `json:"tenant_id"`
	SandboxID       int64          `json:"sandbox_id"`
	SourceRef       string         `json:"source_ref"`
	RuntimeInstance string         `json:"runtime_instance"`
	Payload         map[string]any `json:"payload"`
}

// SandboxChainQueryRequest 是统一链查询能力的内部请求。
type SandboxChainQueryRequest struct {
	TenantID        int64  `json:"tenant_id"`
	SandboxID       int64  `json:"sandbox_id"`
	SourceRef       string `json:"source_ref"`
	RuntimeInstance string `json:"runtime_instance"`
	Target          string `json:"target"`
}

// SandboxChainResetRequest 是统一链重置能力的内部请求。
type SandboxChainResetRequest struct {
	TenantID        int64  `json:"tenant_id"`
	SandboxID       int64  `json:"sandbox_id"`
	SourceRef       string `json:"source_ref"`
	RuntimeInstance string `json:"runtime_instance"`
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

// SandboxReadService 是 M2 面向聚合层(M9 admin)的只读契约;聚合层只读不跨写,不得持有写能力。
type SandboxReadService interface {
	// Stats 返回租户级沙箱资源统计,供 M9 学校看板聚合。
	Stats(ctx context.Context, tenantID int64) (SandboxQuotaStats, error)
}

// SandboxService 是 M2 沙箱引擎对 M3/M7/M8/M9 暴露的标准能力契约。
type SandboxService interface {
	SandboxReadService
	// CompileSandboxComposition 校验声明、展开依赖并返回不可变快照。
	CompileSandboxComposition(ctx context.Context, tenantID int64, spec SandboxCompositionSpec) (SandboxCompositionSnapshot, error)
	// CompilePlatformSandboxComposition 供平台治理面编译不属于任一租户的不可变组合快照。
	CompilePlatformSandboxComposition(ctx context.Context, spec SandboxCompositionSpec) (SandboxCompositionSnapshot, error)
	// ValidateSandboxTemplate 校验业务模块保存/发布的沙箱模板能解析到可调度运行时、镜像和工具,但不创建资源。
	ValidateSandboxTemplate(ctx context.Context, req SandboxCreateRequest) error
	// CreateSandbox 创建沙箱并返回控制面摘要,实际启动过程异步推进。
	CreateSandbox(ctx context.Context, req SandboxCreateRequest) (SandboxInfo, error)
	// GetSandbox 查询单个沙箱当前状态与工具接入信息。
	GetSandbox(ctx context.Context, tenantID, sandboxID int64) (SandboxInfo, error)
	// 下列 Principal 方法只允许 M8 等业务网关在已核验业务授权后调用；普通 M2 用户路由不得转发它们。
	GetSandboxForPrincipal(ctx context.Context, access SandboxPrincipalRequest) (SandboxInfo, error)
	ReadSandboxFileForPrincipal(ctx context.Context, access SandboxPrincipalRequest, relativePath string) (SandboxWorkspaceFileRead, error)
	ListSandboxFilesForPrincipal(ctx context.Context, access SandboxPrincipalRequest, relativePath string) (SandboxWorkspaceFileList, error)
	WriteSandboxFileForPrincipal(ctx context.Context, req SandboxWorkspaceFileWrite) (int64, error)
	SaveSandboxFilesForPrincipal(ctx context.Context, access SandboxPrincipalRequest) (SandboxWorkspaceSave, error)
	RunCommandToolForPrincipal(ctx context.Context, req SandboxCommandToolRequest) (SandboxCommandToolResult, error)
	TerminalTargetForPrincipal(ctx context.Context, access SandboxPrincipalRequest, container string) (SandboxTerminalTarget, error)
	AttachSandboxTerminal(ctx context.Context, access SandboxPrincipalRequest, target SandboxTerminalTarget, stdin io.Reader, stdout io.Writer) error
	ProgressSubscriptionForPrincipal(ctx context.Context, access SandboxPrincipalRequest) (string, SandboxProgressMessage, error)
	ToolProxyTargetForPrincipal(ctx context.Context, access SandboxPrincipalRequest, toolCode string) (SandboxToolProxyTarget, error)
	ObserveSandboxToolAccess(ctx context.Context, access SandboxPrincipalRequest) error
	ChainDeployForPrincipal(ctx context.Context, access SandboxPrincipalRequest, runtimeInstance string, payload map[string]any) (map[string]any, error)
	ChainSendTxForPrincipal(ctx context.Context, access SandboxPrincipalRequest, runtimeInstance string, payload map[string]any) (map[string]any, error)
	ChainQueryForPrincipal(ctx context.Context, access SandboxPrincipalRequest, runtimeInstance, target string) (map[string]any, error)
	// UpdateSandboxAuthorizedAccounts 原子替换共享账号集合,供实验编组变更同步权限。
	UpdateSandboxAuthorizedAccounts(ctx context.Context, req SandboxAuthorizedAccountsRequest) error
	// GetSandboxWorkspaceArchive 查询已保存工作区的服务端对象引用,供实例重建时恢复。
	GetSandboxWorkspaceArchive(ctx context.Context, tenantID, sandboxID int64) (string, error)
	// PauseSandbox 暂停单个沙箱,供实验实例进入已暂停状态时调用。
	PauseSandbox(ctx context.Context, req SandboxControlRequest) error
	// ResumeSandbox 恢复单个沙箱,供实验实例从已暂停状态继续运行。
	ResumeSandbox(ctx context.Context, req SandboxControlRequest) error
	// DestroySandbox 主动销毁单个沙箱,供显式关闭实例或补偿清理使用。
	DestroySandbox(ctx context.Context, req SandboxControlRequest) error
	// RecycleByScopeRef 按生命周期作用域级联回收沙箱,用于实验/竞赛结束收尾。
	RecycleByScopeRef(ctx context.Context, req SandboxRecycleRequest) error
	// PutSandboxFile 把提交代码或公开脚本写入沙箱工作区,不得用于隐藏测试或答案。
	PutSandboxFile(ctx context.Context, req SandboxFileWriteRequest) (int64, error)
	// PutSandboxPrivateArchive 把隐藏测试、答案或评分脚本安全解包到私有判题域。
	PutSandboxPrivateArchive(ctx context.Context, req SandboxPrivateArchiveInjectRequest) error
	// RestoreSandboxArchive 把调用方已保存的公开参战物或代码归档安全恢复到工作区子目录。
	RestoreSandboxArchive(ctx context.Context, req SandboxArchiveRestoreRequest) error
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
}
