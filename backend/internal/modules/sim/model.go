// sim model 文件定义 M4 领域模型和审核报告快照,不依赖 HTTP 或 sqlc 生成类型。
package sim

import (
	"encoding/json"
	"time"
)

// Package 是平台级仿真包版本定义。
type Package struct {
	ID             int64
	Code           string
	Version        string
	Name           string
	Category       string
	Compute        int16
	ScaleLimit     map[string]any
	BundleKey      string
	BundleHash     string
	BackendAdapter string
	BackendConfig  map[string]any
	AuthorType     int16
	AuthorID       int64
	Status         int16
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Review 是仿真包接入审核记录。
type Review struct {
	ID            int64
	PackageID     int64
	SubmitterID   int64
	PreviewReport ValidationReport
	ReviewerID    int64
	Result        int16
	Comment       string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ReviewInfo 是审核列表中附带包摘要的只读投影。
type ReviewInfo struct {
	Review
	PackageCode    string
	PackageVersion string
	PackageName    string
	Category       string
	Compute        int16
	PackageStatus  int16
}

// Session 是一次仿真运行会话。
type Session struct {
	ID             int64
	TenantID       int64
	PackageID      int64
	SourceRef      string
	OwnerAccountID int64
	Seed           int64
	InitParams     map[string]any
	Compute        int16
	Status         int16
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// SessionWithPackage 是回放、分享和后端计算需要的会话加包摘要。
type SessionWithPackage struct {
	Session
	PackageCode    string
	PackageVersion string
	PackageName    string
	Category       string
	BundleKey      string
	BundleHash     string
	BackendAdapter string
	BackendConfig  map[string]any
	PackageStatus  int16
}

// Action 是仿真会话的确定性操作序列项。
type Action struct {
	ID        int64
	TenantID  int64
	SessionID int64
	Seq       int32
	AtTick    int32
	EventType string
	Payload   map[string]any
	CreatedAt time.Time
}

// Checkpoint 是叙事设问或目标达成结果快照。
type Checkpoint struct {
	ID           int64
	TenantID     int64
	SessionID    int64
	CheckpointID string
	Answer       json.RawMessage
	Achieved     bool
	CreatedAt    time.Time
}

// Share 是公开分享码全局索引,正文仍由租户会话与操作序列重建。
type Share struct {
	ID        int64
	TenantID  int64
	SessionID int64
	Code      string
	CreatedBy int64
	Status    int16
	ExpireAt  time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ValidationReport 保存上架审核所需的后端静态与受控预览结论。
type ValidationReport struct {
	BundleHash         string            `json:"bundle_hash,omitempty"`
	MetadataValidation ValidationStatus  `json:"metadata_validation,omitempty"`
	StaticScan         StaticScanReport  `json:"static_scan,omitempty"`
	DeterminismCheck   ValidationStatus  `json:"determinism_check,omitempty"`
	WorkerPreview      ValidationStatus  `json:"worker_preview,omitempty"`
	Details            map[string]string `json:"details,omitempty"`
}

// ValidationStatus 是动态或静态审核子项的标准化结果。
type ValidationStatus struct {
	Status  string `json:"status,omitempty"`
	Message string `json:"message,omitempty"`
}

// StaticScanReport 描述后端上传时执行的危险调用扫描结果。
type StaticScanReport struct {
	Status   string   `json:"status,omitempty"`
	Findings []string `json:"findings,omitempty"`
}

// BackendEvent 是 compute=backend WebSocket 客户端发来的事件。
type BackendEvent struct {
	EventType string         `json:"event_type"`
	Payload   map[string]any `json:"payload"`
}

// BackendState 是 compute=backend WebSocket 推给前端的状态。
type BackendState struct {
	Tick  int64          `json:"tick"`
	State map[string]any `json:"state"`
}
