// M8 HTTP 接口层:注册 /api/v1/contest 下的竞赛、报名、解题、对抗、防作弊、漏洞源和内部聚合接口。
package contest

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/httpx"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
	"chaimir/pkg/response"

	"github.com/gin-gonic/gin"
)

// API 是 M8 的 HTTP 处理器。
type API struct {
	svc      *Service
	authMgr  *auth.Manager
	identity contracts.IdentityService
}

// NewAPI 构造 M8 HTTP 处理器,注入竞赛服务、鉴权管理器和身份只读契约。
func NewAPI(svc *Service, authMgr *auth.Manager, identity contracts.IdentityService) *API {
	return &API{svc: svc, authMgr: authMgr, identity: identity}
}

// Register 注册 M8 路由,竞赛用户路径走登录鉴权,聚合只读接口走服务鉴权。
func (a *API) Register(rg *gin.RouterGroup) {
	g := rg.Group("/contest", a.authMgr.Middleware())
	{
		g.GET("/contests", a.listContests)
		g.POST("/contests", a.requireTeacher(), a.createContest)
		g.PATCH("/contests/:id", a.requireTeacher(), a.updateContest)
		g.POST("/contests/:id/problems", a.requireTeacher(), a.addProblem)
		g.POST("/contests/:id/publish", a.requireTeacher(), a.publishContest)
		g.POST("/contests/:id/start", a.requireTeacher(), a.startContest)
		g.POST("/contests/:id/end", a.requireTeacher(), a.endContest)
		g.POST("/contests/:id/archive", a.requireTeacher(), a.archiveContest)
		g.POST("/contests/:id/signup", a.signup)
		g.POST("/teams/:id/join", a.joinTeam)
		g.GET("/teams/:id", a.getTeam)
		g.POST("/teams/:id/lock", a.lockTeam)
		g.GET("/contests/:id/problems", a.listProblems)
		g.POST("/contests/:id/problems/:pid/env", a.startProblemEnv)
		g.POST("/contests/:id/problems/:pid/submit", a.submitSolve)
		g.GET("/submissions/:id", a.getSubmission)
		g.POST("/contests/:id/battle/entry", a.submitBattleEntry)
		g.GET("/contests/:id/battle/entries", a.listBattleEntries)
		g.GET("/contests/:id/battle/matches", a.listBattleMatches)
		g.GET("/matches/:id/replay", a.getReplay)
		g.GET("/contests/:id/ladder", a.listLadder)
		g.GET("/my/contest-records", a.myRecords)
		g.GET("/contests/:id/result-snapshot", a.resultSnapshot)
		g.GET("/contests/:id/cheat-suspects", a.requireTeacher(), a.cheatSuspects)
		g.POST("/contests/:id/cheat-records", a.requireTeacher(), a.createCheatRecord)
		g.GET("/vuln-sources", a.requireTeacher(), a.listVulnSources)
		g.POST("/vuln-sources", a.requireTeacher(), a.createVulnSource)
		g.POST("/vuln-sources/:id/sync", a.requireTeacher(), a.syncVulnSource)
		g.POST("/vuln-sources/import", a.requireTeacher(), a.importVulnProblem)
		g.POST("/vuln-problems/:id/prevalidate", a.requireTeacher(), a.prevalidateVulnProblem)
		g.POST("/vuln-problems/:id/finalize", a.requireTeacher(), a.finalizeVulnProblem)
	}
	internal := rg.Group("/contest", a.authMgr.ServiceMiddleware())
	{
		internal.GET("/internal/stats", a.internalStats)
		internal.GET("/students/:id/contest-achievements", a.studentAchievements)
	}
}

// requireTeacher 要求当前账号具备教师或学校管理员角色。
func (a *API) requireTeacher() gin.HandlerFunc {
	return auth.RequirePlatformOrAnyRole(a.identity, contracts.RoleTeacher, contracts.RoleSchoolAdmin)
}

// listContests 按状态分页查询竞赛列表,登录账号的可见性由服务层过滤。
func (a *API) listContests(c *gin.Context) {
	items, total, err := a.svc.ListContests(c.Request.Context(), httpx.Int16(c.Query("status")), httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
	if err != nil {
		response.Fail(c, err)
		return
	}
	page, size := pagex.Normalize(httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
	response.OKPage(c, items, total, page, size)
}

// createContest 绑定竞赛创建请求,由服务层生成草稿并记录组织者身份。
func (a *API) createContest(c *gin.Context) {
	var req ContestRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrContestInvalid) {
		out, err := a.svc.CreateContest(c.Request.Context(), req)
		httpx.Write(c, out, err)
	}
}

// updateContest 更新竞赛草稿配置,状态机和组织者权限由服务层校验。
func (a *API) updateContest(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req ContestRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrContestInvalid) {
		out, err := a.svc.UpdateContest(c.Request.Context(), id, req)
		httpx.Write(c, out, err)
	}
}

