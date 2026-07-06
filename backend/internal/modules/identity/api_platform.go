// identity api_platform 文件承接平台入驻申请和租户审核 HTTP 请求。
package identity

import (
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/httpx"

	"github.com/gin-gonic/gin"
)

// platformAPI 封装平台管理 HTTP handler 依赖,避免匿名函数承载核心入口。
type platformAPI struct {
	svc *Service
}

// registerPlatformRoutes 注册平台管理员入口路由。
func registerPlatformRoutes(r gin.IRouter, svc *Service, authn *auth.Manager) {
	api := platformAPI{svc: svc}
	g := r.Group("/platform")
	g.POST("/applications", api.createApplication)
	g.GET("/applications", authn.Middleware(), auth.RequirePlatformIdentity(), api.listApplications)
	g.POST("/applications/:id/approve", authn.Middleware(), auth.RequirePlatformIdentity(), api.approveApplication)
	g.POST("/applications/:id/reject", authn.Middleware(), auth.RequirePlatformIdentity(), api.rejectApplication)
	g.GET("/tenants", authn.Middleware(), auth.RequirePlatformIdentity(), api.listTenants)
	g.GET("/tenants/:id", authn.Middleware(), auth.RequirePlatformIdentity(), api.getTenant)
	g.PATCH("/tenants/:id", authn.Middleware(), auth.RequirePlatformIdentity(), api.updateTenant)
}

// createApplication 绑定公开入驻申请请求,该入口不创建账号也不直接开通学校。
func (a platformAPI) createApplication(c *gin.Context) {
	var req CreateApplicationRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	out, err := a.svc.CreateApplication(c.Request.Context(), req)
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, ToTenantApplicationDTO(out), nil)
}

// listApplications 读取平台入驻申请列表,状态过滤仅在 service/repo 中使用。
func (a platformAPI) listApplications(c *gin.Context) {
	status, ok := httpx.QueryInt(c, "status", httpx.QueryIntRule{BitSize: 16, Min: 0})
	if !ok {
		return
	}
	out, err := a.svc.ListApplicationsByPlatform(c.Request.Context(), int16(status))
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, ToTenantApplicationDTOs(out), nil)
}

// approveApplication 绑定平台审核通过请求,创建租户和首个学校管理员由 service 原子编排。
func (a platformAPI) approveApplication(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req ReviewApplicationRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	tenant, activation, err := a.svc.ApproveApplication(c.Request.Context(), id, req)
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, gin.H{"tenant": tenant, "activation_code": activation}, nil)
}

// rejectApplication 绑定平台驳回申请请求,驳回原因仅作为业务字段传给 service。
func (a platformAPI) rejectApplication(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req ReviewApplicationRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	if err := a.svc.RejectApplication(c.Request.Context(), id, req.Reason); err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, gin.H{}, nil)
}

// listTenants 读取平台租户列表,API 层不直接访问 repo。
func (a platformAPI) listTenants(c *gin.Context) {
	page, size, ok := httpx.Page(c)
	if !ok {
		return
	}
	out, total, p, s, err := a.svc.ListTenantsByPlatform(c.Request.Context(), page, size)
	httpx.WritePage(c, out, total, p, s, err)
}

// getTenant 读取单个租户详情,路径 ID 解析失败时返回统一用户向错误。
func (a platformAPI) getTenant(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.GetTenantByPlatform(c.Request.Context(), id)
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, out, nil)
}

// updateTenant 绑定平台租户状态更新请求,状态机校验由 service 执行。
func (a platformAPI) updateTenant(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req UpdateTenantStatusRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	out, err := a.svc.UpdateTenantStatusByPlatform(c.Request.Context(), id, req)
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, out, nil)
}
