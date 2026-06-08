// M9 HTTP 接口层:注册管理后台平台/学校看板、审计、配置、告警、监控和备份路由。
package admin

import (
	"encoding/csv"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/httpx"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/response"

	"github.com/gin-gonic/gin"
)

// API 是 M9 的 HTTP 处理器。
type API struct {
	svc      *Service
	authMgr  *auth.Manager
	identity contracts.IdentityService
	deploy   config.DeployConfig
}

// NewAPI 构造 M9 API。
func NewAPI(svc *Service, authMgr *auth.Manager, identity contracts.IdentityService, deploy config.DeployConfig) *API {
	return &API{svc: svc, authMgr: authMgr, identity: identity, deploy: deploy}
}

// Register 注册 M9 路由,所有管理后台路径均需登录。
func (a *API) Register(rg *gin.RouterGroup) {
	g := rg.Group("/admin", a.authMgr.Middleware())
	{
		platform := g.Group("/platform", a.requirePlatform())
		platform.GET("/dashboard", a.platformDashboard)
		platform.GET("/statistics", a.platformStatistics)
		platform.GET("/tenants", a.listTenants)
		platform.GET("/applications", a.listApplications)
		platform.POST("/applications/:id/approve", a.approveApplication)
		platform.POST("/applications/:id/reject", a.rejectApplication)

		school := g.Group("/school", a.requireSchoolAdmin())
		school.GET("/dashboard", a.schoolDashboard)
		school.GET("/statistics", a.schoolStatistics)

		admin := g.Group("", a.requireAnyAdmin())
		admin.GET("/audit", a.listAudit)
		admin.GET("/audit/export", a.exportAudit)
		admin.GET("/configs", a.listConfigs)
		admin.PUT("/configs/:key", a.updateConfig)
		admin.GET("/configs/:key/history", a.configHistory)
		admin.POST("/configs/:key/rollback", a.rollbackConfig)
		admin.GET("/alert-rules", a.listAlertRules)
		admin.POST("/alert-rules", a.createAlertRule)
		admin.PATCH("/alert-rules/:id", a.updateAlertRule)
		admin.GET("/alert-events", a.listAlertEvents)
		admin.POST("/alert-events/:id/handle", a.handleAlertEvent)
		admin.GET("/monitoring/panels", a.requirePlatform(), a.monitoringPanels)
		admin.GET("/backups", a.requirePlatform(), a.listBackups)
		admin.POST("/backups/trigger", a.requirePlatform(), a.triggerBackup)
	}
}

// requirePlatform 要求当前请求来自 SaaS 平台管理员。
func (a *API) requirePlatform() gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := tenant.FromContext(c.Request.Context())
		if !ok {
			response.Fail(c, apperr.ErrUnauthorized)
			c.Abort()
			return
		}
		if !id.IsPlatform || !a.deploy.PlatformEnabled {
			response.Fail(c, apperr.ErrForbidden)
			c.Abort()
			return
		}
		c.Next()
	}
}

// requireSchoolAdmin 要求当前请求来自租户内学校管理员。
func (a *API) requireSchoolAdmin() gin.HandlerFunc {
	return auth.RequireTenantAnyRole(a.identity, contracts.RoleSchoolAdmin)
}

// requireAnyAdmin 要求当前请求来自平台管理员或学校管理员。
func (a *API) requireAnyAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := tenant.FromContext(c.Request.Context())
		if !ok {
			response.Fail(c, apperr.ErrUnauthorized)
			c.Abort()
			return
		}
		if id.IsPlatform {
			if !a.deploy.PlatformEnabled {
				response.Fail(c, apperr.ErrForbidden)
				c.Abort()
				return
			}
			c.Next()
			return
		}
		a.requireSchoolAdmin()(c)
	}
}

// platformDashboard 返回平台全局聚合看板。
func (a *API) platformDashboard(c *gin.Context) {
	out, err := a.svc.PlatformDashboard(c.Request.Context())
	httpx.Write(c, out, err)
}

// platformStatistics 返回平台周期统计快照。
func (a *API) platformStatistics(c *gin.Context) {
	from, to, ok := dateRange(c)
	if ok {
		out, err := a.svc.PlatformStatistics(c.Request.Context(), from, to)
		httpx.Write(c, out, err)
	}
}

