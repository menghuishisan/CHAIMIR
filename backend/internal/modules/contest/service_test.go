// M8 服务规则测试:覆盖竞赛状态机、报名队伍、解题计分、对抗积分和归档快照。
package contest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
)

// TestContestStatusTransitionsFollowLifecycle 确认竞赛状态机符合文档生命周期。
func TestContestStatusTransitionsFollowLifecycle(t *testing.T) {
	if err := validateContestTransition(ContestStatusDraft, ContestStatusSignup); err != nil {
		t.Fatalf("draft should publish to signup: %v", err)
	}
	if err := validateContestTransition(ContestStatusRunning, ContestStatusFrozen); err != nil {
		t.Fatalf("running should enter frozen board: %v", err)
	}
	if err := validateContestTransition(ContestStatusEnded, ContestStatusArchived); err != nil {
		t.Fatalf("ended should archive: %v", err)
	}
	if err := validateContestTransition(ContestStatusArchived, ContestStatusRunning); err == nil {
		t.Fatalf("archived contest must not restart")
	}
}

// TestCreateContestRejectsInvalidSchedule 确认赛程必须按报名、开赛、结束顺序配置。
func TestCreateContestRejectsInvalidSchedule(t *testing.T) {
	now := time.Now()
	req := ContestRequest{
		Name: "bad schedule", Mode: ContestModeSolve, TeamMode: TeamModeSolo,
		SignupStart: now.Add(time.Hour), SignupEnd: now, StartAt: now.Add(2 * time.Hour), EndAt: now.Add(3 * time.Hour),
	}
	if err := validateContestRequest(req); err == nil {
		t.Fatalf("invalid schedule must be rejected")
	}
}

// TestSignupCreatesSoloTeamAndRank 确认个人报名会创建单人队并初始化排行。
func TestSignupCreatesSoloTeamAndRank(t *testing.T) {
	store := &fakeContestStore{contest: contestDTO(ContestStatusSignup)}
	svc := &Service{store: store, idgen: fixedIDGen(8201)}

	team, err := svc.Signup(testTenantContext(), 8101, SignupRequest{Name: "alice"})
	if err != nil {
		t.Fatalf("signup rejected: %v", err)
	}
	if team.ID != "8201" || len(team.Members) != 1 {
		t.Fatalf("unexpected team: %#v", team)
	}
	if !store.rankInitialized {
		t.Fatalf("signup should initialize ladder rank")
	}
}

// TestTeamInviteCodeUsesOpaqueTokenAndJoinValidatesCode 守护团队邀请码不可由队伍 ID 推导且入队必须校验。
func TestTeamInviteCodeUsesOpaqueTokenAndJoinValidatesCode(t *testing.T) {
	repoSrc, err := os.ReadFile("repo.go")
	if err != nil {
		t.Fatalf("read repo: %v", err)
	}
	if strings.Contains(string(repoSrc), "invite = ids.Format(teamID)") {
		t.Fatalf("team invite code must not be derived from team id")
	}
	if !strings.Contains(string(repoSrc), "crypto.RandomToken") {
		t.Fatalf("team invite code must use shared crypto.RandomToken")
	}
	serviceSrc, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service: %v", err)
	}
	joinSrc := functionSource(string(serviceSrc), "JoinTeam")
	if !strings.Contains(joinSrc, "validateJoinTeamRequest") || !strings.Contains(joinSrc, "team.InviteCode") {
		t.Fatalf("JoinTeam must validate the server-side team invite code before adding a member")
	}
}

// TestApplySolveJudgementUpdatesRank 确认解题判题完成会回写提交并更新排行分。
func TestApplySolveJudgementUpdatesRank(t *testing.T) {
	store := &fakeContestStore{pendingSubmission: pendingSolveSubmission{TenantID: 100, ContestID: 8101, TeamID: 8201, ProblemID: 8301, MaxScore: 40}}
	svc := &Service{store: store, idgen: fixedIDGen(8701)}

	err := svc.ApplySolveJudgement(context.Background(), 100, 3001, true, 40)
	if err != nil {
		t.Fatalf("judge event rejected: %v", err)
	}
	if store.updatedSubmission.Score != 40 || !store.updatedSubmission.Passed {
		t.Fatalf("submission not updated from judgement: %#v", store.updatedSubmission)
	}
	if store.rankDelta != 40 {
		t.Fatalf("rank delta mismatch: %v", store.rankDelta)
	}
}

