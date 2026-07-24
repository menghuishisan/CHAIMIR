// experiment api 文件负责注册 M7 HTTP 路由、绑定请求和组合鉴权,不承载实验业务逻辑。
package experiment

import (
	"context"
	"strconv"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/httpx"
	"chaimir/internal/platform/response"
	"chaimir/internal/platform/upload"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册实验模块 HTTP API。
func RegisterRoutes(r gin.IRouter, svc *Service, authn *auth.Manager, roles contracts.IdentityService) error {
	if r == nil {
		return apperr.ErrHTTPRouterMissing
	}
	if svc == nil {
		return apperr.ErrHTTPServiceMissing
	}
	if authn == nil {
		return apperr.ErrHTTPAuthMissing
	}
	api := experimentAPI{svc: svc}
	g := r.Group("/api/v1/experiment")
	teacher := g.Group("", authn.Middleware(), auth.RequireTenantAnyRole(roles, contracts.RoleTeacher, contracts.RoleSchoolAdmin))
	student := g.Group("", authn.Middleware(), auth.RequireTenantAnyRole(roles, contracts.RoleStudent))
	all := g.Group("", authn.Middleware(), auth.RequireTenantAnyRole(roles, contracts.RoleStudent, contracts.RoleTeacher, contracts.RoleSchoolAdmin))
	internal := g.Group("/internal", authn.ServiceMiddleware())
	api.registerTeacherRoutes(teacher)
	api.registerStudentRoutes(student)
	api.registerSharedRoutes(all)
	api.registerInternalRoutes(internal)
	return nil
}

type experimentAPI struct {
	svc *Service
}

// registerTeacherRoutes 注册教师实验配置、报告批改和分组管理接口。
func (a experimentAPI) registerTeacherRoutes(g gin.IRouter) {
	g.GET("/experiments", a.listExperiments)
	g.POST("/experiments", a.createExperiment)
	g.PATCH("/experiments/:id", a.updateExperiment)
	g.POST("/experiments/:id/validate", a.validateExperiment)
	g.POST("/experiments/:id/publish", a.publishExperiment)
	g.POST("/experiments/:id/unpublish", a.unpublishExperiment)
	g.GET("/experiments/:id/reports", a.listReports)
	g.POST("/reports/:id/download-grant", a.reportDownloadGrant)
	g.POST("/reports/:id/grade", a.gradeReport)
	g.POST("/experiments/:id/groups", a.createGroup)
	g.POST("/groups/:id/members", a.upsertGroupMember)
}

// registerStudentRoutes 注册学生发起实例、判分和报告接口。
func (a experimentAPI) registerStudentRoutes(g gin.IRouter) {
	g.GET("/student/experiments", a.listPublishedExperiments)
	g.GET("/student/experiments/:id", a.getPublishedExperiment)
	g.POST("/experiments/:id/instances", a.createInstance)
	g.POST("/instances/:id/checkpoints/:cp/judge", a.judgeCheckpoint)
	g.POST("/instances/:id/report", a.submitReport)
}

// listPublishedExperiments 仅向学生返回当前租户已发布实验。
func (a experimentAPI) listPublishedExperiments(c *gin.Context) {
	page, size, ok := httpx.Page(c)
	if !ok {
		return
	}
	out, total, p, s, err := a.svc.ListPublishedExperiments(c.Request.Context(), page, size)
	httpx.WritePage(c, out, total, p, s, err)
}

// getPublishedExperiment 返回单条学生实验，课程实验从请求头读取 M6 启动授权。
func (a experimentAPI) getPublishedExperiment(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.GetPublishedExperiment(c.Request.Context(), id, strings.TrimSpace(c.GetHeader("X-Experiment-Launch-Grant")))
	httpx.Write(c, out, err)
}

// registerSharedRoutes 注册师生共享的实例工作台和控制接口。
func (a experimentAPI) registerSharedRoutes(g gin.IRouter) {
	g.GET("/instances/:id", a.getInstance)
	g.GET("/instances/:id/progress", a.getProgress)
	g.POST("/instances/:id/pause", a.pauseInstance)
	g.POST("/instances/:id/resume", a.resumeInstance)
	g.POST("/instances/:id/stages/:stage/activate", a.activateStage)
	g.POST("/instances/:id/finish", a.finishInstance)
	g.DELETE("/instances/:id", a.recycleInstance)
	g.GET("/groups/:id", a.getGroup)
}

// registerInternalRoutes 注册内部只读接口。
func (a experimentAPI) registerInternalRoutes(g gin.IRouter) {
	g.GET("/instances/:id/score", a.internalScore)
	g.GET("/stats", a.internalStats)
}

// listExperiments 绑定实验列表过滤参数。
func (a experimentAPI) listExperiments(c *gin.Context) {
	courseID, ok := httpx.QueryInt(c, "course_id", httpx.QueryIntRule{Default: 0, Min: 0})
	if !ok {
		return
	}
	status, ok := httpx.QueryInt(c, "status", httpx.QueryIntRule{Default: 0, Min: 0, Max: 3, HasMax: true})
	if !ok {
		return
	}
	page, size, ok := httpx.Page(c)
	if !ok {
		return
	}
	out, total, p, s, err := a.svc.ListExperiments(c.Request.Context(), courseID, int16(status), page, size)
	httpx.WritePage(c, out, total, p, s, err)
}

// createExperiment 绑定创建实验草稿请求。
func (a experimentAPI) createExperiment(c *gin.Context) {
	var req ExperimentRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrExperimentInvalid) {
		return
	}
	out, err := a.svc.CreateExperiment(c.Request.Context(), req)
	httpx.Write(c, out, err)
}

