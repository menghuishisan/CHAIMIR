// M1 请求/响应 DTO。
// 雪花 ID(int64)在 JSON 中转字符串防前端精度丢失:对外 ID 字段统一 string。
package identity

// ---- 认证 ----

// LoginPhoneRequest 手机号密码登录请求。
type LoginPhoneRequest struct {
	Phone    string `json:"phone" binding:"required"`
	Password string `json:"password" binding:"required"`
	TenantID string `json:"tenant_id"` // 一号多校时选定的租户(可空)。
}

// PlatformLoginRequest 平台管理员登录请求。
type PlatformLoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginNoRequest 学号/工号登录请求(备用)。
type LoginNoRequest struct {
	TenantCode string `json:"tenant_code" binding:"required"`
	No         string `json:"no" binding:"required"`
	Password   string `json:"password" binding:"required"`
}

// LoginSmsRequest 短信验证码登录请求。
type LoginSmsRequest struct {
	Phone    string `json:"phone" binding:"required"`
	Code     string `json:"code" binding:"required"`
	TenantID string `json:"tenant_id"`
}

// SendSmsRequest 发送验证码请求。
type SendSmsRequest struct {
	Phone    string `json:"phone" binding:"required"`
	Scene    int16  `json:"scene" binding:"required"` // 1登录/2找回/3换绑
	TenantID string `json:"tenant_id"`                // 登录短信一号多校时必须选择学校。
}

// RefreshRequest 刷新 Token 请求。
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// ResetPasswordRequest 找回密码请求。
type ResetPasswordRequest struct {
	Phone       string `json:"phone" binding:"required"`
	Code        string `json:"code" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
	TenantID    string `json:"tenant_id"` // 一号多校时选定要找回的学校账号。
}

// ActivateAccountRequest 是激活码开通请求。
type ActivateAccountRequest struct {
	ActivationCode string `json:"activation_code" binding:"required"`
	Password       string `json:"password" binding:"required"`
}

// LDAPLoginRequest 是 LDAP SSO 登录请求。
type LDAPLoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// SsoLoginURLResponse 是 CAS 登录地址响应。
type SsoLoginURLResponse struct {
	RedirectURL string `json:"redirect_url"`
}

// TenantBrief 一号多校时供选择的租户摘要。
type TenantBrief struct {
	TenantID   string `json:"tenant_id"`
	TenantCode string `json:"tenant_code"`
	TenantName string `json:"tenant_name"`
}

// AccountBrief 登录响应里的账号摘要。
type AccountBrief struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	BaseIdentity int16    `json:"base_identity"`
	Roles        []string `json:"roles"`
}

// LoginResult 登录结果:要么需选租户,要么下发双 Token。
type LoginResult struct {
	NeedSelectTenant bool          `json:"need_select_tenant,omitempty"`
	Tenants          []TenantBrief `json:"tenants,omitempty"`
	AccessToken      string        `json:"access_token,omitempty"`
	RefreshToken     string        `json:"refresh_token,omitempty"`
	MustChangePwd    bool          `json:"must_change_pwd,omitempty"`
	Account          *AccountBrief `json:"account,omitempty"`
}

// TokenPair 刷新结果。
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// ---- 账号管理 ----

// CreateAccountRequest 单个账号创建(学校管理员)。
type CreateAccountRequest struct {
	Phone          string `json:"phone" binding:"required"`
	Name           string `json:"name" binding:"required"`
	BaseIdentity   int16  `json:"base_identity" binding:"required"` // 1学生/2教师
	No             string `json:"no" binding:"required"`            // 学号/工号
	OrgID          string `json:"org_id" binding:"required"`        // 班级/院系 id
	EnrollmentYear int16  `json:"enrollment_year"`
	Title          string `json:"title"`
	InitPassword   string `json:"init_password"` // 空则生成临时密码 + 首登改密
}

// UpdateAccountRequest 更新账号(不含不可变字段 no/base_identity)。
type UpdateAccountRequest struct {
	Name string `json:"name" binding:"required"`
}

// AccountView 账号详情(对外,手机号脱敏)。
type AccountView struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Phone          string   `json:"phone"` // 脱敏 138****1234
	BaseIdentity   int16    `json:"base_identity"`
	Status         int16    `json:"status"`
	Roles          []string `json:"roles"`
	No             string   `json:"no"`
	OrgID          string   `json:"org_id"`
	EnrollmentYear *int16   `json:"enrollment_year,omitempty"`
	Title          string   `json:"title,omitempty"`
	MustChangePwd  bool     `json:"must_change_pwd"`
}

// BatchAccountStatusRequest 批量账号状态操作请求。
type BatchAccountStatusRequest struct {
	AccountIDs []string `json:"account_ids" binding:"required"`
}

// BatchArchiveAccountsRequest 按学年批量归档学生账号请求。
type BatchArchiveAccountsRequest struct {
	EnrollmentYear int16 `json:"enrollment_year" binding:"required"`
}

// BatchAccountStatusResult 批量账号状态操作结果。
type BatchAccountStatusResult struct {
	Total   int                  `json:"total"`
	Success int                  `json:"success"`
	Failed  int                  `json:"failed"`
	Rows    []BatchAccountStatus `json:"rows"`
}

// BatchAccountStatus 是批量操作单账号结果。
type BatchAccountStatus struct {
	AccountID string `json:"account_id"`
	Error     string `json:"error,omitempty"`
}

// CreateAccountResult 创建账号结果(返回生成的临时密码,仅此一次)。
type CreateAccountResult struct {
	ID             string `json:"id"`
	InitPassword   string `json:"init_password,omitempty"`   // 系统生成时返回,供管理员转交。
	ActivationCode string `json:"activation_code,omitempty"` // 启用激活码开通时返回,仅此一次。
}

// ChangePasswordRequest 本人改密。
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password" binding:"required"`
}