// TestRecordBattleMatchUpdatesElo 确认对抗赛结算会按胜负更新双方 ELO。
func TestRecordBattleMatchUpdatesElo(t *testing.T) {
	store := &fakeContestStore{entryA: BattleEntryDTO{ID: "8401", TeamID: "8201"}, entryB: BattleEntryDTO{ID: "8402", TeamID: "8202"}}
	svc := &Service{store: store, idgen: fixedIDGen(8501)}

	match, err := svc.RecordBattleMatch(context.Background(), 100, BattleMatchResult{ContestID: 8101, EntryAID: 8401, EntryBID: 8402, Result: BattleResultAWin, ReplayRef: "replay/key"})
	if err != nil {
		t.Fatalf("battle match rejected: %v", err)
	}
	if match.ID != "8501" {
		t.Fatalf("unexpected match id: %s", match.ID)
	}
	if store.eloA <= 1000 || store.eloB >= 1000 {
		t.Fatalf("winner should gain elo and loser should lose elo: %v %v", store.eloA, store.eloB)
	}
}

// TestArchiveContestRecyclesResourcesAndSnapshotsRank 确认归档会级联回收竞赛资源并生成最终榜单快照。
func TestArchiveContestRecyclesResourcesAndSnapshotsRank(t *testing.T) {
	store := &fakeContestStore{contest: contestDTO(ContestStatusEnded), ranks: []LadderRankDTO{{TeamID: "8201", Score: 100, Rank: 1}}}
	sandbox := &fakeSandboxService{}
	svc := &Service{
		store:    store,
		idgen:    fixedIDGen(8601),
		sandbox:  sandbox,
		auditor:  &noopContestAuditWriter{},
		identity: &contestAuditIdentity{account: contracts.AccountInfo{AccountID: 200, TenantID: 100, BaseIdentity: 2, Roles: []string{"teacher"}}},
	}

	_, err := svc.ArchiveContest(testTenantContext(), 8101)
	if err != nil {
		t.Fatalf("archive rejected: %v", err)
	}
	if sandbox.recycledSource != "contest:2026:contest:8101" {
		t.Fatalf("contest resources not recycled by source ref: %s", sandbox.recycledSource)
	}
	if !store.snapshotCreated {
		t.Fatalf("archive should create result snapshot")
	}
}

// TestContestSourceRefsUseDocumentedShapeAndSubmissionScope 防止 M8 生成不符合全局规范或过粗粒度的 source_ref。
func TestContestSourceRefsUseDocumentedShapeAndSubmissionScope(t *testing.T) {
	if got := sourceRefForContest(8101); got != "contest:2026:contest:8101" {
		t.Fatalf("contest resource source_ref must use four-segment shape, got %s", got)
	}
	if got := sourceRefForSolveSubmission(8701); got != "contest:2026:submission:8701" {
		t.Fatalf("judge submission source_ref must be per submission, got %s", got)
	}
	src, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service: %v", err)
	}
	submit := functionSource(string(src), "SubmitSolve")
	if !strings.Contains(submit, "sourceRefForSolveSubmission") {
		t.Fatalf("SubmitSolve must use per-submission source_ref so M3 idempotency does not collapse contest submissions")
	}
}

// TestJudgeEventsBindSourceRef 确认 M3 判题事件必须与提交 source_ref 绑定,不能只凭 task_id 回写。
func TestJudgeEventsBindSourceRef(t *testing.T) {
	src, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service: %v", err)
	}
	for _, name := range []string{"HandleJudgeCompleted", "HandleJudgeFailed"} {
		block := functionSource(string(src), name)
		if !strings.Contains(block, "pending.SourceRef") || !strings.Contains(block, "event.SourceRef") {
			t.Fatalf("%s must compare judge event source_ref with pending submission source_ref", name)
		}
	}
}

