// M4 领域模型:定义 service 编排仿真包、审核和会话所需的最小业务投影。
package sim

import "time"

// PackageSnapshot 承载仿真包权限、生命周期和响应转换需要的包版本信息。
type PackageSnapshot struct {
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
	HasAuthorID    bool
	Status         int16
}

// ReviewSnapshot 承载审核规则判断、审核响应和审计记录需要的审核信息。
type ReviewSnapshot struct {
	ID            int64
	PackageID     int64
	SubmitterID   int64
	PreviewReport map[string]any
	ReviewerID    int64
	HasReviewerID bool
	Result        int16
	Comment       string
	HasComment    bool
}

// SessionSnapshot 承载仿真会话归属、状态和创建响应需要的会话信息。
type SessionSnapshot struct {
	ID             int64
	TenantID       int64
	PackageID      int64
	SourceRef      string
	OwnerAccountID int64
	Seed           int64
	InitParams     map[string]any
	Compute        int16
	Status         int16
}

// ActionSnapshot 承载操作序列幂等校验和回放响应需要的操作信息。
type ActionSnapshot struct {
	Seq       int32
	AtTick    int32
	EventType string
	Payload   map[string]any
	CreatedAt time.Time
}

// ReplaySnapshot 承载回放重建需要的会话和包版本信息。
type ReplaySnapshot struct {
	ID                 int64
	TenantID           int64
	PackageID          int64
	SourceRef          string
	OwnerAccountID     int64
	Seed               int64
	InitParams         map[string]any
	Compute            int16
	Status             int16
	PackageCode        string
	PackageVersion     string
	PackageBundleKey   string
	PackageBundleHash  string
	PackageBackend     string
	PackageBackendConf map[string]any
	HasPackageBackend  bool
}

// ShareSnapshot 承载公开分享码解析后回到租户 RLS 的指针。
type ShareSnapshot struct {
	ID        int64
	TenantID  int64
	SessionID int64
	Code      string
}

// BackendSessionSnapshot 承载 compute=backend 会话的适配器和配置。
type BackendSessionSnapshot struct {
	ID             int64
	OwnerAccountID int64
	Compute        int16
	BackendAdapter string
	BackendConfig  map[string]any
	HasBackend     bool
}
