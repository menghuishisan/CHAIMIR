// identity api 文件负责注册 HTTP 路由、绑定请求参数、组合鉴权中间件并调用 service。
package identity

import (
	"chaimir/internal/platform/auth"
	"chaimir/pkg/apperr"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册 identity 模块 HTTP API。
func RegisterRoutes(r gin.IRouter, svc *Service, authn *auth.Manager) error {
	if svc == nil {
		return apperr.ErrIdentityRouteDependencyMissing
	}
	if authn == nil {
		return apperr.ErrIdentityRouteDependencyMissing
	}
	api := r.Group("/api/v1")
	registerAuthRoutes(api, svc, authn)
	// 私有化部署没有平台层,直接不注册 /platform 路由,避免仅靠 handler 内部返回错误。
	if svc.deploy.PlatformEnabled {
		registerPlatformRoutes(api, svc, authn)
	}
	registerTenantRoutes(api, svc, authn)
	registerOrgRoutes(api, svc, authn)
	registerAccountRoutes(api, svc, authn)
	registerMeRoutes(api, svc, authn)
	registerAuditRoutes(api, svc, authn)
	return nil
}