// updateExperiment 绑定实验草稿更新请求。
func (a experimentAPI) updateExperiment(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req ExperimentRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrExperimentInvalid) {
		return
	}
	out, err := a.svc.UpdateExperiment(c.Request.Context(), id, req)
	httpx.Write(c, out, err)
}

// validateExperiment 执行发布前校验。
func (a experimentAPI) validateExperiment(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.ValidateExperiment(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// publishExperiment 发布实验。
func (a experimentAPI) publishExperiment(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.PublishExperiment(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// unpublishExperiment 下架实验。
func (a experimentAPI) unpublishExperiment(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.UnpublishExperiment(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// createInstance 绑定学生发起实例请求。
func (a experimentAPI) createInstance(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req CreateInstanceRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrExperimentInstanceInvalid) {
		return
	}
	out, err := a.svc.CreateInstance(c.Request.Context(), id, req)
	httpx.Write(c, out, err)
}

// getInstance 读取实验工作台。
func (a experimentAPI) getInstance(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.GetInstance(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// getProgress 返回 M10 订阅元信息。
func (a experimentAPI) getProgress(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.GetProgress(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// pauseInstance 暂停实例。
func (a experimentAPI) pauseInstance(c *gin.Context) {
	a.writeInstanceAction(c, a.svc.PauseInstance)
}

// resumeInstance 恢复实例。
func (a experimentAPI) resumeInstance(c *gin.Context) {
	a.writeInstanceAction(c, a.svc.ResumeInstance)
}

// activateStage 激活已解锁的实验阶段。
func (a experimentAPI) activateStage(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	stage, ok := parseStageParam(c)
	if !ok {
		return
	}
	out, err := a.svc.ActivateStage(c.Request.Context(), id, stage)
	httpx.Write(c, out, err)
}

// finishInstance 完成实例并汇总得分。
func (a experimentAPI) finishInstance(c *gin.Context) {
	a.writeInstanceAction(c, a.svc.FinishInstance)
}

// recycleInstance 回收实例引擎资源。
func (a experimentAPI) recycleInstance(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	httpx.Write(c, gin.H{}, a.svc.RecycleInstance(c.Request.Context(), id))
}

// writeInstanceAction 统一处理实例状态类动作,确保 handler 只把请求 context 传入业务层。
func (a experimentAPI) writeInstanceAction(c *gin.Context, fn func(context.Context, int64) (InstanceDTO, error)) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := fn(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// judgeCheckpoint 绑定检查点判分请求。
func (a experimentAPI) judgeCheckpoint(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req JudgeCheckpointRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrExperimentCheckpointInvalid) {
		return
	}
	out, err := a.svc.JudgeCheckpoint(c.Request.Context(), id, c.Param("cp"), req)
	httpx.Write(c, out, err)
}

// submitReport 限量读取 Markdown 文件并交给服务层完成上传与提交。
func (a experimentAPI) submitReport(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	file, err := c.FormFile("file")
	if err != nil || file.Size > a.svc.reportMaxBytes {
		response.Fail(c, apperr.ErrExperimentReportInvalid)
		return
	}
	opened, err := file.Open()
	if err != nil {
		response.Fail(c, apperr.ErrExperimentReportInvalid.WithCause(err))
		return
	}
	defer logging.CloseContext(c.Request.Context(), "关闭实验报告上传文件失败", opened)
	content, result, err := upload.ReadBounded(opened, a.svc.reportMaxBytes)
	if err != nil {
		response.Fail(c, apperr.ErrExperimentReportInvalid.WithCause(err))
		return
	}
	if result != upload.SizeOK {
		response.Fail(c, apperr.ErrExperimentReportInvalid)
		return
	}
	contentType := strings.TrimSpace(file.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "text/markdown"
	}
	out, err := a.svc.SubmitReport(c.Request.Context(), id, ReportUploadRequest{FileName: file.Filename, ContentType: contentType, Content: content})
	httpx.Write(c, out, err)
}

// listReports 查询实验报告列表。
func (a experimentAPI) listReports(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	page, size, ok := httpx.Page(c)
	if !ok {
		return
	}
	out, total, p, s, err := a.svc.ListReports(c.Request.Context(), id, page, size)
	httpx.WritePage(c, out, total, p, s, err)
}

// reportDownloadGrant 为教师已获授权的实验报告签发一次性下载授权。
func (a experimentAPI) reportDownloadGrant(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.IssueReportDownloadGrant(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// gradeReport 绑定教师报告批改请求。
func (a experimentAPI) gradeReport(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req GradeReportRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrExperimentScoreInvalid) {
		return
	}
	out, err := a.svc.GradeReport(c.Request.Context(), id, req)
	httpx.Write(c, out, err)
}

// createGroup 绑定创建协作小组请求。
func (a experimentAPI) createGroup(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req CreateGroupRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrExperimentGroupInvalid) {
		return
	}
	out, err := a.svc.CreateGroup(c.Request.Context(), id, req)
	httpx.Write(c, out, err)
}

// upsertGroupMember 绑定小组成员角色请求。
func (a experimentAPI) upsertGroupMember(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req UpsertGroupMemberRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrExperimentGroupInvalid) {
		return
	}
	out, err := a.svc.UpsertGroupMember(c.Request.Context(), id, req)
	httpx.Write(c, out, err)
}

// getGroup 读取小组信息。
func (a experimentAPI) getGroup(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.GetGroup(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// internalScore 读取内部实例得分快照。
func (a experimentAPI) internalScore(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.GetInstanceScore(c.Request.Context(), 0, id)
	httpx.Write(c, out, err)
}

// internalStats 读取内部实验统计。
func (a experimentAPI) internalStats(c *gin.Context) {
	courseID, ok := httpx.QueryInt(c, "course_id", httpx.QueryIntRule{Default: 0, Min: 0})
	if !ok {
		return
	}
	out, err := a.svc.Stats(c.Request.Context(), contracts.ExperimentStatsQuery{CourseID: courseID})
	httpx.Write(c, out, err)
}

// parseStageParam 显式解析阶段路径参数,避免非法输入被静默转换成第 0 阶段。
func parseStageParam(c *gin.Context) (int32, bool) {
	stage, err := strconv.ParseInt(strings.TrimSpace(c.Param("stage")), 10, 32)
	if err != nil || stage <= 0 {
		response.Fail(c, apperr.ErrPathIDInvalid)
		return 0, false
	}
	return int32(stage), true
}
