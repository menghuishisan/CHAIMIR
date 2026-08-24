// admin api 文件负责注册 M9 HTTP 路由、绑定请求和组合鉴权。
package admin

import (
	"strconv"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/httpx"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/response"
	"chaimir/internal/platform/timex"
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
	platform.GET("/monitoring/panels", api.monitoringPanels)
	school.GET("/dashboard", api.schoolDashboard)
	school.GET("/statistics", api.schoolStatistics)
	mixed.GET("/audit", api.queryAudit)
	mixed.GET("/audit/export", api.exportAudit)
	mixed.GET("/configs", api.listConfigs)
	mixed.GET("/configs/:key", api.getConfig)
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

// getConfig 读取单条配置的脱敏投影,供详情和乐观锁操作使用。
func (a adminAPI) getConfig(c *gin.Context) {
	key := strings.TrimSpace(c.Param("key"))
	if key == "" {
		httpx.Write(c, ConfigDTO{}, apperr.ErrAdminConfigInvalid)
		return
	}
	scope := int16(0)
	if raw := strings.TrimSpace(c.Query("scope")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 16)
		if err != nil {
			httpx.Write(c, ConfigDTO{}, apperr.ErrAdminConfigInvalid)
			return
		}
		scope = int16(parsed)
	}
	out, err := a.svc.GetConfig(c.Request.Context(), scope, key)
	httpx.Write(c, out, err)
}

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
	httpx.WritePageWithFacets(c, auditLogEntryDTOs(result.List), result.Total, int(result.Page), int(result.Size), result.Facets, err)
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
	scope, ok := httpx.QueryInt16(c, "scope", httpx.QueryIntRule{Default: 0, Min: 0, Max: 2, HasMax: true})
	if !ok {
		return
	}
	out2, err := a.svc.ListConfigs(c.Request.Context(), scope)
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
	scope, ok := httpx.QueryInt16(c, "scope", httpx.QueryIntRule{Default: 1, Min: 1, Max: 2, HasMax: true})
	if !ok {
		return
	}
	tenantID, ok := httpx.QueryID(c, "tenant_id", false)
	if !ok {
		return
	}
	out4, total, p, s, err := a.svc.ListConfigHistory(c.Request.Context(), scope, tenantID, c.Param("key"), page, size)
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
	scope, ok := httpx.QueryInt16(c, "scope", httpx.QueryIntRule{Default: 0, Min: 0, Max: 2, HasMax: true})
	if !ok {
		return
	}
	out6, err := a.svc.ListAlertRules(c.Request.Context(), scope)
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
	status, ok := httpx.QueryInt16(c, "status", httpx.QueryIntRule{Default: 0, Min: 0, Max: 3, HasMax: true})
	if !ok {
		return
	}
	level, ok := httpx.QueryInt16(c, "level", httpx.QueryIntRule{Default: 0, Min: 0, Max: int64(AlertLevelCritical), HasMax: true})
	if !ok {
		return
	}
	out9, total, p, s, err := a.svc.ListAlertEvents(c.Request.Context(), status, level, page, size)
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
// status=0 表示不按结果过滤;越界状态在边界处拒绝,不进服务层。
func (a adminAPI) listBackups(c *gin.Context) {
	page, size, ok := httpx.Page(c)
	if !ok {
		return
	}
	status, ok := httpx.QueryInt16(c, "status", httpx.QueryIntRule{Default: 0, Min: 0, Max: int64(BackupStatusFailed), HasMax: true})
	if !ok {
		return
	}
	out11, total, p, s, err := a.svc.ListBackups(c.Request.Context(), status, page, size)
	httpx.WritePage(c, out11, total, p, s, err)
}

// auditQuery 解析审计中心文档定义的过滤条件。
func auditQuery(c *gin.Context, page, size int) (contracts.AuditQuery, bool) {
	actorID, ok := httpx.QueryID(c, "actor_id", false)
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
	queryPage, querySize := pagex.Int32(page, size)
	return contracts.AuditQuery{ActorID: actorID, Action: strings.TrimSpace(c.Query("action")), TargetType: strings.TrimSpace(c.Query("target_type")), From: from, To: to, Page: queryPage, Size: querySize}, true
}

// queryRFC3339 解析可选 RFC3339 时间查询参数。
func queryRFC3339(c *gin.Context, key string) (time.Time, bool) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return time.Time{}, true
	}
	value, err := timex.ParseRFC3339(raw)
	if err != nil {
		response.Fail(c, apperr.ErrAdminAuditQueryInvalid)
		return time.Time{}, false
	}
	return value, true
}

// auditLogEntryDTOs 将统一审计契约转换为管理端 HTTP DTO。
func auditLogEntryDTOs(items []contracts.AuditLogEntry) []AuditLogEntryDTO {
	out := make([]AuditLogEntryDTO, 0, len(items))
	for _, item := range items {
		out = append(out, AuditLogEntryDTO{
			ID:         ids.ID(item.ID),
			TenantID:   ids.ID(item.TenantID),
			ActorID:    ids.ID(item.ActorID),
			ActorRole:  item.ActorRole,
			Action:     item.Action,
			TargetType: item.TargetType,
			TargetID:   ids.ID(item.TargetID),
			Detail:     item.Detail,
			IP:         item.IP,
			TraceID:    item.TraceID,
			CreatedAt:  timex.RFC3339OrEmpty(item.CreatedAt),
		})
	}
	return out
}
