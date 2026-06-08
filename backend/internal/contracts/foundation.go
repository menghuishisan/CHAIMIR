// 第0层 地基:identity(M1)对外【跨模块】接口契约。
// 业务模块经此取用户摘要与角色,不直接访问 account 表。
// 完整的账号/组织/认证管理 API 由 identity 模块自身 HTTP 层暴露,不进 contracts。
package contracts

import (
	"context"
	"time"
)

// AccountInfo 是跨模块传递的账号摘要 DTO(不暴露手机号/密码)。
type AccountInfo struct {
	AccountID    int64
	TenantID     int64
	Name         string
	BaseIdentity int16    // 1学生 / 2教师
	Roles        []string // 角色编码。
	Status       int16
}

// IdentityService 是 identity 模块对外提供的【跨模块】只读身份能力。
type IdentityService interface {
	// GetAccount 取账号摘要。
	GetAccount(ctx context.Context, accountID int64) (AccountInfo, error)
	// BatchGetAccounts 批量取账号摘要(避免 N+1)。
	BatchGetAccounts(ctx context.Context, accountIDs []int64) ([]AccountInfo, error)
	// HasRole 判断账号是否具备某角色。
	HasRole(ctx context.Context, accountID int64, role string) (bool, error)
}

// IdentityStats 是 M1 提供给 M9 看板的账号与租户统计摘要。
type IdentityStats struct {
	TenantCount             int64
	AccountCount            int64
	TeacherCount            int64
	StudentCount            int64
	PendingApplicationCount int64
}

// TenantSummary 是 M9 展示租户列表所需的 M1 租户摘要。
type TenantSummary struct {
	ID                   int64
	Code                 string
	Name                 string
	Type                 int16
	Status               int16
	DeployMode           int16
	LogoURL              string
	DisplayName          string
	AuthMode             int16
	EnableActivationCode bool
	ExpireAt             *time.Time
}

// ApplicationSummary 是 M9 展示入驻申请列表所需的 M1 申请摘要。
type ApplicationSummary struct {
	ID           int64
	SchoolName   string
	SchoolType   int16
	ContactName  string
	ContactPhone string
	ContactEmail string
	Status       int16
	RejectReason string
	CreatedAt    time.Time
}

// ApplicationApproval 是 M9 转发平台入驻审核时提交给 M1 的参数。
type ApplicationApproval struct {
	ApplicationID int64
	ReviewerID    int64
	TenantCode    string
	AdminPhone    string
	AdminName     string
}

// ApplicationApprovalResult 是 M1 完成入驻审核后返回给 M9 的结果摘要。
type ApplicationApprovalResult struct {
	TenantID       int64
	TenantCode     string
	AdminPhone     string
	ActivationCode string
	ActivationHint string
}

// AuditQuery 是 M9 审计中心传给 M1 的过滤条件。
type AuditQuery struct {
	ActorID    int64
	Action     string
	TargetType string
	From       *time.Time
	To         *time.Time
}

// AuditRecord 是 M1 audit_log 的跨模块只读视图。
type AuditRecord struct {
	ID         int64
	TenantID   int64
	ActorID    int64
	ActorRole  int16
	Action     string
	TargetType string
	TargetID   int64
	Detail     map[string]any
	IP         string
	TraceID    string
	CreatedAt  time.Time
}

// IdentityAdminService 是 M1 对 M9 暴露的平台/审计/统计管理契约。
type IdentityAdminService interface {
	// Stats 读取平台级或租户级身份统计。
	Stats(ctx context.Context, tenantID int64) (IdentityStats, error)
	// AdminListTenants 列出租户摘要。
	AdminListTenants(ctx context.Context, status int16, page, size int) ([]TenantSummary, int64, error)
	// AdminListApplications 列出学校入驻申请摘要。
	AdminListApplications(ctx context.Context, status int16, page, size int) ([]ApplicationSummary, int64, error)
	// AdminApproveApplication 转发入驻审核通过动作,业务落在 M1。
	AdminApproveApplication(ctx context.Context, req ApplicationApproval) (ApplicationApprovalResult, error)
	// AdminRejectApplication 转发入驻审核驳回动作,业务落在 M1。
	AdminRejectApplication(ctx context.Context, applicationID, reviewerID int64, reason string) error
	// ListAuditRecords 按权限范围查询审计日志。
	ListAuditRecords(ctx context.Context, query AuditQuery, page, size int) ([]AuditRecord, int64, error)
}
