// M4 DTO 定义:承载仿真包、会话、操作、分享与后端计算的请求响应结构。
package sim

import "time"

// SubmitPackageRequest 是提交仿真包的请求。
type SubmitPackageRequest struct {
	Code           string         `json:"code"`
	Version        string         `json:"version"`
	Name           string         `json:"name"`
	Category       string         `json:"category"`
	Compute        string         `json:"compute"`
	ScaleLimit     map[string]any `json:"scale_limit"`
	BundleKey      string         `json:"bundle_key"`
	BundleHash     string         `json:"bundle_hash"`
	BackendAdapter string         `json:"backend_adapter"`
	BackendConfig  map[string]any `json:"backend_config"`
	AuthorType     int16          `json:"author_type"`
	AuthorID       string         `json:"author_id"`
}

// UpdatePackageRequest 是修改草稿或退回仿真包的请求。
type UpdatePackageRequest struct {
	Name           string         `json:"name"`
	Category       string         `json:"category"`
	ScaleLimit     map[string]any `json:"scale_limit"`
	BundleKey      string         `json:"bundle_key"`
	BundleHash     string         `json:"bundle_hash"`
	BackendAdapter string         `json:"backend_adapter"`
	BackendConfig  map[string]any `json:"backend_config"`
}

// PackageDTO 是对外返回的仿真包摘要。
type PackageDTO struct {
	ID             string         `json:"id"`
	Code           string         `json:"code"`
	Version        string         `json:"version"`
	Name           string         `json:"name"`
	Category       string         `json:"category"`
	Compute        string         `json:"compute"`
	ScaleLimit     map[string]any `json:"scale_limit"`
	BundleHash     string         `json:"bundle_hash"`
	BackendAdapter string         `json:"backend_adapter,omitempty"`
	Status         int16          `json:"status"`
}

// ReviewDTO 是仿真包审核记录摘要。
type ReviewDTO struct {
	ID            string         `json:"id"`
	PackageID     string         `json:"package_id"`
	SubmitterID   string         `json:"submitter_id"`
	PreviewReport map[string]any `json:"preview_report"`
	ReviewerID    string         `json:"reviewer_id,omitempty"`
	Result        int16          `json:"result"`
	Comment       string         `json:"comment,omitempty"`
}

// ReviewRequest 是平台管理员处理审核的请求。
type ReviewRequest struct {
	Comment string `json:"comment"`
}

// ValidationReportRequest 是受控预览流程回写的审核校验报告。
type ValidationReportRequest struct {
	Report map[string]any `json:"report"`
}

// CreateSessionRequest 是创建仿真会话的请求。
type CreateSessionRequest struct {
	PackageCode    string         `json:"package_code"`
	Version        string         `json:"version"`
	Seed           int64          `json:"seed"`
	InitParams     map[string]any `json:"init_params"`
	OwnerAccountID string         `json:"owner_account_id"`
	SourceRef      string         `json:"source_ref"`
}

// SessionDTO 是仿真会话摘要。
type SessionDTO struct {
	SessionID   string         `json:"session_id"`
	Compute     string         `json:"compute"`
	BundleRef   string         `json:"bundle_ref"`
	PackageCode string         `json:"package_code"`
	Version     string         `json:"version"`
	Seed        int64          `json:"seed"`
	InitParams  map[string]any `json:"init_params"`
	Status      int16          `json:"status"`
}

// ReportActionRequest 是前端上报用户操作的请求。
type ReportActionRequest struct {
	Seq       int32          `json:"seq"`
	AtTick    int32          `json:"at_tick"`
	EventType string         `json:"event_type"`
	Payload   map[string]any `json:"payload"`
}

// ActionDTO 是回放中的单条操作。
type ActionDTO struct {
	Seq       int32          `json:"seq"`
	AtTick    int32          `json:"at_tick"`
	EventType string         `json:"event_type"`
	Payload   map[string]any `json:"payload"`
	CreatedAt time.Time      `json:"created_at,omitempty"`
}

// ReplayDTO 是回放和分享剧本的统一响应。
type ReplayDTO struct {
	PackageCode string         `json:"package_code"`
	Version     string         `json:"version"`
	Seed        int64          `json:"seed"`
	InitParams  map[string]any `json:"init_params"`
	Actions     []ActionDTO    `json:"actions"`
}

// ReportCheckpointRequest 是检查点上报请求。
type ReportCheckpointRequest struct {
	CheckpointID string         `json:"checkpoint_id"`
	Answer       map[string]any `json:"answer"`
	Achieved     bool           `json:"achieved"`
}

// ShareDTO 是分享码创建响应。
type ShareDTO struct {
	Code string `json:"code"`
}
