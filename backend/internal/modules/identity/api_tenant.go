// identity api_tenant 文件承接租户配置和统一认证配置 HTTP 请求。
package identity

import (
	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/httpx"
	"chaimir/pkg/response"

	"github.com/gin-gonic/gin"
)

// tenantAPI 封装租户配置 HTTP handler 依赖。
type tenantAPI struct {
	svc *Service
}

// registerTenantRoutes 注册学校管理员维护租户配置和 SSO 配置的路由。
func registerTenantRoutes(r gin.IRouter, svc *Service, authn *auth.Manager) {
	api := tenantAPI{svc: svc}
	g := r.Group("/tenant", authn.Middleware(), auth.RequireTenantAnyRole(svc, contracts.RoleSchoolAdmin))
	g.GET("/config", api.getConfig)
	g.PATCH("/config", api.updateConfig)
	g.GET("/sso", api.listSSO)
	g.PUT("/sso", api.upsertSSO)
}

// getConfig 读取当前租户配置,API 层不直接访问数据库。
func (a tenantAPI) getConfig(c *gin.Context) {
	out, err := a.svc.GetTenantConfig(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, out)
}

// updateConfig 绑定租户配置更新请求并委托 service 校验和落库。
func (a tenantAPI) updateConfig(c *gin.Context) {
	var req TenantConfigRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	out, err := a.svc.UpdateTenantConfigByAdmin(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, out)
}

// listSSO 读取当前租户统一认证配置列表,响应前由 service/convert 脱敏。
func (a tenantAPI) listSSO(c *gin.Context) {
	out, err := a.svc.ListSSOConfigsByAdmin(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, out)
}

// upsertSSO 绑定 CAS/LDAP 配置更新请求,敏感字段加密由 service 执行。
func (a tenantAPI) upsertSSO(c *gin.Context) {
	var req SSOConfigRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	out, err := a.svc.UpsertSSOConfig(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, out)
}
