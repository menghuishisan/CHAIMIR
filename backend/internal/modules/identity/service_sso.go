// identity service_sso 文件实现 CAS/LDAP 配置校验和 SSO 入口安全边界。
package identity

import (
	"context"
	"crypto/tls"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/netx"
	"chaimir/internal/platform/secretmap"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
	ldap "github.com/go-ldap/ldap/v3"
	"github.com/jackc/pgx/v5"
)

// casServiceResponse 描述 CAS serviceValidate 成功响应中身份模块需要的字段。
type casServiceResponse struct {
	XMLName xml.Name `xml:"serviceResponse"`
	Success *struct {
		User string `xml:"user"`
	} `xml:"authenticationSuccess"`
	Failure *struct {
		Code string `xml:"code,attr"`
		Text string `xml:",chardata"`
	} `xml:"authenticationFailure"`
}

// UpsertSSOConfig 保存 CAS/LDAP 配置,校验外部端点避免 SSRF 和明文 LDAP。
func (s *Service) UpsertSSOConfig(ctx context.Context, req SSOConfigRequest) (SSOConfig, error) {
	id, err := requireTenantRole(ctx, s, contracts.RoleSchoolAdmin)
	if err != nil {
		return SSOConfig{}, err
	}
	if err := s.validateSSOConfig(req); err != nil {
		return SSOConfig{}, err
	}
	var out SSOConfig
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		old, err := tx.GetSSOConfig(ctx, id.TenantID, req.Type)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		// 配置入库前先处理敏感字段,脱敏占位表示沿用旧密文而不是写入展示文案。
		configData, err := s.secureSSOConfig(req, old)
		if err != nil {
			return err
		}
		raw, err := jsonx.ObjectBytes(configData, apperr.ErrIdentitySSOConfigInvalid)
		if err != nil {
			return err
		}
		row, err := tx.UpsertSSOConfig(ctx, UpsertSSOInput{ID: s.ids.Generate(), TenantID: id.TenantID, Type: req.Type, Config: raw, MatchField: req.MatchField, Enabled: req.Enabled})
		if err != nil {
			return err
		}
		out = row
		return nil
	}); err != nil {
		if _, ok := apperr.As(err); ok {
			return SSOConfig{}, err
		}
		return SSOConfig{}, apperr.ErrInternal.WithCause(err)
	}
	if err := s.auditTenantOperation(ctx, id, "tenant.sso.update", "identity.sso_config", out.ID, map[string]any{"type": req.Type, "enabled": req.Enabled}); err != nil {
		return SSOConfig{}, err
	}
	return out, nil
}

// CASLoginURL 生成 CAS 跳转地址,service origin 必须命中白名单。
func (s *Service) CASLoginURL(ctx context.Context, tenantCode string, serviceURL string) (string, error) {
	if err := ValidateTenantCode(tenantCode); err != nil {
		return "", err
	}
	if !s.serviceOriginAllowed(serviceURL) {
		return "", apperr.ErrIdentitySSOServiceOriginDenied
	}
	// 只读取已启用配置生成跳转地址,未启用租户不能暴露外部认证入口。
	_, cfg, err := s.loadEnabledSSOConfig(ctx, tenantCode, SSOTypeCAS)
	if err != nil {
		return "", err
	}
	data, err := jsonx.ObjectMapStrict(cfg.Config)
	if err != nil {
		return "", apperr.ErrInternal.WithCause(err)
	}
	server, _ := data["server_url"].(string)
	return casLoginURL(server, serviceURL)
}

// CASCallback 校验 CAS Service Ticket 并按已导入名单签发租户 token。
func (s *Service) CASCallback(ctx context.Context, tenantCode, ticket, serviceURL, device, ip string) (LoginResponse, error) {
	if err := ValidateTenantCode(tenantCode); err != nil {
		return LoginResponse{}, err
	}
	if strings.TrimSpace(ticket) == "" {
		return LoginResponse{}, apperr.ErrIdentitySSOTicketInvalid
	}
	if !s.serviceOriginAllowed(serviceURL) {
		return LoginResponse{}, apperr.ErrIdentitySSOServiceOriginDenied
	}
	tenantID, cfg, err := s.loadEnabledSSOConfig(ctx, tenantCode, SSOTypeCAS)
	if err != nil {
		return LoginResponse{}, err
	}
	username, err := s.validateCASTicket(ctx, cfg, strings.TrimSpace(ticket), serviceURL)
	if err != nil {
		return LoginResponse{}, err
	}
	// SSO 只验证身份,账号必须已由学校管理员导入,严禁回调时自动建号。
	account, err := s.matchSSOAccount(ctx, tenantID, cfg.MatchField, username)
	if err != nil {
		return LoginResponse{}, err
	}
	return s.finishSSOLogin(ctx, tenantID, account, device, ip)
}

