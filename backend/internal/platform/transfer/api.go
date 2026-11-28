package transfer

import (
	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/httpx"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册统一导入导出中心 HTTP API。
func RegisterRoutes(r gin.IRouter, svc *Service, authn *auth.Manager, roles contracts.IdentityService) error {
	if r == nil || svc == nil || authn == nil || roles == nil {
		return apperr.ErrHTTPServiceMissing
	}
	api := transferAPI{svc: svc, roles: roles}
	g := r.Group("/api/v1/transfer", authn.Middleware(), auth.RequirePlatformOrAnyRole(roles, contracts.RoleStudent, contracts.RoleTeacher, contracts.RoleSchoolAdmin))
	g.GET("/tasks", api.listTasks)
	g.GET("/tasks/:id", api.getTask)
	g.POST("/tasks/:id/download-grant", api.downloadGrant)
	return nil
}

type transferAPI struct {
	svc   *Service
	roles contracts.IdentityService
}

func (a transferAPI) listTasks(c *gin.Context) {
	id, ok := currentTenantIdentity(c)
	if !ok {
		return
	}
	page, size, ok := httpx.Page(c)
	if !ok {
		return
	}
	tasks, total, p, s, facets, err := a.svc.ListTasks(c.Request.Context(), TaskListQuery{TenantID: id.TenantID, AccountID: id.AccountID, Channel: Channel(c.Query("channel")), Status: Status(c.Query("status")), Page: page, Size: size})
	httpx.WritePageWithFacets(c, TasksToDTO(tasks), total, p, s, facets.Map(), err)
}

func (a transferAPI) getTask(c *gin.Context) {
	id, ok := currentTenantIdentity(c)
	if !ok {
		return
	}
	taskID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	task, err := a.svc.GetTask(c.Request.Context(), id.TenantID, taskID)
	if err == nil {
		var tenantAdmin bool
		if !id.IsPlatform {
			tenantAdmin, err = a.isSchoolAdmin(c, id.AccountID)
		}
		if err == nil {
			err = EnsureTaskOwner(task, id.TenantID, id.AccountID, tenantAdmin)
		}
	}
	httpx.Write(c, TaskToDTO(task), err)
}

func (a transferAPI) downloadGrant(c *gin.Context) {
	id, ok := currentTenantIdentity(c)
	if !ok {
		return
	}
	taskID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	tenantAdmin := false
	if !id.IsPlatform {
		var err error
		tenantAdmin, err = a.isSchoolAdmin(c, id.AccountID)
		if err != nil {
			httpx.Write(c, struct{}{}, err)
			return
		}
	}
	out, err := a.svc.BuildDownloadGrant(c.Request.Context(), id.TenantID, taskID, id.AccountID, tenantAdmin)
	httpx.Write(c, out, err)
}

// 平台身份在导入导出中心只访问租户编号为 0 的任务。
func currentTenantIdentity(c *gin.Context) (tenant.Identity, bool) {
	id, ok := tenant.FromContext(c.Request.Context())
	if !ok || id.AccountID <= 0 {
		httpx.Write(c, struct{}{}, apperr.ErrUnauthorized)
		return tenant.Identity{}, false
	}
	if id.IsPlatform {
		id.TenantID = 0
		return id, true
	}
	if id.TenantID <= 0 {
		httpx.Write(c, struct{}{}, apperr.ErrUnauthorized)
		return tenant.Identity{}, false
	}
	return id, true
}

func (a transferAPI) isSchoolAdmin(c *gin.Context, accountID int64) (bool, error) {
	if a.roles == nil {
		return false, apperr.ErrHTTPServiceMissing
	}
	ok, err := a.roles.HasRole(c.Request.Context(), accountID, contracts.RoleSchoolAdmin)
	if err != nil {
		return false, apperr.AsAppError(err)
	}
	return ok, nil
}
