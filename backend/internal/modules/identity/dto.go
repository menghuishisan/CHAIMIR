// identity DTO 定义 HTTP 请求与响应结构,不承载业务编排逻辑。
package identity

import (
	"time"

	"chaimir/internal/platform/ids"
)

type LoginPlatformRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginPhoneRequest struct {
	Phone    string `json:"phone"`
	Password string `json:"password"`
	TenantID ids.ID `json:"tenant_id"`
}

type LoginNoRequest struct {
	TenantCode string `json:"tenant_code"`
	No         string `json:"no"`
	Password   string `json:"password"`
}

type LoginSMSRequest struct {
	Phone    string `json:"phone"`
	Code     string `json:"code"`
	TenantID ids.ID `json:"tenant_id"`
}

type SendSMSRequest struct {
	Phone    string `json:"phone"`
	Scene    int16  `json:"scene"`
	TenantID ids.ID `json:"tenant_id"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type WebSocketTicketRequest struct {
	Path string `json:"path"`
}

type WebSocketTicketResponse struct {
	Ticket    string `json:"ticket"`
	ExpiresAt string `json:"expires_at"`
}

type PasswordResetRequest struct {
	Phone       string `json:"phone"`
	Code        string `json:"code"`
	NewPassword string `json:"new_password"`
	TenantID    ids.ID `json:"tenant_id"`
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
	TenantID ids.ID `json:"tenant_id"`
	Name     string `json:"name"`
	Code     string `json:"code"`
}

type AccountDTO struct {
	ID           ids.ID  `json:"id"`
	TenantID     ids.ID  `json:"tenant_id,omitempty"`
	Name         string  `json:"name"`
	PhoneMasked  string  `json:"phone_masked,omitempty"`
	No           string  `json:"no,omitempty"`
	BaseIdentity int16   `json:"base_identity"`
	Roles        []int16 `json:"roles"`
	Status       int16   `json:"status"`
	Title        string  `json:"title,omitempty"`
	CreatedAt    string  `json:"created_at,omitempty"`
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
	ID         ids.ID `json:"id"`
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
	ID         ids.ID `json:"id"`
	TenantID   ids.ID `json:"tenant_id,omitempty"`
	ActorID    ids.ID `json:"actor_id"`
	ActorRole  int16  `json:"actor_role"`
	Action     string `json:"action"`
	TargetType string `json:"target_type"`
	TargetID   ids.ID `json:"target_id,omitempty"`
	Detail     string `json:"detail,omitempty"`
	IP         string `json:"ip,omitempty"`
	TraceID    string `json:"trace_id,omitempty"`
	CreatedAt  string `json:"created_at"`
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

type TenantApplicationDTO struct {
	ApplicationID ids.ID `json:"application_id"`
	SchoolName    string `json:"school_name"`
	SchoolType    int16  `json:"school_type"`
	ContactName   string `json:"contact_name"`
	ContactPhone  string `json:"contact_phone"`
	ContactEmail  string `json:"contact_email"`
	Status        int16  `json:"status"`
	RejectReason  string `json:"reject_reason,omitempty"`
	ReviewedBy    ids.ID `json:"reviewed_by,omitempty"`
	TenantID      ids.ID `json:"tenant_id,omitempty"`
	SubmittedAt   string `json:"submitted_at"`
	ReviewedAt    string `json:"reviewed_at,omitempty"`
}

type UpdateTenantStatusRequest struct {
	Status   int16      `json:"status"`
	ExpireAt *time.Time `json:"expire_at"`
}

type TenantDTO struct {
	ID                   ids.ID     `json:"id"`
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
	LogoURL              string `json:"logo_url"`
	DisplayName          string `json:"display_name"`
	AuthMode             int16  `json:"auth_mode"`
	EnableActivationCode bool   `json:"enable_activation_code"`
}

type SSOConfigDTO struct {
	ID         ids.ID         `json:"id"`
	TenantID   ids.ID         `json:"tenant_id"`
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
	ID       ids.ID `json:"id"`
	TenantID ids.ID `json:"tenant_id"`
	Name     string `json:"name"`
	Code     string `json:"code"`
}

type MajorRequest struct {
	DepartmentID ids.ID `json:"department_id"`
	Name         string `json:"name"`
}

type MajorDTO struct {
	ID           ids.ID `json:"id"`
	TenantID     ids.ID `json:"tenant_id"`
	DepartmentID ids.ID `json:"department_id"`
	Name         string `json:"name"`
}

type ClassRequest struct {
	MajorID        ids.ID `json:"major_id"`
	Name           string `json:"name"`
	EnrollmentYear int16  `json:"enrollment_year"`
	Status         int16  `json:"status"`
}

type ClassDTO struct {
	ID             ids.ID `json:"id"`
	TenantID       ids.ID `json:"tenant_id"`
	MajorID        ids.ID `json:"major_id"`
	Name           string `json:"name"`
	EnrollmentYear int16  `json:"enrollment_year"`
	Status         int16  `json:"status"`
}

type ArchiveClassesRequest struct {
	EnrollmentYear int16 `json:"enrollment_year"`
}

type PromoteClassesRequest struct {
	ClassIDs   []ids.ID `json:"class_ids"`
	TargetYear int16    `json:"target_year"`
}

type CreateAccountRequest struct {
	Phone           string `json:"phone"`
	Name            string `json:"name"`
	No              string `json:"no"`
	BaseIdentity    int16  `json:"base_identity"`
	OrgID           ids.ID `json:"org_id"`
	EnrollmentYear  int16  `json:"enrollment_year,omitempty"`
	Title           string `json:"title,omitempty"`
	InitialPassword string `json:"initial_password,omitempty"`
	UseActivation   bool   `json:"use_activation"`
}

type UpdateAccountRequest struct {
	Name           string `json:"name"`
	OrgID          ids.ID `json:"org_id"`
	EnrollmentYear int16  `json:"enrollment_year,omitempty"`
	Title          string `json:"title,omitempty"`
}

type AdminResetPasswordRequest struct {
	NewPassword string `json:"new_password"`
	MustChange  bool   `json:"must_change_pwd"`
}

type BatchAccountIDsRequest struct {
	AccountIDs []ids.ID `json:"account_ids"`
}

type ImportPreviewRequest struct {
	TargetType  int16
	FileName    string
	ContentType string
	Content     []byte
}

type ImportCommitRequest struct {
	PreviewID ids.ID `json:"preview_id"`
}

type AccountImportCommitResponse struct {
	Batch           ImportBatchDTO            `json:"batch"`
	ActivationCodes []ImportActivationCodeDTO `json:"activation_codes,omitempty"`
}

type ImportBatchDTO struct {
	ID         ids.ID `json:"id"`
	TenantID   ids.ID `json:"tenant_id"`
	OperatorID ids.ID `json:"operator_id"`
	TargetType int16  `json:"target_type"`
	FileName   string `json:"file_name"`
	Total      int32  `json:"total"`
	Success    int32  `json:"success"`
	Failed     int32  `json:"failed"`
	Status     int16  `json:"status"`
	CreatedAt  string `json:"created_at"`
}

type ImportActivationCodeDTO struct {
	AccountID      ids.ID `json:"account_id"`
	No             string `json:"no"`
	Name           string `json:"name"`
	ActivationCode string `json:"activation_code"`
}

type ImportPreviewResponse struct {
	PreviewID ids.ID            `json:"preview_id"`
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