// ChangePhoneRequest 本人换绑手机。
type ChangePhoneRequest struct {
	NewPhone string `json:"new_phone" binding:"required"`
	Code     string `json:"code" binding:"required"`
}

// MeView 个人中心信息(含学籍只读字段)。
type MeView struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Phone        string   `json:"phone"`
	BaseIdentity int16    `json:"base_identity"`
	Roles        []string `json:"roles"`
	No           string   `json:"no"`
	OrgID        string   `json:"org_id"`
	Title        string   `json:"title,omitempty"`
}

// SessionView 是个人中心会话视图。
type SessionView struct {
	ID         string `json:"id"`
	DeviceInfo string `json:"device_info"`
	IP         string `json:"ip"`
	ExpireAt   string `json:"expire_at"`
	CreatedAt  string `json:"created_at"`
}

// ---- 组织 ----

// CreateDepartmentRequest 院系创建。
type CreateDepartmentRequest struct {
	Name string `json:"name" binding:"required"`
	Code string `json:"code"`
}

// UpdateDepartmentRequest 院系更新。
type UpdateDepartmentRequest struct {
	Name string `json:"name" binding:"required"`
	Code string `json:"code"`
}

// CreateMajorRequest 专业创建。
type CreateMajorRequest struct {
	DepartmentID string `json:"department_id" binding:"required"`
	Name         string `json:"name" binding:"required"`
}

// UpdateMajorRequest 专业更新。
type UpdateMajorRequest struct {
	Name string `json:"name" binding:"required"`
}

// CreateClassRequest 班级创建。
type CreateClassRequest struct {
	MajorID        string `json:"major_id" binding:"required"`
	Name           string `json:"name" binding:"required"`
	EnrollmentYear int16  `json:"enrollment_year" binding:"required"`
}

// UpdateClassRequest 班级更新。
type UpdateClassRequest struct {
	Name           string `json:"name" binding:"required"`
	EnrollmentYear int16  `json:"enrollment_year" binding:"required"`
}

