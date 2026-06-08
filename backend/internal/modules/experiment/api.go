// M7 HTTP 接口层:注册 /api/v1/experiment 下的实验定义、实例、检查点、报告、协作和内部统计接口。
package experiment

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/httpx"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/pagex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/response"

	"github.com/gin-gonic/gin"
)

// API 是 M7 的 HTTP 处理器。
type API struct {
	svc      *Service
	authMgr  *auth.Manager
	identity contracts.IdentityService
}

// NewAPI 构造 M7 API。
func NewAPI(svc *Service, authMgr *auth.Manager, identity contracts.IdentityService) *API {
	return &API{svc: svc, authMgr: authMgr, identity: identity}
}

// Register 注册 M7 路由,实验用户路径走登录鉴权,聚合只读接口走服务鉴权。
func (a *API) Register(rg *gin.RouterGroup) {
	g := rg.Group("/experiment", a.authMgr.Middleware())
	{
		g.GET("/experiments", a.requireTeacher(), a.listExperiments)
		g.POST("/experiments", a.requireTeacher(), a.createExperiment)
		g.PATCH("/experiments/:id", a.requireTeacher(), a.updateExperiment)
		g.POST("/experiments/:id/validate", a.requireTeacher(), a.validateExperiment)
		g.POST("/experiments/:id/publish", a.requireTeacher(), a.publishExperiment)
		g.POST("/experiments/:id/unpublish", a.requireTeacher(), a.unpublishExperiment)
		g.POST("/experiments/:id/instances", a.startInstance)
		g.GET("/instances/:id", a.getInstance)
		g.GET("/instances/:id/progress", a.progressInfo)
		g.POST("/instances/:id/pause", a.pauseInstance)
		g.POST("/instances/:id/resume", a.resumeInstance)
		g.POST("/instances/:id/finish", a.finishInstance)
		g.DELETE("/instances/:id", a.recycleInstance)
		g.POST("/instances/:id/checkpoints/:cp/judge", a.judgeCheckpoint)
		g.POST("/instances/:id/report", a.submitReport)
		g.GET("/experiments/:id/reports", a.requireTeacher(), a.listReports)
		g.POST("/reports/:id/grade", a.requireTeacher(), a.gradeReport)
		g.POST("/experiments/:id/groups", a.requireTeacher(), a.createGroup)
		g.POST("/groups/:id/members", a.requireTeacher(), a.addGroupMember)
		g.GET("/groups/:id", a.getGroup)
	}
	internal := rg.Group("/experiment", a.authMgr.ServiceMiddleware())
	{
		internal.GET("/instances/:id/score", a.getInstanceScore)
		internal.GET("/internal/stats", a.internalStats)
	}
}

// requireTeacher 要求当前账号具备教师或学校管理员角色。
func (a *API) requireTeacher() gin.HandlerFunc {
	return auth.RequirePlatformOrAnyRole(a.identity, contracts.RoleTeacher, contracts.RoleSchoolAdmin)
}

// listExperiments 查询实验列表。
func (a *API) listExperiments(c *gin.Context) {
	courseID, _ := ids.Parse(c.Query("course_id"))
	items, total, err := a.svc.ListExperiments(c.Request.Context(), courseID, httpx.Int16(c.Query("status")), httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
	if err != nil {
		response.Fail(c, err)
		return
	}
	page, size := pagex.Normalize(httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
	response.OKPage(c, items, total, page, size)
}

// createExperiment 创建实验草稿。
func (a *API) createExperiment(c *gin.Context) {
	var req ExperimentRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrExperimentInvalid) {
		out, err := a.svc.CreateExperiment(c.Request.Context(), req)
		httpx.Write(c, out, err)
	}
}

// updateExperiment 更新实验草稿。
func (a *API) updateExperiment(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req ExperimentRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrExperimentInvalid) {
		out, err := a.svc.UpdateExperiment(c.Request.Context(), id, req)
		httpx.Write(c, out, err)
	}
}

// validateExperiment 执行发布前校验。
func (a *API) validateExperiment(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.ValidateExperiment(c.Request.Context(), id)
		httpx.Write(c, out, err)
	}
}

// publishExperiment 发布实验。
func (a *API) publishExperiment(c *gin.Context) {
	a.experimentAction(c, a.svc.PublishExperiment)
}

// unpublishExperiment 下架实验。
func (a *API) unpublishExperiment(c *gin.Context) {
	a.experimentAction(c, a.svc.UnpublishExperiment)
}