// listTenants 转发查询 M1 租户列表。
func (a *API) listTenants(c *gin.Context) {
	items, total, err := a.svc.ListTenants(c.Request.Context(), httpx.Int16(c.Query("status")), httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
	page, size := pagex.Normalize(httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
	httpx.WritePage(c, items, total, page, size, err)
}

// listApplications 转发查询 M1 学校入驻申请。
func (a *API) listApplications(c *gin.Context) {
	items, total, err := a.svc.ListApplications(c.Request.Context(), httpx.Int16(c.Query("status")), httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
	page, size := pagex.Normalize(httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
	httpx.WritePage(c, items, total, page, size, err)
}

// approveApplication 转发 M1 完成学校入驻审核通过。
func (a *API) approveApplication(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req ApplicationApproveRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrAdminApplicationInvalid) {
		out, err := a.svc.ApproveApplication(c.Request.Context(), id, req)
		httpx.Write(c, out, err)
	}
}

// rejectApplication 转发 M1 驳回学校入驻申请。
func (a *API) rejectApplication(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req ApplicationRejectRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrAdminApplicationInvalid) {
		err := a.svc.RejectApplication(c.Request.Context(), id, req)
		httpx.Write(c, map[string]any{"rejected": true}, err)
	}
}

// schoolDashboard 返回当前学校聚合看板。
func (a *API) schoolDashboard(c *gin.Context) {
	out, err := a.svc.SchoolDashboard(c.Request.Context())
	httpx.Write(c, out, err)
}

// schoolStatistics 返回当前学校周期统计快照。
func (a *API) schoolStatistics(c *gin.Context) {
	from, to, ok := dateRange(c)
	if ok {
		out, err := a.svc.SchoolStatistics(c.Request.Context(), from, to)
		httpx.Write(c, out, err)
	}
}

// listAudit 查询统一审计中心记录。
func (a *API) listAudit(c *gin.Context) {
	query, ok := auditQuery(c)
	if !ok {
		return
	}
	items, total, err := a.svc.ListAudit(c.Request.Context(), query, httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
	page, size := pagex.Normalize(httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
	httpx.WritePage(c, items, total, page, size, err)
}

// exportAudit 导出审计记录为 CSV。
func (a *API) exportAudit(c *gin.Context) {
	query, ok := auditQuery(c)
	if !ok {
		return
	}
	rows, err := a.svc.ExportAudit(c.Request.Context(), query)
	if err != nil {
		response.Fail(c, err)
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="audit.csv"`)
	writer := csv.NewWriter(c.Writer)
	if err := writer.Write([]string{"id", "tenant_id", "actor_id", "action", "target_type", "target_id", "trace_id", "created_at"}); err != nil {
		response.Fail(c, apperr.ErrAdminAuditExport.WithCause(err))
		return
	}
	for _, row := range rows {
		if err := writer.Write(auditCSVRow(row)); err != nil {
			response.Fail(c, apperr.ErrAdminAuditExport.WithCause(err))
			return
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		response.Fail(c, apperr.ErrAdminAuditExport.WithCause(err))
	}
}

// listConfigs 查询系统配置列表。
func (a *API) listConfigs(c *gin.Context) {
	out, err := a.svc.ListConfigs(c.Request.Context(), httpx.Int16(c.Query("scope")))
	httpx.Write(c, out, err)
}

// updateConfig 使用乐观锁更新系统配置。
func (a *API) updateConfig(c *gin.Context) {
	var req ConfigUpdateRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrAdminConfigInvalid) {
		out, err := a.svc.UpdateConfig(c.Request.Context(), c.Param("key"), req)
		httpx.Write(c, out, err)
	}
}

// configHistory 查询配置变更历史。
func (a *API) configHistory(c *gin.Context) {
	items, total, err := a.svc.ConfigHistory(c.Request.Context(), c.Param("key"), httpx.Int16(c.Query("scope")), httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
	page, size := pagex.Normalize(httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
	httpx.WritePage(c, items, total, page, size, err)
}

// rollbackConfig 按历史记录回退系统配置。
func (a *API) rollbackConfig(c *gin.Context) {
	var req ConfigRollbackRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrAdminConfigInvalid) {
		out, err := a.svc.RollbackConfig(c.Request.Context(), c.Param("key"), req)
		httpx.Write(c, out, err)
	}
}

// listAlertRules 查询业务级告警规则。
func (a *API) listAlertRules(c *gin.Context) {
	items, total, err := a.svc.ListAlertRules(c.Request.Context(), httpx.Int16(c.Query("scope")), httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
	page, size := pagex.Normalize(httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
	httpx.WritePage(c, items, total, page, size, err)
}

// createAlertRule 创建业务级告警规则。
func (a *API) createAlertRule(c *gin.Context) {
	var req AlertRuleRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrAdminAlertInvalid) {
		out, err := a.svc.CreateAlertRule(c.Request.Context(), req)
		httpx.Write(c, out, err)
	}
}

// updateAlertRule 更新业务级告警规则。
func (a *API) updateAlertRule(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req AlertRulePatchRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrAdminAlertInvalid) {
		out, err := a.svc.UpdateAlertRule(c.Request.Context(), id, req)
		httpx.Write(c, out, err)
	}
}

// listAlertEvents 查询业务级告警事件。
func (a *API) listAlertEvents(c *gin.Context) {
	items, total, err := a.svc.ListAlertEvents(c.Request.Context(), httpx.Int16(c.Query("status")), httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
	page, size := pagex.Normalize(httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
	httpx.WritePage(c, items, total, page, size, err)
}

// handleAlertEvent 处理或忽略告警事件。
func (a *API) handleAlertEvent(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req AlertHandleRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrAdminAlertInvalid) {
		out, err := a.svc.HandleAlertEvent(c.Request.Context(), id, req)
		httpx.Write(c, out, err)
	}
}

// monitoringPanels 返回外接监控面板嵌入入口。
func (a *API) monitoringPanels(c *gin.Context) {
	out, err := a.svc.MonitoringPanels(c.Request.Context())
	httpx.Write(c, out, err)
}

// listBackups 查询备份记录。
func (a *API) listBackups(c *gin.Context) {
	items, total, err := a.svc.ListBackups(c.Request.Context(), httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
	page, size := pagex.Normalize(httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
	httpx.WritePage(c, items, total, page, size, err)
}

// triggerBackup 记录一次备份触发请求。
func (a *API) triggerBackup(c *gin.Context) {
	var req BackupTriggerRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrAdminBackupInvalid) {
		out, err := a.svc.TriggerBackup(c.Request.Context(), req)
		httpx.Write(c, out, err)
	}
}

// dateRange 解析统计查询日期范围,默认查询当天。
func dateRange(c *gin.Context) (time.Time, time.Time, bool) {
	from, ok := parseDateParam(c, "from")
	if !ok {
		return time.Time{}, time.Time{}, false
	}
	to, ok := parseDateParam(c, "to")
	if !ok {
		return time.Time{}, time.Time{}, false
	}
	return from, to, true
}

// parseDateParam 解析 YYYY-MM-DD 日期参数,空值由 repo 使用当天默认值。
func parseDateParam(c *gin.Context, name string) (time.Time, bool) {
	raw := c.Query(name)
	if raw == "" {
		return time.Time{}, true
	}
	t, err := time.Parse(time.DateOnly, raw)
	if err != nil {
		response.Fail(c, apperr.ErrAdminStatisticsQueryInvalid)
		return time.Time{}, false
	}
	return t, true
}

// auditQuery 解析审计查询过滤条件。
func auditQuery(c *gin.Context) (contracts.AuditQuery, bool) {
	var out contracts.AuditQuery
	out.ActorID = ids.ParseOrZero(c.Query("actor_id"))
	out.Action = c.Query("action")
	out.TargetType = c.Query("target_type")
	from, ok := parseTimeParam(c, "from")
	if !ok {
		return contracts.AuditQuery{}, false
	}
	to, ok := parseTimeParam(c, "to")
	if !ok {
		return contracts.AuditQuery{}, false
	}
	out.From = from
	out.To = to
	return out, true
}

// parseTimeParam 解析 RFC3339 时间参数。
func parseTimeParam(c *gin.Context, name string) (*time.Time, bool) {
	raw := c.Query(name)
	if raw == "" {
		return nil, true
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		response.Fail(c, apperr.ErrAdminAuditQueryInvalid)
		return nil, false
	}
	return &t, true
}

// auditCSVRow 转换审计记录为 CSV 字段。
func auditCSVRow(row contracts.AuditRecord) []string {
	return []string{
		ids.Format(row.ID),
		ids.Format(row.TenantID),
		ids.Format(row.ActorID),
		row.Action,
		row.TargetType,
		ids.Format(row.TargetID),
		row.TraceID,
		timex.UTC(row.CreatedAt).Format(time.RFC3339),
	}
}