// addProblem 为竞赛追加题目引用,只保存 M5 内容编码和版本而不复制题库数据。
func (a *API) addProblem(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req ContestProblemRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrContestProblem) {
		out, err := a.svc.AddContestProblem(c.Request.Context(), id, req)
		httpx.Write(c, out, err)
	}
}

// publishContest 发布竞赛并开放报名或参赛入口,发布前置条件由服务层检查。
func (a *API) publishContest(c *gin.Context) { a.contestAction(c, a.svc.PublishContest) }

// startContest 触发竞赛开始状态流转,避免 handler 层复制赛程状态机。
func (a *API) startContest(c *gin.Context) { a.contestAction(c, a.svc.StartContest) }

// endContest 触发竞赛结束状态流转,服务层负责结算和事件副作用。
func (a *API) endContest(c *gin.Context) { a.contestAction(c, a.svc.EndContest) }

// archiveContest 归档竞赛并生成成绩快照。
func (a *API) archiveContest(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.ArchiveContest(c.Request.Context(), id)
		httpx.Write(c, out, err)
	}
}

// signup 处理报名请求并创建或加入队伍,成员身份来自服务端会话。
func (a *API) signup(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req SignupRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrContestTeamInvalid) {
		out, err := a.svc.Signup(c.Request.Context(), id, req)
		httpx.Write(c, out, err)
	}
}

// joinTeam 使用邀请码加入已有队伍,服务层校验赛制、租户和队伍状态。
func (a *API) joinTeam(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req JoinTeamRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrContestTeamInvalid) {
		out, err := a.svc.JoinTeam(c.Request.Context(), id, req)
		httpx.Write(c, out, err)
	}
}

// getTeam 查询队伍信息。
func (a *API) getTeam(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.GetTeam(c.Request.Context(), id)
		httpx.Write(c, out, err)
	}
}

// lockTeam 锁定队伍名单,防止开赛前后成员继续变更。
func (a *API) lockTeam(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.LockTeam(c.Request.Context(), id)
		httpx.Write(c, out, err)
	}
}

// listProblems 查询竞赛题面列表,只返回参赛视角允许看到的字段。
func (a *API) listProblems(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.ListProblems(c.Request.Context(), id)
		httpx.Write(c, out, err)
	}
}

// startProblemEnv 为实操题创建沙箱环境,通过服务层调用 M2 契约而不直接编排。
func (a *API) startProblemEnv(c *gin.Context) {
	contestID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	problemID, ok := httpx.PathID(c, "pid")
	if !ok {
		return
	}
	var req StartProblemEnvRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrContestProblem) {
		out, err := a.svc.StartProblemEnv(c.Request.Context(), contestID, problemID, req)
		httpx.Write(c, out, err)
	}
}

// submitSolve 提交解题答案或代码引用,服务层负责创建 M3 判题任务。
func (a *API) submitSolve(c *gin.Context) {
	contestID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	problemID, ok := httpx.PathID(c, "pid")
	if !ok {
		return
	}
	var req SolveSubmitRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrContestSubmissionInvalid) {
		out, err := a.svc.SubmitSolve(c.Request.Context(), contestID, problemID, req)
		httpx.Write(c, out, err)
	}
}

// getSubmission 查询提交结果。
func (a *API) getSubmission(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.GetSubmission(c.Request.Context(), id)
		httpx.Write(c, out, err)
	}
}

// submitBattleEntry 提交对抗赛参战物引用,不在竞赛模块复制制品内容。
func (a *API) submitBattleEntry(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req BattleEntryRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrContestBattleInvalid) {
		out, err := a.svc.SubmitBattleEntry(c.Request.Context(), id, req)
		httpx.Write(c, out, err)
	}
}

// listBattleEntries 查询队伍参战物历史,用于教师审核和学生回看版本。
func (a *API) listBattleEntries(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.ListBattleEntries(c.Request.Context(), id, ids.ParseOrZero(c.Query("team_id")))
		httpx.Write(c, out, err)
	}
}

