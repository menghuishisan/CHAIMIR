// contracts 定义第 0 层身份与租户模块对外暴露的稳定只读契约与共享身份 DTO。
package contracts

import (
	"context"
	"time"
)

// AccountInfo 是跨模块传递的账号摘要,仅保留鉴权、审计和聚合需要的最小字段。
type AccountInfo struct {
	AccountID    int64
	TenantID     int64
	Name         string
	PhoneMasked  string
	No           string
	BaseIdentity int16
	Roles        []string
	Status       int16
}

// TenantSummary 是聚合层读取租户信息时使用的稳定摘要。
type TenantSummary struct {
	TenantID   int64
	Code       string
	Name       string
	Type       int16
	Status     int16
	DeployMode int16
	ExpireAt   *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// TenantApplicationSummary 是平台审核入口使用的入驻申请摘要。
type TenantApplicationSummary struct {
	ApplicationID int64
	SchoolName    string
	SchoolType    int16
	ContactName   string
	ContactPhone  string
	ContactEmail  string
	Status        int16
	SubmittedAt   time.Time
	ReviewedAt    *time.Time
}

// TenantApplicationQuery 是平台审核入口读取申请列表时使用的过滤条件。
type TenantApplicationQuery struct {
	Status int16
}

// AuditLogEntry 是统一审计查询中心读取的共享审计记录视图。
type AuditLogEntry struct {
	ID         int64
	TenantID   int64
	ActorID    int64
	ActorRole  int16
	Action     string
	TargetType string
	TargetID   int64
	Detail     string
	IP         string
	TraceID    string
	CreatedAt  time.Time
}

// IdentityService 是 identity 模块对外提供的最小只读身份契约。
type IdentityService interface {
	// GetAccount 读取单个账号摘要,供平台鉴权、审计与聚合逻辑复用。
	GetAccount(ctx context.Context, accountID int64) (AccountInfo, error)
	// BatchGetAccounts 批量读取账号摘要,避免上层形成 N+1 查询。
	BatchGetAccounts(ctx context.Context, accountIDs []int64) ([]AccountInfo, error)
	// HasRole 判断账号是否具备指定角色,供平台守卫和聚合入口复用。
	HasRole(ctx context.Context, accountID int64, role string) (bool, error)
}

// IdentityTenantReadService 是 M1 对 M9 等聚合层开放的租户与申请只读契约。
type IdentityTenantReadService interface {
	// ListTenants 读取租户列表,供平台管理入口与平台看板聚合使用。
	ListTenants(ctx context.Context) ([]TenantSummary, error)
	// GetTenant 读取单个租户摘要,供平台管理页详情使用。
	GetTenant(ctx context.Context, tenantID int64) (TenantSummary, error)
	// ListTenantApplications 按过滤条件读取入驻申请列表,供平台审核入口展示。
	ListTenantApplications(ctx context.Context, query TenantApplicationQuery) ([]TenantApplicationSummary, error)
}

// IdentityStats 是 M1 提供给 M9 看板的只读统计摘要。
type IdentityStats struct {
	TenantID             int64
	TenantCount          int64
	AccountCount         int64
	TeacherCount         int64
	StudentCount         int64
	SchoolAdminCount     int64
	PlatformAdminCount   int64
	ActiveAccountCount   int64
	ActiveTenantCount    int64
	PendingApplyCount    int64
	DisabledAccountCount int64
}

// IdentityStatsService 是 M1 对 M9 看板开放的统计只读契约。
type IdentityStatsService interface {
	// PlatformStats 返回全平台身份与租户统计,仅供平台级聚合看板读取。
	PlatformStats(ctx context.Context) (IdentityStats, error)
	// TenantStats 返回单租户身份统计,供学校级运营看板读取。
	TenantStats(ctx context.Context, tenantID int64) (IdentityStats, error)
}

// AuditQuery 是统一审计查询中心使用的只读过滤条件。
type AuditQuery struct {
	TenantID        int64
	ActorID         int64
	Action          string
	TargetType      string
	From            time.Time
	To              time.Time
	Page            int32
	Size            int32
	IncludePlatform bool
}

// AuditQueryResult 是审计查询中心统一返回的分页结果。
type AuditQueryResult struct {
	List  []AuditLogEntry
	Total int64
	Page  int32
	Size  int32
}

// IdentityAuditReadService 是 M1 对 M9 审计中心开放的审计只读契约。
type IdentityAuditReadService interface {
	// QueryAuditLogs 按条件分页查询共享 audit_log,遵循调用侧权限范围。
	QueryAuditLogs(ctx context.Context, query AuditQuery) (AuditQueryResult, error)
}
