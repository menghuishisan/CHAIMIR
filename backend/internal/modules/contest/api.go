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

// NewAPI 构造 M8 API。
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

// listContests 查询竞赛列表。
func (a *API) listContests(c *gin.Context) {
	items, total, err := a.svc.ListContests(c.Request.Context(), httpx.Int16(c.Query("status")), httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
	if err != nil {
		response.Fail(c, err)
		return
	}
	page, size := pagex.Normalize(httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
	response.OKPage(c, items, total, page, size)
}

// createContest 创建竞赛草稿。
func (a *API) createContest(c *gin.Context) {
	var req ContestRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrContestInvalid) {
		out, err := a.svc.CreateContest(c.Request.Context(), req)
		httpx.Write(c, out, err)
	}
}

// updateContest 更新竞赛草稿。
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

// addProblem 新增竞赛题目引用。
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

// publishContest 发布竞赛。
func (a *API) publishContest(c *gin.Context) { a.contestAction(c, a.svc.PublishContest) }

// startContest 开始竞赛。
func (a *API) startContest(c *gin.Context) { a.contestAction(c, a.svc.StartContest) }

// endContest 结束竞赛。
func (a *API) endContest(c *gin.Context) { a.contestAction(c, a.svc.EndContest) }

// archiveContest 归档竞赛并生成成绩快照。
func (a *API) archiveContest(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.ArchiveContest(c.Request.Context(), id)
		httpx.Write(c, out, err)
	}
}

// signup 报名并创建队伍。
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

// joinTeam 加入队伍。
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

// lockTeam 锁定队伍。
func (a *API) lockTeam(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.LockTeam(c.Request.Context(), id)
		httpx.Write(c, out, err)
	}
}

// listProblems 查询题面列表。
func (a *API) listProblems(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.ListProblems(c.Request.Context(), id)
		httpx.Write(c, out, err)
	}
}

// startProblemEnv 创建实操题环境。
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

// submitSolve 提交解题判题。
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

// submitBattleEntry 提交参战物。
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

// listBattleEntries 查询参战物历史。
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

// listLadder 查询排行榜。
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

// createCheatRecord 写入作弊判定。
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

// listVulnSources 查询漏洞源。
func (a *API) listVulnSources(c *gin.Context) {
	out, err := a.svc.ListVulnSources(c.Request.Context(), httpx.Int(c.Query("page")), httpx.Int(c.Query("size")))
	httpx.Write(c, out, err)
}

// createVulnSource 创建漏洞源。
func (a *API) createVulnSource(c *gin.Context) {
	var req VulnSourceRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrContestVulnSourceInvalid) {
		out, err := a.svc.CreateVulnSource(c.Request.Context(), req)
		httpx.Write(c, out, err)
	}
}

// syncVulnSource 触发漏洞源同步。
func (a *API) syncVulnSource(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out, err := a.svc.SyncVulnSource(c.Request.Context(), id)
		httpx.Write(c, out, err)
	}
}

// importVulnProblem 导入漏洞案例草稿。
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
