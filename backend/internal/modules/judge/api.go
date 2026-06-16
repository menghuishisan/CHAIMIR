// judge api 文件负责注册 M3 HTTP/WS 路由、绑定请求和组合鉴权,不承载判题业务逻辑。
package judge

import (
	"fmt"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/httpx"
	"chaimir/internal/platform/response"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册评测引擎 HTTP 与 WebSocket API。
func RegisterRoutes(r gin.IRouter, svc *Service, authn *auth.Manager, roles contracts.IdentityService) error {
	if r == nil {
		return fmt.Errorf("judge routes 缺少 HTTP router")
	}
	if svc == nil {
		return fmt.Errorf("judge routes 缺少 service")
	}
	if authn == nil {
		return fmt.Errorf("judge routes 缺少 auth manager")
	}
	api := judgeAPI{svc: svc}
	g := r.Group("/api/v1/judge")
	api.registerPlatformRoutes(g.Group("", authn.Middleware(), auth.RequirePlatformIdentity()))
	api.registerInternalRoutes(g.Group("", authn.ServiceMiddleware()))
	api.registerUserRoutes(g.Group("", authn.Middleware(), auth.RequireTenantAnyRole(roles, contracts.RoleTeacher, contracts.RoleSchoolAdmin)))
	g.POST("/tasks/:id/rejudge", authn.ServiceOrTenantAnyRoleMiddleware(roles, contracts.RoleTeacher, contracts.RoleSchoolAdmin), api.rejudgeTask)
	return nil
}

type judgeAPI struct {
	svc *Service
}

// registerPlatformRoutes 注册平台管理员判题器管理接口。
func (a judgeAPI) registerPlatformRoutes(g gin.IRouter) {
	g.GET("/judgers", a.listJudgers)
	g.POST("/judgers", a.createJudger)
	g.PATCH("/judgers/:id", a.updateJudger)
	g.POST("/judgers/:id/selftest", a.runJudgerSelftest)
}

// registerInternalRoutes 注册服务间判题、取消、重判和查重接口。
func (a judgeAPI) registerInternalRoutes(g gin.IRouter) {
	g.POST("/tasks", a.submitTask)
	g.DELETE("/tasks/:id", a.cancelTask)
	g.POST("/rejudge/batch", a.rejudgeBatch)
	g.GET("/fingerprints/exact", a.exactFingerprints)
	g.POST("/fingerprints/similarity", a.similarity)
}

// registerUserRoutes 注册教师侧查询、进度和人工评分接口。
func (a judgeAPI) registerUserRoutes(g gin.IRouter) {
	g.GET("/tasks", a.listTasks)
	g.GET("/tasks/:id", a.getTask)
	g.GET("/tasks/:id/progress", a.progress)
	g.POST("/tasks/:id/manual-score", a.manualScore)
}

// listJudgers 返回判题器列表。
func (a judgeAPI) listJudgers(c *gin.Context) {
	out, err := a.svc.ListJudgers(c.Request.Context())
	httpx.Write(c, out, err)
}

// createJudger 绑定判题器注册请求。
func (a judgeAPI) createJudger(c *gin.Context) {
	var req CreateJudgerRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrJudgerConfigInvalid) {
		return
	}
	out, err := a.svc.CreateJudger(c.Request.Context(), req)
	httpx.Write(c, out, err)
}

// updateJudger 绑定判题器更新请求。
func (a judgeAPI) updateJudger(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req UpdateJudgerRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrJudgerConfigInvalid) {
		return
	}
	out, err := a.svc.UpdateJudger(c.Request.Context(), id, req)
	httpx.Write(c, out, err)
}

// runJudgerSelftest 触发判题器真实路径自检。
func (a judgeAPI) runJudgerSelftest(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.RunJudgerSelftest(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// submitTask 绑定内部服务提交判题请求,租户和来源以服务签名为准。
func (a judgeAPI) submitTask(c *gin.Context) {
	var req SubmitTaskRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrJudgeSubmitInvalid) {
		return
	}
	tenantID, ok := currentServiceTenantID(c)
	if !ok {
		return
	}
	sourceRef, ok := serviceSourceRef(c)
	if !ok {
		return
	}
	req.SourceRef = sourceRef
	out, err := a.svc.SubmitJudgeTask(c.Request.Context(), contractSubmitFromDTO(tenantID, req))
	httpx.Write(c, out, err)
}

// cancelTask 取消排队中的内部判题任务。
func (a judgeAPI) cancelTask(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	tenantID, ok := currentServiceTenantID(c)
	if !ok {
		return
	}
	httpx.Write(c, gin.H{}, a.svc.CancelTask(c.Request.Context(), tenantID, id))
}

// rejudgeTask 绑定内部服务或教师重判请求。
func (a judgeAPI) rejudgeTask(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	if current, ok := tenant.FromContext(c.Request.Context()); ok {
		if current.IsSystem && current.TenantID > 0 {
			out, err := a.svc.RejudgeTask(c.Request.Context(), current.TenantID, id)
			httpx.Write(c, taskInfoToMap(out), err)
			return
		}
		if current.TenantID > 0 && current.AccountID > 0 {
			out, err := a.svc.RejudgeTaskForUser(c.Request.Context(), current.TenantID, current.AccountID, id)
			httpx.Write(c, taskInfoToMap(out), err)
			return
		}
	}
	response.Fail(c, apperr.ErrUnauthorized)
}

// rejudgeBatch 绑定按来源批量重判请求。
func (a judgeAPI) rejudgeBatch(c *gin.Context) {
	var req RejudgeBatchRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrJudgeSubmitInvalid) {
		return
	}
	tenantID, ok := currentServiceTenantID(c)
	if !ok {
		return
	}
	httpx.Write(c, gin.H{}, a.svc.RejudgeBatch(c.Request.Context(), tenantID, req.SourceRef))
}

