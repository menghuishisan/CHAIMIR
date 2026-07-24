// identity 领域模型定义服务层使用的真实内部快照,避免向业务编排泄漏 sqlc 行类型。
package identity

import "time"

// Tenant 是学校租户及其内联配置的领域快照。
type Tenant struct {
	ID                   int64
	Code                 string
	Name                 string
	Type                 int16
	Status               int16
	DeployMode           int16
	ExpireAt             *time.Time
	LogoURL              string
	DisplayName          string
	AuthMode             int16
	EnableActivationCode bool
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// TenantProvisionOutbox 是新租户跨模块初始化事件的可靠投递记录。
type TenantProvisionOutbox struct {
	ID            int64
	TenantID      int64
	DeployMode    int16
	TraceID       string
	ProvisionedAt time.Time
	Status        int16
	RetryCount    int32
	LastError     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// TenantApplication 是学校入驻申请的领域快照。
type TenantApplication struct {
	ID           int64
	SchoolName   string
	SchoolType   int16
	ContactName  string
	ContactPhone string
	ContactEmail string
	Status       int16
	RejectReason string
	ReviewedBy   int64
	TenantID     int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Account 是租户账号、角色和组织档案的领域快照。
type Account struct {
	ID             int64
	TenantID       int64
	PhoneEnc       []byte
	PhoneHash      string
	PasswordHash   string
	Name           string
	BaseIdentity   int16
	Status         int16
	MustChangePwd  bool
	PwdFailedCount int16
	LockedUntil    *time.Time
	ActivatedAt    *time.Time
	No             string
	OrgID          int64
	EnrollmentYear int16
	Title          string
	Roles          []int16
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// LoginCandidate 表示手机号登录预认证时跨租户定位到的候选账号。
type LoginCandidate struct {
	AccountID      int64
	TenantID       int64
	TenantCode     string
	TenantName     string
	PasswordHash   string
	Name           string
	BaseIdentity   int16
	Status         int16
	MustChangePwd  bool
	PwdFailedCount int16
	LockedUntil    *time.Time
}

// PlatformAdmin 是平台管理员账号领域快照。
type PlatformAdmin struct {
	ID           int64
	Username     string
	PasswordHash string
	Name         string
	Status       int16
}

// AuthSession 是租户 Refresh 会话快照。
type AuthSession struct {
	ID               int64
	TenantID         int64
	AccountID        int64
	RefreshTokenHash string
	DeviceInfo       string
	IP               string
	Status           int16
	ExpireAt         time.Time
	CreatedAt        time.Time
}

// PlatformAuthSession 是平台管理员 Refresh 会话快照。
type PlatformAuthSession struct {
	ID               int64
	PlatformAdminID  int64
	RefreshTokenHash string
	DeviceInfo       string
	IP               string
	Status           int16
	ExpireAt         time.Time
	CreatedAt        time.Time
}

// ActivationCode 是激活码状态快照,明文激活码永不落入该模型。
type ActivationCode struct {
	ID        int64
	TenantID  int64
	AccountID int64
	CodeHash  string
	Status    int16
	ExpireAt  time.Time
}

// SMSCode 是短信验证码状态快照,明文验证码永不落入该模型。
type SMSCode struct {
	ID             int64
	TenantID       int64
	PhoneHash      string
	CodeHash       string
	Scene          int16
	ExpireAt       time.Time
	VerifyAttempts int16
	Used           bool
	CreatedAt      time.Time
}

// SSOConfig 是 CAS/LDAP 配置的领域快照,密码字段在 JSON 内以密文保存。
type SSOConfig struct {
	ID         int64
	TenantID   int64
	Type       int16
	Config     []byte
	MatchField int16
	Enabled    bool
}

// Department 是租户院系领域快照。
type Department struct {
	ID       int64
	TenantID int64
	Name     string
	Code     string
}

// Major 是租户专业领域快照。
type Major struct {
	ID           int64
	TenantID     int64
	DepartmentID int64
	Name         string
}

// Class 是租户班级领域快照。
type Class struct {
	ID             int64
	TenantID       int64
	MajorID        int64
	Name           string
	EnrollmentYear int16
	Status         int16
}

// ImportPreview 是服务端持久化导入预览快照。
type ImportPreview struct {
	ID            int64
	TenantID      int64
	OperatorID    int64
	TargetType    int16
	FileName      string
	Rows          []byte
	PreviewResult []byte
	Status        int16
	ExpireAt      time.Time
}

// ImportBatch 是导入提交后的批次结果快照。
type ImportBatch struct {
	ID         int64
	TenantID   int64
	OperatorID int64
	TargetType int16
	FileName   string
	Total      int32
	Success    int32
	Failed     int32
	Status     int16
	CreatedAt  time.Time
}

// ImportTemplateFile 是账号导入模板下载前的内部文件快照。
type ImportTemplateFile struct {
	FileName    string
	ContentType string
	Content     []byte
}