// listBattleMatches 按队伍查询对局列表。
func (a *API) listBattleMatches(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.ListBattleMatches(c.Request.Context(), id, ids.ParseOrZero(c.Query("team_id")), httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
		httpx.Write(c, out, err)
	}
}

// getReplay 返回对局回放引用。
func (a *API) getReplay(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.GetMatchReplay(c.Request.Context(), id)
		httpx.Write(c, out, err)
	}
}

// listLadder 查询排行榜分页数据,排名计算和快照持久化留在服务层。
func (a *API) listLadder(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.ListLadder(c.Request.Context(), id, httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
		httpx.Write(c, out, err)
	}
}

// myRecords 查询当前学生竞赛战绩。
func (a *API) myRecords(c *gin.Context) {
	out, err := a.svc.ListMyContestRecords(c.Request.Context())
	httpx.Write(c, out, err)
}

// resultSnapshot 查询竞赛成绩快照。
func (a *API) resultSnapshot(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.GetResultSnapshot(c.Request.Context(), id)
		httpx.Write(c, out, err)
	}
}

// cheatSuspects 查询作弊疑似记录。
func (a *API) cheatSuspects(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.ListCheatSuspects(c.Request.Context(), id, httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
		httpx.Write(c, out, err)
	}
}

// createCheatRecord 写入教师确认后的作弊处理记录,同时保留审计证据。
func (a *API) createCheatRecord(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req CheatRecordRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrContestBattleInvalid) {
		out, err := a.svc.CreateCheatRecord(c.Request.Context(), id, req)
		httpx.Write(c, out, err)
	}
}

// listVulnSources 查询教师维护的漏洞源配置,敏感字段在 DTO 转换时隔离。
func (a *API) listVulnSources(c *gin.Context) {
	out, err := a.svc.ListVulnSources(c.Request.Context(), httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
	httpx.Write(c, out, err)
}

// createVulnSource 创建漏洞源配置,外部端点和映射规则由服务层安全校验。
func (a *API) createVulnSource(c *gin.Context) {
	var req VulnSourceRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrContestVulnSourceInvalid) {
		out, err := a.svc.CreateVulnSource(c.Request.Context(), req)
		httpx.Write(c, out, err)
	}
}

// syncVulnSource 手动触发漏洞源拉取,外部请求必须走受限 HTTP 客户端。
func (a *API) syncVulnSource(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.SyncVulnSource(c.Request.Context(), id)
		httpx.Write(c, out, err)
	}
}

// importVulnProblem 导入单个漏洞案例草稿,只生成 M8 草稿不直接发布到题库。
func (a *API) importVulnProblem(c *gin.Context) {
	var req VulnProblemImportRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrContestVulnProblemInvalid) {
		out, err := a.svc.ImportVulnProblem(c.Request.Context(), req)
		httpx.Write(c, out, err)
	}
}

// prevalidateVulnProblem 写入预验证结果。
func (a *API) prevalidateVulnProblem(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req VulnPrevalidateRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrContestVulnPrevalidate) {
		out, err := a.svc.PrevalidateVulnProblem(c.Request.Context(), id, req)
		httpx.Write(c, out, err)
	}
}

// finalizeVulnProblem 固化漏洞题到 M5。
func (a *API) finalizeVulnProblem(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req VulnFinalizeRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrContestVulnFinalize) {
		out, err := a.svc.FinalizeVulnProblem(c.Request.Context(), id, req)
		httpx.Write(c, out, err)
	}
}

// internalStats 返回 M8 内部统计。
func (a *API) internalStats(c *gin.Context) {
	tenantID, ok := ids.Parse(c.Query("tenant_id"))
	if !ok {
		response.Fail(c, apperr.ErrContestStatsQueryInvalid)
		return
	}
	out, err := a.svc.StatsDTO(c.Request.Context(), tenantID)
	httpx.Write(c, out, err)
}

// studentAchievements 返回学生竞赛成就。
func (a *API) studentAchievements(c *gin.Context) {
	studentID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	id, ok := tenant.FromContext(c.Request.Context())
	if !ok {
		response.Fail(c, apperr.ErrUnauthorized)
		return
	}
	out, err := a.svc.ListStudentAchievements(c.Request.Context(), id.TenantID, studentID)
	httpx.Write(c, out, err)
}

// contestAction 执行只需竞赛 ID 的操作。
func (a *API) contestAction(c *gin.Context, fn func(context.Context, int64) (ContestDTO, error)) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := fn(c.Request.Context(), id)
		httpx.Write(c, out, err)
	}
}
