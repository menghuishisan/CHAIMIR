// 鉴权中间件:校验 access Token,把租户身份注入 context 供 RLS/授权使用。
package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
	"chaimir/pkg/response"

	"github.com/gin-gonic/gin"
)

const (
	// ServiceNameHeader 标识调用方服务,用于内部接口审计和签名输入。
	ServiceNameHeader = "X-Chaimir-Service"
	// ServiceTenantHeader 是内部请求显式声明的租户边界,下游 RLS 只使用该服务端校验值。
	ServiceTenantHeader = "X-Chaimir-Tenant-Id"
	// ServiceSourceRefHeader 标识业务来源,用于回收、判题、通知等内部流程追踪。
	ServiceSourceRefHeader = "X-Chaimir-Source-Ref"
	// ServiceTimestampHeader 是服务签名时间戳,用于后续接入重放窗口校验。
	ServiceTimestampHeader = "X-Chaimir-Timestamp"
	// ServiceSignatureHeader 是 HMAC-SHA256 十六进制签名。
	ServiceSignatureHeader = "X-Chaimir-Signature"
)

type serviceSourceRefKey struct{}

// RoleChecker 是 API 角色守卫需要的最小身份契约,由 identity 模块实现提供。
type RoleChecker interface {
	// HasRole 判断账号是否具备指定服务端角色。
	HasRole(ctx context.Context, accountID int64, role string) (bool, error)
}

// ServiceSourceRefFromContext 读取服务间鉴权签名绑定的来源标识。
func ServiceSourceRefFromContext(ctx context.Context) (string, bool) {
	sourceRef, ok := ctx.Value(serviceSourceRefKey{}).(string)
	return sourceRef, ok && strings.TrimSpace(sourceRef) != ""
}

// WithServiceSourceRef 把已验证的来源标识注入上下文,供模块内 contracts 直连和 HTTP 服务鉴权共用。
func WithServiceSourceRef(ctx context.Context, sourceRef string) context.Context {
	return context.WithValue(ctx, serviceSourceRefKey{}, sourceRef)
}

// Middleware 校验 Authorization: Bearer <access>,失败即 11001;
// 成功把 tenant.Identity 注入 request.Context(下游 db.WithTenantTx 据此 SET RLS)。
func (m *Manager) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := m.accessClaims(c)
		if !ok {
			return
		}
		injectAccessIdentity(c, claims)
		c.Next()
	}
}

// ServiceMiddleware 校验内部服务请求的 HMAC 签名,并把已验证租户注入 context。
func (m *Manager) ServiceMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.injectServiceIdentity(c) {
			return
		}
		c.Next()
	}
}

// PlatformOrServiceMiddleware 接受平台管理员 JWT 或内部服务 HMAC,用于审核等双入口控制面。
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
		injectAccessIdentity(c, claims)
		c.Next()
	}
}

// RequirePlatformIdentity 要求当前请求来自已登录的平台身份。
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

// RequirePlatformOrAnyRole 要求当前请求来自平台身份,或租户账号具备任一指定角色。
func RequirePlatformOrAnyRole(identity RoleChecker, roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !AuthorizePlatformOrAnyRole(c, identity, roles...) {
			return
		}
		c.Next()
	}
}

// RequireTenantAnyRole 要求当前请求来自租户账号,且具备任一指定角色。
func RequireTenantAnyRole(identity RoleChecker, roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !AuthorizeTenantAnyRole(c, identity, roles...) {
			return
		}
		c.Next()
	}
}

// AuthorizePlatformOrAnyRole 校验当前请求角色并写出统一失败响应,供路由中间件和 handler 内条件鉴权共用。
func AuthorizePlatformOrAnyRole(c *gin.Context, identity RoleChecker, roles ...string) bool {
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

// AuthorizeTenantAnyRole 校验租户账号角色并写出统一失败响应,平台身份不会被当作租户角色放行。
func AuthorizeTenantAnyRole(c *gin.Context, identity RoleChecker, roles ...string) bool {
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

// accessClaims 校验 Bearer access token 并把失败转换为统一响应。
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

// injectAccessIdentity 将已验证 JWT claims 写入请求上下文,供 RLS、审计与日志使用。
func injectAccessIdentity(c *gin.Context, claims *Claims) {
	id := tenant.Identity{
		TenantID:   claims.TenantID,
		AccountID:  claims.AccountID,
		IsPlatform: claims.IsPlatform,
	}
	// 鉴权成功后才注入租户日志字段:租户/账号只能来自服务端 JWT claims,不能由请求参数决定。
	ctx := tenant.WithContext(c.Request.Context(), id)
	ctx = logging.WithAttrs(ctx,
		slog.Int64("tenant_id", claims.TenantID),
		slog.Int64("account_id", claims.AccountID),
		slog.Bool("is_platform", claims.IsPlatform),
	)
	c.Request = c.Request.WithContext(ctx)
	// 会话 ID 存入 gin 供 logout 等使用。
	c.Set("session_id", claims.SessionID)
}

// injectServiceIdentity 校验服务 HMAC 签名并注入租户与来源边界。
func (m *Manager) injectServiceIdentity(c *gin.Context) bool {
	// 第一步:读取全部签名输入,缺任一字段都拒绝,避免服务端推断产生歧义。
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
	// 第二步:校验租户边界和时间窗口,防止非法租户和过期签名进入 RLS 上下文。
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
	// 第三步:用固定字段顺序重新计算 HMAC,并用常量时间比较避免泄露签名差异。
	expected := m.serviceSignature(c.Request.Method, c.Request.URL.EscapedPath(), tenantIDRaw, sourceRef, timestamp, traceID)
	if !constantTimeHexEqual(signature, expected) {
		response.Fail(c, apperr.ErrServiceUnauthorized)
		c.Abort()
		return false
	}

	// 第四步:只注入租户与服务来源,服务请求不得伪装成用户账号身份。
	// 服务请求没有用户账号语义,只建立租户边界与结构化日志字段,禁止映射成任意用户身份。
	ctx := tenant.WithContext(c.Request.Context(), tenant.Identity{TenantID: tenantID})
	ctx = WithServiceSourceRef(ctx, sourceRef)
	ctx = logging.WithAttrs(ctx,
		slog.Int64("tenant_id", tenantID),
		slog.String("service", service),
		slog.String("source_ref", sourceRef),
	)
	c.Request = c.Request.WithContext(ctx)
	return true
}

// serviceTimestampFresh 校验服务签名时间窗口,防止截获的内部请求被长期重放。
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

// serviceSignature 固定内部服务签名输入,保证各模块对同一请求有唯一校验语义。
func (m *Manager) serviceSignature(method, path, tenantID, sourceRef, timestamp, traceID string) string {
	mac := hmac.New(sha256.New, m.hmacKey)
	mac.Write([]byte(strings.ToUpper(method) + "\n" + path + "\n" + tenantID + "\n" + sourceRef + "\n" + timestamp + "\n" + traceID))
	return hex.EncodeToString(mac.Sum(nil))
}

// constantTimeHexEqual 比较十六进制 HMAC,格式非法时直接失败且不泄露差异位置。
func constantTimeHexEqual(actual, expected string) bool {
	actualBytes, err := hex.DecodeString(actual)
	if err != nil {
		return false
	}
	expectedBytes, err := hex.DecodeString(expected)
	if err != nil {
		return false
	}
	return len(actualBytes) == len(expectedBytes) && hmac.Equal(actualBytes, expectedBytes)
}
