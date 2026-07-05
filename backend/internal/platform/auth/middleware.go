// auth 提供用户 JWT、中间服务 HMAC 和平台角色守卫中间件。
package auth

import (
	"context"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/response"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	pkgcrypto "chaimir/pkg/crypto"
	"chaimir/pkg/logging"

	"github.com/gin-gonic/gin"
)

const (
	// BrowserAccessCookieName 是浏览器内嵌工具入口使用的路径受限 access cookie 名称。
	BrowserAccessCookieName = "chaimir_access"
	// BrowserAccessTokenQuery 是浏览器无法设置 Authorization 头时使用的一次性入口参数。
	BrowserAccessTokenQuery = "token"
	// ServiceNameHeader 标识内部服务调用方。
	ServiceNameHeader = "X-Chaimir-Service"
	// ServiceTenantHeader 显式携带内部服务请求绑定的租户边界。
	ServiceTenantHeader = "X-Chaimir-Tenant-Id"
	// ServiceSourceRefHeader 携带内部服务调用来源标识。
	ServiceSourceRefHeader = "X-Chaimir-Source-Ref"
	// ServiceTimestampHeader 携带内部服务签名时间戳。
	ServiceTimestampHeader = "X-Chaimir-Timestamp"
	// ServiceSignatureHeader 携带内部服务 HMAC-SHA256 十六进制签名。
	ServiceSignatureHeader = "X-Chaimir-Signature"
)

type serviceSourceRefKey struct{}
type browserAccessTokenSource string

const (
	browserAccessTokenSourceHeader browserAccessTokenSource = "header"
	browserAccessTokenSourceQuery  browserAccessTokenSource = "query"
	browserAccessTokenSourceCookie browserAccessTokenSource = "cookie"
)

const (
	browserAccessTokenContextKey = "auth_browser_access_token"
	browserAccessSourceKey       = "auth_browser_access_source"
)

var serviceSourceRefRe = regexp.MustCompile(`^[a-z]+:[0-9]{4}:[a-z][a-z0-9_-]*:[0-9A-Za-z_-]+$`)

// ValidSourceRef 校验 source_ref 是否符合全局四段规范。
func ValidSourceRef(sourceRef string) bool {
	return serviceSourceRefRe.MatchString(strings.TrimSpace(sourceRef))
}

// ServiceSourceRefFromContext 读取已经服务端验签后的来源标识。
func ServiceSourceRefFromContext(ctx context.Context) (string, bool) {
	sourceRef, ok := ctx.Value(serviceSourceRefKey{}).(string)
	return sourceRef, ok && strings.TrimSpace(sourceRef) != ""
}

// WithServiceSourceRef 把已验证来源标识注入上下文。
func WithServiceSourceRef(ctx context.Context, sourceRef string) context.Context {
	return context.WithValue(ctx, serviceSourceRefKey{}, sourceRef)
}

// WithServiceIdentity 为进程内跨模块契约调用建立系统服务身份边界。
func WithServiceIdentity(ctx context.Context, tenantID int64, sourceRef string) (context.Context, error) {
	sourceRef = strings.TrimSpace(sourceRef)
	if tenantID <= 0 || !ValidSourceRef(sourceRef) {
		return nil, apperr.ErrServiceUnauthorized
	}
	ctx = tenant.WithContext(ctx, tenant.Identity{TenantID: tenantID, IsSystem: true})
	return WithServiceSourceRef(ctx, sourceRef), nil
}

// ServiceSourceRefAuthorized 检查当前上下文是否允许访问目标来源;普通用户上下文不受此限制。
func ServiceSourceRefAuthorized(ctx context.Context, sourceRef string) bool {
	signedSourceRef, ok := ServiceSourceRefFromContext(ctx)
	return !ok || signedSourceRef == strings.TrimSpace(sourceRef)
}

// Middleware 校验 Bearer access token 并注入租户身份上下文。
func (m *Manager) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.AuthenticateAccess(c) {
			return
		}
		c.Next()
	}
}

