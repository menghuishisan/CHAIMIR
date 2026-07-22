// sim dto 文件定义 M4 HTTP 请求结构,不承载业务编排逻辑。
package sim

import (
	"chaimir/internal/platform/ids"
	"encoding/json"
	"time"
)

// SubmitPackageRequest 是教师或第三方提交仿真包时的元数据。
type SubmitPackageRequest struct {
	Code           string          `json:"code"`
	Version        string          `json:"version"`
	Name           string          `json:"name"`
	Category       string          `json:"category"`
	Compute        string          `json:"compute"`
	ScaleLimit     json.RawMessage `json:"scale_limit"`
	BackendAdapter string          `json:"backend_adapter"`
	BackendConfig  json.RawMessage `json:"backend_config"`
}

// ValidationReportRequest 是受控预览流程回写的动态校验结果。
type ValidationReportRequest struct {
	DeterminismCheck ValidationStatus  `json:"determinism_check"`
	WorkerPreview    ValidationStatus  `json:"worker_preview"`
	Details          map[string]string `json:"details"`
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

// BundleDownloadGrantDTO 是仿真包短时下载授权响应。
type BundleDownloadGrantDTO struct {
	Token       string `json:"token,omitempty"`
	BundleHash  string `json:"bundle_hash"`
	ExpiresAt   string `json:"expires_at"`
	ModuleURL   string `json:"module_url,omitempty"`
	BuiltinCode string `json:"builtin_code,omitempty"`
}

// BackendAdapterDescriptor 描述当前部署已装配的后端计算能力。
type BackendAdapterDescriptor struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Protocol    string `json:"protocol"`
	Description string `json:"description"`
}

// BackendCapabilitiesDTO 是教师端选择计算方式时的权威能力响应。
type BackendCapabilitiesDTO struct {
	BackendCompute bool                       `json:"backend_compute"`
	Adapters       []BackendAdapterDescriptor `json:"adapters"`
}
