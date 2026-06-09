// contracts 定义第 1 层仿真引擎对外暴露的会话、回放与检查点契约。
package contracts

import "context"

// SimCreateSessionRequest 是业务模块创建仿真会话时提交的稳定请求。
type SimCreateSessionRequest struct {
	TenantID       int64
	PackageCode    string
	Version        string
	Seed           int64
	InitParams     map[string]any
	OwnerAccountID int64
	SourceRef      string
}

// SimSessionInfo 是跨模块传递的仿真会话摘要。
type SimSessionInfo struct {
	SessionID   int64
	TenantID    int64
	PackageCode string
	Version     string
	Compute     string
	BundleRef   string
	SourceRef   string
}

// SimActionInfo 是回放剧本中的单条用户操作。
type SimActionInfo struct {
	Seq       int32
	AtTick    int32
	EventType string
	Payload   map[string]any
}

// SimReplayInfo 是仿真回放所需的完整数据。
type SimReplayInfo struct {
	PackageCode string
	Version     string
	Seed        int64
	InitParams  map[string]any
	Actions     []SimActionInfo
}

// SimCheckpointRequest 是业务模块上报仿真检查点结果的内部请求。
type SimCheckpointRequest struct {
	TenantID     int64
	SessionID    int64
	CheckpointID string
	Answer       string
	Achieved     bool
}

// SimRecycleRequest 是按来源回收仿真会话的内部请求。
type SimRecycleRequest struct {
	TenantID  int64
	SourceRef string
	Reason    string
}

// SimService 是 M4 仿真引擎对 M6/M7 暴露的会话能力契约。
type SimService interface {
	// CreateSession 创建仿真会话并锁定仿真包版本。
	CreateSession(ctx context.Context, req SimCreateSessionRequest) (SimSessionInfo, error)
	// GetReplay 返回可复现的 seed、参数与操作序列。
	GetReplay(ctx context.Context, tenantID, sessionID int64) (SimReplayInfo, error)
	// ReportCheckpoint 保存仿真检查点结果快照,供 M3 后续判分读取。
	ReportCheckpoint(ctx context.Context, req SimCheckpointRequest) error
	// DestroySession 回收单个仿真会话,供来源模块显式关闭实例时释放资源。
	DestroySession(ctx context.Context, tenantID, sessionID int64) error
	// RecycleBySourceRef 按来源标识归档仿真会话并释放后端计算资源。
	RecycleBySourceRef(ctx context.Context, req SimRecycleRequest) error
}
