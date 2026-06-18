// admin api 文件负责注册 M9 HTTP 路由、绑定请求和组合鉴权。
package admin

import (
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/httpx"
	"chaimir/internal/platform/response"
	"chaimir/pkg/apperr"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册管理后台 HTTP API。
func RegisterRoutes(r gin.IRouter, svc *Service, authn *auth.Manager, roles contracts.IdentityService) error {
	if r == nil || svc == nil || authn == nil {
		return apperr.ErrHTTPServiceMissing
	}
	api := adminAPI{svc: svc}
	g := r.Group("/api/v1/admin", authn.Middleware())
	platform := g.Group("/platform", auth.RequirePlatformIdentity())
	school := g.Group("/school", auth.RequireTenantAnyRole(roles, contracts.RoleSchoolAdmin))
	mixed := g.Group("", auth.RequirePlatformOrAnyRole(roles, contracts.RoleSchoolAdmin))
	platform.GET("/dashboard", api.platformDashboard)
	platform.GET("/statistics", api.platformStatistics)
	platform.GET("/tenants", api.listTenants)
	platform.GET("/applications", api.listApplications)
	platform.GET("/monitoring/panels", api.monitoringPanels)
	school.GET("/dashboard", api.schoolDashboard)
	school.GET("/statistics", api.schoolStatistics)
	mixed.GET("/audit", api.queryAudit)
	mixed.GET("/audit/export", api.exportAudit)
	mixed.GET("/configs", api.listConfigs)
	mixed.PUT("/configs/:key", api.updateConfig)
	mixed.GET("/configs/:key/history", api.configHistory)
	mixed.POST("/configs/:key/rollback", api.rollbackConfig)
	mixed.GET("/alert-rules", api.listAlertRules)
	mixed.POST("/alert-rules", api.createAlertRule)
	mixed.PATCH("/alert-rules/:id", api.updateAlertRule)
	mixed.GET("/alert-events", api.listAlertEvents)
	mixed.POST("/alert-events/:id/handle", api.handleAlertEvent)
	platform.GET("/backups", api.listBackups)
	return nil
}

type adminAPI struct{ svc *Service }

// platformDashboard 返回平台看板。
func (a adminAPI) platformDashboard(c *gin.Context) {
	out, err := a.svc.PlatformDashboard(c.Request.Context())
	httpx.Write(c, out, err)
}

// schoolDashboard 返回学校看板。
func (a adminAPI) schoolDashboard(c *gin.Context) {
	out, err := a.svc.SchoolDashboard(c.Request.Context())
	httpx.Write(c, out, err)
}

// platformStatistics 返回平台运营趋势统计。
func (a adminAPI) platformStatistics(c *gin.Context) {
	out, err := a.svc.PlatformStatistics(c.Request.Context(), c.Query("from"), c.Query("to"))
	httpx.Write(c, out, err)
}

// schoolStatistics 返回学校运营趋势统计。
func (a adminAPI) schoolStatistics(c *gin.Context) {
	out, err := a.svc.SchoolStatistics(c.Request.Context(), c.Query("from"), c.Query("to"))
	httpx.Write(c, out, err)
}

// listTenants 返回租户列表。
func (a adminAPI) listTenants(c *gin.Context) {
	out, err := a.svc.ListTenants(c.Request.Context())
	httpx.Write(c, out, err)
}

// listApplications 返回入驻申请列表。
func (a adminAPI) listApplications(c *gin.Context) {
	status, ok := httpx.QueryInt(c, "status", httpx.QueryIntRule{Default: 0, Min: 0, Max: 5, HasMax: true})
	if !ok {
		return
	}
	out1, err := a.svc.ListApplications(c.Request.Context(), int16(status))
	httpx.Write(c, out1, err)
}

// queryAudit 查询审计。
func (a adminAPI) queryAudit(c *gin.Context) {
	page, size, ok := httpx.Page(c)
	if !ok {
		return
	}
	query, ok := auditQuery(c, page, size)
	if !ok {
		return
	}
	result, err := a.svc.QueryAudit(c.Request.Context(), query)
	httpx.WritePage(c, result.List, result.Total, int(result.Page), int(result.Size), err)
}

// exportAudit 导出审计 CSV。
func (a adminAPI) exportAudit(c *gin.Context) {
	query, ok := auditQuery(c, 1, 0)
	if !ok {
		return
	}
	out, err := a.svc.ExportAuditCSV(c.Request.Context(), query)
	httpx.Write(c, out, err)
}

// listConfigs 查询配置。
func (a adminAPI) listConfigs(c *gin.Context) {
	scope, ok := httpx.QueryInt(c, "scope", httpx.QueryIntRule{Default: 0, Min: 0, Max: 2, HasMax: true})
	if !ok {
		return
	}
	out2, err := a.svc.ListConfigs(c.Request.Context(), int16(scope))
	httpx.Write(c, out2, err)
}

// updateConfig 更新配置。
func (a adminAPI) updateConfig(c *gin.Context) {
	var req ConfigUpdateRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrAdminConfigInvalid) {
		return
	}
	out3, err := a.svc.UpdateConfig(c.Request.Context(), c.Param("key"), req)
	httpx.Write(c, out3, err)
}

