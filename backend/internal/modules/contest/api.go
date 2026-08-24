// contest api 文件负责注册 M8 HTTP 路由、绑定请求和组合鉴权,不承载竞赛业务逻辑。
package contest

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/httpx"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/response"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册竞赛模块 HTTP API。
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
	api := contestAPI{svc: svc, authn: authn}
	g := r.Group("/api/v1/contest")
	teacher := g.Group("", authn.Middleware(), auth.RequireTenantAnyRole(roles, contracts.RoleTeacher, contracts.RoleSchoolAdmin))
	student := g.Group("", authn.Middleware(), auth.RequireTenantAnyRole(roles, contracts.RoleStudent))
	all := g.Group("", authn.Middleware(), auth.RequireTenantAnyRole(roles, contracts.RoleStudent, contracts.RoleTeacher, contracts.RoleSchoolAdmin))
	platform := g.Group("/platform", authn.Middleware(), auth.RequirePlatformIdentity())
	internal := g.Group("/internal", authn.ServiceMiddleware())
	api.registerTeacherRoutes(teacher)
	api.registerStudentRoutes(student)
	api.registerSharedRoutes(all)
	api.registerContestSandboxRoutes(student)
	api.registerContestSandboxInteractiveRoutes(g.Group("", authn.WebSocketMiddleware(), auth.RequireTenantAnyRole(roles, contracts.RoleStudent)))
	api.registerContestSandboxToolProxyRoutes(g.Group("", authn.BrowserAccessMiddleware(), auth.RequireTenantAnyRole(roles, contracts.RoleStudent)))
	api.registerPlatformRoutes(platform)
	api.registerInternalRoutes(internal)
	return nil
}

type contestAPI struct {
	svc   *Service
	authn *auth.Manager
}

// registerTeacherRoutes 注册教师/管理员竞赛管理接口。
func (a contestAPI) registerTeacherRoutes(g gin.IRouter) {
	g.GET("/contests", a.listContests)
	g.POST("/contests", a.createContest)
	g.GET("/contests/:id", a.getContest)
	g.PATCH("/contests/:id", a.updateContest)
	g.POST("/contests/:id/problems", a.addProblem)
	g.POST("/contests/:id/publish", a.publishContest)
	g.POST("/contests/:id/start", a.startContest)
	g.POST("/contests/:id/freeze", a.freezeContest)
	g.POST("/contests/:id/end", a.endContest)
	g.POST("/contests/:id/archive", a.archiveContest)
	g.GET("/contests/:id/result-snapshot", a.getSnapshot)
	g.POST("/contests/:id/cheat-records", a.createCheatRecord)
	g.GET("/contests/:id/cheat-records", a.listCheatRecords)
	g.GET("/contests/:id/cheat-suspects", a.listCheatSuspects)
	g.GET("/vuln-sources", a.listVulnSources)
	g.POST("/vuln-sources", a.upsertVulnSource)
	g.POST("/vuln-sources/:id/sync", a.syncVulnSource)
	g.GET("/vuln-problems", a.listVulnProblems)
	// 草稿录入只有这一条:请求体的可选 source_id 已表达来源归属,不再按路径区分手工与导入
	g.POST("/vuln-problems", a.importVulnProblem)
	g.POST("/vuln-problems/:id/prevalidate", a.prevalidateVulnProblem)
	g.POST("/vuln-problems/:id/finalize", a.finalizeVulnProblem)
}