// AuthenticateAccess 校验当前 HTTP 请求的 Bearer access token 并注入服务端身份上下文。
func (m *Manager) AuthenticateAccess(c *gin.Context) bool {
	claims, ok := m.accessClaims(c)
	if !ok {
		return false
	}
	if !m.validateAccessSession(c, claims) {
		return false
	}
	injectAccessIdentity(c, claims)
	return true
}

// WebSocketMiddleware 校验短时 WebSocket 票据并注入租户身份上下文。
func (m *Manager) WebSocketMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := m.webSocketTicketClaims(c)
		if !ok {
			return
		}
		if !m.validateAccessSession(c, claims) {
			return
		}
		injectAccessIdentity(c, claims)
		c.Next()
	}
}

// BrowserAccessMiddleware 校验浏览器内嵌工具入口的 Bearer、一次性 query token 或路径受限 Cookie。
func (m *Manager) BrowserAccessMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, token, source, ok := m.browserAccessClaims(c)
		if !ok {
			return
		}
		if !m.validateAccessSession(c, claims) {
			return
		}
		injectAccessIdentity(c, claims)
		c.Set(browserAccessTokenContextKey, token)
		c.Set(browserAccessSourceKey, string(source))
		c.Next()
	}
}

// ServiceMiddleware 校验内部服务 HMAC 签名并注入租户边界。
func (m *Manager) ServiceMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.injectServiceIdentity(c) {
			return
		}
		c.Next()
	}
}

// PlatformOrServiceMiddleware 允许平台管理员 JWT 或内部服务 HMAC 任一通过。
func (m *Manager) PlatformOrServiceMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if hasServiceAuthHeaders(c) {
			if !m.injectServiceIdentity(c) {
				return
			}
			c.Next()
			return
		}
		claims, ok := m.accessClaims(c)
		if !ok {
			return
		}
		if !claims.IsPlatform {
			response.Fail(c, apperr.ErrForbidden)
			c.Abort()
			return
		}
		if !m.validateAccessSession(c, claims) {
			return
		}
		injectAccessIdentity(c, claims)
		c.Next()
	}
}

// ServiceOrTenantAnyRoleMiddleware 允许内部服务签名或指定租户角色访问同一路由,用于同一 API 同时服务业务回调和教师操作。
func (m *Manager) ServiceOrTenantAnyRoleMiddleware(identity contracts.IdentityService, roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if hasServiceAuthHeaders(c) {
			if !m.injectServiceIdentity(c) {
				return
			}
			c.Next()
			return
		}
		claims, ok := m.accessClaims(c)
		if !ok {
			return
		}
		if !m.validateAccessSession(c, claims) {
			return
		}
		injectAccessIdentity(c, claims)
		if !AuthorizeTenantAnyRole(c, identity, roles...) {
			return
		}
		c.Next()
	}
}

// RequirePlatformIdentity 要求当前请求来自平台管理员身份。
func RequirePlatformIdentity() gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := tenant.FromContext(c.Request.Context())
		if !ok {
			response.Fail(c, apperr.ErrUnauthorized)
			c.Abort()
			return
		}
		if !id.IsPlatform {
			response.Fail(c, apperr.ErrForbidden)
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequirePlatformOrAnyRole 要求平台身份或租户账号具备任一指定角色。
func RequirePlatformOrAnyRole(identity contracts.IdentityService, roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !AuthorizePlatformOrAnyRole(c, identity, roles...) {
			return
		}
		c.Next()
	}
}

// RequireTenantAnyRole 要求租户账号具备任一指定角色,平台身份不会被视为租户角色。
func RequireTenantAnyRole(identity contracts.IdentityService, roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !AuthorizeTenantAnyRole(c, identity, roles...) {
			return
		}
		c.Next()
	}
}

