// contest api 文件负责注册 M8 HTTP 路由、绑定请求和组合鉴权,不承载竞赛业务逻辑。
package contest

import (
	"context"
	"strconv"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/httpx"
	"chaimir/pkg/apperr"
	"chaimir/pkg/response"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册竞赛模块 HTTP API。
func RegisterRoutes(r gin.IRouter, svc *Service, authn *auth.Manager, roles auth.RoleChecker) error {
	if r == nil {
		return apperr.ErrInternal.WithMessage("contest routes 缺少 HTTP router")
	}
	if svc == nil {
		return apperr.ErrInternal.WithMessage("contest routes 缺少 service")
	}
	if authn == nil {
		return apperr.ErrInternal.WithMessage("contest routes 缺少 auth manager")
	}
	api := contestAPI{svc: svc}
	g := r.Group("/api/v1/contest")
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

type contestAPI struct{ svc *Service }

// registerTeacherRoutes 注册教师/管理员竞赛管理接口。
func (a contestAPI) registerTeacherRoutes(g gin.IRouter) {
	g.GET("/contests", a.listContests)
	g.POST("/contests", a.createContest)
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
	g.POST("/vuln-problems", a.importVulnProblem)
	g.POST("/vuln-sources/import", a.importVulnProblem)
	g.POST("/vuln-problems/:id/prevalidate", a.prevalidateVulnProblem)
	g.POST("/vuln-problems/:id/finalize", a.finalizeVulnProblem)
}

// registerStudentRoutes 注册学生参赛接口。
func (a contestAPI) registerStudentRoutes(g gin.IRouter) {
	g.POST("/contests/:id/signup", a.signup)
	g.POST("/teams/:id/join", a.joinTeamByTeamID)
	g.POST("/teams/:id/lock", a.lockTeam)
	g.POST("/contests/:id/problems/:problem_id/env", a.createEnv)
	g.POST("/contests/:id/problems/:problem_id/submit", a.submitSolve)
	g.GET("/submissions/:id", a.getSubmission)
	g.POST("/contests/:id/battle/entry", a.submitBattleEntry)
	g.GET("/contests/:id/battle/entries", a.listBattleEntries)
	g.GET("/contests/:id/battle/matches", a.listBattleMatches)
	g.GET("/matches/:id/replay", a.getBattleReplay)
	g.GET("/my/contest-records", a.myRecords)
}

// registerSharedRoutes 注册师生共享读取接口。
func (a contestAPI) registerSharedRoutes(g gin.IRouter) {
	g.GET("/contests/:id/problems", a.listProblems)
	g.GET("/contests/:id/ladder", a.listLadder)
	g.GET("/teams/:id", a.getTeam)
}

// registerInternalRoutes 注册内部只读接口。
func (a contestAPI) registerInternalRoutes(g gin.IRouter) {
	g.GET("/stats", a.internalStats)
	g.GET("/students/:id/achievements", a.internalAchievements)
	g.GET("/students/:id/contest-achievements", a.internalAchievements)
}

// listContests 绑定竞赛列表参数。
func (a contestAPI) listContests(c *gin.Context) {
	status, ok := httpx.QueryInt(c, "status", httpx.QueryIntRule{Default: 0, Min: 0, Max: 6, HasMax: true})
	if !ok {
		return
	}
	page, size, ok := contestPage(c)
	if !ok {
		return
	}
	out, total, p, s, err := a.svc.ListContests(c.Request.Context(), int16(status), page, size)
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

// joinTeam 绑定加入队伍请求。
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

// joinTeamByTeamID 绑定按队伍 ID 加入队伍请求。
func (a contestAPI) joinTeamByTeamID(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var req JoinTeamRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrContestTeamInvalid) {
		return
	}
	out, err := a.svc.JoinTeamByID(c.Request.Context(), id, req)
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
	out, err := a.svc.ListBattleEntries(c.Request.Context(), id)
	httpx.Write(c, out, err)
}

// listBattleMatches 查询对局历史。
func (a contestAPI) listBattleMatches(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	page, size, ok := contestPage(c)
	if !ok {
		return
	}
	out, err := a.svc.ListBattleMatches(c.Request.Context(), id, page, size)
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

// listLadder 查询排行榜。
func (a contestAPI) listLadder(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	page, size, ok := contestPage(c)
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
	page, size, ok := contestPage(c)
	if !ok {
		return
	}
	out, err := a.svc.ListCheatRecords(c.Request.Context(), id, page, size)
	httpx.Write(c, out, err)
}

// listCheatSuspects 查询查重疑似线索。
func (a contestAPI) listCheatSuspects(c *gin.Context) {
	contestID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	problemID, ok := httpx.QueryInt(c, "problem_id", httpx.QueryIntRule{Min: 1})
	if !ok {
		return
	}
	threshold := 0.8
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
	sourceID, ok := httpx.QueryInt(c, "source_id", httpx.QueryIntRule{Default: 0, Min: 0})
	if !ok {
		return
	}
	status, ok := httpx.QueryInt(c, "status", httpx.QueryIntRule{Default: 0, Min: 0, Max: 3, HasMax: true})
	if !ok {
		return
	}
	page, size, ok := contestPage(c)
	if !ok {
		return
	}
	out, err := a.svc.ListVulnProblems(c.Request.Context(), sourceID, int16(status), page, size)
	httpx.Write(c, out, err)
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
	tenantID, ok := httpx.QueryInt(c, "tenant_id", httpx.QueryIntRule{Min: 1})
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
	tenantID, ok := httpx.QueryInt(c, "tenant_id", httpx.QueryIntRule{Min: 1})
	if !ok {
		return
	}
	out, err := a.svc.ListStudentAchievements(c.Request.Context(), tenantID, studentID)
	httpx.Write(c, out, err)
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

// contestPage 统一解析竞赛模块分页参数。
func contestPage(c *gin.Context) (int, int, bool) {
	page, ok := httpx.QueryInt(c, "page", httpx.QueryIntRule{Default: 1, Min: 1})
	if !ok {
		return 0, 0, false
	}
	size, ok := httpx.QueryInt(c, "size", httpx.QueryIntRule{Default: 20, Min: 1, Max: 100, HasMax: true})
	if !ok {
		return 0, 0, false
	}
	return int(page), int(size), true
}
