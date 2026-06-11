// sim dto 文件定义 M4 HTTP 请求结构,不承载业务编排逻辑。
package sim

import (
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
	AuthorType     int16           `json:"author_type"`
}

// UpdatePackageRequest 是更新草稿或退回包时提交的新元数据和 bundle。
type UpdatePackageRequest = SubmitPackageRequest

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
	OwnerAccountID int64          `json:"owner_account_id"`
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
