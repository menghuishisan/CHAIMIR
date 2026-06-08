// 第1层 共享引擎:sandbox/judge/sim/content 对外【跨模块】接口契约。
// 业务模块经这些接口调用引擎能力,不得 import engine 模块内部 package。
package contracts

import "context"

const (
	// SandboxPhaseReady 表示 M2 沙箱已完成个性化初始化,可供 M3 执行判题命令。
	SandboxPhaseReady int16 = 4
	// SandboxStatusRunning 表示 M2 沙箱运行中。
	SandboxStatusRunning int16 = 3
	// SandboxStatusError 表示 M2 沙箱启动或运行失败。
	SandboxStatusError int16 = 7
)

// SandboxCreateRequest 是业务模块创建沙箱时提交的最小引擎参数。
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
	KeepAliveMinutes         int32
	SnapshotEnabled          bool
	SnapshotRetentionMinutes int32
}

// SandboxInfo 是跨模块传递的沙箱摘要,不暴露 K8s 内部凭据。
type SandboxInfo struct {
	SandboxID           int64
	TenantID            int64
	Namespace           string
	SourceRef           string
	OwnerID             int64
	RuntimeImageVersion string
	Phase               int16
	Status              int16
	ToolAccess          []SandboxToolAccess
}

// SandboxToolAccess 是某个工具在沙箱中的接入端点。
type SandboxToolAccess struct {
	ToolCode string
	Kind     int16
	Endpoint string
	Status   int16
}

// SandboxFileWrite 是跨模块写入沙箱文件的输入。
type SandboxFileWrite struct {
	SandboxID     int64
	RelativePath  string
	ContentBase64 string
}

// SandboxExecRequest 是跨模块在沙箱内执行受限命令的输入。
type SandboxExecRequest struct {
	SandboxID  int64
	Command    []string
	Stdin      []byte
	TimeoutSec int32
}

// SandboxExecResult 是沙箱命令执行结果,只包含判题解析所需输出。
type SandboxExecResult struct {
	Stdout []byte
	Stderr []byte
}

// SandboxStats 是 M2 给 M9 看板的租户资源统计摘要。
type SandboxStats struct {
	TenantID             int64
	ActiveSandboxCount   int64
	MaxConcurrentSandbox int32
	MaxCPU               int32
	MaxMemoryMB          int32
	IdleTimeoutMin       int32
	MaxLifetimeMin       int32
	MaxKeepaliveMin      int32
	MaxSnapshotRetention int32
}

// SandboxService 是 M2 沙箱引擎对 M3/M7/M8 暴露的生命周期能力。
type SandboxService interface {
	// CreateSandbox 创建沙箱控制面记录并触发编排。
	CreateSandbox(ctx context.Context, req SandboxCreateRequest) (SandboxInfo, error)
	// GetSandbox 查询沙箱摘要。
	GetSandbox(ctx context.Context, sandboxID int64) (SandboxInfo, error)
	// RecycleBySourceRef 按来源标识级联回收沙箱。
	RecycleBySourceRef(ctx context.Context, tenantID int64, sourceRef, reason string) error
	// PutSandboxFile 向沙箱工作区写入文件,供 judge 注入提交与判题套件。
	PutSandboxFile(ctx context.Context, req SandboxFileWrite) error
	// SaveSandboxFiles 持久化沙箱工作区,返回最新代码哈希。
	SaveSandboxFiles(ctx context.Context, sandboxID int64) (string, error)
	// ExecSandboxCommand 在沙箱工作区执行受限命令,供 judge worker 运行判题器。
	ExecSandboxCommand(ctx context.Context, req SandboxExecRequest) (SandboxExecResult, error)
	// ChainDeploy 调用运行时部署能力。
	ChainDeploy(ctx context.Context, sandboxID int64, payload map[string]any) (map[string]any, error)
	// ChainSendTx 调用运行时交易能力。
	ChainSendTx(ctx context.Context, sandboxID int64, payload map[string]any) (map[string]any, error)
	// ChainQuery 调用运行时查询能力。
	ChainQuery(ctx context.Context, sandboxID int64, target string) (map[string]any, error)
	// ChainReset 调用运行时重置能力。
	ChainReset(ctx context.Context, sandboxID int64) error
	// Stats 读取租户沙箱资源统计,供 M9 看板聚合。
	Stats(ctx context.Context, tenantID int64) (SandboxStats, error)
}

// ContentJudgeSpec 是 M5 提供给 M3 的判题配置快照,不暴露给学生前端。
type ContentJudgeSpec struct {
	ItemCode    string
	ItemVersion string
	JudgerCode  string
	MaxScore    int32
	SuiteRef    string
	Expectation map[string]any
	VersionHash string
}

// ContentJudgeService 是 M5 对 M3 暴露的判题配置读取能力。
type ContentJudgeService interface {
	// GetJudgeSpec 按锁定题目版本读取判题配置与答案快照。
	GetJudgeSpec(ctx context.Context, itemCode, itemVersion string) (ContentJudgeSpec, error)
}

// ContentItemRef 是业务模块锁定内容版本时使用的引用。
type ContentItemRef struct {
	ItemCode    string
	ItemVersion string
}

