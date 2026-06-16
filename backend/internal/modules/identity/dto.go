// identity DTO 定义 HTTP 请求与响应结构,不承载业务编排逻辑。
package identity

import "time"

type LoginPlatformRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginPhoneRequest struct {
	Phone    string `json:"phone"`
	Password string `json:"password"`
	TenantID int64  `json:"tenant_id"`
}

type LoginNoRequest struct {
	TenantCode string `json:"tenant_code"`
	No         string `json:"no"`
	Password   string `json:"password"`
}

type LoginSMSRequest struct {
	Phone    string `json:"phone"`
	Code     string `json:"code"`
	TenantID int64  `json:"tenant_id"`
}

type SendSMSRequest struct {
	Phone    string `json:"phone"`
	Scene    int16  `json:"scene"`
	TenantID int64  `json:"tenant_id"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type PasswordResetRequest struct {
	Phone       string `json:"phone"`
	Code        string `json:"code"`
	NewPassword string `json:"new_password"`
	TenantID    int64  `json:"tenant_id"`
}

type ActivateRequest struct {
	ActivationCode string `json:"activation_code"`
	Password       string `json:"password"`
}

type LoginResponse struct {
	AccessToken      string            `json:"access_token,omitempty"`
	RefreshToken     string            `json:"refresh_token,omitempty"`
	MustChangePwd    bool              `json:"must_change_pwd,omitempty"`
	NeedSelectTenant bool              `json:"need_select_tenant,omitempty"`
	Tenants          []TenantOptionDTO `json:"tenants,omitempty"`
	Account          *AccountDTO       `json:"account,omitempty"`
}

type TenantOptionDTO struct {
	TenantID int64  `json:"tenant_id,string"`
	Name     string `json:"name"`
	Code     string `json:"code"`
}

type AccountDTO struct {
	ID           int64   `json:"id,string"`
	TenantID     int64   `json:"tenant_id,string"`
	Name         string  `json:"name"`
	PhoneMasked  string  `json:"phone_masked,omitempty"`
	No           string  `json:"no,omitempty"`
	BaseIdentity int16   `json:"base_identity"`
	Roles        []int16 `json:"roles"`
	Status       int16   `json:"status"`
	Title        string  `json:"title,omitempty"`
	CreatedAt    string  `json:"created_at,omitempty"`
}

type AccountListResponse struct {
	List  []AccountDTO `json:"list"`
	Total int64        `json:"total"`
	Page  int32        `json:"page"`
	Size  int32        `json:"size"`
}

type MeResponse struct {
	Account AccountDTO `json:"account"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

type ChangePhoneRequest struct {
	Phone string `json:"phone"`
	Code  string `json:"code"`
}

type SessionDTO struct {
	ID         int64  `json:"id,string"`
	DeviceInfo string `json:"device_info,omitempty"`
	IP         string `json:"ip,omitempty"`
	Status     int16  `json:"status"`
	ExpireAt   string `json:"expire_at"`
	CreatedAt  string `json:"created_at"`
}

type AuditQueryRequest struct {
	TenantID   int64
	ActorID    int64
	Action     string
	TargetType string
	From       time.Time
	To         time.Time
	Page       int32
	Size       int32
}

type AuditLogDTO struct {
	ID         int64  `json:"id,string"`
	TenantID   int64  `json:"tenant_id,string,omitempty"`
	ActorID    int64  `json:"actor_id,string"`
	ActorRole  int16  `json:"actor_role"`
	Action     string `json:"action"`
	TargetType string `json:"target_type"`
	TargetID   int64  `json:"target_id,string,omitempty"`
	Detail     string `json:"detail,omitempty"`
	IP         string `json:"ip,omitempty"`
	TraceID    string `json:"trace_id,omitempty"`
	CreatedAt  string `json:"created_at"`
}

type AuditListResponse struct {
	List  []AuditLogDTO `json:"list"`
	Total int64         `json:"total"`
	Page  int32         `json:"page"`
	Size  int32         `json:"size"`
}

type CreateApplicationRequest struct {
	SchoolName   string `json:"school_name"`
	SchoolType   int16  `json:"school_type"`
	ContactName  string `json:"contact_name"`
	ContactPhone string `json:"contact_phone"`
	ContactEmail string `json:"contact_email"`
}

type ReviewApplicationRequest struct {
	TenantCode string `json:"tenant_code"`
	AdminName  string `json:"admin_name"`
	AdminPhone string `json:"admin_phone"`
	Reason     string `json:"reason"`
}

type UpdateTenantStatusRequest struct {
	Status   int16      `json:"status"`
	ExpireAt *time.Time `json:"expire_at"`
}

type TenantDTO struct {
	ID                   int64      `json:"id,string"`
	Code                 string     `json:"code"`
	Name                 string     `json:"name"`
	Type                 int16      `json:"type"`
	Status               int16      `json:"status"`
	DeployMode           int16      `json:"deploy_mode"`
	ExpireAt             *time.Time `json:"expire_at,omitempty"`
	LogoURL              string     `json:"logo_url,omitempty"`
	DisplayName          string     `json:"display_name,omitempty"`
	AuthMode             int16      `json:"auth_mode"`
	EnableActivationCode bool       `json:"enable_activation_code"`
}

type TenantConfigRequest struct {
	LogoURL              string         `json:"logo_url"`
	DisplayName          string         `json:"display_name"`
	FeatureFlags         map[string]any `json:"feature_flags"`
	AuthMode             int16          `json:"auth_mode"`
	EnableActivationCode bool           `json:"enable_activation_code"`
}

type SSOConfigDTO struct {
	ID         int64          `json:"id,string"`
	TenantID   int64          `json:"tenant_id,string"`
	Type       int16          `json:"type"`
	Config     map[string]any `json:"config"`
	MatchField int16          `json:"match_field"`
	Enabled    bool           `json:"enabled"`
}

type DepartmentRequest struct {
	Name string `json:"name"`
	Code string `json:"code"`
}

type DepartmentDTO struct {
	ID       int64  `json:"id,string"`
	TenantID int64  `json:"tenant_id,string"`
	Name     string `json:"name"`
	Code     string `json:"code"`
}

type MajorRequest struct {
	DepartmentID int64  `json:"department_id,string"`
	Name         string `json:"name"`
}

type MajorDTO struct {
	ID           int64  `json:"id,string"`
	TenantID     int64  `json:"tenant_id,string"`
	DepartmentID int64  `json:"department_id,string"`
	Name         string `json:"name"`
}

type ClassRequest struct {
	MajorID        int64  `json:"major_id,string"`
	Name           string `json:"name"`
	EnrollmentYear int16  `json:"enrollment_year"`
	Status         int16  `json:"status"`
}

type ClassDTO struct {
	ID             int64  `json:"id,string"`
	TenantID       int64  `json:"tenant_id,string"`
	MajorID        int64  `json:"major_id,string"`
	Name           string `json:"name"`
	EnrollmentYear int16  `json:"enrollment_year"`
	Status         int16  `json:"status"`
}

type ArchiveClassesRequest struct {
	EnrollmentYear int16 `json:"enrollment_year"`
}

type CreateAccountRequest struct {
	Phone           string `json:"phone"`
	Name            string `json:"name"`
	No              string `json:"no"`
	BaseIdentity    int16  `json:"base_identity"`
	OrgID           int64  `json:"org_id,string"`
	EnrollmentYear  int16  `json:"enrollment_year,omitempty"`
	Title           string `json:"title,omitempty"`
	InitialPassword string `json:"initial_password,omitempty"`
	UseActivation   bool   `json:"use_activation"`
}

type UpdateAccountRequest struct {
	Name           string `json:"name"`
	OrgID          int64  `json:"org_id,string"`
	EnrollmentYear int16  `json:"enrollment_year,omitempty"`
	Title          string `json:"title,omitempty"`
}

type AdminResetPasswordRequest struct {
	NewPassword string `json:"new_password"`
	MustChange  bool   `json:"must_change_pwd"`
}

type BatchAccountIDsRequest struct {
	AccountIDs []int64 `json:"account_ids"`
}

type ImportPreviewRequest struct {
	TargetType  int16
	FileName    string
	ContentType string
	Content     []byte
}

type ImportCommitRequest struct {
	PreviewID int64 `json:"preview_id,string"`
}

type AccountImportCommitResponse struct {
	Batch           ImportBatchDTO            `json:"batch"`
	ActivationCodes []ImportActivationCodeDTO `json:"activation_codes,omitempty"`
}

type ImportBatchDTO struct {
	ID         int64  `json:"id,string"`
	TenantID   int64  `json:"tenant_id,string"`
	OperatorID int64  `json:"operator_id,string"`
	TargetType int16  `json:"target_type"`
	FileName   string `json:"file_name"`
	Total      int32  `json:"total"`
	Success    int32  `json:"success"`
	Failed     int32  `json:"failed"`
	Status     int16  `json:"status"`
	CreatedAt  string `json:"created_at"`
}

type ImportActivationCodeDTO struct {
	AccountID      int64  `json:"account_id,string"`
	No             string `json:"no"`
	Name           string `json:"name"`
	ActivationCode string `json:"activation_code"`
}

type ImportPreviewResponse struct {
	PreviewID int64             `json:"preview_id,string"`
	Total     int               `json:"total"`
	Valid     int               `json:"valid"`
	Invalid   int               `json:"invalid"`
	Rows      []ImportRowResult `json:"rows"`
}

type ImportRowResult struct {
	Line  int    `json:"line"`
	Error string `json:"error,omitempty"`
}

type SSOConfigRequest struct {
	Type       int16          `json:"type"`
	Config     map[string]any `json:"config"`
	MatchField int16          `json:"match_field"`
	Enabled    bool           `json:"enabled"`
}

type LDAPLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
