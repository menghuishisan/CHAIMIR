// M3 HTTP 接口层:注册 /api/v1/judge 下的判题器、任务、重判、人工评分与查重接口。
package judge

import (
	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/httpx"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
	"chaimir/pkg/response"

	"github.com/gin-gonic/gin"
)

// API 是 M3 的 HTTP 处理器。
type API struct {
	svc      *Service
	authMgr  *auth.Manager
	identity contracts.IdentityService
}

// NewAPI 构造 M3 API。
func NewAPI(svc *Service, authMgr *auth.Manager, identity contracts.IdentityService) *API {
	return &API{svc: svc, authMgr: authMgr, identity: identity}
}

// Register 注册 M3 路由,判题提交/取消/查重等内部能力走服务鉴权。
func (a *API) Register(rg *gin.RouterGroup) {
	g := rg.Group("/judge", a.authMgr.Middleware())
	{
		platformG := g.Group("", a.requirePlatformAdmin())
		platformG.GET("/judgers", a.listJudgers)
		platformG.POST("/judgers", a.createJudger)
		platformG.PATCH("/judgers/:id", a.updateJudger)
		platformG.POST("/judgers/:id/selftest", a.runJudgerSelftest)

		teacherG := g.Group("", a.requireTeacher())
		teacherG.POST("/tasks/:id/rejudge", a.rejudgeTask)
		teacherG.POST("/tasks/:id/manual-score", a.manualScore)

		g.GET("/tasks", a.listTasks)
		g.GET("/tasks/:id", a.getTask)
		g.GET("/tasks/:id/progress", a.progressWS)
	}
	internal := rg.Group("/judge", a.authMgr.ServiceMiddleware())
	{
		internal.POST("/tasks", a.submitTask)
		internal.DELETE("/tasks/:id", a.cancelTask)
		internal.POST("/rejudge/batch", a.rejudgeBatch)
		internal.GET("/fingerprints/exact", a.exactFingerprints)
		internal.POST("/fingerprints/similarity", a.similarity)
	}
}

// requirePlatformAdmin 要求当前请求来自平台管理员。
func (a *API) requirePlatformAdmin() gin.HandlerFunc {
	return auth.RequirePlatformIdentity()
}

// requireTeacher 要求当前账号具备教师或学校管理员角色。
func (a *API) requireTeacher() gin.HandlerFunc {
	return func(c *gin.Context) {
		if a.authorizeTeacher(c) {
			c.Next()
		}
	}
}

// authorizeTeacher 在 handler 内复用教师权限校验,用于按查询参数区分权限的接口。
func (a *API) authorizeTeacher(c *gin.Context) bool {
	return auth.AuthorizePlatformOrAnyRole(c, a.identity, contracts.RoleTeacher, contracts.RoleSchoolAdmin)
}

// authorizeTaskReader 限定任务详情与进度读取者:提交者本人或具备教师侧权限的账号。
func (a *API) authorizeTaskReader(c *gin.Context, info contracts.JudgeTaskInfo) bool {
	id, ok := tenant.FromContext(c.Request.Context())
	if !ok {
		response.Fail(c, apperr.ErrUnauthorized)
		c.Abort()
		return false
	}
	if id.IsPlatform {
		return true
	}
	if info.TenantID > 0 && id.TenantID != info.TenantID {
		response.Fail(c, apperr.ErrCrossTenant)
		c.Abort()
		return false
	}
	if info.SubmitterID > 0 && id.AccountID == info.SubmitterID {
		return true
	}
	return a.authorizeTeacher(c)
}

// listJudgers 查询判题器配置列表。
func (a *API) listJudgers(c *gin.Context) {
	rows, err := a.svc.ListJudgers(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, rows)
}

// createJudger 注册判题器定义。
func (a *API) createJudger(c *gin.Context) {
	var req CreateJudgerRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrJudgerInvalid) {
		return
	}
	row, err := a.svc.CreateJudger(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, row)
}

// updateJudger 更新判题器定义。
func (a *API) updateJudger(c *gin.Context) {
	judgerID, ok := judgerPathID(c)
	if !ok {
		return
	}
	var req UpdateJudgerRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrJudgerInvalid) {
		return
	}
	row, err := a.svc.UpdateJudger(c.Request.Context(), judgerID, req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, row)
}

// runJudgerSelftest 触发判题器接入自检。
func (a *API) runJudgerSelftest(c *gin.Context) {
	judgerID, ok := judgerPathID(c)
	if !ok {
		return
	}
	row, err := a.svc.RunJudgerSelftest(c.Request.Context(), judgerID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, row)
}

