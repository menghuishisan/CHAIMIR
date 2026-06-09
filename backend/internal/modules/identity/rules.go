// identity 规则文件集中放置输入校验和状态机校验,不访问数据库或跨模块契约。
package identity

import (
	"regexp"
	"strings"
	"time"

	"chaimir/pkg/apperr"
)

var (
	phoneRe      = regexp.MustCompile(`^1[3-9]\d{9}$`)
	tenantCodeRe = regexp.MustCompile(`^[a-z][a-z0-9-]{1,30}[a-z0-9]$`)
)

// ValidatePhone 校验国内高校场景使用的中国大陆手机号。
func ValidatePhone(phone string) error {
	if !phoneRe.MatchString(strings.TrimSpace(phone)) {
		return apperr.ErrIdentityInvalidPhone
	}
	return nil
}

// MaskPhone 按安全规范对手机号做用户向掩码展示。
func MaskPhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if len(phone) != 11 {
		return ""
	}
	return phone[:3] + "****" + phone[7:]
}

// ValidatePassword 校验本地密码强度,避免弱口令进入哈希流程。
func ValidatePassword(password string) error {
	if len(password) < 8 {
		return apperr.ErrIdentityWeakPassword
	}
	hasLetter, hasDigit := false, false
	for _, r := range password {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			hasLetter = true
		}
		if r >= '0' && r <= '9' {
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return apperr.ErrIdentityWeakPassword
	}
	return nil
}

// ValidateTenantCode 校验租户短码,防止把路径或特殊字符写入全局入口。
func ValidateTenantCode(code string) error {
	if !tenantCodeRe.MatchString(strings.TrimSpace(code)) {
		return apperr.ErrIdentityInvalidTenantCode
	}
	return nil
}

// ValidateAuthMode 校验租户认证模式稳定取值,避免非法配置影响登录入口判断。
func ValidateAuthMode(mode int16) error {
	switch mode {
	case AuthModeLocal, AuthModeCAS, AuthModeLDAP:
		return nil
	default:
		return apperr.ErrIdentityTenantConfigInvalid
	}
}

// EnsureAccountCanLogin 校验账号状态是否允许进入认证成功路径。
func EnsureAccountCanLogin(account Account, now time.Time) error {
	if account.LockedUntil != nil && now.Before(*account.LockedUntil) {
		return apperr.ErrIdentityAccountLocked
	}
	if account.Status != AccountStatusActive && account.Status != AccountStatusPending {
		return apperr.ErrIdentityAccountDisabled
	}
	return nil
}

// EnsureTenantCanLogin 校验租户状态是否允许校内账号登录。
func EnsureTenantCanLogin(tenant Tenant, now time.Time) error {
	if tenant.Status == TenantStatusDisabled {
		return apperr.ErrIdentityTenantDisabled
	}
	if tenant.Status == TenantStatusExpired || (tenant.ExpireAt != nil && now.After(*tenant.ExpireAt)) {
		return apperr.ErrIdentityTenantExpired
	}
	return nil
}

// BaseRole 返回基础身份对应的固定 RBAC 角色。
func BaseRole(baseIdentity int16) (int16, error) {
	switch baseIdentity {
	case BaseIdentityTeacher:
		return RoleTeacher, nil
	case BaseIdentityStudent:
		return RoleStudent, nil
	default:
		return 0, apperr.ErrIdentityBaseRoleInvalid
	}
}
