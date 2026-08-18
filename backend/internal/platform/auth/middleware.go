// auth 提供用户 JWT、中间服务 HMAC 和平台角色守卫中间件。
package auth

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
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
	// RefreshCookieName 是仅后端可读的刷新会话 cookie 名称。
	RefreshCookieName = "chaimir_refresh"
	// RefreshRequestHeader 是刷新接口的 CSRF 自定义请求头,跨站表单不能伪造该头。
	RefreshRequestHeader = "X-Chaimir-Refresh"
	// RefreshRequestHeaderValue 是刷新接口要求的固定请求头值,仅用于确认请求来自受控客户端。
	RefreshRequestHeaderValue = "1"
	// BrowserAccessTicketQuery 是浏览器无法设置 Authorization 头时使用的短时入口票据参数。
	BrowserAccessTicketQuery = "ticket"
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

// ValidRefreshRequestHeader 校验刷新请求的自定义头,与 SameSite Cookie 共同阻断跨站会话轮转。
func ValidRefreshRequestHeader(value string) bool {
	return strings.TrimSpace(value) == RefreshRequestHeaderValue
}

type serviceSourceRefKey struct{}
type browserAccessSource string

const (
	browserAccessSourceHeader browserAccessSource = "header"
	browserAccessSourceTicket browserAccessSource = "ticket"
	browserAccessSourceCookie browserAccessSource = "cookie"
)

const (
	verifiedAccessContextKey = "auth_verified_value"
	browserAccessSourceKey   = "auth_browser_access_source"
)

var serviceSourcePattern = regexp.MustCompile(`^[a-z]+:[0-9]{4}:[a-z][a-z0-9_-]*:[0-9A-Za-z_-]+$`)

// ValidSourceRef 校验 source_ref 是否符合全局四段规范。
func ValidSourceRef(sourceRef string) bool {
	return serviceSourcePattern.MatchString(strings.TrimSpace(sourceRef))
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
	claims, token, ok := m.accessClaims(c)
	if !ok {
		return false
	}
	if !m.validateAccessSession(c, claims) {
		return false
	}
	injectAccessIdentity(c, claims)
	c.Set(verifiedAccessContextKey, token)
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

// browserAccessCSRFBlocked 拒绝路径 Cookie 对非安全方法的跨站调用。
// 工具代理不能统一改造上游页面,因此用浏览器 Fetch Metadata 优先、Origin/Referer 兜底建立同源边界。
func browserAccessCSRFBlocked(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return true
	}
	switch c.Request.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	if site := strings.ToLower(strings.TrimSpace(c.GetHeader("Sec-Fetch-Site"))); site != "" {
		switch site {
		case "same-origin", "none":
			return false
		case "same-site":
			// same-site 仍可能来自不同子域,继续精确校验 Origin/Referer 主机。
		default:
			return true
		}
	}
	if rawOrigin := strings.TrimSpace(c.GetHeader("Origin")); rawOrigin != "" {
		return !sameRequestHost(rawOrigin, c.Request.Host)
	}
	if rawReferer := strings.TrimSpace(c.GetHeader("Referer")); rawReferer != "" {
		return !sameRequestHost(rawReferer, c.Request.Host)
	}
	return true
}

// sameRequestHost 校验 Origin/Referer 只有合法 HTTP(S) 地址且主机精确匹配当前请求。
func sameRequestHost(raw, requestHost string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host == requestHost && parsed.User == nil
}

// BrowserAccessMiddleware 校验浏览器内嵌工具入口的 Bearer、短时路径票据或路径受限 Cookie。
func (m *Manager) BrowserAccessMiddleware() gin.HandlerFunc {
	return m.browserAccessGuard(true)
}

// FileAccessMiddleware 校验统一文件服务入口的 Bearer 或路径受限 Cookie。
// 与工具代理入口的差别只有一项:本入口不接受浏览器路径票据 ——
// `token` 参数位已被投放授权占用,同名两义会让验签路径产生歧义
// (见 docs/总-API接口总览.md §统一文件服务「鉴权载体」)。
func (m *Manager) FileAccessMiddleware() gin.HandlerFunc {
	return m.browserAccessGuard(false)
}