// PromoteClassRequest 班级升级请求,只调整名称与入学年份。
type PromoteClassRequest struct {
	Name           string `json:"name" binding:"required"`
	EnrollmentYear int16  `json:"enrollment_year" binding:"required"`
}

// OrgNode 组织节点(院系/专业/班级通用对外视图)。
type OrgNode struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Code           string `json:"code,omitempty"`
	ParentID       string `json:"parent_id,omitempty"`
	EnrollmentYear *int16 `json:"enrollment_year,omitempty"`
	Status         *int16 `json:"status,omitempty"`
}

// OrgImportRequest 是组织结构批量导入请求。
type OrgImportRequest struct {
	FileName    string                `json:"file_name"`
	Departments []OrgImportDepartment `json:"departments" binding:"required"`
}

// OrgImportDepartment 是导入中的院系节点。
type OrgImportDepartment struct {
	Name   string           `json:"name" binding:"required"`
	Code   string           `json:"code"`
	Majors []OrgImportMajor `json:"majors"`
}

// OrgImportMajor 是导入中的专业节点。
type OrgImportMajor struct {
	Name    string           `json:"name" binding:"required"`
	Classes []OrgImportClass `json:"classes"`
}

// OrgImportClass 是导入中的班级节点。
type OrgImportClass struct {
	Name           string `json:"name" binding:"required"`
	EnrollmentYear int16  `json:"enrollment_year" binding:"required"`
}

// OrgImportResult 是组织结构导入结果。
type OrgImportResult struct {
	BatchID string             `json:"batch_id"`
	Total   int                `json:"total"`
	Success int                `json:"success"`
	Failed  int                `json:"failed"`
	Rows    []ImportPreviewRow `json:"rows"`
}

// BatchClassArchiveRequest 是批量班级归档请求。
type BatchClassArchiveRequest struct {
	ClassIDs []string `json:"class_ids" binding:"required"`
}

// BatchClassPromoteRequest 是批量班级升级请求。
type BatchClassPromoteRequest struct {
	Rows []ClassPromoteInput `json:"rows" binding:"required"`
}

// ClassPromoteInput 是单个班级升级输入。
type ClassPromoteInput struct {
	ClassID        string `json:"class_id" binding:"required"`
	Name           string `json:"name" binding:"required"`
	EnrollmentYear int16  `json:"enrollment_year" binding:"required"`
}

// BatchClassOperationResult 是批量班级操作结果。
type BatchClassOperationResult struct {
	Total   int                      `json:"total"`
	Success int                      `json:"success"`
	Failed  int                      `json:"failed"`
	Rows    []BatchClassOperationRow `json:"rows"`
}

// BatchClassOperationRow 是批量班级操作的逐行结果。
type BatchClassOperationRow struct {
	ClassID string `json:"class_id"`
	Error   string `json:"error,omitempty"`
}

// ---- 租户/入驻(平台)----

// CreateApplicationRequest 入驻申请提交。
type CreateApplicationRequest struct {
	SchoolName   string `json:"school_name" binding:"required"`
	SchoolType   int16  `json:"school_type" binding:"required"`
	ContactName  string `json:"contact_name" binding:"required"`
	ContactPhone string `json:"contact_phone" binding:"required"`
	ContactEmail string `json:"contact_email" binding:"required"`
}

// RejectApplicationRequest 驳回申请。
type RejectApplicationRequest struct {
	Reason string `json:"reason" binding:"required"`
}

// ApproveApplicationResult 通过申请结果(返回新租户与首个管理员激活信息)。
type ApproveApplicationResult struct {
	TenantID       string `json:"tenant_id"`
	TenantCode     string `json:"tenant_code"`
	AdminPhone     string `json:"admin_phone"`
	ActivationCode string `json:"activation_code,omitempty"`
	ActivationHint string `json:"activation_hint"` // 激活方式提示(临时密码/激活码)。
}

