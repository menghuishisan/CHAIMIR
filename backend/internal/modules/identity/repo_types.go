// identity repo 类型文件定义数据访问层入参和查询结果,供 service 编排调用。
package identity

import "time"

// CreateTenantInput 描述创建租户所需的持久化字段。
type CreateTenantInput struct {
	ID                   int64
	Code                 string
	Name                 string
	Type                 int16
	Status               int16
	DeployMode           int16
	ExpireAt             *time.Time
	LogoURL              string
	DisplayName          string
	FeatureFlags         []byte
	AuthMode             int16
	EnableActivationCode bool
}

// CreatePlatformAdminInput 描述 SaaS 首个平台管理员初始化字段。
type CreatePlatformAdminInput struct {
	ID           int64
	Username     string
	PasswordHash string
	Name         string
	Status       int16
}

// UpdateTenantConfigInput 描述租户内联配置更新字段。
type UpdateTenantConfigInput struct {
	TenantID             int64
	LogoURL              string
	DisplayName          string
	FeatureFlags         []byte
	AuthMode             int16
	EnableActivationCode bool
}

// UpdateTenantStatusInput 描述平台修改租户状态的字段。
type UpdateTenantStatusInput struct {
	TenantID int64
	Status   int16
	ExpireAt *time.Time
}

// CreateAccountInput 描述创建租户账号、角色和可选档案的一致性写入字段。
type CreateAccountInput struct {
	ID            int64
	TenantID      int64
	PhoneEnc      []byte
	PhoneHash     string
	PasswordHash  string
	Name          string
	BaseIdentity  int16
	Status        int16
	MustChangePwd bool
	ActivatedAt   *time.Time
	Roles         []RoleCreateInput
	Profile       *CreateProfileInput
}

// RoleCreateInput 描述账号角色写入字段。
type RoleCreateInput struct {
	ID   int64
	Role int16
}

// CreateProfileInput 描述账号组织档案写入字段。
type CreateProfileInput struct {
	No             string
	OrgID          int64
	EnrollmentYear int16
	Title          string
}

// AccountQuery 描述账号列表过滤和分页字段。
type AccountQuery struct {
	Status       int16
	BaseIdentity int16
	ClassID      int64
	Keyword      string
	Page         int32
	Size         int32
}

// CreateSessionInput 描述租户 Refresh 会话写入字段。
type CreateSessionInput struct {
	ID               int64
	TenantID         int64
	AccountID        int64
	RefreshTokenHash string
	DeviceInfo       string
	IP               string
	ExpireAt         time.Time
}

// CreatePlatformSessionInput 描述平台管理员 Refresh 会话写入字段。
type CreatePlatformSessionInput struct {
	ID               int64
	PlatformAdminID  int64
	RefreshTokenHash string
	DeviceInfo       string
	IP               string
	ExpireAt         time.Time
}

// CreateSMSCodeInput 描述短信验证码哈希写入字段。
type CreateSMSCodeInput struct {
	ID        int64
	TenantID  int64
	PhoneHash string
	CodeHash  string
	Scene     int16
	ExpireAt  time.Time
}

// CreateActivationInput 描述激活码哈希写入字段。
type CreateActivationInput struct {
	ID        int64
	TenantID  int64
	AccountID int64
	CodeHash  string
	ExpireAt  time.Time
	CreatedBy int64
}

// UpsertSSOInput 描述 CAS/LDAP 配置落库字段。
type UpsertSSOInput struct {
	ID         int64
	TenantID   int64
	Type       int16
	Config     []byte
	MatchField int16
	Enabled    bool
}

// CreateImportPreviewInput 描述导入预览持久化字段。
type CreateImportPreviewInput struct {
	ID            int64
	TenantID      int64
	OperatorID    int64
	TargetType    int16
	FileName      string
	Rows          []byte
	PreviewResult []byte
	ExpireAt      time.Time
}

// CreateImportBatchInput 描述导入批次持久化字段。
type CreateImportBatchInput struct {
	ID          int64
	TenantID    int64
	OperatorID  int64
	TargetType  int16
	FileName    string
	Total       int32
	Success     int32
	Failed      int32
	ErrorDetail []byte
	Status      int16
}

// WriteAuditInput 描述写入共享 audit_log 的字段。
type WriteAuditInput struct {
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

// AuditQueryInput 描述审计查询过滤和分页字段。
type AuditQueryInput struct {
	TenantID   int64
	ActorID    int64
	Action     string
	TargetType string
	From       time.Time
	To         time.Time
	Page       int32
	Size       int32
}

// AuditLogRow 是审计查询领域结果。
type AuditLogRow struct {
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

// StatsRow 是 identity 对聚合层开放的统计领域结果。
type StatsRow struct {
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
