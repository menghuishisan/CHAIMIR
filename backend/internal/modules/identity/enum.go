// identity 枚举定义身份模块内部持久化状态、角色和导入类型的稳定取值。
package identity

const (
	TenantStatusActive   int16 = 1
	TenantStatusDisabled int16 = 2
	TenantStatusExpired  int16 = 3
)

const (
	DeployModeSaaS   int16 = 1
	DeployModeSchool int16 = 2
)

const (
	AuthModeLocal int16 = 1
	AuthModeCAS   int16 = 2
	AuthModeLDAP  int16 = 3
)

const (
	ApplicationStatusPending  int16 = 1
	ApplicationStatusApproved int16 = 2
	ApplicationStatusRejected int16 = 3
)

const (
	BaseIdentityStudent int16 = 1
	BaseIdentityTeacher int16 = 2
)

const (
	AccountStatusPending   int16 = 1
	AccountStatusActive    int16 = 2
	AccountStatusDisabled  int16 = 3
	AccountStatusArchived  int16 = 4
	AccountStatusCancelled int16 = 5
)

const (
	SessionStatusActive  int16 = 1
	SessionStatusRevoked int16 = 2
)

const (
	SMSSceneLogin       int16 = 1
	SMSSceneReset       int16 = 2
	SMSSceneChangePhone int16 = 3
)

const (
	ActivationStatusActive  int16 = 1
	ActivationStatusUsed    int16 = 2
	ActivationStatusRevoked int16 = 3
)

const (
	SSOTypeCAS  int16 = 1
	SSOTypeLDAP int16 = 2
)

const (
	SSOMatchNo    int16 = 1
	SSOMatchPhone int16 = 2
)

const (
	ImportTargetTeacher int16 = 1
	ImportTargetStudent int16 = 2
	ImportTargetOrg     int16 = 3
)

const (
	ImportPreviewPending   int16 = 1
	ImportPreviewSubmitted int16 = 2
	ImportPreviewExpired   int16 = 3
)

const (
	ImportBatchProcessing int16 = 1
	ImportBatchCompleted  int16 = 2
	ImportBatchFailed     int16 = 3
)

const (
	ClassStatusActive   int16 = 1
	ClassStatusArchived int16 = 2
)
