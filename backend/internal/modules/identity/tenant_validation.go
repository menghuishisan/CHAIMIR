// M1 租户与配置枚举校验。
package identity

import "chaimir/pkg/apperr"

// validateTenantStatus 校验租户状态机枚举。
func validateTenantStatus(status int16) error {
	switch status {
	case TenantActive, TenantDisabled, TenantExpired:
		return nil
	default:
		return apperr.ErrTenantStatusInvalid
	}
}

// validateOptionalTenantStatus 校验可选租户状态查询参数;0 表示不过滤。
func validateOptionalTenantStatus(status int16) error {
	if status == 0 {
		return nil
	}
	return validateTenantStatus(status)
}

// validateAuthMode 校验租户认证方式枚举。
func validateAuthMode(mode int16) error {
	switch mode {
	case AuthModeLocal, AuthModeCAS, AuthModeLDAP:
		return nil
	default:
		return apperr.ErrTenantAuthModeInvalid
	}
}

// validateSchoolType 校验学校类型枚举。
func validateSchoolType(kind int16) error {
	switch kind {
	case SchoolTypeDoctor, SchoolTypeMaster, SchoolTypeCollege, SchoolTypeJunior:
		return nil
	default:
		return apperr.ErrSchoolTypeInvalid
	}
}

// validateApplicationStatus 校验入驻申请状态查询参数;0 表示不过滤。
func validateApplicationStatus(status int16) error {
	switch status {
	case 0, ApplicationPending, ApplicationApproved, ApplicationRejected:
		return nil
	default:
		return apperr.ErrApplicationStatusInvalid
	}
}