// exactFingerprints 绑定精确哈希查重查询。
func (a judgeAPI) exactFingerprints(c *gin.Context) {
	tenantID, ok := currentServiceTenantID(c)
	if !ok {
		return
	}
	out, err := a.svc.ExactFingerprints(c.Request.Context(), tenantID, c.Query("problem_ref"), c.Query("code_hash"))
	httpx.Write(c, out, err)
}

// similarity 绑定相似度查重请求。
func (a judgeAPI) similarity(c *gin.Context) {
	var req FingerprintSimilarityRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrFingerprintRequestInvalid) {
		return
	}
	tenantID, ok := currentServiceTenantID(c)
	if !ok {
		return
	}
	out, err := a.svc.Similarity(c.Request.Context(), tenantID, req)
	httpx.Write(c, out, err)
}

// listTasks 查询教师可见的判题任务分页。
func (a judgeAPI) listTasks(c *gin.Context) {
	current, ok := currentTenantIdentity(c)
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
	items, total, p, s, err := a.svc.ListTasks(c.Request.Context(), current.TenantID, current.AccountID, c.Query("source_ref"), c.Query("pending_manual") == "true", int(page), int(size))
	httpx.WritePage(c, items, total, p, s, err)
}

// getTask 查询单个判题任务状态和结果。
func (a judgeAPI) getTask(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	current, ok := currentTenantIdentity(c)
	if !ok {
		return
	}
	out, err := a.svc.GetTaskInfoForUser(c.Request.Context(), current.TenantID, current.AccountID, id)
	httpx.Write(c, taskInfoToMap(out), err)
}

// manualScore 绑定教师人工评分请求。
func (a judgeAPI) manualScore(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	current, ok := currentTenantIdentity(c)
	if !ok {
		return
	}
	var req ManualScoreRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrJudgeManualScoreInvalid) {
		return
	}
	out, err := a.svc.ManualScore(c.Request.Context(), current.TenantID, id, current.AccountID, req)
	httpx.Write(c, out, err)
}

// progress 建立判题进度 WebSocket 订阅。
func (a judgeAPI) progress(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	current, ok := currentTenantIdentity(c)
	if !ok {
		return
	}
	if a.svc.wsHub == nil {
		response.Fail(c, apperr.ErrJudgeTaskStateInvalid)
		return
	}
	if err := a.svc.wsHub.Serve(c.Writer, c.Request, func(conn *ws.Conn) error {
		return a.svc.bindProgressConn(c.Request.Context(), conn, current.TenantID, current.AccountID, id)
	}); err != nil {
		response.Fail(c, apperr.ErrJudgeTaskStateInvalid.WithCause(err))
	}
}

// currentTenantIdentity 从服务端鉴权上下文读取租户身份。
func currentTenantIdentity(c *gin.Context) (tenant.Identity, bool) {
	id, ok := tenant.FromContext(c.Request.Context())
	if !ok || id.TenantID <= 0 || id.AccountID <= 0 {
		response.Fail(c, apperr.ErrUnauthorized)
		return tenant.Identity{}, false
	}
	return id, true
}

// currentServiceTenantID 读取内部服务租户边界,缺失时立即返回统一鉴权错误。
func currentServiceTenantID(c *gin.Context) (int64, bool) {
	id, ok := tenant.FromContext(c.Request.Context())
	if !ok || id.TenantID <= 0 || !id.IsSystem {
		response.Fail(c, apperr.ErrServiceUnauthorized)
		return 0, false
	}
	return id.TenantID, true
}

// serviceSourceRef 读取服务签名已校验来源,防止调用方伪造请求体来源。
func serviceSourceRef(c *gin.Context) (string, bool) {
	if sourceRef, ok := auth.ServiceSourceRefFromContext(c.Request.Context()); ok {
		return sourceRef, true
	}
	response.Fail(c, apperr.ErrServiceUnauthorized)
	return "", false
}