// AuthorizePlatformOrAnyRole 执行平台或任一租户角色校验,并在失败时写统一响应。
func AuthorizePlatformOrAnyRole(c *gin.Context, identity contracts.IdentityService, roles ...string) bool {
	id, ok := tenant.FromContext(c.Request.Context())
	if !ok {
		response.Fail(c, apperr.ErrUnauthorized)
		c.Abort()
		return false
	}
	if id.IsPlatform {
		return true
	}
	if identity == nil {
		response.Fail(c, apperr.ErrForbidden)
		c.Abort()
		return false
	}
	for _, role := range roles {
		has, err := identity.HasRole(c.Request.Context(), id.AccountID, role)
		if err != nil {
			response.Fail(c, apperr.ErrForbidden.WithCause(err))
			c.Abort()
			return false
		}
		if has {
			return true
		}
	}
	response.Fail(c, apperr.ErrForbidden)
	c.Abort()
	return false
}

// AuthorizeTenantAnyRole 执行租户角色校验,并拒绝平台身份绕过租户范围。
func AuthorizeTenantAnyRole(c *gin.Context, identity contracts.IdentityService, roles ...string) bool {
	id, ok := tenant.FromContext(c.Request.Context())
	if !ok {
		response.Fail(c, apperr.ErrUnauthorized)
		c.Abort()
		return false
	}
	if id.IsPlatform || id.TenantID <= 0 {
		response.Fail(c, apperr.ErrForbidden)
		c.Abort()
		return false
	}
	if identity == nil {
		response.Fail(c, apperr.ErrForbidden)
		c.Abort()
		return false
	}
	for _, role := range roles {
		has, err := identity.HasRole(c.Request.Context(), id.AccountID, role)
		if err != nil {
			response.Fail(c, apperr.ErrForbidden.WithCause(err))
			c.Abort()
			return false
		}
		if has {
			return true
		}
	}
	response.Fail(c, apperr.ErrForbidden)
	c.Abort()
	return false
}

// accessClaims 提取并校验 Bearer access token。
func (m *Manager) accessClaims(c *gin.Context) (*Claims, bool) {
	raw := c.GetHeader("Authorization")
	token, ok := strings.CutPrefix(raw, "Bearer ")
	if !ok || strings.TrimSpace(token) == "" {
		response.Fail(c, apperr.ErrUnauthorized)
		c.Abort()
		return nil, false
	}
	claims, err := m.VerifyAccess(strings.TrimSpace(token))
	if err != nil {
		response.Fail(c, apperr.ErrUnauthorized.WithCause(err))
		c.Abort()
		return nil, false
	}
	return claims, true
}

// browserAccessClaims 按浏览器真实能力依次读取 Header、query token 和路径受限 Cookie。
func (m *Manager) browserAccessClaims(c *gin.Context) (*Claims, string, browserAccessTokenSource, bool) {
	if token, ok := bearerAccessToken(c); ok {
		claims, ok := m.verifyAccessClaims(c, token)
		return claims, token, browserAccessTokenSourceHeader, ok
	}
	if token := strings.TrimSpace(c.Query(BrowserAccessTokenQuery)); token != "" {
		claims, ok := m.verifyAccessClaims(c, token)
		return claims, token, browserAccessTokenSourceQuery, ok
	}
	if cookie, err := c.Request.Cookie(BrowserAccessCookieName); err == nil {
		token := strings.TrimSpace(cookie.Value)
		if token != "" {
			claims, ok := m.verifyAccessClaims(c, token)
			return claims, token, browserAccessTokenSourceCookie, ok
		}
	}
	response.Fail(c, apperr.ErrUnauthorized)
	c.Abort()
	return nil, "", "", false
}

// bearerAccessToken 从 Authorization 头读取 Bearer token,不写响应便于浏览器入口继续尝试 Cookie。
func bearerAccessToken(c *gin.Context) (string, bool) {
	raw := c.GetHeader("Authorization")
	token, ok := strings.CutPrefix(raw, "Bearer ")
	token = strings.TrimSpace(token)
	return token, ok && token != ""
}