// getContest 读取教师可管理的竞赛定义,草稿也可访问。
func (a contestAPI) getContest(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.GetContestForTeacher(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// registerStudentRoutes 注册学生参赛接口。
func (a contestAPI) registerStudentRoutes(g gin.IRouter) {
	g.GET("/student/contests", a.listStudentContests)
	g.GET("/student/contests/:id", a.getStudentContest)
	g.POST("/contests/:id/signup", a.signup)
	g.POST("/contests/:id/join-team", a.joinTeam)
	g.POST("/teams/:id/lock", a.lockTeam)
	g.POST("/contests/:id/problems/:problem_id/env", a.createEnv)
	g.POST("/contests/:id/problems/:problem_id/submit", a.submitSolve)
	g.GET("/submissions/:id", a.getSubmission)
	g.POST("/contests/:id/battle/entry", a.submitBattleEntry)
	g.GET("/contests/:id/battle/entries", a.listBattleEntries)
	g.GET("/contests/:id/battle/replay-window", a.getBattleReplayWindow)
	g.GET("/matches/:id/replay", a.getBattleReplay)
	g.POST("/matches/:id/replay/download-grant", a.issueBattleReplayDownloadGrant)
	g.GET("/my/contest-records", a.myRecords)
}

// registerContestSandboxRoutes 注册 M8 作为跨校授权网关的普通 HTTP 工作区和链操作入口。
func (a contestAPI) registerContestSandboxRoutes(g gin.IRouter) {
	g.GET("/sandboxes/:id", a.getContestSandbox)
	g.GET("/sandboxes/:id/files", a.getContestSandboxFiles)
	g.PUT("/sandboxes/:id/files", a.writeContestSandboxFile)
	g.POST("/sandboxes/:id/files/save", a.saveContestSandboxFiles)
	g.POST("/sandboxes/:id/command-tools/:tool_code/run", a.runContestSandboxCommandTool)
	g.POST("/sandboxes/:id/chain/deploy", a.deployContestSandboxChain)
	g.POST("/sandboxes/:id/chain/tx", a.sendContestSandboxChainTx)
	g.GET("/sandboxes/:id/chain/query", a.queryContestSandboxChain)
}

// registerContestSandboxInteractiveRoutes 注册经短时票据保护的竞赛进度和终端 WebSocket。
func (a contestAPI) registerContestSandboxInteractiveRoutes(g gin.IRouter) {
	g.GET("/sandboxes/:id/progress", a.contestSandboxProgress)
	g.GET("/sandboxes/:id/terminal", a.contestSandboxTerminal)
}

// registerContestSandboxToolProxyRoutes 注册受 M8 grant 校验保护的浏览器工具代理。
func (a contestAPI) registerContestSandboxToolProxyRoutes(g gin.IRouter) {
	g.Match([]string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions}, "/sandboxes/:id/tools/:tool_code/*proxy_path", a.contestSandboxToolProxy)
}

// registerPlatformRoutes 注册平台级漏洞源治理接口，不暴露租户漏洞题写入能力。
func (a contestAPI) registerPlatformRoutes(g gin.IRouter) {
	g.GET("/vuln-sources", a.listPlatformVulnSources)
	g.POST("/vuln-sources", a.upsertPlatformVulnSource)
}

// listStudentContests 返回学生可发现的非草稿竞赛。
// status=0 表示不过滤;传具体状态时仍受可见区间约束(草稿态永不可见)。
func (a contestAPI) listStudentContests(c *gin.Context) {
	page, size, ok := httpx.Page(c)
	if !ok {
		return
	}
	status, ok := httpx.QueryInt16(c, "status", httpx.QueryIntRule{Default: 0, Min: 0, Max: 6, HasMax: true})
	if !ok {
		return
	}
	out, total, p, s, err := a.svc.ListStudentContests(c.Request.Context(), status, page, size)
	httpx.WritePage(c, out, total, p, s, err)
}

// getStudentContest 返回单个学生可发现的非草稿竞赛。
func (a contestAPI) getStudentContest(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.GetStudentContest(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// listPlatformVulnSources 返回平台维护的全局漏洞源。
func (a contestAPI) listPlatformVulnSources(c *gin.Context) {
	out, err := a.svc.ListPlatformVulnSources(c.Request.Context())
	httpx.Write(c, out, err)
}

// upsertPlatformVulnSource 创建或更新平台全局漏洞源。
func (a contestAPI) upsertPlatformVulnSource(c *gin.Context) {
	var req VulnSourceRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrContestVulnSourceInvalid) {
		return
	}
	out, err := a.svc.UpsertPlatformVulnSource(c.Request.Context(), req)
	httpx.Write(c, out, err)
}

// registerSharedRoutes 注册师生共享读取接口。
func (a contestAPI) registerSharedRoutes(g gin.IRouter) {
	g.GET("/contests/:id/problems", a.listProblems)
	g.GET("/contests/:id/ladder", a.listLadder)
	g.GET("/teams/:id", a.getTeam)
	// 对局列表师生同路由按身份分视角:组织者见本赛事全部对局(实时监控),
	// 学生只见本队对局(时空回溯器)。service 判定视角,不为教师另开一条同义路由。
	g.GET("/contests/:id/battle/matches", a.listBattleMatches)
}

// registerInternalRoutes 注册内部只读接口。
func (a contestAPI) registerInternalRoutes(g gin.IRouter) {
	g.GET("/stats", a.internalStats)
	g.GET("/students/:id/contest-achievements", a.internalAchievements)
}

// listContests 绑定竞赛列表参数。
func (a contestAPI) listContests(c *gin.Context) {
	status, ok := httpx.QueryInt16(c, "status", httpx.QueryIntRule{Default: 0, Min: 0, Max: 6, HasMax: true})
	if !ok {
		return
	}
	page, size, ok := httpx.Page(c)
	if !ok {
		return
	}
	out, total, p, s, err := a.svc.ListContests(c.Request.Context(), status, page, size)
	httpx.WritePage(c, out, total, p, s, err)
}

// createContest 绑定创建竞赛请求。
func (a contestAPI) createContest(c *gin.Context) {
	var req ContestRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrContestInvalid) {
		return
	}
	out, err := a.svc.CreateContest(c.Request.Context(), req)
	httpx.Write(c, out, err)
}

// updateContest 绑定更新竞赛请求。
func (a contestAPI) updateContest(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req ContestRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrContestInvalid) {
		return
	}
	out, err := a.svc.UpdateContest(c.Request.Context(), id, req)
	httpx.Write(c, out, err)
}