// ContentItemSnapshot 是 M5 给业务模块的题面或全量内容快照。
type ContentItemSnapshot struct {
	ItemCode        string
	ItemVersion     string
	Type            int16
	Title           string
	Difficulty      int16
	Tags            []string
	KnowledgePoints []string
	Body            map[string]any
	VersionHash     string
	Status          int16
}

// ContentReadService 是 M5 对 M6/M7/M8 暴露的内容读取与引用计数能力。
type ContentReadService interface {
	// GetContentFace 按锁定版本读取题面视角内容,敏感字段已剥离。
	GetContentFace(ctx context.Context, tenantID int64, ref ContentItemRef) (ContentItemSnapshot, error)
	// GetContentFull 按锁定版本读取全量内容,仅供内部服务或教师授权路径使用。
	GetContentFull(ctx context.Context, tenantID int64, ref ContentItemRef) (ContentItemSnapshot, error)
	// BatchGetContentFace 批量读取题面内容,用于组卷展开。
	BatchGetContentFace(ctx context.Context, tenantID int64, refs []ContentItemRef) ([]ContentItemSnapshot, error)
	// IncrementContentUsage 记录内容被业务引用,用于删除和复用统计。
	IncrementContentUsage(ctx context.Context, tenantID int64, ref ContentItemRef) error
}

// ContentSystemImportRequest 是系统/外部源固化内容时提交给 M5 的内部请求。
type ContentSystemImportRequest struct {
	TenantID         int64
	Code             string
	Version          string
	Type             int16
	Title            string
	CategoryID       int64
	Difficulty       int16
	Tags             []string
	KnowledgePoints  []string
	AuthorID         int64
	AuthorType       int16
	Visibility       int16
	Body             map[string]any
	SensitiveFields  []string
	AutoPublish      bool
	SystemImportNote map[string]any
}

// ContentImportService 是 M5 对 M8 等内部模块暴露的系统建题能力。
type ContentImportService interface {
	// SystemImportContent 把外部源预验证后的自包含内容固化入 M5。
	SystemImportContent(ctx context.Context, req ContentSystemImportRequest) (ContentItemSnapshot, error)
}

// JudgeSubmitRequest 是业务模块提交一次判题任务的契约。
type JudgeSubmitRequest struct {
	TenantID         int64
	JudgerCode       string
	ItemCode         string
	ItemVersion      string
	CodeStorageKey   string
	CodeHash         string
	SubmitterID      int64
	SourceRef        string
	SandboxMode      string
	TargetSandboxRef string
	ExtraInput       map[string]any
	Priority         int16
}

// JudgeTaskInfo 是 M3 返回给调用方的任务摘要。
type JudgeTaskInfo struct {
	TaskID      int64
	TenantID    int64
	SourceRef   string
	SubmitterID int64
	Status      int16
	Score       int32
	Passed      bool
}

// JudgeService 是 M3 评测引擎对 M6/M7/M8 暴露的判题能力。
type JudgeService interface {
	// SubmitJudgeTask 创建判题任务并入队。
	SubmitJudgeTask(ctx context.Context, req JudgeSubmitRequest) (JudgeTaskInfo, error)
	// GetJudgeTask 查询任务与结果摘要。
	GetJudgeTask(ctx context.Context, taskID int64) (JudgeTaskInfo, error)
	// Rejudge 按原输入快照重新判题。
	Rejudge(ctx context.Context, taskID int64) (JudgeTaskInfo, error)
}

// SimCreateSessionRequest 是业务模块创建仿真会话的契约输入。
type SimCreateSessionRequest struct {
	TenantID       int64
	PackageCode    string
	Version        string
	Seed           int64
	InitParams     map[string]any
	OwnerAccountID int64
	SourceRef      string
}

// SimSessionInfo 是 M4 返回给调用方的会话摘要。
type SimSessionInfo struct {
	SessionID   int64
	TenantID    int64
	PackageCode string
	Version     string
	Compute     string
	BundleRef   string
	SourceRef   string
}

// SimActionInfo 是回放剧本中的单条操作。
type SimActionInfo struct {
	Seq       int32
	AtTick    int32
	EventType string
	Payload   map[string]any
}

// SimReplayInfo 是 M4 提供给业务模块和 M3 输入快照使用的回放数据。
type SimReplayInfo struct {
	PackageCode string
	Version     string
	Seed        int64
	InitParams  map[string]any
	Actions     []SimActionInfo
}

// SimCheckpointRequest 是业务模块上报仿真检查点的契约输入。
type SimCheckpointRequest struct {
	TenantID     int64
	SessionID    int64
	CheckpointID string
	Answer       map[string]any
	Achieved     bool
}

// SimService 是 M4 仿真引擎对 M6/M7/M8 暴露的会话能力。
type SimService interface {
	// CreateSimSession 创建仿真会话并锁定仿真包版本。
	CreateSimSession(ctx context.Context, req SimCreateSessionRequest) (SimSessionInfo, error)
	// GetSimReplay 查询可复现回放数据。
	GetSimReplay(ctx context.Context, tenantID, sessionID int64) (SimReplayInfo, error)
	// ReportSimCheckpoint 保存仿真检查点结果快照。
	ReportSimCheckpoint(ctx context.Context, req SimCheckpointRequest) error
	// RecycleSimBySourceRef 按来源标识归档仿真会话。
	RecycleSimBySourceRef(ctx context.Context, tenantID int64, sourceRef, reason string) error
}
