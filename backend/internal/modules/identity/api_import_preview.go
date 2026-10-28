// identity api_import_preview注册账号与组织共用的学校管理员预览资源接口。
package identity

import (
	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/httpx"
	"chaimir/pkg/apperr"
	"github.com/gin-gonic/gin"
)

// registerImportPreviewRoutes复用租户角色鉴权,不为账号与组织各造恢复入口。
func registerImportPreviewRoutes(r gin.IRouter, svc *Service, authn *auth.Manager) {
	a := accountAPI{svc: svc}
	g := r.Group("/import-previews", authn.Middleware(), auth.RequireTenantAnyRole(svc, contracts.RoleSchoolAdmin))
	g.GET("", a.listImportPreviews)
	g.GET("/:id", a.getImportPreview)
	g.DELETE("/:id", a.discardImportPreview)
}

// listImportPreviews只接受明确的业务目标和统一分页参数。
func (a accountAPI) listImportPreviews(c *gin.Context) {
	target, ok := parseImportTarget(c.Query("type"))
	if c.Query("type") == "org" {
		target, ok = ImportTargetOrg, true
	}
	if !ok {
		httpx.Write(c, struct{}{}, apperr.ErrIdentityImportTypeInvalid)
		return
	}
	page, size, ok := httpx.Page(c)
	if !ok {
		return
	}
	items, total, page, size, err := a.svc.ListPendingImportPreviews(c.Request.Context(), target, page, size)
	httpx.WritePage(c, items, total, page, size, err)
}

// getImportPreview绑定预览ID并读取本人有效快照。
func (a accountAPI) getImportPreview(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.GetImportPreviewByAdmin(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// discardImportPreview明确放弃服务端预览,普通弹层关闭不调用此接口。
func (a accountAPI) discardImportPreview(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	httpx.Write(c, struct{}{}, a.svc.DiscardImportPreviewByAdmin(c.Request.Context(), id))
}