// startInstance 创建实验实例。
func (a *API) startInstance(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req StartInstanceRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrExperimentInstanceInvalid) {
		out, err := a.svc.StartInstance(c.Request.Context(), id, req)
		httpx.Write(c, out, err)
	}
}

// getInstance 查询实验实例工作台摘要。
func (a *API) getInstance(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.GetInstance(c.Request.Context(), id)
		httpx.Write(c, out, err)
	}
}

// progressInfo 返回实例进度订阅元信息,业务实时广播由 M10 统一 WS Hub 承载。
func (a *API) progressInfo(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		if _, err := a.svc.GetInstance(c.Request.Context(), id); err != nil {
			response.Fail(c, err)
			return
		}
		response.OK(c, map[string]any{"instance_id": ids.Format(id), "topic": "experiment:" + ids.Format(id) + ":progress"})
	}
}

// pauseInstance 暂停实例。
func (a *API) pauseInstance(c *gin.Context) {
	a.instanceAction(c, a.svc.PauseInstance)
}

// resumeInstance 恢复实例。
func (a *API) resumeInstance(c *gin.Context) {
	a.instanceAction(c, a.svc.ResumeInstance)
}

// finishInstance 完成实例并汇总得分。
func (a *API) finishInstance(c *gin.Context) {
	a.instanceAction(c, a.svc.FinishInstance)
}

// recycleInstance 回收实例资源。
func (a *API) recycleInstance(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		err := a.svc.RecycleInstance(c.Request.Context(), id)
		httpx.Write(c, map[string]any{"recycled": true}, err)
	}
}

// judgeCheckpoint 触发检查点判分。
func (a *API) judgeCheckpoint(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.JudgeCheckpoint(c.Request.Context(), id, c.Param("cp"))
		httpx.Write(c, out, err)
	}
}

// submitReport 提交实验报告。
func (a *API) submitReport(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req ReportRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrExperimentReportInvalid) {
		out, err := a.svc.SubmitReport(c.Request.Context(), id, req)
		httpx.Write(c, out, err)
	}
}

// listReports 查询实验报告列表。
func (a *API) listReports(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.ListReports(c.Request.Context(), id, httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
		httpx.Write(c, out, err)
	}
}

// gradeReport 批改实验报告。
func (a *API) gradeReport(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req ReportGradeRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrExperimentReportInvalid) {
		out, err := a.svc.GradeReport(c.Request.Context(), id, req)
		httpx.Write(c, out, err)
	}
}

// createGroup 创建协作小组。
func (a *API) createGroup(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req GroupRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrExperimentGroupInvalid) {
		out, err := a.svc.CreateGroup(c.Request.Context(), id, req)
		httpx.Write(c, out, err)
	}
}

// addGroupMember 添加小组成员。
func (a *API) addGroupMember(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req GroupMemberRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrExperimentGroupInvalid) {
		out, err := a.svc.AddGroupMember(c.Request.Context(), id, req)
		httpx.Write(c, out, err)
	}
}

// getGroup 查询协作小组。
func (a *API) getGroup(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.GetGroup(c.Request.Context(), id)
		httpx.Write(c, out, err)
	}
}

// getInstanceScore 读取实例得分。
func (a *API) getInstanceScore(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	instance, err := a.svc.GetInstance(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, map[string]any{"instance_id": instance.ID, "score": instance.Score})
}

// internalStats 返回 M7 内部统计。
func (a *API) internalStats(c *gin.Context) {
	tenantID, ok := ids.Parse(c.Query("tenant_id"))
	if !ok {
		response.Fail(c, apperr.ErrExperimentStatsQueryInvalid)
		return
	}
	courseID, _ := ids.Parse(c.Query("course_id"))
	out, err := a.svc.StatsDTO(c.Request.Context(), tenantID, courseID)
	httpx.Write(c, out, err)
}

// experimentAction 执行只需实验 ID 的操作。
func (a *API) experimentAction(c *gin.Context, fn func(context.Context, int64) (ExperimentDTO, error)) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := fn(c.Request.Context(), id)
		httpx.Write(c, out, err)
	}
}

// instanceAction 执行只需实例 ID 的操作。
func (a *API) instanceAction(c *gin.Context, fn func(context.Context, int64) (ExperimentInstanceDTO, error)) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := fn(c.Request.Context(), id)
		httpx.Write(c, out, err)
	}
}