// TestContestUserDataAccessUsesServerSideTeamBoundary 防止选手接口信任前端传入 team_id 或资源 id。
func TestContestUserDataAccessUsesServerSideTeamBoundary(t *testing.T) {
	src, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service: %v", err)
	}
	text := string(src)
	for _, required := range []string{"ensureTeamMemberAccess", "ensureSubmissionAccess", "ensureBattleMatchAccess", "contracts.RoleTeacher", "contracts.RoleSchoolAdmin"} {
		if !strings.Contains(text, required) {
			t.Fatalf("contest user data access must use unified server-side boundary, missing %s", required)
		}
	}
	for _, fn := range []string{"SubmitSolve", "SubmitBattleEntry", "ListBattleEntries", "ListBattleMatches"} {
		if !strings.Contains(functionSource(text, fn), "ensureTeamMemberAccess") {
			t.Fatalf("%s must verify current actor belongs to the requested team", fn)
		}
	}
	if !strings.Contains(functionSource(text, "GetSubmission"), "ensureSubmissionAccess") {
		t.Fatalf("GetSubmission must verify owner/team membership before returning result")
	}
	if !strings.Contains(functionSource(text, "GetMatchReplay"), "ensureBattleMatchAccess") {
		t.Fatalf("GetMatchReplay must verify participant team membership before returning replay")
	}
	if strings.Contains(text, `"school_admin"`) || strings.Contains(text, `"teacher"`) {
		t.Fatalf("role checks must use internal/contracts role constants")
	}
}

// TestStartProblemEnvUsesContestTeamBoundary 防止实操题环境创建绕过报名队伍边界。
func TestStartProblemEnvUsesContestTeamBoundary(t *testing.T) {
	src, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service: %v", err)
	}
	startEnv := functionSource(string(src), "StartProblemEnv")
	for _, required := range []string{"GetProblem", "problem.ContestID", "currentTeamForContest", "ensureTeamMemberAccess"} {
		if !strings.Contains(startEnv, required) {
			t.Fatalf("StartProblemEnv must validate contest problem and current team before creating sandbox, missing %s", required)
		}
	}
	if strings.Index(startEnv, "ensureTeamMemberAccess") > strings.Index(startEnv, "CreateSandbox") {
		t.Fatalf("StartProblemEnv must verify team access before creating sandbox side effects")
	}
}