// LDAPLogin 使用学校 LDAPS 配置完成目录绑定,再按已导入名单签发租户 token。
func (s *Service) LDAPLogin(ctx context.Context, tenantCode string, req LDAPLoginRequest, device, ip string) (LoginResponse, error) {
	if err := ValidateTenantCode(tenantCode); err != nil {
		return LoginResponse{}, err
	}
	if strings.TrimSpace(req.Username) == "" || strings.TrimSpace(req.Password) == "" {
		return LoginResponse{}, apperr.ErrIdentityInvalidCredentials
	}
	tenantID, cfg, err := s.loadEnabledSSOConfig(ctx, tenantCode, SSOTypeLDAP)
	if err != nil {
		return LoginResponse{}, err
	}
	matchValue, err := s.validateLDAPCredentials(ctx, cfg, strings.TrimSpace(req.Username), req.Password)
	if err != nil {
		return LoginResponse{}, err
	}
	// LDAP 与 CAS 统一名单匹配语义:目录认证成功也不能绕过本地已导入名单。
	account, err := s.matchSSOAccount(ctx, tenantID, cfg.MatchField, matchValue)
	if err != nil {
		return LoginResponse{}, err
	}
	return s.finishSSOLogin(ctx, tenantID, account, device, ip)
}

// validateSSOConfig 校验 SSO 配置的协议安全要求。
func (s *Service) validateSSOConfig(req SSOConfigRequest) error {
	if req.MatchField != SSOMatchNo && req.MatchField != SSOMatchPhone {
		return apperr.ErrIdentitySSOMatchFieldInvalid
	}
	switch req.Type {
	case SSOTypeCAS:
		raw, _ := req.Config["server_url"].(string)
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			return apperr.ErrIdentitySSOCASServerInsecure
		}
		if _, err := netx.ValidatePublicHTTPURL(raw); err != nil {
			return apperr.ErrIdentitySSOCASServerInsecure
		}
	case SSOTypeLDAP:
		raw, _ := req.Config["url"].(string)
		if _, err := netx.ValidatePublicLDAPSURL(raw); err != nil {
			return apperr.ErrIdentityLDAPServerInsecure
		}
		required := []string{"bind_dn", "bind_password", "base_dn", "user_filter", "match_attribute"}
		for _, key := range required {
			if stringField(req.Config, key) == "" {
				return apperr.ErrIdentitySSOConfigInvalid
			}
		}
	default:
		return apperr.ErrIdentitySSOTypeInvalid
	}
	return nil
}

// secureSSOConfig 在配置入库前复用基础层加密凭据字段,避免敏感配置明文落库。
func (s *Service) secureSSOConfig(req SSOConfigRequest, old SSOConfig) (map[string]any, error) {
	out := make(map[string]any, len(req.Config))
	for key, value := range req.Config {
		out[key] = value
	}
	if req.Type != SSOTypeLDAP {
		return out, nil
	}
	password := stringField(out, "bind_password")
	if password == "" {
		return nil, apperr.ErrIdentitySSOConfigInvalid
	}
	if password == secretmap.MaskedValue {
		oldData, err := jsonx.ObjectMapStrict(old.Config)
		if err != nil {
			return nil, apperr.ErrIdentitySSOConfigInvalid.WithCause(err)
		}
		oldPassword, ok := oldData["bind_password"]
		if !ok {
			return nil, apperr.ErrIdentitySSOConfigInvalid
		}
		out["bind_password"] = oldPassword
	}
	protected, err := secretmap.Protect(s.cipher, out, "identity sso config")
	if err != nil {
		return nil, apperr.ErrIdentitySSOSecretInvalid.WithCause(err)
	}
	return protected, nil
}

