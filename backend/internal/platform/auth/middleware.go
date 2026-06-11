// auth 提供用户 JWT、中间服务 HMAC 和平台角色守卫中间件。
package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"regexp"
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

var serviceSourceRefRe = regexp.MustCompile(`^[a-z]+:[0-9]{4}:[a-z][a-z0-9_-]*:[0-9A-Za-z_-]+$`)

// RoleChecker 是平台通用角色守卫所需的最小身份只读契约。
type RoleChecker interface {
	// HasRole 判断账号是否具备指定角色。
	HasRole(ctx context.Context, accountID int64, role string) (bool, error)
}

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

// ServiceSourceRefAuthorized 检查当前上下文是否允许访问目标来源;普通用户上下文不受此限制。
func ServiceSourceRefAuthorized(ctx context.Context, sourceRef string) bool {
	signedSourceRef, ok := ServiceSourceRefFromContext(ctx)
	return !ok || signedSourceRef == strings.TrimSpace(sourceRef)
}

// Middleware 校验 Bearer access token 并注入租户身份上下文。
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
		injectAccessIdentity(c, claims)
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
func RequirePlatformOrAnyRole(identity RoleChecker, roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !AuthorizePlatformOrAnyRole(c, identity, roles...) {
			return
		}
		c.Next()
	}
}

// RequireTenantAnyRole 要求租户账号具备任一指定角色,平台身份不会被视为租户角色。
func RequireTenantAnyRole(identity RoleChecker, roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !AuthorizeTenantAnyRole(c, identity, roles...) {
			return
		}
		c.Next()
	}
}

// AuthorizePlatformOrAnyRole 执行平台或任一租户角色校验,并在失败时写统一响应。
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

// AuthorizeTenantAnyRole 执行租户角色校验,并拒绝平台身份绕过租户范围。
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
	if !constantTimeHexEqual(signature, expected) {
		response.Fail(c, apperr.ErrServiceUnauthorized)
		c.Abort()
		return false
	}

	ctx := tenant.WithContext(c.Request.Context(), tenant.Identity{TenantID: tenantID, IsSystem: true})
	ctx = WithServiceSourceRef(ctx, sourceRef)
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
	mac := hmac.New(sha256.New, m.hmacKey)
	mac.Write([]byte(strings.ToUpper(method) + "\n" + path + "\n" + tenantID + "\n" + sourceRef + "\n" + timestamp + "\n" + traceID))
	return hex.EncodeToString(mac.Sum(nil))
}

// constantTimeHexEqual 比较十六进制 HMAC,避免泄露差异位置。
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
