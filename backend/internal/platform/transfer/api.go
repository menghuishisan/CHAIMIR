// transfer api 文件负责注册统一导入导出中心 HTTP 路由和访问边界。
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
	if r == nil || svc == nil || authn == nil {
		return apperr.ErrHTTPServiceMissing
	}
	api := transferAPI{svc: svc, roles: roles}
	g := r.Group("/api/v1/transfer", authn.Middleware(), auth.RequireTenantAnyRole(roles, contracts.RoleStudent, contracts.RoleTeacher, contracts.RoleSchoolAdmin))
	g.GET("/tasks", api.listTasks)
	g.GET("/tasks/:id", api.getTask)
	g.POST("/tasks/:id/download-grant", api.downloadGrant)
	return nil
}

type transferAPI struct {
	svc   *Service
	roles contracts.IdentityService
}

// listTasks 查询当前账号的导入导出任务。
func (a transferAPI) listTasks(c *gin.Context) {
	id, ok := currentTenantIdentity(c)
	if !ok {
		return
	}
	page, size, ok := httpx.Page(c)
	if !ok {
		return
	}
	tasks, p, s, err := a.svc.ListTasks(c.Request.Context(), TaskListQuery{TenantID: id.TenantID, AccountID: id.AccountID, Channel: Channel(c.Query("channel")), Status: Status(c.Query("status")), Page: page, Size: size})
	httpx.Write(c, gin.H{"items": TasksToDTO(tasks), "page": p, "size": s}, err)
}

// getTask 读取当前账号可见的单个导入导出任务。
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
		tenantAdmin, err = a.isSchoolAdmin(c, id.AccountID)
		if err == nil {
			err = EnsureTaskOwner(task, id.TenantID, id.AccountID, tenantAdmin)
		}
	}
	httpx.Write(c, TaskToDTO(task), err)
}

// downloadGrant 为已完成任务签发短时下载授权。
func (a transferAPI) downloadGrant(c *gin.Context) {
	id, ok := currentTenantIdentity(c)
	if !ok {
		return
	}
	taskID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	tenantAdmin, err := a.isSchoolAdmin(c, id.AccountID)
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	out, err := a.svc.BuildDownloadGrant(c.Request.Context(), id.TenantID, taskID, id.AccountID, tenantAdmin)
	httpx.Write(c, out, err)
}

// currentTenantIdentity 读取已鉴权租户身份并拒绝平台身份访问租户任务中心。
func currentTenantIdentity(c *gin.Context) (tenant.Identity, bool) {
	id, ok := tenant.FromContext(c.Request.Context())
	if !ok || id.IsPlatform || id.TenantID <= 0 || id.AccountID <= 0 {
		httpx.Write(c, gin.H{}, apperr.ErrUnauthorized)
		return tenant.Identity{}, false
	}
	return id, true
}

// isSchoolAdmin 判断当前账号是否可读取同租户内其他账号任务,角色查询失败必须显式返回。
func (a transferAPI) isSchoolAdmin(c *gin.Context, accountID int64) (bool, error) {
	if a.roles == nil {
		return false, nil
	}
	ok, err := a.roles.HasRole(c.Request.Context(), accountID, contracts.RoleSchoolAdmin)
	if err != nil {
		return false, apperr.ErrTransferTaskForbidden.WithCause(err)
	}
	return ok, nil
}