// validateLDAPCredentials 通过 LDAPS 目录验证用户密码并读取名单匹配字段。
func (s *Service) validateLDAPCredentials(ctx context.Context, cfg SSOConfig, username, password string) (string, error) {
	data, err := s.ldapConfigFromJSON(cfg.Config)
	if err != nil {
		return "", err
	}
	resolvedURL, serverName, err := netx.PrivateResolvedURL(ctx, data.URL, "636")
	if err != nil {
		return "", apperr.ErrIdentityLDAPServerInsecure.WithCause(err)
	}
	conn, err := ldap.DialURL(resolvedURL, ldap.DialWithTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12, ServerName: serverName}))
	if err != nil {
		return "", apperr.ErrIdentityInvalidCredentials.WithCause(err)
	}
	defer logging.CloseContext(ctx, "关闭 LDAP 连接失败", conn)
	// 先使用学校配置的服务账号绑定,只用于目录查询,不代表本系统登录成功。
	if err := conn.Bind(data.BindDN, data.BindPassword); err != nil {
		return "", apperr.ErrIdentityInvalidCredentials.WithCause(err)
	}
	// 用户名进入 LDAP filter 前必须转义,避免目录查询注入。
	filter := strings.ReplaceAll(data.UserFilter, "{username}", ldap.EscapeFilter(username))
	search := ldap.NewSearchRequest(
		data.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		2,
		s.cfg.SSONetworkTimeoutSeconds,
		false,
		filter,
		[]string{data.MatchAttribute},
		nil,
	)
	result, err := conn.Search(search)
	if err != nil {
		return "", apperr.ErrIdentityInvalidCredentials.WithCause(err)
	}
	if len(result.Entries) != 1 {
		return "", apperr.ErrIdentitySSOAccountNotMatched
	}
	// 目录命中后取配置指定的名单匹配字段,仍需后续匹配本地已导入账号。
	userDN := result.Entries[0].DN
	matchValue := strings.TrimSpace(result.Entries[0].GetAttributeValue(data.MatchAttribute))
	if matchValue == "" {
		return "", apperr.ErrIdentitySSOAccountNotMatched
	}
	// 最后用用户 DN 和用户密码绑定,确认密码属于该目录账号本人。
	if err := conn.Bind(userDN, password); err != nil {
		return "", apperr.ErrIdentityInvalidCredentials.WithCause(err)
	}
	return matchValue, nil
}

// ldapConfig 描述租户 LDAP 配置 JSON 中服务端使用的字段。
type ldapConfig struct {
	URL            string
	BindDN         string
	BindPassword   string
	BaseDN         string
	UserFilter     string
	MatchAttribute string
}

// ldapConfigFromJSON 解析、还原并校验 LDAP 配置字段,避免缺字段时发起不确定外部请求。
func (s *Service) ldapConfigFromJSON(raw []byte) (ldapConfig, error) {
	data, err := jsonx.ObjectMapStrict(raw)
	if err != nil {
		return ldapConfig{}, apperr.ErrIdentitySSOConfigInvalid.WithCause(err)
	}
	revealed, err := secretmap.Reveal(s.cipher, data, "identity sso config")
	if err != nil {
		return ldapConfig{}, apperr.ErrIdentitySSOSecretInvalid.WithCause(err)
	}
	cfg := ldapConfig{
		URL:            stringField(revealed, "url"),
		BindDN:         stringField(revealed, "bind_dn"),
		BindPassword:   stringField(revealed, "bind_password"),
		BaseDN:         stringField(revealed, "base_dn"),
		UserFilter:     stringField(revealed, "user_filter"),
		MatchAttribute: stringField(revealed, "match_attribute"),
	}
	if cfg.URL == "" || cfg.BindDN == "" || cfg.BindPassword == "" || cfg.BaseDN == "" || cfg.UserFilter == "" || cfg.MatchAttribute == "" {
		return ldapConfig{}, apperr.ErrIdentitySSOConfigInvalid
	}
	return cfg, nil
}

// stringField 从配置 JSON map 中读取字符串字段并去除空白。
func stringField(data map[string]any, key string) string {
	value, _ := data[key].(string)
	return strings.TrimSpace(value)
}

// loadEnabledSSOConfig 读取租户启用中的指定 SSO 配置,避免 API 层碰数据库。
func (s *Service) loadEnabledSSOConfig(ctx context.Context, tenantCode string, typ int16) (int64, SSOConfig, error) {
	var tenantID int64
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		t, err := tx.GetTenantByCode(ctx, tenantCode)
		if err != nil {
			return err
		}
		tenantID = t.ID
		return nil
	}); err != nil {
		return 0, SSOConfig{}, apperr.ErrIdentitySSONotEnabled
	}
	var cfg SSOConfig
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		row, err := tx.GetSSOConfig(ctx, tenantID, typ)
		if err != nil {
			return err
		}
		cfg = row
		return nil
	}); err != nil || !cfg.Enabled {
		return 0, SSOConfig{}, apperr.ErrIdentitySSONotEnabled
	}
	return tenantID, cfg, nil
}