// browserAccessGuard 是两个浏览器直连入口共用的鉴权实现,只由 allowBrowserTicket 表达差异。
func (m *Manager) browserAccessGuard(allowBrowserTicket bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, token, source, ok := m.browserAccessClaims(c, allowBrowserTicket)
		if !ok {
			return
		}
		if !m.validateAccessSession(c, claims) {
			return
		}
		if source == browserAccessSourceTicket {
			var err error
			token, err = m.IssueAccess(claims.TenantID, claims.AccountID, claims.SessionID, claims.IsPlatform)
			if err != nil {
				response.Fail(c, apperr.ErrInternal.WithCause(err))
				c.Abort()
				return
			}
		}
		if source == browserAccessSourceCookie && browserAccessCSRFBlocked(c) {
			response.Fail(c, apperr.ErrForbidden)
			c.Abort()
			return
		}
		injectAccessIdentity(c, claims)
		c.Set(verifiedAccessContextKey, token)
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
		claims, _, ok := m.accessClaims(c)
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

// accessClaims 从 Authorization 头读取并校验 access token,同时返回原始 token 供后续入口复用。
func (m *Manager) accessClaims(c *gin.Context) (*Claims, string, bool) {
	raw := c.GetHeader("Authorization")
	token, ok := strings.CutPrefix(raw, "Bearer ")
	token = strings.TrimSpace(token)
	if !ok || token == "" {
		response.Fail(c, apperr.ErrUnauthorized)
		c.Abort()
		return nil, "", false
	}
	claims, err := m.VerifyAccess(token)
	if err != nil {
		response.Fail(c, apperr.ErrUnauthorized.WithCause(err))
		c.Abort()
		return nil, "", false
	}
	return claims, token, true
}

// browserAccessClaims 按浏览器真实能力依次读取 Header、短时路径票据和路径受限 Cookie。
// allowBrowserTicket=false 时跳过票据分支,供统一文件服务等已有 query 语义的入口使用。
func (m *Manager) browserAccessClaims(c *gin.Context, allowBrowserTicket bool) (*Claims, string, browserAccessSource, bool) {
	if token, ok := bearerAccessToken(c); ok {
		claims, ok := m.verifyAccessClaims(c, token)
		return claims, token, browserAccessSourceHeader, ok
	}
	if allowBrowserTicket {
		if ticket := strings.TrimSpace(c.Query(BrowserAccessTicketQuery)); ticket != "" {
			if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
				response.Fail(c, apperr.ErrUnauthorized)
				c.Abort()
				return nil, "", "", false
			}
			claims, err := m.VerifyBrowserAccessTicket(ticket, c.Request.URL.Path)
			if err != nil {
				response.Fail(c, apperr.ErrUnauthorized.WithCause(err))
				c.Abort()
				return nil, "", "", false
			}
			return claims, ticket, browserAccessSourceTicket, true
		}
	}
	if cookie, err := c.Request.Cookie(BrowserAccessCookieName); err == nil {
		token := strings.TrimSpace(cookie.Value)
		if token != "" {
			claims, ok := m.verifyAccessClaims(c, token)
			return claims, token, browserAccessSourceCookie, ok
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

// VerifiedAccessToken 返回本次请求已通过校验的原始 access token。
// 供需要把同一凭据下传给浏览器直连入口的场景复用(工具代理 Cookie、文件服务 Cookie),
// 调用方不得自行从 Header/Cookie 再解一遍 —— 那会绕过中间件的会话有效性校验。
func VerifiedAccessToken(c *gin.Context) (string, bool) {
	token, ok := c.Get(verifiedAccessContextKey)
	if !ok {
		return "", false
	}
	raw, ok := token.(string)
	return raw, ok && strings.TrimSpace(raw) != ""
}

// BrowserAccessFromTicket 判断当前请求是否通过短时浏览器路径票据完成鉴权。
func BrowserAccessFromTicket(c *gin.Context) bool {
	source, ok := c.Get(browserAccessSourceKey)
	return ok && source == string(browserAccessSourceTicket)
}

// SetBrowserAccessCookie 写入路径受限 HttpOnly access cookie,供浏览器直连入口
// (内嵌工具代理、统一文件服务流式投放)在发不出 Authorization 头时复用同一会话凭据。
// 有效期与 access token 一致,由 platform/config 保证 JWT_ACCESS_TTL_MIN 为正数。
func (m *Manager) SetBrowserAccessCookie(c *gin.Context, pathPrefix, token string) {
	pathPrefix = "/" + strings.Trim(strings.TrimSpace(pathPrefix), "/")
	if pathPrefix == "/" || strings.TrimSpace(token) == "" {
		return
	}
	http.SetCookie(c.Writer, newBrowserAccessCookie(BrowserAccessCookieName, strings.TrimSpace(token), pathPrefix, int(m.accessTTL.Seconds())))
}

// SetRefreshCookie 写入 HttpOnly refresh cookie；持久化标记只在 cookie 值内部编码，令牌永不进入 JS。
func (m *Manager) SetRefreshCookie(c *gin.Context, token string, persistent bool) {
	if strings.TrimSpace(token) == "" {
		return
	}
	marker := "s."
	maxAge := 0
	if persistent {
		marker = "p."
		maxAge = int(m.refreshTTL.Seconds())
	}
	http.SetCookie(c.Writer, newAuthCookie(RefreshCookieName, marker+strings.TrimSpace(token), "/", maxAge))
}

// RefreshCookieFromRequest 读取并解码 refresh cookie，同时返回登录时的持久化选择。
func RefreshCookieFromRequest(c *gin.Context) (string, bool, bool) {
	cookie, err := c.Request.Cookie(RefreshCookieName)
	if err != nil {
		return "", false, false
	}
	value := strings.TrimSpace(cookie.Value)
	if strings.HasPrefix(value, "p.") {
		value = strings.TrimPrefix(value, "p.")
		return value, true, value != ""
	}
	if strings.HasPrefix(value, "s.") {
		value = strings.TrimPrefix(value, "s.")
		return value, false, value != ""
	}
	return "", false, false
}

// ClearRefreshCookie 立即删除浏览器刷新会话。
func (m *Manager) ClearRefreshCookie(c *gin.Context) {
	http.SetCookie(c.Writer, newAuthCookie(RefreshCookieName, "", "/", -1))
}

// newAuthCookie 以固定安全属性构造鉴权 Cookie;所有部署形态都必须通过 HTTPS 访问。
func newAuthCookie(name, value, path string, maxAge int) *http.Cookie {
	cookie := http.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	}
	return &cookie
}

// newBrowserAccessCookie 构造浏览器顶层导航仍可携带的路径受限 Cookie;Lax 是该入口的必要策略。
func newBrowserAccessCookie(name, value, path string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}
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
	tenantID, ok := ids.Parse(tenantIDRaw)
	if !ok {
		response.Fail(c, apperr.ErrServiceUnauthorized)
		c.Abort()
		return false
	}
	if !m.serviceTimestampFresh(timestamp) {
		response.Fail(c, apperr.ErrServiceUnauthorized)
		c.Abort()
		return false
	}
	expected := m.serviceSignature(service, c.Request.Method, c.Request.URL.EscapedPath(), tenantIDRaw, sourceRef, timestamp, traceID)
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

// serviceSignature 计算固定字段顺序的内部服务签名,把调用方服务名纳入签名防止身份头被替换。
func (m *Manager) serviceSignature(service, method, path, tenantID, sourceRef, timestamp, traceID string) string {
	signature, err := pkgcrypto.HMACSHA256Hex(m.hmacKey, service+"\n"+strings.ToUpper(method)+"\n"+path+"\n"+tenantID+"\n"+sourceRef+"\n"+timestamp+"\n"+traceID)
	if err != nil {
		return ""
	}
	return signature
}
