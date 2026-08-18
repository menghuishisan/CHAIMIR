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
	emailRe      = regexp.MustCompile(`^[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}$`)
)

// ValidatePhone 校验国内高校场景使用的中国大陆手机号。
func ValidatePhone(phone string) error {
	if !phoneRe.MatchString(strings.TrimSpace(phone)) {
		return apperr.ErrIdentityInvalidPhone
	}
	return nil
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

// ValidateEmail 校验平台入驻联系人邮箱,避免审核和通知链路写入不可用地址。
func ValidateEmail(email string) error {
	if !emailRe.MatchString(strings.TrimSpace(email)) {
		return apperr.ErrIdentityApplicationInvalid
	}
	return nil
}

// ValidateAccountStatusTransition 校验管理员可触发的账号状态机,开通只能由激活、首登改密或 SSO 首登完成。
func ValidateAccountStatusTransition(fromStatus, toStatus int16) error {
	switch toStatus {
	case AccountStatusActive, AccountStatusDisabled, AccountStatusArchived, AccountStatusCancelled:
	default:
		return apperr.ErrIdentityAccountUpdateInvalid
	}
	// 注销是不可逆的终态，待激活账号也必须能被管理员取消，避免遗留无法登录且无法清理的账号。
	if fromStatus == AccountStatusPending && toStatus != AccountStatusCancelled {
		return apperr.ErrIdentityAccountUpdateInvalid
	}
	if fromStatus == AccountStatusCancelled && toStatus != AccountStatusCancelled {
		return apperr.ErrIdentityAccountUpdateInvalid
	}
	if fromStatus == AccountStatusDisabled && toStatus == AccountStatusArchived {
		return apperr.ErrIdentityAccountUpdateInvalid
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

// ValidateTenantFeatureFlags 校验租户功能开关中的 modules 封闭集合,缺省未配置表示全部启用。
func ValidateTenantFeatureFlags(flags map[string]any) error {
	raw, ok := flags["modules"]
	if !ok {
		return nil
	}
	items, ok := raw.([]any)
	if !ok || len(items) == 0 {
		return apperr.ErrIdentityTenantConfigInvalid
	}
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		module, ok := item.(string)
		if !ok || !isKnownTenantModule(module) {
			return apperr.ErrIdentityTenantConfigInvalid
		}
		if _, exists := seen[module]; exists {
			return apperr.ErrIdentityTenantConfigInvalid
		}
		seen[module] = struct{}{}
	}
	return nil
}

// isKnownTenantModule 判断租户功能开关是否属于平台登记的模块集合。
func isKnownTenantModule(module string) bool {
	switch module {
	case TenantModuleTeaching, TenantModuleExperiment, TenantModuleContest, TenantModuleSim, TenantModuleGrade:
		return true
	default:
		return false
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

// ValidateBaseIdentity 校验基础身份取值,角色契约映射由 service 边界负责。
func ValidateBaseIdentity(baseIdentity int16) error {
	switch baseIdentity {
	case BaseIdentityTeacher:
		return nil
	case BaseIdentityStudent:
		return nil
	default:
		return apperr.ErrIdentityBaseRoleInvalid
	}
}
