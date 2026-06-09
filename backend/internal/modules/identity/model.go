// M1 领域投影:承接 repo 到 service 的内部数据结构,避免 sqlc 行类型扩散到业务层。
package identity

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// AccountInfoSnapshot 是跨模块账号摘要读取使用的领域投影。
type AccountInfoSnapshot struct {
	AccountID    int64
	TenantID     int64
	Name         string
	BaseIdentity int16
	Roles        []string
	Status       int16
}

// AccountViewSnapshot 是账号列表视图组装使用的内部投影。
type AccountViewSnapshot struct {
	ID             int64
	Name           string
	PhoneEnc       []byte
	BaseIdentity   int16
	Status         int16
	Roles          []string
	No             string
	OrgID          int64
	EnrollmentYear *int16
	Title          string
	MustChangePwd  bool
}

// AccountMutationSnapshot 是账号写操作前校验状态和角色时使用的投影。
type AccountMutationSnapshot struct {
	ID             int64
	TenantID       int64
	PhoneEnc       []byte
	PasswordHash   string
	HasPassword    bool
	Name           string
	BaseIdentity   int16
	Status         int16
	MustChangePwd  bool
	Roles          []string
	No             string
	OrgID          int64
	EnrollmentYear *int16
	Title          string
}

// SessionSnapshot 是个人中心会话列表使用的投影。
type SessionSnapshot struct {
	ID         int64
	DeviceInfo string
	IP         string
	ExpireAt   time.Time
	CreatedAt  time.Time
}

// ImportBatchSnapshot 是导入批次列表使用的投影。
type ImportBatchSnapshot struct {
	ID         int64
	OperatorID int64
	TargetType int16
	FileName   string
	Total      int32
	Success    int32
	Failed     int32
	Status     int16
	CreatedAt  time.Time
}

// ImportPreviewSnapshot 是提交阶段读取的服务端预览投影。
type ImportPreviewSnapshot struct {
	ID         int64
	TargetType int16
	FileName   string
	Rows       []byte
}

// ImportAccountCreate 是导入提交阶段写账号所需的已处理数据。
type ImportAccountCreate struct {
	AccountID      int64
	PhoneEnc       []byte
	PhoneHash      string
	PasswordHash   pgtype.Text
	Name           string
	BaseIdentity   int16
	MustChangePwd  bool
	No             string
	OrgID          int64
	EnrollmentYear int16
	Title          string
	Role           int16
	ActivationHash string
	ActivationAt   time.Time
	HasActivation  bool
}

// IdentityStatsSnapshot 是身份统计的内部投影。
type IdentityStatsSnapshot struct {
	TenantCount             int64
	AccountCount            int64
	TeacherCount            int64
	StudentCount            int64
	PendingApplicationCount int64
}

// AuditLogSnapshot 是审计查询输出的内部投影。
type AuditLogSnapshot struct {
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

// SmsCodeSnapshot 是验证码校验流程使用的内部投影。
type SmsCodeSnapshot struct {
	ID       int64
	CodeHash string
	ExpireAt time.Time
}

// AccountTenantCandidate 是手机号定位租户时使用的账号候选。
type AccountTenantCandidate struct {
	TenantID int64
}

// LoginTenantCandidate 是登录前按手机号定位学校时使用的候选投影。
type LoginTenantCandidate struct {
	AccountID    int64
	TenantID     int64
	Name         string
	TenantCode   string
	TenantName   string
	TenantStatus int16
}

// LoginAccountSnapshot 是认证流程使用的账号投影。
type LoginAccountSnapshot struct {
	ID             int64
	TenantID       int64
	PasswordHash   string
	HasPassword    bool
	Name           string
	BaseIdentity   int16
	Status         int16
	MustChangePwd  bool
	LockedUntil    time.Time
	HasLockedUntil bool
	Roles          []string
}

// AuthSessionSnapshot 是租户 Refresh 流程定位出的会话投影。
type AuthSessionSnapshot struct {
	ID        int64
	TenantID  int64
	AccountID int64
	Status    int16
	ExpireAt  time.Time
}

// TenantLoginSnapshot 是认证入口校验租户状态使用的投影。
type TenantLoginSnapshot struct {
	ID     int64
	Status int16
}

// SsoConfigSnapshot 是启用 SSO 配置的持久化投影。
type SsoConfigSnapshot struct {
	ID         int64
	Type       int16
	Config     []byte
	MatchField int16
	Enabled    bool
}

// TenantApplicationSnapshot 是平台入驻申请列表使用的投影。
type TenantApplicationSnapshot struct {
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

// TenantSnapshot 是租户详情和配置读取使用的投影。
type TenantSnapshot struct {
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
	ExpireAt             time.Time
	HasExpireAt          bool
}

// PlatformAdminSnapshot 是平台管理员认证流程需要的最小账号投影。
type PlatformAdminSnapshot struct {
	ID           int64
	PasswordHash string
	Name         string
	Status       int16
}

// PlatformSessionSnapshot 是平台管理员 Refresh 流程定位出的会话投影。
type PlatformSessionSnapshot struct {
	ID              int64
	PlatformAdminID int64
	Status          int16
	ExpireAt        time.Time
}

// ActivationCodeSnapshot 是激活码登录前定位出的租户与账号投影。
type ActivationCodeSnapshot struct {
	ID           int64
	TenantID     int64
	AccountID    int64
	Status       int16
	ExpireAt     time.Time
	BaseIdentity int16
	Roles        []string
}

// AuditLogCreate 是 repo 写入 audit_log 时使用的内部持久化投影。
type AuditLogCreate struct {
	ID         int64
	TenantID   int64
	ActorID    int64
	ActorRole  int16
	Action     string
	TargetType string
	TargetID   int64
	Detail     []byte
	IP         string
	TraceID    string
}