// submitTask 接收内部判题提交并入队。
func (a *API) submitTask(c *gin.Context) {
	id, ok := tenant.FromContext(c.Request.Context())
	if !ok {
		response.Fail(c, apperr.ErrUnauthorized)
		return
	}
	var req SubmitTaskRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrJudgeTaskInvalid) {
		return
	}
	submitterID, ok := ids.Parse(req.SubmitterID)
	if !ok {
		response.Fail(c, apperr.ErrJudgeTaskInvalid)
		return
	}
	info, err := a.svc.SubmitJudgeTask(c.Request.Context(), contracts.JudgeSubmitRequest{
		TenantID: id.TenantID, JudgerCode: req.JudgerCode, ItemCode: req.ItemCode,
		ItemVersion: req.ItemVersion, CodeStorageKey: req.CodeStorageKey, CodeHash: req.CodeHash,
		SubmitterID: submitterID, SourceRef: req.SourceRef, SandboxMode: req.SandboxMode,
		TargetSandboxRef: req.TargetSandboxRef, ExtraInput: req.ExtraInput, Priority: req.Priority,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, taskInfoToMap(info))
}

// listTasks 查询任务列表,支持待人工评分和 source_ref 过滤。
func (a *API) listTasks(c *gin.Context) {
	pendingManual := c.Query("pending_manual") == "true"
	if pendingManual && !a.authorizeTeacher(c) {
		return
	}
	rows, err := a.svc.ListTasks(c.Request.Context(), c.Query("source_ref"), pendingManual)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, rows)
}

// getTask 查询判题任务摘要与结果。
func (a *API) getTask(c *gin.Context) {
	taskID, ok := taskPathID(c)
	if !ok {
		return
	}
	view, err := a.svc.getJudgeTaskView(c.Request.Context(), taskID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	if !a.authorizeTaskReader(c, view.JudgeTaskInfo) {
		return
	}
	response.OK(c, taskViewToMap(view))
}

// progressWS 建立判题进度 WebSocket。
func (a *API) progressWS(c *gin.Context) {
	taskID, ok := taskPathID(c)
	if !ok {
		return
	}
	info, err := a.svc.GetJudgeTask(c.Request.Context(), taskID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	if !a.authorizeTaskReader(c, info) {
		return
	}
	if err := a.serveProgressWS(c.Writer, c.Request, taskID); err != nil {
		response.Fail(c, err)
	}
}

// cancelTask 取消仍在队列中的判题任务。
func (a *API) cancelTask(c *gin.Context) {
	taskID, ok := taskPathID(c)
	if !ok {
		return
	}
	info, err := a.svc.CancelTask(c.Request.Context(), taskID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, taskInfoToMap(info))
}

// rejudgeTask 按原输入快照重新提交判题。
func (a *API) rejudgeTask(c *gin.Context) {
	taskID, ok := taskPathID(c)
	if !ok {
		return
	}
	info, err := a.svc.Rejudge(c.Request.Context(), taskID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, taskInfoToMap(info))
}

// manualScore 写入人工评分结果。
func (a *API) manualScore(c *gin.Context) {
	taskID, ok := taskPathID(c)
	if !ok {
		return
	}
	var req ManualScoreRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrJudgeManualScoreInvalid) {
		return
	}
	info, err := a.svc.ManualScore(c.Request.Context(), taskID, req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, taskInfoToMap(info))
}

// rejudgeBatch 按 source_ref 批量重判。
func (a *API) rejudgeBatch(c *gin.Context) {
	var req RejudgeBatchRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrJudgeTaskInvalid) {
		return
	}
	rows, err := a.svc.RejudgeBatch(c.Request.Context(), req.SourceRef)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, rows)
}

// exactFingerprints 查询同题完全相同代码哈希的提交。
func (a *API) exactFingerprints(c *gin.Context) {
	rows, err := a.svc.ExactFingerprints(c.Request.Context(), c.Query("problem_ref"), c.Query("code_hash"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, rows)
}

// similarity 计算相似度命中列表。
func (a *API) similarity(c *gin.Context) {
	var req FingerprintSimilarityRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrFingerprintInvalid) {
		return
	}
	rows, err := a.svc.Similarity(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, rows)
}

// judgerPathID 解析判题器路径 ID,失败时返回判题器配置错误码。
func judgerPathID(c *gin.Context) (int64, bool) {
	id, ok := ids.Parse(c.Param("id"))
	if !ok {
		response.Fail(c, apperr.ErrJudgerInvalid)
		return 0, false
	}
	return id, true
}

// taskPathID 解析判题任务路径 ID,失败时返回任务请求错误码。
func taskPathID(c *gin.Context) (int64, bool) {
	id, ok := ids.Parse(c.Param("id"))
	if !ok {
		response.Fail(c, apperr.ErrJudgeTaskInvalid)
		return 0, false
	}
	return id, true
}
