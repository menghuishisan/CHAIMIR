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
	page, ok := httpx.QueryInt(c, "page", httpx.QueryIntRule{Default: 1, Min: 1})
	if !ok {
		return
	}
	size, ok := httpx.QueryInt(c, "size", httpx.QueryIntRule{Default: 20, Min: 1, Max: 100, HasMax: true})
	if !ok {
		return
	}
	tasks, p, s, err := a.svc.ListTasks(c.Request.Context(), TaskListQuery{TenantID: id.TenantID, AccountID: id.AccountID, Channel: Channel(c.Query("channel")), Status: Status(c.Query("status")), Page: int(page), Size: int(size)})
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
		err = EnsureTaskOwner(c.Request.Context(), task, id.TenantID, id.AccountID, a.isSchoolAdmin(c, id.AccountID))
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
	out, err := a.svc.BuildDownloadGrant(c.Request.Context(), id.TenantID, taskID, id.AccountID, a.isSchoolAdmin(c, id.AccountID))
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

// isSchoolAdmin 判断当前账号是否可读取同租户内其他账号任务。
func (a transferAPI) isSchoolAdmin(c *gin.Context, accountID int64) bool {
	if a.roles == nil {
		return false
	}
	ok, err := a.roles.HasRole(c.Request.Context(), accountID, contracts.RoleSchoolAdmin)
	return err == nil && ok
}
