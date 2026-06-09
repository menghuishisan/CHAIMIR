// SSO 协议适配器:CAS 登录/验票与 LDAP 绑定/查询。
package identity

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"chaimir/internal/platform/netx"
	"chaimir/pkg/apperr"

	cas "github.com/cloudogu/go-cas/v2"
	ldap "github.com/go-ldap/ldap/v3"
)

// validateSsoConfigForStorage 在保存配置时做协议级必填校验。
func validateSsoConfigForStorage(typ int16, cfg map[string]any) error {
	switch typ {
	case SsoTypeCAS:
		if _, err := parseRequiredURL(cfg, "server_url"); err != nil {
			return apperr.ErrSsoCASURLInvalid
		}
		return nil
	case SsoTypeLDAP:
		if _, err := parseLDAPConfig(cfg); err != nil {
			return apperr.ErrSsoLDAPConfigInvalid
		}
		return nil
	default:
		return apperr.ErrSsoTypeInvalid
	}
}

// casProfile 是 CAS 服务返回的身份标识。
type casProfile struct {
	Username   string
	Attributes map[string][]string
}

// ldapProfile 是 LDAP 目录查询得到的身份标识。
type ldapProfile struct {
	MatchValue string
}

// buildCASLoginURL 依据 CAS 标准 URLScheme 生成登录地址。
func buildCASLoginURL(cfg map[string]any, serviceURL string, allowedServiceOrigins []string) (string, error) {
	base, err := parseRequiredURL(cfg, "server_url")
	if err != nil {
		return "", err
	}
	service, err := parseAllowedCASServiceURL(serviceURL, allowedServiceOrigins)
	if err != nil {
		return "", err
	}
	loginURL, err := cas.NewDefaultURLScheme(base).Login()
	if err != nil {
		return "", err
	}
	q := loginURL.Query()
	q.Set("service", service.String())
	loginURL.RawQuery = q.Encode()
	return loginURL.String(), nil
}

// validateCASTicket 调用 CAS serviceValidate 校验票据并解析用户属性。
func validateCASTicket(cfg map[string]any, ticket, serviceURL string, allowedServiceOrigins []string, timeout time.Duration) (*casProfile, error) {
	base, err := parseRequiredURL(cfg, "server_url")
	if err != nil {
		return nil, err
	}
	service, err := parseAllowedCASServiceURL(serviceURL, allowedServiceOrigins)
	if err != nil {
		return nil, err
	}
	client := netx.NewPublicHTTPClient(timeout)
	validator := cas.NewServiceTicketValidator(client, cas.NewDefaultURLScheme(base))
	resp, err := validator.ValidateTicket(service, ticket)
	if err != nil {
		return nil, err
	}
	if resp == nil || strings.TrimSpace(resp.User) == "" {
		return nil, fmt.Errorf("CAS 未返回有效用户")
	}
	return &casProfile{Username: resp.User, Attributes: map[string][]string(resp.Attributes)}, nil
}

// parseAllowedCASServiceURL 校验 CAS service 回调地址属于平台白名单 origin。
func parseAllowedCASServiceURL(serviceURL string, allowedOrigins []string) (*url.URL, error) {
	service, err := url.Parse(serviceURL)
	if err != nil || service.Scheme == "" || service.Host == "" {
		return nil, fmt.Errorf("CAS service 地址不正确")
	}
	serviceOrigin := service.Scheme + "://" + service.Host
	for _, raw := range allowedOrigins {
		allowed, err := url.Parse(strings.TrimSpace(raw))
		if err != nil || allowed.Scheme == "" || allowed.Host == "" {
			continue
		}
		if serviceOrigin == allowed.Scheme+"://"+allowed.Host {
			return service, nil
		}
	}
	return nil, fmt.Errorf("CAS service 地址不在允许范围")
}

// ldapConfig 是 LDAP 连接与匹配所需的完整配置。
type ldapConfig struct {
	URL            string
	BindDN         string
	BindPassword   string
	BaseDN         string
	UserFilter     string
	MatchAttribute string
}

