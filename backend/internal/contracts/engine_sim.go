// contracts 定义第 1 层仿真引擎对外暴露的会话、回放与检查点契约。
package contracts

import (
	"context"
	"encoding/json"
)

// SimCreateSessionRequest 是业务模块创建仿真会话时提交的稳定请求。
// SourceRef 必须使用全局四段规范,实验实例必须传 experiment:<year>:instance:<id>,不得使用 exp 等短前缀别名。
type SimCreateSessionRequest struct {
	TenantID       int64          `json:"tenant_id"`
	PackageCode    string         `json:"package_code"`
	Version        string         `json:"version"`
	Seed           int64          `json:"seed"`
	InitParams     map[string]any `json:"init_params"`
	OwnerAccountID int64          `json:"owner_account_id"`
	SourceRef      string         `json:"source_ref"`
}

// SimSessionInfo 是跨模块传递的仿真会话摘要。
type SimSessionInfo struct {
	SessionID   int64  `json:"session_id"`
	TenantID    int64  `json:"tenant_id"`
	PackageCode string `json:"package_code"`
	Version     string `json:"version"`
	Compute     string `json:"compute"`
	BundleRef   string `json:"bundle_ref"`
	SourceRef   string `json:"source_ref"`
}

// SimActionInfo 是回放剧本中的单条用户操作。
type SimActionInfo struct {
	Seq       int32          `json:"seq"`
	AtTick    int32          `json:"at_tick"`
	EventType string         `json:"event_type"`
	Payload   map[string]any `json:"payload"`
}

// SimReplayInfo 是仿真回放所需的完整数据。
type SimReplayInfo struct {
	PackageCode string          `json:"package_code"`
	Version     string          `json:"version"`
	Seed        int64           `json:"seed"`
	InitParams  map[string]any  `json:"init_params"`
	Actions     []SimActionInfo `json:"actions"`
}

// SimCheckpointRequest 是业务模块上报仿真检查点结果的内部请求。
type SimCheckpointRequest struct {
	TenantID     int64           `json:"tenant_id"`
	SessionID    int64           `json:"session_id"`
	SourceRef    string          `json:"source_ref"`
	CheckpointID string          `json:"checkpoint_id"`
	Answer       json.RawMessage `json:"answer"`
	Achieved     bool            `json:"achieved"`
}

// SimDestroySessionRequest 是业务模块按来源回收单个仿真会话的内部请求。
type SimDestroySessionRequest struct {
	TenantID  int64  `json:"tenant_id"`
	SessionID int64  `json:"session_id"`
	SourceRef string `json:"source_ref"`
}

// SimRecycleRequest 是按来源回收仿真会话的内部请求。
type SimRecycleRequest struct {
	TenantID  int64  `json:"tenant_id"`
	SourceRef string `json:"source_ref"`
	Reason    string `json:"reason"`
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
	DestroySession(ctx context.Context, req SimDestroySessionRequest) error
	// RecycleBySourceRef 按来源标识归档仿真会话并释放后端计算资源。
	RecycleBySourceRef(ctx context.Context, req SimRecycleRequest) error
}