// configHistory 查询配置历史。
func (a adminAPI) configHistory(c *gin.Context) {
	page, size, ok := httpx.Page(c)
	if !ok {
		return
	}
	scope, ok := httpx.QueryInt(c, "scope", httpx.QueryIntRule{Default: 1, Min: 1, Max: 2, HasMax: true})
	if !ok {
		return
	}
	tenantID, ok := httpx.QueryInt(c, "tenant_id", httpx.QueryIntRule{Default: 0, Min: 0})
	if !ok {
		return
	}
	out4, total, p, s, err := a.svc.ListConfigHistory(c.Request.Context(), int16(scope), tenantID, c.Param("key"), page, size)
	httpx.WritePage(c, out4, total, p, s, err)
}

// rollbackConfig 回滚配置。
func (a adminAPI) rollbackConfig(c *gin.Context) {
	var req ConfigRollbackRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrAdminConfigInvalid) {
		return
	}
	out5, err := a.svc.RollbackConfig(c.Request.Context(), c.Param("key"), req)
	httpx.Write(c, out5, err)
}

// listAlertRules 查询告警规则。
func (a adminAPI) listAlertRules(c *gin.Context) {
	scope, ok := httpx.QueryInt(c, "scope", httpx.QueryIntRule{Default: 0, Min: 0, Max: 2, HasMax: true})
	if !ok {
		return
	}
	out6, err := a.svc.ListAlertRules(c.Request.Context(), int16(scope))
	httpx.Write(c, out6, err)
}

// createAlertRule 创建告警规则。
func (a adminAPI) createAlertRule(c *gin.Context) {
	var req AlertRuleRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrAdminAlertInvalid) {
		return
	}
	out7, err := a.svc.CreateAlertRule(c.Request.Context(), req)
	httpx.Write(c, out7, err)
}

// updateAlertRule 更新告警规则。
func (a adminAPI) updateAlertRule(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req AlertRuleRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrAdminAlertInvalid) {
		return
	}
	out8, err := a.svc.UpdateAlertRule(c.Request.Context(), id, req)
	httpx.Write(c, out8, err)
}

// listAlertEvents 查询告警事件。
func (a adminAPI) listAlertEvents(c *gin.Context) {
	page, size, ok := httpx.Page(c)
	if !ok {
		return
	}
	status, ok := httpx.QueryInt(c, "status", httpx.QueryIntRule{Default: 0, Min: 0, Max: 3, HasMax: true})
	if !ok {
		return
	}
	out9, total, p, s, err := a.svc.ListAlertEvents(c.Request.Context(), int16(status), page, size)
	httpx.WritePage(c, out9, total, p, s, err)
}

// handleAlertEvent 处理告警事件。
func (a adminAPI) handleAlertEvent(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req AlertEventRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrAdminAlertInvalid) {
		return
	}
	out10, err := a.svc.HandleAlertEvent(c.Request.Context(), id, req)
	httpx.Write(c, out10, err)
}

// monitoringPanels 查询监控面板。
func (a adminAPI) monitoringPanels(c *gin.Context) {
	out, err := a.svc.MonitoringPanels(c.Request.Context())
	httpx.Write(c, out, err)
}

// listBackups 查询备份记录。
func (a adminAPI) listBackups(c *gin.Context) {
	page, size, ok := httpx.Page(c)
	if !ok {
		return
	}
	out11, total, p, s, err := a.svc.ListBackups(c.Request.Context(), page, size)
	httpx.WritePage(c, out11, total, p, s, err)
}

// auditQuery 解析审计中心文档定义的过滤条件。
func auditQuery(c *gin.Context, page, size int) (contracts.AuditQuery, bool) {
	actorID, ok := httpx.QueryInt(c, "actor_id", httpx.QueryIntRule{Min: 0})
	if !ok {
		return contracts.AuditQuery{}, false
	}
	from, ok := queryRFC3339(c, "from")
	if !ok {
		return contracts.AuditQuery{}, false
	}
	to, ok := queryRFC3339(c, "to")
	if !ok {
		return contracts.AuditQuery{}, false
	}
	if !from.IsZero() && !to.IsZero() && to.Before(from) {
		response.Fail(c, apperr.ErrAdminAuditQueryInvalid)
		return contracts.AuditQuery{}, false
	}
	return contracts.AuditQuery{ActorID: actorID, Action: strings.TrimSpace(c.Query("action")), TargetType: strings.TrimSpace(c.Query("target_type")), From: from, To: to, Page: int32(page), Size: int32(size)}, true
}

// queryRFC3339 解析可选 RFC3339 时间查询参数。
func queryRFC3339(c *gin.Context, key string) (time.Time, bool) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return time.Time{}, true
	}
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		response.Fail(c, apperr.ErrAdminAuditQueryInvalid)
		return time.Time{}, false
	}
	return value, true
}