// UpdateTenantRequest 平台改租户状态/到期。
type UpdateTenantRequest struct {
	Status   int16  `json:"status"`
	ExpireAt string `json:"expire_at"` // RFC3339;空表示不改。
}

// TenantConfigRequest 学校管理员改本校配置。
type TenantConfigRequest struct {
	LogoURL              string         `json:"logo_url"`
	DisplayName          string         `json:"display_name"`
	FeatureFlags         map[string]any `json:"feature_flags"`
	AuthMode             int16          `json:"auth_mode"`
	EnableActivationCode bool           `json:"enable_activation_code"`
}

// SsoConfigRequest 保存 SSO/LDAP 配置。
type SsoConfigRequest struct {
	Type       int16          `json:"type" binding:"required"`        // 1 CAS / 2 LDAP
	Config     map[string]any `json:"config" binding:"required"`      // 协议参数;敏感字段加密后入库
	MatchField int16          `json:"match_field" binding:"required"` // 1学工号 / 2手机号
	Enabled    bool           `json:"enabled"`
}

// SsoConfigView 是 SSO/LDAP 配置响应。
type SsoConfigView struct {
	ID         string         `json:"id"`
	Type       int16          `json:"type"`
	Config     map[string]any `json:"config"`
	MatchField int16          `json:"match_field"`
	Enabled    bool           `json:"enabled"`
}

// ---- 导入(两步)----

// ImportPreviewRow 导入预览的逐行结果。
type ImportPreviewRow struct {
	Line  int    `json:"line"`
	Error string `json:"error,omitempty"`
}

// ImportPreviewResult 导入预览结果(不落库)。
type ImportPreviewResult struct {
	PreviewID string             `json:"preview_id,omitempty"`
	Total     int                `json:"total"`
	Valid     int                `json:"valid"`
	Invalid   int                `json:"invalid"`
	Rows      []ImportPreviewRow `json:"rows"`
}

// ImportRowInput 导入的单行数据(预览/提交共用)。
type ImportRowInput struct {
	Phone          string `json:"phone"`
	Name           string `json:"name"`
	No             string `json:"no"`
	OrgID          string `json:"org_id"`
	EnrollmentYear int16  `json:"enrollment_year"`
	Title          string `json:"title"`
}

// ImportRequest 是服务端解析上传文件后的内部导入数据。
type ImportRequest struct {
	TargetType int16            `json:"target_type" binding:"required"` // 1教师/2学生
	FileName   string           `json:"file_name"`
	Rows       []ImportRowInput `json:"rows" binding:"required"`
}

// ImportCommitRequest 是确认提交导入预览的请求。
type ImportCommitRequest struct {
	PreviewID string `json:"preview_id" binding:"required"`
}

// ImportCommitResult 提交结果。
type ImportCommitResult struct {
	BatchID         string                  `json:"batch_id"`
	Total           int                     `json:"total"`
	Success         int                     `json:"success"`
	Failed          int                     `json:"failed"`
	ActivationCodes []ActivationCodeIssued  `json:"activation_codes,omitempty"`
	InitPasswords   []InitialPasswordIssued `json:"init_passwords,omitempty"`
}

// ActivationCodeIssued 是创建/导入账号时一次性返回的激活码。
type ActivationCodeIssued struct {
	AccountID      string `json:"account_id"`
	ActivationCode string `json:"activation_code"`
	InitPassword   string `json:"-"`
}

// InitialPasswordIssued 是导入账号时一次性返回的临时密码。
type InitialPasswordIssued struct {
	AccountID    string `json:"account_id"`
	InitPassword string `json:"init_password"`
}

// ImportBatchView 是导入批次历史视图。
type ImportBatchView struct {
	ID         string `json:"id"`
	OperatorID string `json:"operator_id"`
	TargetType int16  `json:"target_type"`
	FileName   string `json:"file_name"`
	Total      int32  `json:"total"`
	Success    int32  `json:"success"`
	Failed     int32  `json:"failed"`
	Status     int16  `json:"status"`
	CreatedAt  string `json:"created_at"`
}