// verifyAccessClaims 校验 access token 并统一输出用户向未登录错误。
func (m *Manager) verifyAccessClaims(c *gin.Context, token string) (*Claims, bool) {
	claims, err := m.VerifyAccess(strings.TrimSpace(token))
	if err != nil {
		response.Fail(c, apperr.ErrUnauthorized.WithCause(err))
		c.Abort()
		return nil, false
	}
	return claims, true
}

// BrowserAccessToken 返回浏览器入口中间件已验证过的原始 access token。
func BrowserAccessToken(c *gin.Context) (string, bool) {
	token, ok := c.Get(browserAccessTokenContextKey)
	if !ok {
		return "", false
	}
	raw, ok := token.(string)
	return raw, ok && strings.TrimSpace(raw) != ""
}

// BrowserAccessFromQuery 判断当前请求是否通过一次性 query token 完成鉴权。
func BrowserAccessFromQuery(c *gin.Context) bool {
	source, ok := c.Get(browserAccessSourceKey)
	return ok && source == string(browserAccessTokenSourceQuery)
}

// SetBrowserAccessCookie 写入路径受限 HttpOnly access cookie,供内嵌工具后续资源请求复用平台代理鉴权。
func (m *Manager) SetBrowserAccessCookie(c *gin.Context, pathPrefix, token string) {
	pathPrefix = "/" + strings.Trim(strings.TrimSpace(pathPrefix), "/")
	if pathPrefix == "/" || strings.TrimSpace(token) == "" {
		return
	}
	maxAge := int(m.accessTTL.Seconds())
	if maxAge <= 0 {
		maxAge = 900
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     BrowserAccessCookieName,
		Value:    strings.TrimSpace(token),
		Path:     pathPrefix,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   browserCookieSecure(c),
		SameSite: http.SameSiteLaxMode,
	})
}

