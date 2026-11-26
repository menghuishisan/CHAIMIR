// sim model 文件定义 M4 领域模型和审核报告快照,不依赖 HTTP 或 sqlc 生成类型。
package sim

import (
	"encoding/json"
	"time"
)

// Package 是平台级仿真包版本定义。
// 平台内置包在浏览器运行；教师及第三方包须提供入口模块并在隔离容器运行。
type Package struct {
	ID                  int64
	Code                string
	Version             string
	Name                string
	Category            string
	Compute             int16
	ScaleLimit          map[string]any
	BundleKey           string
	BundleHash          string
	Entry               string
	BackendAdapter      string
	BackendConfig       map[string]any
	InteractionSchema   InteractionSchema
	CodeTrace           CodeTraceAudit
	AuthorType          int16
	AuthorID            int64
	Status              int16
	CreatedAt           time.Time
	UpdatedAt           time.Time
	PreviewReviewID     int64
	PreviewLeaseToken   string
	PreviewAttemptCount int32
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
	ID               int64
	TenantID         int64
	PackageID        int64
	SourceRef        string
	ScopeRef         string
	OwnerAccountID   int64
	SharedAccountIDs []int64
	Seed             int64
	InitParams       map[string]any
	Compute          int16
	Status           int16
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// SessionWithPackage 是回放、分享和隔离执行需要的会话加包摘要。
type SessionWithPackage struct {
	Session
	PackageCode       string
	PackageVersion    string
	PackageName       string
	Category          string
	ScaleLimit        map[string]any
	BundleKey         string
	BundleHash        string
	Entry             string
	BackendAdapter    string
	BackendConfig     map[string]any
	InteractionSchema InteractionSchema
	PackageStatus     int16
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

// InteractionSchema 是后端从 sim-package.json 提取的交互白名单,用于操作上报校验。
type InteractionSchema struct {
	Events map[string]InteractionEventSchema `json:"events"`
}

// InteractionEventSchema 描述单类事件允许的目标策略和 payload 字段。
type InteractionEventSchema struct {
	InteractionID string                      `json:"interaction_id"`
	Kind          string                      `json:"kind"`
	Target        string                      `json:"target"`
	Params        []InteractionParam          `json:"params"`
	ParamIndex    map[string]InteractionParam `json:"-"`
}

// InteractionParam 描述交互参数字段,只保留后端校验所需的最小协议摘要。
type InteractionParam struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Required bool     `json:"required"`
	Min      *float64 `json:"min,omitempty"`
	Max      *float64 `json:"max,omitempty"`
	Options  []string `json:"options,omitempty"`
}

// CodeTraceAudit 保存代码追踪协议审核摘要,源码正文仍只存在 bundle 对象内。
type CodeTraceAudit struct {
	Enabled       bool   `json:"enabled"`
	Language      string `json:"language,omitempty"`
	LineCount     int    `json:"line_count,omitempty"`
	MappingCount  int    `json:"mapping_count,omitempty"`
	VariableCount int    `json:"variable_count,omitempty"`
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

// ValidationReport 保存上架审核所需的静态与隔离预览结论。
// 上传流程写静态结论，预览任务写确定性、执行结果与供人工审核的样例帧。
type ValidationReport struct {
	BundleHash         string           `json:"bundle_hash,omitempty"`
	MetadataValidation ValidationStatus `json:"metadata_validation,omitzero"`
	StaticScan         StaticScanReport `json:"static_scan,omitzero"`
	DeterminismCheck   ValidationStatus `json:"determinism_check,omitzero"`
	WorkerPreview      ValidationStatus `json:"worker_preview,omitzero"`
	PreviewFrames      json.RawMessage  `json:"preview_frames,omitempty"`
}

// ValidationStatus 是动态或静态审核子项的标准化结果。
type ValidationStatus struct {
	Status  string `json:"status,omitempty"`
	Message string `json:"message,omitempty"`
}

// StaticScanReport 描述后端上传时执行的危险调用扫描结果。
// 扫描命中仅供人工审核；不可信代码的安全边界仍是隔离容器。
type StaticScanReport struct {
	Status   string   `json:"status,omitempty"`
	Findings []string `json:"findings,omitempty"`
}

// BackendEvent 是隔离执行 WebSocket 客户端发来的交互事件。
type BackendEvent struct {
	EventType string         `json:"event_type"`
	Payload   map[string]any `json:"payload"`
}

// BackendClientMessage 是浏览器在隔离执行连接上下发的一条消息。
// 时刻推进可由 seed 重算，不写用户操作记录；回退和重来重放到目标位置。
type BackendClientMessage struct {
	Type      string         `json:"type"`
	EventType string         `json:"event_type,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
}

// BackendCommand 是经交互白名单校验后交给适配器执行的受控命令。
type BackendCommand struct {
	Kind  BackendCommandKind
	Event BackendEvent
}

// BackendSnapshot 是隔离容器算出、经后端协议校验后推给前端的完整教学快照。
// 包内 render、叙事和检查点在容器求值，浏览器只渲染纯数据。
type BackendSnapshot struct {
	Tick                    int64          `json:"tick"`
	State                   map[string]any `json:"state"`
	View                    map[string]any `json:"view"`
	CurrentStep             map[string]any `json:"current_step,omitempty"`
	InteractionAvailability map[string]any `json:"interaction_availability"`
	CheckpointResults       map[string]any `json:"checkpoint_results"`
}

// BackendDescriptor 是隔离容器回传的仿真包自描述信息:操作清单、教学步骤、检查点与代码追踪。
// 浏览器不执行包代码，操作清单和教学声明也须经后端校验后下发。
type BackendDescriptor struct {
	Meta         map[string]any   `json:"meta"`
	Interactions []map[string]any `json:"interactions"`
	Narrative    []map[string]any `json:"narrative"`
	CodeTrace    map[string]any   `json:"codeTrace,omitempty"`
	Checkpoints  []map[string]any `json:"checkpoints"`
}

// BackendStreamMessage 是隔离执行 WebSocket 推给浏览器的一帧。
// Descriptor 只随首帧(type=ready)下发一次:它是包的静态声明,不随 tick 变化,
// 每帧重发会把代码追踪源码这类大字段重复推送。帧类型取值见 enum.go。
type BackendStreamMessage struct {
	Type       string             `json:"type"`
	ReadOnly   bool               `json:"read_only"`
	Descriptor *BackendDescriptor `json:"descriptor,omitempty"`
	Snapshot   BackendSnapshot    `json:"snapshot"`
	// EventCount 含重连恢复的事件，供浏览器判断回退位置。
	EventCount int64 `json:"event_count"`
}
