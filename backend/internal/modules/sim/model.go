// sim model 文件定义 M4 领域模型和审核报告快照,不依赖 HTTP 或 sqlc 生成类型。
package sim

import (
	"encoding/json"
	"time"
)

// Package 是平台级仿真包版本定义。
// Compute 与 Entry 按 AuthorType 派生:平台内置 → 浏览器执行、无入口模块;
// 教师/第三方 → 隔离容器执行、必须有入口模块与运行能力(见 docs/04-仿真可视化引擎/02-架构设计.md §8)。
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
	Enabled         bool     `json:"enabled"`
	Language        string   `json:"language,omitempty"`
	LineCount       int      `json:"line_count,omitempty"`
	MappingCount    int      `json:"mapping_count,omitempty"`
	VariableCount   int      `json:"variable_count,omitempty"`
	ValidationNotes []string `json:"validation_notes,omitempty"`
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

// ValidationReport 保存上架审核所需的静态与隔离预览结论。
//
// 四项门禁各有明确生产者:BundleHash/MetadataValidation/StaticScan 由上传流程写入;
// DeterminismCheck/WorkerPreview/PreviewFrames 由 M4 自己的隔离预览任务写入(见 service_preview.go)。
// PreviewFrames 是容器渲出的样例教学帧 —— 自动校验只能回答"能不能跑、是否确定性",
// 回答不了"这个算法实现对不对",故必须把帧摊给平台管理员看(见 06 流程 §4)。
type ValidationReport struct {
	BundleHash         string           `json:"bundle_hash,omitempty"`
	MetadataValidation ValidationStatus `json:"metadata_validation,omitempty"`
	StaticScan         StaticScanReport `json:"static_scan,omitempty"`
	DeterminismCheck   ValidationStatus `json:"determinism_check,omitempty"`
	WorkerPreview      ValidationStatus `json:"worker_preview,omitempty"`
	PreviewFrames      json.RawMessage  `json:"preview_frames,omitempty"`
}

// ValidationStatus 是动态或静态审核子项的标准化结果。
type ValidationStatus struct {
	Status  string `json:"status,omitempty"`
	Message string `json:"message,omitempty"`
}

// StaticScanReport 描述后端上传时执行的危险调用扫描结果。
//
// 它是**给审核人的信号,不是隔离边界**:正则拦不住 `globalThis['fe'+'tch']` 这类拼接,
// 动态语言下的能力访问无法靠模式匹配穷尽。真正的边界是隔离容器(见 07 安全设计 §3),
// 扫描命中只用于把可疑包提请人工重点查看。
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
//
// 四种受控命令(取值见 enum.go 的 BackendCommandKind):推进一个推演时刻、注入一次包内声明的交互、
// 回退一步、回到初始状态。
// 时刻推进不是交互,不走交互白名单也不进操作记录 —— 它由 seed 决定、可复算,
// 与内置包在浏览器 Worker 里的语义一致(见 docs/04-仿真可视化引擎/05-接口设计.md §4)。
// 回退与重来按 M4 需求 C2「基于确定性重算到上一 tick」实现:容器从初始状态重放到目标位置,
// 不做就地反算。
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
//
// 为什么推完整快照而非仅 State:render 是仿真包自己的函数,扩展包的 render 属外部代码,
// 浏览器不得执行(见 07 安全设计 §2)。容器内完成 reducer + render + 叙事命中 + 检查点求值,
// 前端拿到的已是纯数据帧,交给平台自己的封闭模式渲染器绘制。
type BackendSnapshot struct {
	Tick                    int64          `json:"tick"`
	State                   map[string]any `json:"state"`
	View                    map[string]any `json:"view"`
	CurrentStep             map[string]any `json:"current_step,omitempty"`
	InteractionAvailability map[string]any `json:"interaction_availability,omitempty"`
	CheckpointResults       map[string]any `json:"checkpoint_results,omitempty"`
}

// BackendDescriptor 是隔离容器回传的仿真包自描述信息:操作清单、教学步骤、检查点与代码追踪。
//
// 为什么必须回传:浏览器不执行扩展包代码,拿不到包内声明,而"可用操作"的名称、参数字段、
// 检查点标题和教学步骤总数都是渲染工作台的必要材料 —— 只推快照的话页面能画舞台却画不出操作面板。
// 它与快照一样来自不可信容器,故同样要在后端校验后才转发(见 validateBackendDescriptor)。
type BackendDescriptor struct {
	Meta         map[string]any   `json:"meta"`
	Interactions []map[string]any `json:"interactions"`
	Narrative    []map[string]any `json:"narrative,omitempty"`
	CodeTrace    map[string]any   `json:"codeTrace,omitempty"`
	Checkpoints  []map[string]any `json:"checkpoints,omitempty"`
}

// BackendStreamMessage 是隔离执行 WebSocket 推给浏览器的一帧。
// Descriptor 只随首帧(type=ready)下发一次:它是包的静态声明,不随 tick 变化,
// 每帧重发会把代码追踪源码这类大字段重复推送。帧类型取值见 enum.go。
type BackendStreamMessage struct {
	Type       string             `json:"type"`
	Descriptor *BackendDescriptor `json:"descriptor,omitempty"`
	Snapshot   BackendSnapshot    `json:"snapshot"`
	// EventCount 是当前过程已执行的事件数。过程由服务端持有,浏览器据此判断还能不能回退 ——
	// 它自己数不出来:刷新重连后服务端会按已登记操作把过程重放回来,前端计数会与服务端不一致。
	EventCount int64 `json:"event_count"`
}