// browserCookieSecure 判断当前入口是否应写 Secure cookie,支持 TLS 终止在前置网关的部署。
func browserCookieSecure(c *gin.Context) bool {
	return c.Request.TLS != nil || strings.EqualFold(strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")), "https")
}

// webSocketTicketClaims 提取并校验查询参数中的短时连接票据。
func (m *Manager) webSocketTicketClaims(c *gin.Context) (*Claims, bool) {
	ticket := strings.TrimSpace(c.Query("ticket"))
	if ticket == "" {
		response.Fail(c, apperr.ErrUnauthorized)
		c.Abort()
		return nil, false
	}
	claims, err := m.VerifyWebSocketTicket(ticket, c.Request.URL.Path)
	if err != nil {
		response.Fail(c, apperr.ErrUnauthorized.WithCause(err))
		c.Abort()
		return nil, false
	}
	return claims, true
}

// validateAccessSession 通过业务模块注入的校验器确认 JWT 对应服务端会话仍有效。
func (m *Manager) validateAccessSession(c *gin.Context, claims *Claims) bool {
	if m.sessions == nil {
		response.Fail(c, apperr.ErrUnauthorized)
		c.Abort()
		return false
	}
	path := c.FullPath()
	if strings.TrimSpace(path) == "" {
		path = c.Request.URL.Path
	}
	err := m.sessions.ValidateAccessSession(c.Request.Context(), SessionIdentity{
		TenantID:   claims.TenantID,
		AccountID:  claims.AccountID,
		SessionID:  claims.SessionID,
		IsPlatform: claims.IsPlatform,
		Method:     c.Request.Method,
		Path:       path,
	})
	if err == nil {
		return true
	}
	if appErr, ok := apperr.As(err); ok {
		response.Fail(c, appErr)
	} else {
		response.Fail(c, apperr.ErrUnauthorized.WithCause(err))
	}
	c.Abort()
	return false
}

// injectAccessIdentity 将已验证 JWT 身份写入上下文和结构化日志字段。
func injectAccessIdentity(c *gin.Context, claims *Claims) {
	id := tenant.Identity{
		TenantID:   claims.TenantID,
		AccountID:  claims.AccountID,
		IsPlatform: claims.IsPlatform,
	}
	ctx := tenant.WithContext(c.Request.Context(), id)
	ctx = logging.WithAttrs(ctx,
		slog.Int64("tenant_id", claims.TenantID),
		slog.Int64("account_id", claims.AccountID),
		slog.Bool("is_platform", claims.IsPlatform),
	)
	c.Request = c.Request.WithContext(ctx)
	c.Set("session_id", claims.SessionID)
}

// injectServiceIdentity 校验内部服务签名并建立租户边界与来源边界。
func (m *Manager) injectServiceIdentity(c *gin.Context) bool {
	service := strings.TrimSpace(c.GetHeader(ServiceNameHeader))
	tenantIDRaw := strings.TrimSpace(c.GetHeader(ServiceTenantHeader))
	sourceRef := strings.TrimSpace(c.GetHeader(ServiceSourceRefHeader))
	timestamp := strings.TrimSpace(c.GetHeader(ServiceTimestampHeader))
	signature := strings.TrimSpace(c.GetHeader(ServiceSignatureHeader))
	traceID := response.TraceFromGin(c)

	if service == "" || tenantIDRaw == "" || sourceRef == "" || timestamp == "" || signature == "" || traceID == "" || len(m.hmacKey) == 0 {
		response.Fail(c, apperr.ErrServiceUnauthorized)
		c.Abort()
		return false
	}
	if !ValidSourceRef(sourceRef) {
		response.Fail(c, apperr.ErrServiceUnauthorized)
		c.Abort()
		return false
	}
	tenantID, err := strconv.ParseInt(tenantIDRaw, 10, 64)
	if err != nil || tenantID <= 0 {
		response.Fail(c, apperr.ErrServiceUnauthorized)
		c.Abort()
		return false
	}
	if !m.serviceTimestampFresh(timestamp) {
		response.Fail(c, apperr.ErrServiceUnauthorized)
		c.Abort()
		return false
	}
	expected := m.serviceSignature(c.Request.Method, c.Request.URL.EscapedPath(), tenantIDRaw, sourceRef, timestamp, traceID)
	if !pkgcrypto.EqualHexHMAC(signature, expected) {
		response.Fail(c, apperr.ErrServiceUnauthorized)
		c.Abort()
		return false
	}

	ctx, err := WithServiceIdentity(c.Request.Context(), tenantID, sourceRef)
	if err != nil {
		response.Fail(c, err)
		c.Abort()
		return false
	}
	ctx = logging.WithAttrs(ctx,
		slog.Int64("tenant_id", tenantID),
		slog.String("service", service),
		slog.String("source_ref", sourceRef),
	)
	c.Request = c.Request.WithContext(ctx)
	return true
}

// serviceTimestampFresh 校验服务签名时间窗口,防止内部请求被长期重放。
func (m *Manager) serviceTimestampFresh(raw string) bool {
	seconds, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return false
	}
	signedAt := time.Unix(seconds, 0).UTC()
	now := timex.Now()
	return !signedAt.Before(now.Add(-m.serviceMaxSkew)) && !signedAt.After(now.Add(m.serviceMaxSkew))
}

// hasServiceAuthHeaders 判断请求是否声明内部服务签名身份。
func hasServiceAuthHeaders(c *gin.Context) bool {
	return strings.TrimSpace(c.GetHeader(ServiceNameHeader)) != "" ||
		strings.TrimSpace(c.GetHeader(ServiceSignatureHeader)) != ""
}

// serviceSignature 计算固定字段顺序的内部服务签名。
func (m *Manager) serviceSignature(method, path, tenantID, sourceRef, timestamp, traceID string) string {
	signature, err := pkgcrypto.HMACSHA256Hex(m.hmacKey, strings.ToUpper(method)+"\n"+path+"\n"+tenantID+"\n"+sourceRef+"\n"+timestamp+"\n"+traceID)
	if err != nil {
		return ""
	}
	return signature
}
