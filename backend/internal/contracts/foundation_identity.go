// contracts 定义第 0 层身份与租户模块对外暴露的稳定只读契约与共享身份 DTO。
package contracts

import (
	"context"
	"time"
)

// AccountInfo 是跨模块传递的账号摘要,仅保留鉴权、审计和聚合需要的最小字段。
type AccountInfo struct {
	AccountID    int64    `json:"account_id"`
	TenantID     int64    `json:"tenant_id"`
	Name         string   `json:"name"`
	PhoneMasked  string   `json:"phone_masked"`
	No           string   `json:"no"`
	BaseIdentity int16    `json:"base_identity"`
	Roles        []string `json:"roles"`
	Status       int16    `json:"status"`
}

// TenantSummary 是聚合层读取租户信息时使用的稳定摘要。
type TenantSummary struct {
	TenantID   int64      `json:"tenant_id"`
	Code       string     `json:"code"`
	Name       string     `json:"name"`
	Type       int16      `json:"type"`
	Status     int16      `json:"status"`
	DeployMode int16      `json:"deploy_mode"`
	ExpireAt   *time.Time `json:"expire_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// AuditLogEntry 是统一审计查询中心读取的共享审计记录视图。
type AuditLogEntry struct {
	ID         int64     `json:"id"`
	TenantID   int64     `json:"tenant_id"`
	ActorID    int64     `json:"actor_id"`
	ActorRole  int16     `json:"actor_role"`
	Action     string    `json:"action"`
	TargetType string    `json:"target_type"`
	TargetID   int64     `json:"target_id"`
	Detail     string    `json:"detail"`
	IP         string    `json:"ip"`
	TraceID    string    `json:"trace_id"`
	CreatedAt  time.Time `json:"created_at"`
}

// IdentityService 是 identity 模块对外提供的最小只读身份契约。
type IdentityService interface {
	// GetAccount 读取单个账号摘要,供平台鉴权、审计与聚合逻辑复用。
	GetAccount(ctx context.Context, accountID int64) (AccountInfo, error)
	// BatchGetAccounts 批量读取账号摘要,避免上层形成 N+1 查询。
	BatchGetAccounts(ctx context.Context, accountIDs []int64) ([]AccountInfo, error)
	// HasRole 判断账号是否具备指定角色,供平台守卫和聚合入口复用。
	HasRole(ctx context.Context, accountID int64, role string) (bool, error)
	// ListClassStudents 读取指定班级的在校学生摘要,供 M6 按班级批量选课使用。
	// 班级是组织维度,教师可读(`/org/classes` 就在教师组);而账号目录 `/accounts` 是
	// 学校管理员能力,故按班级取学生走本契约,不把账号目录放开给业务模块或教师端。
	ListClassStudents(ctx context.Context, tenantID, classID int64, page, size int) ([]AccountInfo, error)
}

// IdentityTenantReadService 是 M1 对 M9 等聚合层开放的租户只读契约。
type IdentityTenantReadService interface {
	// ListTenants 读取租户列表,供平台看板聚合使用。
	ListTenants(ctx context.Context) ([]TenantSummary, error)
	// GetTenant 读取单个租户摘要,供聚合层按租户投影使用。
	GetTenant(ctx context.Context, tenantID int64) (TenantSummary, error)
}

// IdentityStats 是 M1 提供给 M9 看板的只读统计摘要。
type IdentityStats struct {
	TenantID             int64 `json:"tenant_id"`
	TenantCount          int64 `json:"tenant_count"`
	AccountCount         int64 `json:"account_count"`
	TeacherCount         int64 `json:"teacher_count"`
	StudentCount         int64 `json:"student_count"`
	SchoolAdminCount     int64 `json:"school_admin_count"`
	PlatformAdminCount   int64 `json:"platform_admin_count"`
	ActiveAccountCount   int64 `json:"active_account_count"`
	ActiveTenantCount    int64 `json:"active_tenant_count"`
	PendingApplyCount    int64 `json:"pending_apply_count"`
	DisabledAccountCount int64 `json:"disabled_account_count"`
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
	TenantID        int64     `json:"tenant_id"`
	ActorID         int64     `json:"actor_id"`
	Action          string    `json:"action"`
	TargetType      string    `json:"target_type"`
	From            time.Time `json:"from"`
	To              time.Time `json:"to"`
	Page            int32     `json:"page"`
	Size            int32     `json:"size"`
	IncludePlatform bool      `json:"include_platform"`
}

// AuditQueryResult 是审计查询中心统一返回的分页结果。
type AuditQueryResult struct {
	List  []AuditLogEntry `json:"list"`
	Total int64           `json:"total"`
	Page  int32           `json:"page"`
	Size  int32           `json:"size"`
}

// IdentityAuditReadService 是 M1 对 M9 审计中心开放的审计只读契约。
type IdentityAuditReadService interface {
	// QueryAuditLogs 按条件分页查询共享 audit_log,遵循调用侧权限范围。
	QueryAuditLogs(ctx context.Context, query AuditQuery) (AuditQueryResult, error)
}
