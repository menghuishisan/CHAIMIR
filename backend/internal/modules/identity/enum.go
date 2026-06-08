// M1 领域枚举常量(对应数据模型的 SMALLINT 字段)。
// 依据 docs/01-身份与租户/02-数据模型.md。集中定义,避免各处魔法数字。
package identity

import "chaimir/internal/contracts"

// 账号基础身份(account.base_identity,不可变)。
const (
	BaseIdentityStudent int16 = 1
	BaseIdentityTeacher int16 = 2
)

// 账号状态(account.status)。状态机见 docs/01 §5。
const (
	AccountPending   int16 = 1 // 待激活
	AccountActive    int16 = 2 // 正常(唯一可登录态)
	AccountDisabled  int16 = 3 // 停用
	AccountArchived  int16 = 4 // 归档
	AccountCancelled int16 = 5 // 注销(终态,软删)
)

// 角色(account_role.role)。四级固定 RBAC。
const (
	RolePlatformAdmin = contracts.RoleNumPlatformAdmin // 平台管理员(独立 platform_admin 表,不入 account_role)
	RoleSchoolAdmin   = contracts.RoleNumSchoolAdmin   // 学校管理员(教师附加)
	RoleTeacher       = contracts.RoleNumTeacher
	RoleStudent       = contracts.RoleNumStudent
)

// 租户状态(tenant.status)。
const (
	TenantActive   int16 = 1
	TenantDisabled int16 = 2
	TenantExpired  int16 = 3
)

// 租户部署模式(tenant.deploy_mode)。
const (
	DeployModeSaaS   int16 = 1
	DeployModeSchool int16 = 2
)

// 学校类型(tenant.type / tenant_application.school_type)。
const (
	SchoolTypeDoctor  int16 = 1
	SchoolTypeMaster  int16 = 2
	SchoolTypeCollege int16 = 3
	SchoolTypeJunior  int16 = 4
)

// 租户认证方式(tenant.auth_mode)。
const (
	AuthModeLocal int16 = 1
	AuthModeCAS   int16 = 2
	AuthModeLDAP  int16 = 3
)

// 入驻申请状态(tenant_application.status)。
const (
	ApplicationPending  int16 = 1
	ApplicationApproved int16 = 2
	ApplicationRejected int16 = 3
)

// 班级状态(class.status)。
const (
	ClassActive   int16 = 1
	ClassArchived int16 = 2
)

// 会话状态(auth_session.status)。
const (
	SessionActive  int16 = 1
	SessionRevoked int16 = 2
)

// 短信验证码场景(sms_code.scene)。
const (
	SmsSceneLogin  int16 = 1
	SmsSceneReset  int16 = 2
	SmsSceneRebind int16 = 3
)

// 激活码状态(activation_code.status)。
const (
	ActivationCodeActive  int16 = 1
	ActivationCodeUsed    int16 = 2
	ActivationCodeRevoked int16 = 3
)

// 导入目标类型(import_batch.target_type)。
const (
	ImportTargetTeacher int16 = 1
	ImportTargetStudent int16 = 2
	ImportTargetOrg     int16 = 3
)

// 导入批次状态(import_batch.status)。
const (
	ImportProcessing int16 = 1
	ImportDone       int16 = 2
	ImportFailed     int16 = 3
)

// SSO 类型(sso_config.type)。
const (
	SsoTypeCAS  int16 = 1
	SsoTypeLDAP int16 = 2
)