// TestAddContestProblemRequiresContentContract 确认题目编排必须经 M5 契约校验锁定版本,缺契约不能写入引用。
func TestAddContestProblemRequiresContentContract(t *testing.T) {
	store := &fakeContestStore{contest: contestDTO(ContestStatusDraft)}
	svc := &Service{store: store, idgen: fixedIDGen(8301), identity: &contestAuditIdentity{account: contracts.AccountInfo{AccountID: 200, TenantID: 100, BaseIdentity: 2, Roles: []string{"teacher"}}}}

	_, err := svc.AddContestProblem(testTenantContext(), 8101, ContestProblemRequest{ItemCode: "p1", ItemVersion: "1.0.0", Score: 40, Seq: 1})
	if err == nil {
		t.Fatalf("missing content contract must fail before creating contest problem")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrContestContentUnavailable.Code {
		t.Fatalf("expected contest content unavailable error, got %v", err)
	}
}

// TestListProblemsRequiresContentContract 确认题面列表必须由 M5 题面接口提供过滤内容,缺契约不能返回无题面数据。
func TestListProblemsRequiresContentContract(t *testing.T) {
	store := &fakeContestStore{problems: []ContestProblemDTO{{ID: "8301", ContestID: "8101", ItemCode: "p1", ItemVersion: "1.0.0", Score: 40}}}
	svc := &Service{store: store, content: nil}

	_, err := svc.ListProblems(testTenantContext(), 8101)
	if err == nil {
		t.Fatalf("missing content contract must fail")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrContestContentUnavailable.Code {
		t.Fatalf("expected contest content unavailable error, got %v", err)
	}
}

// TestArchiveContestRequiresSandboxContract 确认归档需要 M2 回收契约,缺契约不能生成快照并假装资源已释放。
func TestArchiveContestRequiresSandboxContract(t *testing.T) {
	store := &fakeContestStore{contest: contestDTO(ContestStatusEnded), ranks: []LadderRankDTO{{TeamID: "8201", Score: 100, Rank: 1}}}
	svc := &Service{
		store:    store,
		idgen:    fixedIDGen(8601),
		sandbox:  nil,
		auditor:  &noopContestAuditWriter{},
		identity: &contestAuditIdentity{account: contracts.AccountInfo{AccountID: 200, TenantID: 100, BaseIdentity: 2, Roles: []string{"teacher"}}},
	}

	_, err := svc.ArchiveContest(testTenantContext(), 8101)
	if err == nil {
		t.Fatalf("archive without sandbox recycle contract must fail")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrContestSandboxUnavailable.Code {
		t.Fatalf("expected contest sandbox unavailable error, got %v", err)
	}
	if store.snapshotCreated {
		t.Fatalf("archive must not create final snapshot before resource recycle succeeds")
	}
}

// TestStartProblemEnvRequiresSandboxContract 确认实操环境创建缺 M2 契约返回专用错误码。
func TestStartProblemEnvRequiresSandboxContract(t *testing.T) {
	store := &fakeContestStore{contest: contestDTO(ContestStatusRunning)}
	svc := &Service{store: store, sandbox: nil}

	_, err := svc.StartProblemEnv(testTenantContext(), 8101, 8301, StartProblemEnvRequest{RuntimeCode: "evm-hardhat"})
	if err == nil {
		t.Fatalf("missing sandbox contract must fail")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrContestSandboxUnavailable.Code {
		t.Fatalf("expected contest sandbox unavailable error, got %v", err)
	}
}

// TestSubmitSolveRequiresJudgeContract 确认解题提交缺 M3 契约返回专用错误码而不是 panic。
func TestSubmitSolveRequiresJudgeContract(t *testing.T) {
	store := &fakeContestStore{contest: contestDTO(ContestStatusRunning)}
	svc := &Service{store: store, idgen: fixedIDGen(8701), judge: nil}

	_, err := svc.SubmitSolve(testTenantContext(), 8101, 8301, SolveSubmitRequest{TeamID: "8201", JudgerCode: "testcase"})
	if err == nil {
		t.Fatalf("missing judge contract must fail")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrContestJudgeUnavailable.Code {
		t.Fatalf("expected contest judge unavailable error, got %v", err)
	}
}

// TestSubmitSolvePersistsJudgeSourceRef 确认提交入库的 source_ref 与 M3 判题任务 source_ref 一致,事件回写可精确绑定。
func TestSubmitSolvePersistsJudgeSourceRef(t *testing.T) {
	store := &fakeContestStore{contest: contestDTO(ContestStatusRunning)}
	svc := &Service{store: store, idgen: fixedIDGen(8701), judge: &fakeJudgeService{}}

	submission, err := svc.SubmitSolve(testTenantContext(), 8101, 8301, SolveSubmitRequest{TeamID: "8201", JudgerCode: "testcase"})
	if err != nil {
		t.Fatalf("submit solve rejected: %v", err)
	}
	if submission.SourceRef == "" || submission.SourceRef != store.lastSubmissionSource {
		t.Fatalf("submission source_ref must be persisted, got dto=%q store=%q", submission.SourceRef, store.lastSubmissionSource)
	}
	if store.lastJudgeTaskRef != "3001" {
		t.Fatalf("judge task ref not persisted: %q", store.lastJudgeTaskRef)
	}
}

// TestSubscribeEventsUsesDedicatedSubscribeErrorCode 确认订阅链路与判题结果同步链路使用不同错误码。
func TestSubscribeEventsUsesDedicatedSubscribeErrorCode(t *testing.T) {
	svc := &Service{store: &fakeContestStore{}, bus: &fakeEventBus{subErr: errContestSubscribe}}

	err := svc.SubscribeEvents()
	if err == nil {
		t.Fatalf("subscribe failure must be returned")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrContestEventSubscribeFailed.Code {
		t.Fatalf("expected contest event subscribe error, got %v", err)
	}
}

// TestHandleJudgeCompletedUsesUnmatchedEventCode 确认判题事件 source_ref 不匹配时返回专用错误码并保留原因。
func TestHandleJudgeCompletedUsesUnmatchedEventCode(t *testing.T) {
	store := &fakeContestStore{pendingSubmission: pendingSolveSubmission{TenantID: 100, ID: 8701, ContestID: 8101, TeamID: 8201, SourceRef: "contest:2026:submission:8701", MaxScore: 40}}
	svc := &Service{store: store}

	err := svc.HandleJudgeCompleted(context.Background(), contracts.JudgeCompletedEvent{TenantID: 100, TaskID: 3001, SourceRef: "contest:2026:submission:9999", Score: 40})
	if err == nil {
		t.Fatalf("source_ref mismatch must fail")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrContestEventUnmatched.Code {
		t.Fatalf("expected contest event unmatched error, got %v", err)
	}
	if !strings.Contains(err.Error(), "source_ref") {
		t.Fatalf("event mismatch must keep source_ref cause context, got %v", err)
	}
}

// TestContestPlayActionsRequirePlayableState 防止非比赛阶段创建环境、提交判题或提交参战物。
func TestContestPlayActionsRequirePlayableState(t *testing.T) {
	src, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service: %v", err)
	}
	text := string(src)
	if !strings.Contains(text, "ensureContestPlayable") {
		t.Fatalf("play actions must share one contest state gate")
	}
	for _, fn := range []string{"StartProblemEnv", "SubmitSolve", "SubmitBattleEntry"} {
		body := functionSource(text, fn)
		if !strings.Contains(body, "GetContest") || !strings.Contains(body, "ensureContestPlayable") {
			t.Fatalf("%s must load contest and verify playable state before side effects", fn)
		}
		if strings.Contains(body, "ensureContestPlayable") && strings.Contains(body, "CreateSandbox") && strings.Index(body, "ensureContestPlayable") > strings.Index(body, "CreateSandbox") {
			t.Fatalf("%s must verify state before creating sandbox", fn)
		}
		if strings.Contains(body, "ensureContestPlayable") && strings.Contains(body, "SubmitJudgeTask") && strings.Index(body, "ensureContestPlayable") > strings.Index(body, "SubmitJudgeTask") {
			t.Fatalf("%s must verify state before submitting judge task", fn)
		}
		if strings.Contains(body, "ensureContestPlayable") && strings.Contains(body, "CreateBattleEntry") && strings.Index(body, "ensureContestPlayable") > strings.Index(body, "CreateBattleEntry") {
			t.Fatalf("%s must verify state before writing battle entry", fn)
		}
	}
}

// TestContestAPIErrorsUseModuleCodes 防止 M8 API 边界退回通用 ErrBadRequest。
func TestContestAPIErrorsUseModuleCodes(t *testing.T) {
	apiSrc, err := os.ReadFile("api.go")
	if err != nil {
		t.Fatalf("read api: %v", err)
	}
	if strings.Contains(string(apiSrc), "ErrBadRequest") {
		t.Fatalf("M8 API must return contest module error codes instead of ErrBadRequest")
	}
	codes, err := os.ReadFile("../../../pkg/apperr/contest_codes.go")
	if err != nil {
		t.Fatalf("read contest codes: %v", err)
	}
	if !strings.Contains(string(codes), "ErrContestStatsQueryInvalid") {
		t.Fatalf("invalid stats query must have a dedicated M8 error code")
	}
}

// TestContestProductionCodeAvoidsGlobalInternalErrors 防止 M8 生产路径把模块错误退回全局内部错误码。
func TestContestProductionCodeAvoidsGlobalInternalErrors(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("list contest files: %v", err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		if strings.Contains(string(src), "ErrInternal") {
			t.Fatalf("%s must use M8 dedicated error codes instead of global ErrInternal", file)
		}
	}
}

// functionSource 返回指定服务方法源码片段,用于守护高风险权限入口不退回旧路径。
func functionSource(src, name string) string {
	start := strings.Index(src, "func (s *Service) "+name+"(")
	if start < 0 {
		return ""
	}
	next := strings.Index(src[start+1:], "\nfunc (s *Service) ")
	if next < 0 {
		return src[start:]
	}
	return src[start : start+1+next]
}

// testTenantContext 构造带租户身份的服务测试上下文。
func testTenantContext() context.Context {
	return tenant.WithContext(context.Background(), tenant.Identity{TenantID: 100, AccountID: 200})
}

type noopContestAuditWriter struct{}

func (w *noopContestAuditWriter) Write(context.Context, audit.Entry) error { return nil }