// parseLDAPConfig 校验 LDAP 配置完整性,避免运行时半配置导致错误不可定位。
func parseLDAPConfig(cfg map[string]any) (ldapConfig, error) {
	out := ldapConfig{
		URL:            stringFromMap(cfg, "url"),
		BindDN:         stringFromMap(cfg, "bind_dn"),
		BindPassword:   stringFromMap(cfg, "bind_password"),
		BaseDN:         stringFromMap(cfg, "base_dn"),
		UserFilter:     stringFromMap(cfg, "user_filter"),
		MatchAttribute: stringFromMap(cfg, "match_attribute"),
	}
	if out.URL == "" || out.BindDN == "" || out.BindPassword == "" ||
		out.BaseDN == "" || out.UserFilter == "" || out.MatchAttribute == "" {
		return ldapConfig{}, fmt.Errorf("LDAP 配置不完整")
	}
	if !strings.Contains(out.UserFilter, "%s") {
		return ldapConfig{}, fmt.Errorf("LDAP user_filter 必须包含 %%s 参数位")
	}
	normalizedURL, err := netx.ValidatePublicLDAPSURL(out.URL)
	if err != nil {
		return ldapConfig{}, fmt.Errorf("LDAP url 不正确")
	}
	out.URL = normalizedURL
	if v, ok := cfg["insecure_skip_verify"].(bool); ok && v {
		return ldapConfig{}, fmt.Errorf("LDAP 不允许跳过 TLS 证书校验")
	}
	return out, nil
}

// authenticateLDAP 使用服务账号搜索用户 DN,再用用户密码绑定确认身份。
func authenticateLDAP(ctx context.Context, cfg map[string]any, username, password string, timeout time.Duration) (profile *ldapProfile, err error) {
	if strings.TrimSpace(username) == "" || strings.TrimSpace(password) == "" {
		return nil, fmt.Errorf("LDAP 用户名或密码为空")
	}
	// 第一步解析并建立受控 TLS 连接,避免把未校验配置直接用于外部认证。
	parsed, err := parseLDAPConfig(cfg)
	if err != nil {
		return nil, err
	}
	conn, err := dialLDAP(ctx, parsed, timeout)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()

	// 第二步用服务账号搜索唯一用户 DN,不允许模糊匹配结果继续认证。
	if err := conn.Bind(parsed.BindDN, parsed.BindPassword); err != nil {
		return nil, err
	}
	filter := fmt.Sprintf(parsed.UserFilter, ldap.EscapeFilter(username))
	req := ldap.NewSearchRequest(
		parsed.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		2,
		int(timeout.Seconds()),
		false,
		filter,
		[]string{parsed.MatchAttribute},
		nil,
	)
	result, err := conn.Search(req)
	if err != nil {
		return nil, err
	}
	if len(result.Entries) != 1 {
		return nil, fmt.Errorf("LDAP 用户匹配结果不是唯一")
	}
	userDN := result.Entries[0].DN
	matchValue := result.Entries[0].GetAttributeValue(parsed.MatchAttribute)
	if strings.TrimSpace(matchValue) == "" {
		return nil, fmt.Errorf("LDAP 用户缺少匹配属性")
	}
	// 第三步再用用户密码绑定,只有目录服务确认密码后才返回本地匹配属性。
	if err := conn.Bind(userDN, password); err != nil {
		return nil, err
	}
	return &ldapProfile{MatchValue: matchValue}, nil
}

// dialLDAP 创建带超时和 TLS 设置的 LDAP 连接。
func dialLDAP(ctx context.Context, cfg ldapConfig, timeout time.Duration) (*ldap.Conn, error) {
	dialer := &net.Dialer{Timeout: timeout}
	if deadline, ok := ctx.Deadline(); ok {
		dialer.Timeout = time.Until(deadline)
	}
	// 第一步:解析并固定为已验证公网地址,避免域名在保存校验后解析到内网。
	resolvedURL, serverName, err := netx.PublicResolvedURL(ctx, cfg.URL, "636")
	if err != nil {
		return nil, err
	}
	// 第二步:拨号目标使用公网 IP,TLS 仍按原域名校验证书。
	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: serverName}
	return ldap.DialURL(resolvedURL, ldap.DialWithDialer(dialer), ldap.DialWithTLSConfig(tlsCfg))
}

// parseRequiredURL 从配置读取必填 URL。
func parseRequiredURL(cfg map[string]any, key string) (*url.URL, error) {
	raw := stringFromMap(cfg, key)
	normalizedURL, err := netx.ValidatePublicHTTPURL(raw)
	if err != nil {
		return nil, fmt.Errorf("%s 地址不正确", key)
	}
	u, err := url.Parse(normalizedURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("%s 地址不正确", key)
	}
	return u, nil
}

// stringFromMap 安全读取字符串配置并去除首尾空白。
func stringFromMap(cfg map[string]any, key string) string {
	if v, ok := cfg[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}