// validateCASTicket 通过 CAS serviceValidate 校验票据并返回 CAS 用户标识。
func (s *Service) validateCASTicket(ctx context.Context, cfg SSOConfig, ticket, serviceURL string) (string, error) {
	data, err := jsonx.ObjectMapStrict(cfg.Config)
	if err != nil {
		return "", apperr.ErrIdentitySSOResponseInvalid.WithCause(err)
	}
	server, _ := data["server_url"].(string)
	endpoint, err := casValidateURL(server, ticket, serviceURL)
	if err != nil {
		return "", err
	}
	// CAS 校验请求使用统一公网受限 HTTP client,防止租户配置导致服务端访问内网。
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", apperr.ErrIdentitySSOInsecureConfig.WithCause(err)
	}
	client, err := netx.NewPublicHTTPClient(time.Duration(s.cfg.SSONetworkTimeoutSeconds) * time.Second)
	if err != nil {
		return "", apperr.ErrIdentitySSOInsecureConfig.WithCause(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", apperr.ErrIdentitySSOTicketInvalid.WithCause(err)
	}
	defer logging.CloseContext(ctx, "关闭 CAS 校验响应失败", resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", apperr.ErrIdentitySSOTicketInvalid
	}
	if s.cfg.SSOCASResponseMaxBytes <= 0 {
		return "", apperr.ErrIdentitySSOResponseInvalid
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, s.cfg.SSOCASResponseMaxBytes+1))
	if err != nil {
		return "", apperr.ErrIdentitySSOResponseInvalid.WithCause(err)
	}
	if int64(len(body)) > s.cfg.SSOCASResponseMaxBytes {
		return "", apperr.ErrIdentitySSOResponseInvalid
	}
	var parsed casServiceResponse
	if err := xml.Unmarshal(body, &parsed); err != nil {
		return "", apperr.ErrIdentitySSOResponseInvalid.WithCause(err)
	}
	if parsed.Success == nil || strings.TrimSpace(parsed.Success.User) == "" {
		return "", apperr.ErrIdentitySSOTicketInvalid
	}
	return strings.TrimSpace(parsed.Success.User), nil
}

// casValidateURL 构造 CAS serviceValidate 地址,只允许 HTTPS CAS 服务端点。
func casValidateURL(server, ticket, serviceURL string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(server))
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return "", apperr.ErrIdentitySSOCASServerInsecure
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/serviceValidate"
	q := u.Query()
	q.Set("ticket", ticket)
	q.Set("service", serviceURL)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// casLoginURL 构造 CAS 登录跳转地址,显式拼接 /login 避免把用户带到 CAS 根路径。
func casLoginURL(server, serviceURL string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(server))
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return "", apperr.ErrIdentitySSOCASServerInsecure
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/login"
	q := u.Query()
	q.Set("service", serviceURL)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// matchSSOAccount 按 SSO 配置的名单匹配字段查找已导入账号,未命中不得自动创建。
func (s *Service) matchSSOAccount(ctx context.Context, tenantID int64, matchField int16, value string) (Account, error) {
	var account Account
	err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		switch matchField {
		case SSOMatchNo:
			row, err := tx.GetAccountByNo(ctx, strings.TrimSpace(value))
			if err != nil {
				return err
			}
			account = row
		case SSOMatchPhone:
			if err := ValidatePhone(value); err != nil {
				return err
			}
			hash, err := s.phoneHash(value)
			if err != nil {
				return err
			}
			row, err := tx.GetAccountByPhoneHash(ctx, tenantID, hash)
			if err != nil {
				return err
			}
			account = row
		default:
			return apperr.ErrIdentitySSOMatchFieldInvalid
		}
		return nil
	})
	if err != nil {
		if _, ok := apperr.As(err); ok {
			return Account{}, err
		}
		return Account{}, apperr.ErrIdentitySSOAccountNotMatched.WithCause(fmt.Errorf("match sso account: %w", err))
	}
	return account, nil
}

// finishSSOLogin 校验租户和名单账号状态,并把 SSO 首登的待激活账号推进为正常账号。
func (s *Service) finishSSOLogin(ctx context.Context, tenantID int64, account Account, device, ip string) (LoginResponse, error) {
	var tenantSnapshot Tenant
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		t, err := tx.GetTenantByID(ctx, tenantID)
		if err != nil {
			return err
		}
		if account.Status == AccountStatusPending {
			// SSO 首登只证明已导入名单账号完成外部认证,不创建账号或补组织档案。
			activated, err := tx.ActivateSSOAccount(ctx, account.ID, tenantID)
			if err != nil {
				return err
			}
			account = activated
		}
		tenantSnapshot = t
		return nil
	}); err != nil {
		return LoginResponse{}, apperr.ErrInternal.WithCause(err)
	}
	if err := EnsureTenantCanLogin(tenantSnapshot, timex.Now()); err != nil {
		return LoginResponse{}, err
	}
	if err := EnsureAccountCanLogin(account, timex.Now()); err != nil {
		return LoginResponse{}, err
	}
	return s.issueTenantLogin(ctx, account, device, ip)
}

// serviceOriginAllowed 校验 CAS service 回调 origin 是否在部署白名单内。
func (s *Service) serviceOriginAllowed(serviceURL string) bool {
	u, err := url.Parse(serviceURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}
	origin := u.Scheme + "://" + u.Host
	for _, allowed := range s.cfg.SSOAllowedServiceOrigins {
		if strings.EqualFold(strings.TrimSpace(allowed), origin) {
			return true
		}
	}
	return false
}
