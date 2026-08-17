// sim dto 文件定义 M4 HTTP 请求结构,不承载业务编排逻辑。
package sim

import (
	"chaimir/internal/platform/ids"
	"encoding/json"
	"time"
)

// SubmitPackageRequest 是教师或第三方提交仿真包时的元数据。
//
// 刻意不含 compute 与 backend_adapter:执行位置按 author_type 派生(教师/第三方恒为隔离容器,
// 见 docs/04-仿真可视化引擎/02-架构设计.md §8),让客户端声明它只会产生
// "提交了一个平台没有任何路径能运行的包"这类无效状态。
type SubmitPackageRequest struct {
	Code       string          `json:"code"`
	Version    string          `json:"version"`
	Name       string          `json:"name"`
	Category   string          `json:"category"`
	ScaleLimit json.RawMessage `json:"scale_limit"`
}

// RejectReviewRequest 是平台管理员退回审核的意见。
type RejectReviewRequest struct {
	Comment string `json:"comment"`
}

// CreateSessionRequest 是内部服务创建仿真会话的 HTTP 请求。
type CreateSessionRequest struct {
	PackageCode    string         `json:"package_code"`
	Version        string         `json:"version"`
	Seed           int64          `json:"seed"`
	InitParams     map[string]any `json:"init_params"`
	OwnerAccountID ids.ID         `json:"owner_account_id"`
	SourceRef      string         `json:"source_ref"`
}

// ReportActionRequest 是前端异步上报的确定性操作记录。
type ReportActionRequest struct {
	Seq       int32          `json:"seq"`
	AtTick    int32          `json:"at_tick"`
	EventType string         `json:"event_type"`
	Payload   map[string]any `json:"payload"`
}

// ReportCheckpointRequest 是内部服务上报的检查点快照。
type ReportCheckpointRequest struct {
	CheckpointID string          `json:"checkpoint_id"`
	Answer       json.RawMessage `json:"answer"`
	Achieved     bool            `json:"achieved"`
}

// RecycleRequest 是内部服务按来源归档会话的请求。
type RecycleRequest struct {
	SourceRef string `json:"source_ref"`
	Reason    string `json:"reason"`
}

// CreateShareRequest 是用户创建分享码时提交的可选过期时间。
type CreateShareRequest struct {
	ExpireAt time.Time `json:"expire_at"`
}

// BackendAdapterDescriptor 描述当前部署已装配的隔离执行能力。
// 它只在启动装配时用于核对能力目录与适配器实现一致,不对外暴露 —— 执行位置与运行能力
// 都由服务端按作者类型派生,客户端没有可选项,也就没有能力列表接口。
type BackendAdapterDescriptor struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Protocol    string `json:"protocol"`
	Description string `json:"description"`
}

// SimScaleLimitResponse 是仿真包公开的固定规模上限。
//
// 这三个字段由 SDK manifest 协议固定,不能用无类型对象绕过 API 契约。
type SimScaleLimitResponse struct {
	Nodes     int `json:"nodes"`
	MaxTick   int `json:"max_tick"`
	MaxEvents int `json:"max_events"`
}

// SimPackageResponse 是仿真包元数据的公开 HTTP 响应。
//
// 后端适配器、执行配置、归档对象引用和入口模块都仅服务端可见,不在本 DTO 中定义。
type SimPackageResponse struct {
	ID         ids.ID                `json:"id"`
	Code       string                `json:"code"`
	Version    string                `json:"version"`
	Name       string                `json:"name"`
	Category   string                `json:"category"`
	Compute    string                `json:"compute"`
	ScaleLimit SimScaleLimitResponse `json:"scale_limit"`
	BundleHash string                `json:"bundle_hash,omitempty"`
	Status     string                `json:"status"`
	CreatedAt  time.Time             `json:"created_at"`
	UpdatedAt  time.Time             `json:"updated_at"`
}

// SimValidationStatusResponse 是一项审核门禁的公开结论。
type SimValidationStatusResponse struct {
	Status  string `json:"status,omitempty"`
	Message string `json:"message,omitempty"`
}

// SimStaticScanReportResponse 是静态扫描给审核人的只读结论。
type SimStaticScanReportResponse struct {
	Status   string   `json:"status,omitempty"`
	Findings []string `json:"findings,omitempty"`
}

