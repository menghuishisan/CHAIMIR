// M1 SSO 认证服务:CAS 与 LDAP 登录、名单匹配、Token 签发。
package identity

import (
	"context"
	"fmt"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
)

// BuildSsoLoginURL 读取租户 CAS 配置并生成登录跳转地址。
func (s *Service) BuildSsoLoginURL(ctx context.Context, tenantCode, serviceURL string) (string, error) {
	tenantID, err := s.tenantIDByCode(ctx, tenantCode)
	if err != nil {
		return "", err
	}
	cfg, typ, _, err := s.loadSsoConfig(ctx, tenantID)
	if err != nil {
		return "", err
	}
	if typ != SsoTypeCAS {
		return "", apperr.ErrSsoUnavailable
	}
	redirectURL, err := buildCASLoginURL(cfg, serviceURL, s.ssoAllowedServiceOrigins)
	if err != nil {
		return "", apperr.ErrSsoUnavailable.WithCause(err)
	}
	return redirectURL, nil
}

// LoginByCasCallback 校验 CAS ticket 后按名单匹配账号并签发 Token。
func (s *Service) LoginByCasCallback(ctx context.Context, tenantCode, ticket, serviceURL, device, ip string) (*LoginResult, error) {
	tenantID, err := s.tenantIDByCode(ctx, tenantCode)
	if err != nil {
		return nil, err
	}
	cfg, typ, matchField, err := s.loadSsoConfig(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if typ != SsoTypeCAS {
		return nil, apperr.ErrSsoUnavailable
	}
	profile, err := validateCASTicket(cfg, ticket, serviceURL, s.ssoAllowedServiceOrigins, s.ssoNetworkTimeout)
	if err != nil {
		return nil, apperr.ErrSsoLoginFailed.WithCause(err)
	}
	matchValue := profile.Username
	if attr := stringFromMap(cfg, "username_attribute"); attr != "" {
		if vals := profile.Attributes[attr]; len(vals) > 0 && strings.TrimSpace(vals[0]) != "" {
			matchValue = vals[0]
		}
	}
	return s.finishSsoLogin(ctx, tenantID, matchField, matchValue, device, ip)
}

// LoginByLDAP 校验 LDAP 账号密码后按名单匹配账号并签发 Token。
func (s *Service) LoginByLDAP(ctx context.Context, tenantCode string, req LDAPLoginRequest, device, ip string) (*LoginResult, error) {
	tenantID, err := s.tenantIDByCode(ctx, tenantCode)
	if err != nil {
		return nil, err
	}
	cfg, typ, matchField, err := s.loadSsoConfig(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if typ != SsoTypeLDAP {
		return nil, apperr.ErrSsoUnavailable
	}
	profile, err := authenticateLDAP(ctx, cfg, req.Username, req.Password, s.ssoNetworkTimeout)
	if err != nil {
		return nil, apperr.ErrSsoLoginFailed.WithCause(err)
	}
	return s.finishSsoLogin(ctx, tenantID, matchField, profile.MatchValue, device, ip)
}

// loadSsoConfig 读取并解密启用的 SSO 配置。
func (s *Service) loadSsoConfig(ctx context.Context, tenantID int64) (map[string]any, int16, int16, error) {
	row, err := s.repo.getEnabledSsoConfig(ctx, tenantID)
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return nil, 0, 0, ae
		}
		return nil, 0, 0, err
	}
	raw, err := jsonx.ObjectMapStrict(row.Config)
	if err != nil {
		return nil, 0, 0, apperr.ErrSsoConfigReadFailed.WithCause(err)
	}
	cfg, err := revealSsoConfig(s.cipher, raw)
	if err != nil {
		return nil, 0, 0, apperr.ErrSsoConfigReadFailed.WithCause(err)
	}
	return cfg, row.Type, row.MatchField, nil
}

// finishSsoLogin 根据匹配字段加载已导入账号,校验状态并签发租户账号 Token。
func (s *Service) finishSsoLogin(ctx context.Context, tenantID int64, matchField int16, matchValue, device, ip string) (*LoginResult, error) {
	matchValue = strings.TrimSpace(matchValue)
	if matchValue == "" {
		return nil, apperr.ErrSsoNotInRoster
	}
	acc, err := s.loadSsoAccountByMatch(ctx, tenantID, matchField, matchValue)
	if err != nil {
		return nil, err
	}
	if acc.Status == AccountPending {
		if err := s.activateSsoAccount(ctx, acc); err != nil {
			return nil, err
		}
		acc.Status = AccountActive
	} else if err := loginableStatus(acc.Status); err != nil {
		return nil, err
	}
	return s.issueLogin(ctx, acc, device, ip, true)
}

// activateSsoAccount 在 SSO 名单匹配成功后激活待激活账号,SSO 不自动创建账号。
func (s *Service) activateSsoAccount(ctx context.Context, acc LoginAccountSnapshot) error {
	entry, err := buildAccountAuditEntry(ctx, acc.TenantID, acc.ID, audit.ActorRoleFromAccount(contracts.AccountInfo{
		BaseIdentity: acc.BaseIdentity,
		Roles:        acc.Roles,
	}), AuditActionAccountUpdate, AuditTargetAccount, acc.ID, map[string]any{
		"fields": []string{"status"},
		"source": "sso",
	})
	if err != nil {
		return err
	}
	return s.repo.activateSsoAccountWithAudit(ctx, acc, buildAuditLogCreate(s.idgen.Generate(), entry))
}

// loadSsoAccountByMatch 按 SSO 配置的名单匹配字段加载账号。
func (s *Service) loadSsoAccountByMatch(ctx context.Context, tenantID int64, matchField int16, matchValue string) (LoginAccountSnapshot, error) {
	switch matchField {
	case 1:
		return s.repo.loadAccountByNo(ctx, tenantID, matchValue)
	case 2:
		return s.repo.loadAccountByPhone(ctx, tenantID, s.phoneHash(matchValue))
	default:
		return LoginAccountSnapshot{}, apperr.ErrSsoUnavailable.WithCause(fmt.Errorf("未知 SSO 匹配字段: %d", matchField))
	}
}