// addProblem 绑定竞赛题目请求。
func (a contestAPI) addProblem(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req ProblemRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrContestProblemInvalid) {
		return
	}
	out, err := a.svc.AddProblem(c.Request.Context(), id, req)
	httpx.Write(c, out, err)
}

// publishContest 发布竞赛。
func (a contestAPI) publishContest(c *gin.Context) { a.writeContestAction(c, a.svc.PublishContest) }

// startContest 启动竞赛。
func (a contestAPI) startContest(c *gin.Context) { a.writeContestAction(c, a.svc.StartContest) }

// endContest 结束竞赛。
func (a contestAPI) endContest(c *gin.Context) { a.writeContestAction(c, a.svc.EndContest) }

// freezeContest 进入封榜期。
func (a contestAPI) freezeContest(c *gin.Context) { a.writeContestAction(c, a.svc.FreezeContest) }

// writeContestAction 统一绑定竞赛状态动作。
func (a contestAPI) writeContestAction(c *gin.Context, fn func(context.Context, int64) (ContestDTO, error)) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := fn(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// archiveContest 归档竞赛。
func (a contestAPI) archiveContest(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.ArchiveContest(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// getSnapshot 读取归档快照。
func (a contestAPI) getSnapshot(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.GetSnapshot(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// listProblems 读取竞赛题目。
func (a contestAPI) listProblems(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.ListProblems(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// signup 绑定报名请求。
func (a contestAPI) signup(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req SignupRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrContestTeamInvalid) {
		return
	}
	out, err := a.svc.Signup(c.Request.Context(), id, req)
	httpx.Write(c, out, err)
}

// joinTeam 绑定按赛事编号和邀请码加入队伍请求。
func (a contestAPI) joinTeam(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req JoinTeamRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrContestTeamInvalid) {
		return
	}
	out, err := a.svc.JoinTeam(c.Request.Context(), id, req)
	httpx.Write(c, out, err)
}

// getTeam 读取队伍。
func (a contestAPI) getTeam(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.GetTeam(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// lockTeam 锁定队伍。
func (a contestAPI) lockTeam(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.LockTeam(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// createEnv 绑定竞赛环境创建请求。
func (a contestAPI) createEnv(c *gin.Context) {
	contestID, problemID, ok := contestProblemPath(c)
	if !ok {
		return
	}
	var req EnvRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrContestSandboxUnavailable) {
		return
	}
	out, err := a.svc.CreateEnv(c.Request.Context(), contestID, problemID, req)
	httpx.Write(c, out, err)
}

// submitSolve 绑定解题提交请求。
func (a contestAPI) submitSolve(c *gin.Context) {
	contestID, problemID, ok := contestProblemPath(c)
	if !ok {
		return
	}
	var req SubmitRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrContestSubmissionInvalid) {
		return
	}
	out, err := a.svc.SubmitSolve(c.Request.Context(), contestID, problemID, req)
	httpx.Write(c, out, err)
}

// getSubmission 读取提交。
func (a contestAPI) getSubmission(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.GetSubmission(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// submitBattleEntry 绑定参战物提交。
func (a contestAPI) submitBattleEntry(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req BattleEntryRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrContestBattleEntryInvalid) {
		return
	}
	out, err := a.svc.SubmitBattleEntry(c.Request.Context(), id, req)
	httpx.Write(c, out, err)
}

// listBattleEntries 查询参战物。
func (a contestAPI) listBattleEntries(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	page, size, ok := httpx.Page(c)
	if !ok {
		return
	}
	out, total, p, s, err := a.svc.ListBattleEntries(c.Request.Context(), id, page, size)
	httpx.WritePage(c, out, total, p, s, err)
}

// listBattleMatches 查询对局历史。
func (a contestAPI) listBattleMatches(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	page, size, ok := httpx.Page(c)
	if !ok {
		return
	}
	out, total, p, s, err := a.svc.ListBattleMatches(c.Request.Context(), id, page, size)
	httpx.WritePage(c, out, total, p, s, err)
}

// getBattleReplayWindow 返回按完成时间排序的回放时间窗和服务端检查点。
func (a contestAPI) getBattleReplayWindow(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	page, size, ok := httpx.Page(c)
	if !ok {
		return
	}
	out, err := a.svc.GetBattleReplayWindow(c.Request.Context(), id, page, size)
	httpx.Write(c, out, err)
}

// getBattleReplay 读取回放引用。
func (a contestAPI) getBattleReplay(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.GetBattleReplay(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// issueBattleReplayDownloadGrant 为已授权学生签发统一文件服务回放取件授权。
func (a contestAPI) issueBattleReplayDownloadGrant(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.IssueBattleReplayDownloadGrant(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// listLadder 查询排行榜。
func (a contestAPI) listLadder(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	page, size, ok := httpx.Page(c)
	if !ok {
		return
	}
	out, total, p, s, err := a.svc.ListLadder(c.Request.Context(), id, page, size)
	httpx.WritePage(c, out, total, p, s, err)
}

// myRecords 查询当前学生战绩。
func (a contestAPI) myRecords(c *gin.Context) {
	out, err := a.svc.ListMyContestRecords(c.Request.Context())
	httpx.Write(c, out, err)
}

// createCheatRecord 绑定违规处理记录。
func (a contestAPI) createCheatRecord(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req CheatRecordRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrContestCheatInvalid) {
		return
	}
	out, err := a.svc.CreateCheatRecord(c.Request.Context(), id, req)
	httpx.Write(c, out, err)
}

// listCheatRecords 查询违规处理记录。
func (a contestAPI) listCheatRecords(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	page, size, ok := httpx.Page(c)
	if !ok {
		return
	}
	out, total, p, s, err := a.svc.ListCheatRecords(c.Request.Context(), id, page, size)
	httpx.WritePage(c, out, total, p, s, err)
}

// listCheatSuspects 查询查重疑似线索。
func (a contestAPI) listCheatSuspects(c *gin.Context) {
	contestID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	problemID, ok := httpx.QueryID(c, "problem_id", true)
	if !ok {
		return
	}
	var threshold float64
	if raw := strings.TrimSpace(c.Query("threshold")); raw != "" {
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil || v <= 0 || v >= 1 {
			response.Fail(c, apperr.ErrQueryParamInvalid)
			return
		}
		threshold = v
	}
	out, err := a.svc.ListCheatSuspects(c.Request.Context(), contestID, problemID, c.Query("code_hash"), c.Query("exclude_source_ref"), threshold)
	httpx.Write(c, out, err)
}

// listVulnSources 查询漏洞源。
func (a contestAPI) listVulnSources(c *gin.Context) {
	out, err := a.svc.ListVulnSources(c.Request.Context())
	httpx.Write(c, out, err)
}

// upsertVulnSource 绑定漏洞源配置。
func (a contestAPI) upsertVulnSource(c *gin.Context) {
	var req VulnSourceRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrContestVulnSourceInvalid) {
		return
	}
	out, err := a.svc.UpsertVulnSource(c.Request.Context(), req)
	httpx.Write(c, out, err)
}

// syncVulnSource 同步漏洞源。
func (a contestAPI) syncVulnSource(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.SyncVulnSource(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// listVulnProblems 查询漏洞题草稿。
func (a contestAPI) listVulnProblems(c *gin.Context) {
	sourceID, ok := httpx.QueryID(c, "source_id", false)
	if !ok {
		return
	}
	status, ok := httpx.QueryInt16(c, "status", httpx.QueryIntRule{Default: 0, Min: 0, Max: 3, HasMax: true})
	if !ok {
		return
	}
	page, size, ok := httpx.Page(c)
	if !ok {
		return
	}
	prevalidateStatus, ok := httpx.QueryInt16(c, "prevalidate_status", httpx.QueryIntRule{Default: 0, Min: 0, Max: 3, HasMax: true})
	if !ok {
		return
	}
	out, total, p, s, err := a.svc.ListVulnProblems(c.Request.Context(), sourceID, status, prevalidateStatus, page, size)
	httpx.WritePage(c, out, total, p, s, err)
}

// importVulnProblem 手动导入漏洞题。
func (a contestAPI) importVulnProblem(c *gin.Context) {
	var req ImportVulnProblemRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrContestVulnProblemInvalid) {
		return
	}
	out, err := a.svc.ImportVulnProblem(c.Request.Context(), req)
	httpx.Write(c, out, err)
}

// prevalidateVulnProblem 保存预验证结果。
func (a contestAPI) prevalidateVulnProblem(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req PrevalidateRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrContestVulnProblemInvalid) {
		return
	}
	out, err := a.svc.SetVulnPrevalidate(c.Request.Context(), id, req)
	httpx.Write(c, out, err)
}

// finalizeVulnProblem 固化漏洞题到 M5。
func (a contestAPI) finalizeVulnProblem(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.FinalizeVulnProblem(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// internalStats 读取内部竞赛统计。
func (a contestAPI) internalStats(c *gin.Context) {
	tenantID, ok := httpx.QueryID(c, "tenant_id", true)
	if !ok {
		return
	}
	out, err := a.svc.Stats(c.Request.Context(), tenantID)
	httpx.Write(c, out, err)
}

// internalAchievements 读取内部学生竞赛成就。
func (a contestAPI) internalAchievements(c *gin.Context) {
	studentID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	tenantID, ok := httpx.QueryID(c, "tenant_id", true)
	if !ok {
		return
	}
	out, err := a.svc.ListStudentAchievements(c.Request.Context(), tenantID, studentID)
	httpx.Write(c, out, err)
}

// getContestSandbox 返回 M8 grant 校验后的竞赛沙箱摘要。
func (a contestAPI) getContestSandbox(c *gin.Context) {
	sandboxID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.GetContestSandbox(c.Request.Context(), sandboxID)
	if err != nil {
		httpx.Write(c, nil, err)
		return
	}
	httpx.Write(c, contestSandboxResponseFromInfo(out), nil)
}

// getContestSandboxFiles 读取单个文件或列出目录，能力由 M8 grant 在每次请求时核验。
func (a contestAPI) getContestSandboxFiles(c *gin.Context) {
	sandboxID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	if strings.EqualFold(c.Query("mode"), "list") {
		out, err := a.svc.ListContestSandboxFiles(c.Request.Context(), sandboxID, c.Query("path"))
		httpx.Write(c, out, err)
		return
	}
	out, err := a.svc.ReadContestSandboxFile(c.Request.Context(), sandboxID, c.Query("path"))
	httpx.Write(c, out, err)
}

// writeContestSandboxFile 写入竞赛工作区公开文件。
func (a contestAPI) writeContestSandboxFile(c *gin.Context) {
	sandboxID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req ContestSandboxFileWriteRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSandboxFileWriteRequestInvalid) {
		return
	}
	if strings.TrimSpace(req.RelativePath) == "" {
		req.RelativePath = c.Query("path")
	}
	revision, err := a.svc.WriteContestSandboxFile(c.Request.Context(), sandboxID, req.RelativePath, req.ContentBase64, req.ExpectedRevision)
	httpx.Write(c, ContestSandboxFileWriteResponse{WorkspaceRevision: revision}, err)
}

// saveContestSandboxFiles 立即持久化竞赛工作区。
func (a contestAPI) saveContestSandboxFiles(c *gin.Context) {
	sandboxID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.SaveContestSandboxFiles(c.Request.Context(), sandboxID)
	httpx.Write(c, out, err)
}

// runContestSandboxCommandTool 执行受 grant 和工具白名单双重约束的命令。
func (a contestAPI) runContestSandboxCommandTool(c *gin.Context) {
	sandboxID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req ContestSandboxToolRunRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSandboxToolRunRequestInvalid) {
		return
	}
	out, err := a.svc.RunContestSandboxCommandTool(c.Request.Context(), sandboxID, c.Param("tool_code"), req.Command, req.StdinBase64, req.TimeoutSec)
	httpx.Write(c, out, err)
}

// deployContestSandboxChain 调用 grant 允许的统一链部署能力。
func (a contestAPI) deployContestSandboxChain(c *gin.Context) {
	sandboxID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req ContestSandboxChainRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSandboxDeployRequestInvalid) {
		return
	}
	out, err := a.svc.DeployContestSandboxChain(c.Request.Context(), sandboxID, req.Payload)
	httpx.Write(c, out, err)
}

// sendContestSandboxChainTx 调用 grant 允许的统一链交易能力。
func (a contestAPI) sendContestSandboxChainTx(c *gin.Context) {
	sandboxID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req ContestSandboxChainRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSandboxTxRequestInvalid) {
		return
	}
	out, err := a.svc.SendContestSandboxChainTx(c.Request.Context(), sandboxID, req.Payload)
	httpx.Write(c, out, err)
}

// queryContestSandboxChain 调用 grant 允许的统一链查询能力。
func (a contestAPI) queryContestSandboxChain(c *gin.Context) {
	sandboxID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	out, err := a.svc.QueryContestSandboxChain(c.Request.Context(), sandboxID, c.Query("target"))
	httpx.Write(c, out, err)
}

// contestSandboxProgress 建立已授权竞赛成员的沙箱进度订阅。
func (a contestAPI) contestSandboxProgress(c *gin.Context) {
	sandboxID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	id, err := currentIdentity(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	if a.svc.wsHub == nil {
		response.Fail(c, apperr.ErrSandboxToolProxyUnavailable)
		return
	}
	if err := a.svc.wsHub.Serve(c.Writer, c.Request, func(conn *ws.Conn) error {
		access, topic, initial, err := a.svc.ContestSandboxProgressSubscription(c.Request.Context(), sandboxID)
		if err != nil {
			return err
		}
		if err := conn.BindSession(ws.SessionKey{TenantID: id.TenantID, AccountID: id.AccountID, Scope: contestSandboxSessionScope(access.Principal.AuthorizationID, sandboxID) + ":progress"}); err != nil {
			return apperr.ErrContestTeamAccessDenied.WithCause(err)
		}
		if err := a.svc.wsHub.Subscribe(conn, topic); err != nil {
			return apperr.ErrSandboxToolProxyUnavailable.WithCause(err)
		}
		return conn.SendJSON(initial)
	}); err != nil {
		response.Fail(c, apperr.ErrSandboxToolProxyUnavailable.WithCause(err))
	}
}

// contestSandboxTerminal 建立已授权竞赛成员的终端字节流连接。
func (a contestAPI) contestSandboxTerminal(c *gin.Context) {
	sandboxID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	id, err := currentIdentity(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	if a.svc.wsHub == nil {
		response.Fail(c, apperr.ErrSandboxToolProxyUnavailable)
		return
	}
	access, target, err := a.svc.ContestSandboxTerminalTarget(c.Request.Context(), sandboxID, strings.TrimSpace(c.Query("container")))
	if err != nil {
		response.Fail(c, err)
		return
	}
	if err := a.svc.wsHub.ServeInteractive(c.Writer, c.Request, func(conn *ws.Conn) error {
		if err := conn.BindSession(ws.SessionKey{TenantID: id.TenantID, AccountID: id.AccountID, Scope: contestSandboxSessionScope(access.Principal.AuthorizationID, sandboxID)}); err != nil {
			return apperr.ErrContestTeamAccessDenied.WithCause(err)
		}
		return a.svc.AttachContestSandboxTerminal(c.Request.Context(), access, target, conn.Reader(), conn.Writer())
	}); err != nil {
		response.Fail(c, apperr.ErrSandboxToolProxyUnavailable.WithCause(err))
	}
}

// contestSandboxToolProxy 将已授权的浏览器工具请求代理到组织租户的集群内 Service。
func (a contestAPI) contestSandboxToolProxy(c *gin.Context) {
	sandboxID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	access, target, err := a.svc.ContestSandboxToolProxyTarget(c.Request.Context(), sandboxID, c.Param("tool_code"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	parsed, err := url.Parse(target.TargetURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		response.Fail(c, apperr.ErrSandboxToolProxyUnavailable)
		return
	}
	externalPrefix := contestToolProxyExternalPrefix(sandboxID, c.Param("tool_code"))
	if !a.prepareContestToolBrowserAccess(c, externalPrefix) {
		return
	}
	proxy := httpx.NewPrefixReverseProxy(httpx.PrefixReverseProxyConfig{
		Target: parsed, ProxyPath: c.Param("proxy_path"), ExternalPrefix: externalPrefix,
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			response.Fail(c, apperr.ErrSandboxToolProxyUnavailable.WithCause(err))
		},
	})
	proxy.ServeHTTP(c.Writer, c.Request)
	_ = a.svc.ObserveContestSandboxToolAccess(c.Request.Context(), access)
}

// prepareContestToolBrowserAccess 将浏览器入口票据收敛为路径受限 Cookie，避免令牌进入上游工具日志。
func (a contestAPI) prepareContestToolBrowserAccess(c *gin.Context, externalPrefix string) bool {
	token, ok := auth.VerifiedAccessToken(c)
	if ok && a.authn != nil {
		a.authn.SetBrowserAccessCookie(c, externalPrefix, token)
	}
	if !auth.BrowserAccessFromTicket(c) {
		return true
	}
	u := *c.Request.URL
	query := u.Query()
	query.Del(auth.BrowserAccessTicketQuery)
	u.RawQuery = query.Encode()
	c.Redirect(http.StatusFound, u.RequestURI())
	return false
}

// contestToolProxyExternalPrefix 返回 M8 网关对浏览器暴露的工具代理前缀。
func contestToolProxyExternalPrefix(sandboxID int64, toolCode string) string {
	return "/api/v1/contest/sandboxes/" + ids.Format(sandboxID) + "/tools/" + url.PathEscape(strings.TrimSpace(toolCode))
}

// contestProblemPath 统一解析竞赛和题目路径 ID。
func contestProblemPath(c *gin.Context) (int64, int64, bool) {
	contestID, ok := httpx.PathID(c, "id")
	if !ok {
		return 0, 0, false
	}
	problemID, ok := httpx.PathID(c, "problem_id")
	if !ok {
		return 0, 0, false
	}
	return contestID, problemID, true
}