// SimTeachingSnapshotResponse 是隔离执行输出的教学快照。
// state、view 和检查点结果属于包定义的动态教学状态,保留对象边界。
type SimTeachingSnapshotResponse struct {
	Tick                    int64          `json:"tick"`
	State                   map[string]any `json:"state"`
	View                    map[string]any `json:"view"`
	CurrentStep             map[string]any `json:"current_step,omitempty"`
	InteractionAvailability map[string]any `json:"interaction_availability,omitempty"`
	CheckpointResults       map[string]any `json:"checkpoint_results,omitempty"`
}

// SimValidationReportResponse 是审核报告的公开投影。
type SimValidationReportResponse struct {
	BundleHash         string                        `json:"bundle_hash,omitempty"`
	MetadataValidation SimValidationStatusResponse   `json:"metadata_validation,omitempty"`
	StaticScan         SimStaticScanReportResponse   `json:"static_scan,omitempty"`
	DeterminismCheck   SimValidationStatusResponse   `json:"determinism_check,omitempty"`
	WorkerPreview      SimValidationStatusResponse   `json:"worker_preview,omitempty"`
	PreviewFrames      []SimTeachingSnapshotResponse `json:"preview_frames,omitempty"`
}

// SimReviewPackageResponse 是审核记录内嵌的包摘要。
type SimReviewPackageResponse struct {
	Code     string `json:"code"`
	Version  string `json:"version"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Compute  string `json:"compute"`
	Status   string `json:"status"`
}

// SimPackageReviewResponse 是审核记录的公开 HTTP 响应。
type SimPackageReviewResponse struct {
	ID            ids.ID                      `json:"id"`
	PackageID     ids.ID                      `json:"package_id"`
	SubmitterID   ids.ID                      `json:"submitter_id"`
	PreviewReport SimValidationReportResponse `json:"preview_report"`
	ReviewerID    ids.ID                      `json:"reviewer_id,omitempty"`
	Result        string                      `json:"result"`
	Comment       string                      `json:"comment,omitempty"`
	CreatedAt     time.Time                   `json:"created_at"`
	UpdatedAt     time.Time                   `json:"updated_at,omitempty"`
	Package       *SimReviewPackageResponse   `json:"package,omitempty"`
}

// SimPackageSubmissionResponse 是教师提交或更新包后返回的包与审核记录。
type SimPackageSubmissionResponse struct {
	SimPackageResponse
	Review SimPackageReviewResponse `json:"review"`
}

// SimPackagePreviewResponse 是作者预览自己的审核报告时的响应。
type SimPackagePreviewResponse struct {
	Package SimPackageResponse       `json:"package"`
	Review  SimPackageReviewResponse `json:"review"`
}

// SimReviewDecisionResponse 是平台管理员审核决策后的响应。
type SimReviewDecisionResponse struct {
	Package SimPackageResponse       `json:"package"`
	Review  SimPackageReviewResponse `json:"review"`
}

// SimSessionCreateResponse 是内部服务创建会话后的公开 HTTP 响应。
type SimSessionCreateResponse struct {
	SessionID ids.ID `json:"session_id"`
	Compute   string `json:"compute"`
}

// SimActionResponse 是一条已持久化的确定性用户操作。
type SimActionResponse struct {
	Seq       int32          `json:"seq"`
	AtTick    int32          `json:"at_tick"`
	EventType string         `json:"event_type"`
	Payload   map[string]any `json:"payload"`
	CreatedAt time.Time      `json:"created_at,omitempty"`
}

// SimReplayResponse 是可复现回放的公开 HTTP 响应。
type SimReplayResponse struct {
	PackageCode string              `json:"package_code"`
	Version     string              `json:"version"`
	Seed        int64               `json:"seed"`
	InitParams  map[string]any      `json:"init_params"`
	Actions     []SimActionResponse `json:"actions"`
}

// SimShareResponse 是创建分享码后的公开 HTTP 响应。
type SimShareResponse struct {
	Code     string    `json:"code"`
	ExpireAt time.Time `json:"expire_at"`
	Status   string    `json:"status"`
}
