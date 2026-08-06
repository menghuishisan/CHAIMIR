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
