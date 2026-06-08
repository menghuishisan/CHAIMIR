// M1 平台管理与本校配置 HTTP 处理器。
package identity

import (
	"chaimir/internal/platform/httpx"
	"chaimir/internal/platform/pagex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/response"

	"github.com/gin-gonic/gin"
)

// createApplication 访客提交入驻申请(公开)。
func (a *API) createApplication(c *gin.Context) {
	var req CreateApplicationRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrApplicationInvalid) {
		return
	}
	id, err := a.svc.CreateApplication(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"application_id": id})
}

// listApplications 平台列申请(?status=)。
func (a *API) listApplications(c *gin.Context) {
	page, size := pagex.Normalize(httpx.QueryInt(c, "page"), httpx.QueryInt(c, "size"))
	status := httpx.Int16(c.Query("status"))
	rows, total, err := a.svc.ListApplications(c.Request.Context(), status, page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OKPage(c, rows, total, page, size)
}

// approveApplication 通过申请(创建租户 + 首个管理员)。
func (a *API) approveApplication(c *gin.Context) {
	appID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	id, _ := currentID(c)
	var body struct {
		TenantCode string `json:"tenant_code" binding:"required"`
		AdminPhone string `json:"admin_phone" binding:"required"`
		AdminName  string `json:"admin_name" binding:"required"`
	}
	if !httpx.BindJSONWithError(c, &body, apperr.ErrApplicationInvalid) {
		return
	}
	res, err := a.svc.ApproveApplication(c.Request.Context(), appID, id.AccountID, body.TenantCode, body.AdminPhone, body.AdminName)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// rejectApplication 驳回申请。
func (a *API) rejectApplication(c *gin.Context) {
	appID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	id, _ := currentID(c)
	var req RejectApplicationRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrApplicationInvalid) {
		return
	}
	if err := a.svc.RejectApplication(c.Request.Context(), appID, id.AccountID, req.Reason); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"rejected": true})
}

// listTenants 平台列租户(?status=)。
func (a *API) listTenants(c *gin.Context) {
	page, size := pagex.Normalize(httpx.QueryInt(c, "page"), httpx.QueryInt(c, "size"))
	status := httpx.Int16(c.Query("status"))
	rows, total, err := a.svc.ListTenants(c.Request.Context(), status, page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OKPage(c, rows, total, page, size)
}

// getTenant 平台取租户详情。
func (a *API) getTenant(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	m, err := a.svc.GetTenant(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, m)
}

// updateTenant 平台改租户状态/到期。
func (a *API) updateTenant(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req UpdateTenantRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrTenantUpdateInvalid) {
		return
	}
	if err := a.svc.UpdateTenant(c.Request.Context(), id, req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"updated": true})
}

// getTenantConfig 学校管理员取本校配置。
func (a *API) getTenantConfig(c *gin.Context) {
	id, ok := currentID(c)
	if !ok {
		response.Fail(c, apperr.ErrUnauthorized)
		return
	}
	m, err := a.svc.GetTenantConfig(c.Request.Context(), id.TenantID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, m)
}

// updateTenantConfig 学校管理员改本校配置。
func (a *API) updateTenantConfig(c *gin.Context) {
	id, ok := currentID(c)
	if !ok {
		response.Fail(c, apperr.ErrUnauthorized)
		return
	}
	var req TenantConfigRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrTenantConfigInvalid) {
		return
	}
	if err := a.svc.UpdateTenantConfig(c.Request.Context(), id.TenantID, req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"updated": true})
}

// getSsoConfig 学校管理员读取本校 SSO/LDAP 配置。
func (a *API) getSsoConfig(c *gin.Context) {
	id, ok := currentID(c)
	if !ok {
		response.Fail(c, apperr.ErrUnauthorized)
		return
	}
	cfg, err := a.svc.GetSsoConfig(c.Request.Context(), id.TenantID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, cfg)
}

// upsertSsoConfig 学校管理员保存本校 SSO/LDAP 配置。
func (a *API) upsertSsoConfig(c *gin.Context) {
	id, ok := currentID(c)
	if !ok {
		response.Fail(c, apperr.ErrUnauthorized)
		return
	}
	var req SsoConfigRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSsoConfigRequestInvalid) {
		return
	}
	cfg, err := a.svc.UpsertSsoConfig(c.Request.Context(), id.TenantID, req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, cfg)
}
