// M8 服务入口:定义竞赛编排服务依赖、构造函数与核心业务操作。
package contest

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/netx"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/crypto"
	"chaimir/pkg/snowflake"
)

// contestSandbox 是 M8 只需要的 M2 沙箱能力子集。
type contestSandbox interface {
	CreateSandbox(ctx context.Context, req contracts.SandboxCreateRequest) (contracts.SandboxInfo, error)
	RecycleBySourceRef(ctx context.Context, tenantID int64, sourceRef, reason string) error
}

// contestJudge 是 M8 只需要的 M3 判题能力子集。
type contestJudge interface {
	SubmitJudgeTask(ctx context.Context, req contracts.JudgeSubmitRequest) (contracts.JudgeTaskInfo, error)
}

// vulnHTTPClient 抽象外部漏洞源 HTTP 客户端,便于服务测试注入。
type vulnHTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

// Service 是 M8 竞赛模块服务。
type Service struct {
	store                      contestStore
	idgen                      snowflake.Generator
	auditor                    audit.Writer
	cipher                     *crypto.Cipher
	httpClient                 vulnHTTPClient
	vulnSourceMaxResponseBytes int64
	vulnSourceTimeoutSeconds   int
	identity                   contracts.IdentityService
	content                    contracts.ContentReadService
	contentImport              contracts.ContentImportService
	sandbox                    contestSandbox
	judge                      contestJudge
	bus                        eventbus.Bus
}

// NewService 构造带竞赛模块运行边界的 M8 服务。
func NewService(database *db.DB, idgen *snowflake.Node, auditor audit.Writer, cipher *crypto.Cipher, cfg config.ContestConfig, identity contracts.IdentityService, content contracts.ContentReadService, contentImport contracts.ContentImportService, sandbox contracts.SandboxService, judge contracts.JudgeService, bus eventbus.Bus) *Service {
	return &Service{store: newRepo(database), idgen: idgen, auditor: auditor, cipher: cipher, httpClient: netx.NewPublicHTTPClient(time.Duration(cfg.VulnSourceTimeoutSeconds) * time.Second), vulnSourceMaxResponseBytes: cfg.VulnSourceMaxResponseBytes, vulnSourceTimeoutSeconds: cfg.VulnSourceTimeoutSeconds, identity: identity, content: content, contentImport: contentImport, sandbox: sandbox, judge: judge, bus: bus}
}

// ListContests 查询当前租户竞赛列表。
func (s *Service) ListContests(ctx context.Context, status int16, page, size int) ([]ContestDTO, int64, error) {
	return s.store.ListContests(ctx, status, page, size)
}

// CreateContest 创建竞赛草稿。
func (s *Service) CreateContest(ctx context.Context, req ContestRequest) (ContestDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return ContestDTO{}, apperr.ErrUnauthorized
	}
	if err := validateContestRequest(req); err != nil {
		return ContestDTO{}, err
	}
	out, err := s.store.CreateContest(ctx, id, s.nextID(), req)
	if err != nil {
		return ContestDTO{}, err
	}
	return out, s.writeAudit(ctx, id.TenantID, auditActionContestCreate, auditTargetContest, ids.ParseOrZero(out.ID), map[string]any{"name": out.Name})
}

// UpdateContest 更新草稿竞赛配置。
func (s *Service) UpdateContest(ctx context.Context, contestID int64, req ContestRequest) (ContestDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return ContestDTO{}, apperr.ErrUnauthorized
	}
	if err := validateContestRequest(req); err != nil {
		return ContestDTO{}, err
	}
	current, err := s.store.GetContest(ctx, contestID)
	if err != nil {
		return ContestDTO{}, err
	}
	if err := s.ensureContestManager(ctx, current); err != nil {
		return ContestDTO{}, err
	}
	if current.Status != ContestStatusDraft {
		return ContestDTO{}, apperr.ErrContestState
	}
	out, err := s.store.UpdateContest(ctx, contestID, req)
	if err != nil {
		return ContestDTO{}, err
	}
	return out, s.writeAudit(ctx, id.TenantID, auditActionContestUpdate, auditTargetContest, contestID, map[string]any{"status": out.Status})
}

// AddContestProblem 绑定 M5 锁定版本题目到竞赛。
func (s *Service) AddContestProblem(ctx context.Context, contestID int64, req ContestProblemRequest) (ContestProblemDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return ContestProblemDTO{}, apperr.ErrUnauthorized
	}
	if err := validateProblemRequest(req); err != nil {
		return ContestProblemDTO{}, err
	}
	contest, err := s.store.GetContest(ctx, contestID)
	if err != nil {
		return ContestProblemDTO{}, err
	}
	if err := s.ensureContestManager(ctx, contest); err != nil {
		return ContestProblemDTO{}, err
	}
	if contest.Status != ContestStatusDraft {
		return ContestProblemDTO{}, apperr.ErrContestState
	}
	if s.content != nil {
		ref := contracts.ContentItemRef{ItemCode: req.ItemCode, ItemVersion: req.ItemVersion}
		if _, err := s.content.GetContentFace(ctx, id.TenantID, ref); err != nil {
			return ContestProblemDTO{}, apperr.ErrContestProblem.WithCause(err)
		}
		if err := s.content.IncrementContentUsage(ctx, id.TenantID, ref); err != nil {
			return ContestProblemDTO{}, apperr.ErrContestProblem.WithCause(err)
		}
	}
	return s.store.CreateProblem(ctx, id, s.nextID(), contestID, req)
}

// PublishContest 发布竞赛并进入报名中。
func (s *Service) PublishContest(ctx context.Context, contestID int64) (ContestDTO, error) {
	return s.transitionContest(ctx, contestID, ContestStatusSignup, auditActionContestPublish)
}

// StartContest 开始竞赛并锁定报名阶段。
func (s *Service) StartContest(ctx context.Context, contestID int64) (ContestDTO, error) {
	return s.transitionContest(ctx, contestID, ContestStatusRunning, auditActionContestStart)
}

// EndContest 结束竞赛。
func (s *Service) EndContest(ctx context.Context, contestID int64) (ContestDTO, error) {
	return s.transitionContest(ctx, contestID, ContestStatusEnded, auditActionContestEnd)
}

// ArchiveContest 归档竞赛:级联回收 M2 资源并生成最终榜单快照。
func (s *Service) ArchiveContest(ctx context.Context, contestID int64) (ResultSnapshotDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return ResultSnapshotDTO{}, apperr.ErrUnauthorized
	}
	contest, err := s.store.GetContest(ctx, contestID)
	if err != nil {
		return ResultSnapshotDTO{}, err
	}
	if err := s.ensureContestManager(ctx, contest); err != nil {
		return ResultSnapshotDTO{}, err
	}
	if err := validateContestTransition(contest.Status, ContestStatusArchived); err != nil {
		return ResultSnapshotDTO{}, err
	}
	sourceRef := sourceRefForContest(contestID)
	if s.sandbox != nil {
		if err := s.sandbox.RecycleBySourceRef(ctx, id.TenantID, sourceRef, "contest-archive"); err != nil {
			return ResultSnapshotDTO{}, apperr.ErrContestState.WithCause(err)
		}
	}
	ranks, err := s.store.ListRanks(ctx, contestID, 1, 100)
	if err != nil {
		return ResultSnapshotDTO{}, err
	}
	snapshot, err := s.store.CreateSnapshot(ctx, id.TenantID, s.nextID(), contestID, ranks)
	if err != nil {
		return ResultSnapshotDTO{}, err
	}
	if _, err := s.store.UpdateContestStatus(ctx, contestID, ContestStatusArchived); err != nil {
		return ResultSnapshotDTO{}, err
	}
	return snapshot, s.writeAudit(ctx, id.TenantID, auditActionContestArchive, auditTargetContest, contestID, map[string]any{"source_ref": sourceRef})
}

// Signup 创建个人队或团队队伍,并初始化排行榜。
func (s *Service) Signup(ctx context.Context, contestID int64, req SignupRequest) (TeamDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return TeamDTO{}, apperr.ErrUnauthorized
	}
	contest, err := s.store.GetContest(ctx, contestID)
	if err != nil {
		return TeamDTO{}, err
	}
	if err := validateSignupWindow(contest, timex.Now()); err != nil {
		return TeamDTO{}, err
	}
	name := req.Name
	if name == "" {
		name = fmt.Sprintf("team-%d", id.AccountID)
	}
	teamID := s.nextID()
	if _, err := s.store.CreateTeam(ctx, id, teamID, contestID, name, contest.TeamMode == TeamModeTeam); err != nil {
		return TeamDTO{}, err
	}
	if _, err := s.store.AddTeamMember(ctx, id, s.nextID(), teamID, id.AccountID, id.TenantID, true); err != nil {
		return TeamDTO{}, err
	}
	if _, err := s.store.UpsertRank(ctx, id.TenantID, s.nextID(), contestID, teamID, 0, 0); err != nil {
		return TeamDTO{}, err
	}
	return s.store.GetTeam(ctx, teamID)
}

// JoinTeam 使用邀请码加入队伍,跨校成员只记录授权队伍范围。
func (s *Service) JoinTeam(ctx context.Context, teamID int64, req JoinTeamRequest) (TeamDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return TeamDTO{}, apperr.ErrUnauthorized
	}
	if err := validateJoinTeamRequest(req); err != nil {
		return TeamDTO{}, err
	}
	team, err := s.store.GetTeam(ctx, teamID)
	if err != nil {
		return TeamDTO{}, err
	}
	if team.Status == TeamStatusLocked {
		return TeamDTO{}, apperr.ErrContestTeamLocked
	}
	if req.InviteCode != team.InviteCode {
		return TeamDTO{}, apperr.ErrContestTeamInvalid
	}
	if _, err := s.store.AddTeamMember(ctx, id, s.nextID(), teamID, id.AccountID, id.TenantID, false); err != nil {
		return TeamDTO{}, err
	}
	return s.store.GetTeam(ctx, teamID)
}

// GetTeam 查询队伍与队员。
func (s *Service) GetTeam(ctx context.Context, teamID int64) (TeamDTO, error) {
	team, err := s.store.GetTeam(ctx, teamID)
	if err != nil {
		return TeamDTO{}, err
	}
	if err := s.ensureTeamAccess(ctx, team); err != nil {
		return TeamDTO{}, err
	}
	return team, nil
}

// LockTeam 锁定队伍成员。
func (s *Service) LockTeam(ctx context.Context, teamID int64) (TeamDTO, error) {
	team, err := s.GetTeam(ctx, teamID)
	if err != nil {
		return TeamDTO{}, err
	}
	if team.Status == TeamStatusLocked {
		return team, nil
	}
	return s.store.LockTeam(ctx, teamID)
}

// ListProblems 查询竞赛题面列表,题面由 M5 过滤敏感内容。
func (s *Service) ListProblems(ctx context.Context, contestID int64) ([]ContestProblemDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return nil, apperr.ErrUnauthorized
	}
	problems, err := s.store.ListProblems(ctx, contestID)
	if err != nil {
		return nil, err
	}
	for i := range problems {
		if s.content == nil {
			continue
		}
		face, err := s.content.GetContentFace(ctx, id.TenantID, contracts.ContentItemRef{ItemCode: problems[i].ItemCode, ItemVersion: problems[i].ItemVersion})
		if err != nil {
			return nil, apperr.ErrContestProblem.WithCause(err)
		}
		problems[i].Face = face.Body
	}
	return problems, nil
}

// StartProblemEnv 为实操题创建 M2 环境。
func (s *Service) StartProblemEnv(ctx context.Context, contestID, problemID int64, req StartProblemEnvRequest) (ProblemEnvDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return ProblemEnvDTO{}, apperr.ErrUnauthorized
	}
	if s.sandbox == nil {
		return ProblemEnvDTO{}, apperr.ErrContestState
	}
	contest, err := s.store.GetContest(ctx, contestID)
	if err != nil {
		return ProblemEnvDTO{}, err
	}
	if err := s.ensureContestPlayable(contest); err != nil {
		return ProblemEnvDTO{}, err
	}
	problem, err := s.store.GetProblem(ctx, problemID)
	if err != nil {
		return ProblemEnvDTO{}, err
	}
	if problem.ContestID != ids.Format(contestID) {
		return ProblemEnvDTO{}, apperr.ErrContestProblem
	}
	team, err := s.currentTeamForContest(ctx, contestID, id.AccountID)
	if err != nil {
		return ProblemEnvDTO{}, err
	}
	if err := s.ensureTeamMemberAccess(ctx, ids.ParseOrZero(team.ID)); err != nil {
		return ProblemEnvDTO{}, err
	}
	sourceRef := sourceRefForContest(contestID)
	info, err := s.sandbox.CreateSandbox(ctx, contracts.SandboxCreateRequest{
		TenantID: id.TenantID, RuntimeCode: req.RuntimeCode, ToolCodes: req.ToolCodes, InitCodeRef: req.InitCodeRef,
		InitScriptRef: req.InitScriptRef, OwnerAccountID: id.AccountID, SourceRef: sourceRef, KeepAlive: req.KeepAlive, SnapshotEnabled: req.SnapshotEnabled,
	})
	if err != nil {
		return ProblemEnvDTO{}, apperr.ErrContestState.WithCause(err)
	}
	return ProblemEnvDTO{SandboxID: ids.Format(info.SandboxID), SourceRef: sourceRef, Status: info.Status}, nil
}

// SubmitSolve 提交解题判题任务并记录等待事件回写的提交。
func (s *Service) SubmitSolve(ctx context.Context, contestID, problemID int64, req SolveSubmitRequest) (SolveSubmissionDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return SolveSubmissionDTO{}, apperr.ErrUnauthorized
	}
	if err := validateSolveSubmitRequest(req); err != nil {
		return SolveSubmissionDTO{}, err
	}
	contest, err := s.store.GetContest(ctx, contestID)
	if err != nil {
		return SolveSubmissionDTO{}, err
	}
	if err := s.ensureContestPlayable(contest); err != nil {
		return SolveSubmissionDTO{}, err
	}
	problem, err := s.store.GetProblem(ctx, problemID)
	if err != nil {
		return SolveSubmissionDTO{}, err
	}
	if problem.ContestID != ids.Format(contestID) {
		return SolveSubmissionDTO{}, apperr.ErrContestProblem
	}
	teamID := ids.ParseOrZero(req.TeamID)
	if err := s.ensureTeamMemberAccess(ctx, teamID); err != nil {
		return SolveSubmissionDTO{}, err
	}
	submissionID := s.nextID()
	sourceRef := sourceRefForSolveSubmission(submissionID)
	task, err := s.judge.SubmitJudgeTask(ctx, contracts.JudgeSubmitRequest{
		TenantID: id.TenantID, JudgerCode: req.JudgerCode, ItemCode: problem.ItemCode, ItemVersion: problem.ItemVersion,
		CodeStorageKey: req.CodeStorageKey, CodeHash: req.CodeHash, SubmitterID: id.AccountID, SourceRef: sourceRef,
		SandboxMode: "reuse", TargetSandboxRef: req.SandboxRef, ExtraInput: req.ExtraInput, Priority: 8,
	})
	if err != nil {
		return SolveSubmissionDTO{}, apperr.ErrContestJudgeFailed.WithCause(err)
	}
	return s.store.CreateSolveSubmission(ctx, id, submissionID, contestID, problemID, teamID, req, sourceRef, ids.Format(task.TaskID))
}

// GetSubmission 查询提交结果。
func (s *Service) GetSubmission(ctx context.Context, submissionID int64) (SolveSubmissionDTO, error) {
	submission, err := s.store.GetSolveSubmission(ctx, submissionID)
	if err != nil {
		return SolveSubmissionDTO{}, err
	}
	if err := s.ensureSubmissionAccess(ctx, submission); err != nil {
		return SolveSubmissionDTO{}, err
	}
	return submission, nil
}

// ApplySolveJudgement 处理 M3 判题完成事件并更新提交和排行榜。
func (s *Service) ApplySolveJudgement(ctx context.Context, tenantID, taskID int64, passed bool, score int32) error {
	pending, err := s.store.PendingSubmissionByJudgeTask(ctx, tenantID, taskID)
	if err != nil {
		return err
	}
	return s.applySolveJudgement(ctx, pending, passed, score)
}

// applySolveJudgement 回写已绑定来源的解题判题结果并刷新排行榜。
func (s *Service) applySolveJudgement(ctx context.Context, pending pendingSolveSubmission, passed bool, score int32) error {
	if score > pending.MaxScore {
		score = pending.MaxScore
	}
	if !passed {
		score = 0
	}
	if _, err := s.store.UpdateSubmissionResult(ctx, pending.ID, passed, score); err != nil {
		return err
	}
	return s.store.AddRankScore(ctx, pending.TenantID, s.nextID(), pending.ContestID, pending.TeamID, float64(score))
}

// SubmitBattleEntry 保存参战物并使同角色旧版本失效。
func (s *Service) SubmitBattleEntry(ctx context.Context, contestID int64, req BattleEntryRequest) (BattleEntryDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return BattleEntryDTO{}, apperr.ErrUnauthorized
	}
	if err := validateBattleEntryRequest(req); err != nil {
		return BattleEntryDTO{}, err
	}
	contest, err := s.store.GetContest(ctx, contestID)
	if err != nil {
		return BattleEntryDTO{}, err
	}
	if err := s.ensureContestPlayable(contest); err != nil {
		return BattleEntryDTO{}, err
	}
	teamID := ids.ParseOrZero(req.TeamID)
	if err := s.ensureTeamMemberAccess(ctx, teamID); err != nil {
		return BattleEntryDTO{}, err
	}
	entries, err := s.store.ListBattleEntries(ctx, contestID, teamID)
	if err != nil {
		return BattleEntryDTO{}, err
	}
	return s.store.CreateBattleEntry(ctx, id, s.nextID(), contestID, teamID, req, int32(len(entries)+1))
}

// ListBattleEntries 查询当前队伍参战物历史。
func (s *Service) ListBattleEntries(ctx context.Context, contestID, teamID int64) ([]BattleEntryDTO, error) {
	if err := s.ensureTeamMemberAccess(ctx, teamID); err != nil {
		return nil, err
	}
	return s.store.ListBattleEntries(ctx, contestID, teamID)
}

// RecordBattleMatch 记录撮合结果并更新双方 ELO。
func (s *Service) RecordBattleMatch(ctx context.Context, tenantID int64, result BattleMatchResult) (BattleMatchDTO, error) {
	if err := validateBattleResult(result); err != nil {
		return BattleMatchDTO{}, err
	}
	entryA, err := s.store.GetBattleEntry(ctx, result.EntryAID)
	if err != nil {
		return BattleMatchDTO{}, err
	}
	entryB, err := s.store.GetBattleEntry(ctx, result.EntryBID)
	if err != nil {
		return BattleMatchDTO{}, err
	}
	rankA, err := s.store.GetRankOrDefault(ctx, tenantID, result.ContestID, ids.ParseOrZero(entryA.TeamID))
	if err != nil {
		return BattleMatchDTO{}, err
	}
	rankB, err := s.store.GetRankOrDefault(ctx, tenantID, result.ContestID, ids.ParseOrZero(entryB.TeamID))
	if err != nil {
		return BattleMatchDTO{}, err
	}
	deltaA, deltaB := eloDelta(rankA.Score, rankB.Score, result.Result)
	scoreDelta := map[string]any{"entry_a": deltaA, "entry_b": deltaB}
	match, err := s.store.CreateBattleMatch(ctx, tenantID, s.nextID(), result, scoreDelta)
	if err != nil {
		return BattleMatchDTO{}, err
	}
	if _, err := s.store.UpsertRank(ctx, tenantID, s.nextID(), result.ContestID, ids.ParseOrZero(entryA.TeamID), rankA.Score+deltaA, rankA.SolvedCount); err != nil {
		return BattleMatchDTO{}, err
	}
	if _, err := s.store.UpsertRank(ctx, tenantID, s.nextID(), result.ContestID, ids.ParseOrZero(entryB.TeamID), rankB.Score+deltaB, rankB.SolvedCount); err != nil {
		return BattleMatchDTO{}, err
	}
	return match, nil
}

// ListBattleMatches 按队伍查询对局列表。
func (s *Service) ListBattleMatches(ctx context.Context, contestID, teamID int64, page, size int) ([]BattleMatchDTO, error) {
	if err := s.ensureTeamMemberAccess(ctx, teamID); err != nil {
		return nil, err
	}
	return s.store.ListBattleMatches(ctx, contestID, teamID, page, size)
}

// GetMatchReplay 返回对局回放引用。
func (s *Service) GetMatchReplay(ctx context.Context, matchID int64) (map[string]any, error) {
	match, err := s.store.GetBattleMatch(ctx, matchID)
	if err != nil {
		return nil, err
	}
	if err := s.ensureBattleMatchAccess(ctx, match); err != nil {
		return nil, err
	}
	return map[string]any{"match_id": match.ID, "replay_ref": match.ReplayRef, "result": match.Result}, nil
}

// ListLadder 查询排行榜。
func (s *Service) ListLadder(ctx context.Context, contestID int64, page, size int) ([]LadderRankDTO, error) {
	return s.store.ListRanks(ctx, contestID, page, size)
}

// GetResultSnapshot 查询归档快照。
func (s *Service) GetResultSnapshot(ctx context.Context, contestID int64) (ResultSnapshotDTO, error) {
	return s.store.GetSnapshot(ctx, contestID)
}

// ListMyContestRecords 查询当前学生战绩。
func (s *Service) ListMyContestRecords(ctx context.Context) ([]ContestAchievementDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return nil, apperr.ErrUnauthorized
	}
	return s.store.ListAchievements(ctx, id.TenantID, id.AccountID)
}

// ListCheatSuspects 查询已判定作弊记录作为可疑列表落点。
func (s *Service) ListCheatSuspects(ctx context.Context, contestID int64, page, size int) ([]CheatRecordDTO, error) {
	return s.store.ListCheatRecords(ctx, contestID, page, size)
}

// CreateCheatRecord 写入作弊判定并保留审计。
func (s *Service) CreateCheatRecord(ctx context.Context, contestID int64, req CheatRecordRequest) (CheatRecordDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return CheatRecordDTO{}, apperr.ErrUnauthorized
	}
	teamID := ids.ParseOrZero(req.TeamID)
	out, err := s.store.CreateCheatRecord(ctx, id, s.nextID(), contestID, teamID, req)
	if err != nil {
		return CheatRecordDTO{}, err
	}
	return out, s.writeAudit(ctx, id.TenantID, auditActionCheatRecord, auditTargetContest, contestID, map[string]any{"team_id": teamID, "action": req.Action})
}

// ListVulnSources 查询漏洞源配置。
func (s *Service) ListVulnSources(ctx context.Context, page, size int) ([]VulnSourceDTO, error) {
	sources, err := s.store.ListVulnSources(ctx, page, size)
	if err != nil {
		return nil, err
	}
	for i := range sources {
		sources[i].Config = maskVulnSourceConfig(sources[i].Config)
	}
	return sources, nil
}

// CreateVulnSource 创建漏洞源配置。
func (s *Service) CreateVulnSource(ctx context.Context, req VulnSourceRequest) (VulnSourceDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return VulnSourceDTO{}, apperr.ErrUnauthorized
	}
	if err := validateVulnSourceRequest(req); err != nil {
		return VulnSourceDTO{}, err
	}
	protected, err := protectVulnSourceConfig(s.cipher, req.Config)
	if err != nil {
		return VulnSourceDTO{}, apperr.ErrContestVulnSourceInvalid.WithCause(err)
	}
	req.Config = protected
	out, err := s.store.CreateVulnSource(ctx, id, s.nextID(), req)
	if err != nil {
		return VulnSourceDTO{}, err
	}
	out.Config = maskVulnSourceConfig(out.Config)
	return out, nil
}

// ImportVulnProblem 导入漏洞案例并生成转化草稿。
func (s *Service) ImportVulnProblem(ctx context.Context, req VulnProblemImportRequest) (VulnProblemDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return VulnProblemDTO{}, apperr.ErrUnauthorized
	}
	if err := validateVulnProblemImport(req); err != nil {
		return VulnProblemDTO{}, err
	}
	return s.store.CreateVulnProblem(ctx, id, s.nextID(), req)
}

// PrevalidateVulnProblem 写入隔离预验证结果。
func (s *Service) PrevalidateVulnProblem(ctx context.Context, problemID int64, req VulnPrevalidateRequest) (VulnProblemDTO, error) {
	status := VulnPrevalidateFailed
	if req.Passed {
		status = VulnPrevalidatePassed
	}
	return s.store.UpdateVulnPrevalidate(ctx, problemID, status, req.Detail)
}

// FinalizeVulnProblem 将预验证通过的漏洞题固化入 M5。
func (s *Service) FinalizeVulnProblem(ctx context.Context, problemID int64, req VulnFinalizeRequest) (VulnProblemDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return VulnProblemDTO{}, apperr.ErrUnauthorized
	}
	problem, err := s.store.GetVulnProblem(ctx, problemID)
	if err != nil {
		return VulnProblemDTO{}, err
	}
	if err := validateVulnFinalizeGate(problem); err != nil {
		return VulnProblemDTO{}, err
	}
	if s.contentImport == nil {
		return VulnProblemDTO{}, apperr.ErrContestVulnFinalize
	}
	item, err := s.contentImport.SystemImportContent(ctx, contracts.ContentSystemImportRequest{
		TenantID: id.TenantID, Code: req.Code, Version: req.Version, Type: 3, Title: problem.Title,
		CategoryID: ids.ParseOrZero(req.CategoryID), Difficulty: req.Difficulty, Tags: req.Tags, KnowledgePoints: req.KnowledgePoints,
		AuthorID: id.AccountID, AuthorType: 2, Visibility: 1, Body: problem.DraftBody, SensitiveFields: req.SensitiveFields,
		AutoPublish: true, SystemImportNote: map[string]any{"source": "contest.vuln_problem", "problem_id": problem.ID},
	})
	if err != nil {
		return VulnProblemDTO{}, apperr.ErrContestVulnFinalize.WithCause(err)
	}
	return s.store.FinalizeVulnProblem(ctx, problemID, item.ItemCode, item.ItemVersion)
}

// Stats 实现 M9 看板竞赛统计契约。
func (s *Service) Stats(ctx context.Context, tenantID int64) (contracts.ContestStats, error) {
	stats, err := s.store.Stats(ctx, tenantID)
	if err != nil {
		return contracts.ContestStats{}, err
	}
	return contracts.ContestStats{TenantID: tenantID, ContestCount: stats.ContestCount, ActiveContestCount: stats.ActiveContestCount, TeamCount: stats.TeamCount}, nil
}

// StatsDTO 返回 HTTP 内部统计 DTO。
func (s *Service) StatsDTO(ctx context.Context, tenantID int64) (StatsDTO, error) {
	return s.store.Stats(ctx, tenantID)
}

// ListStudentAchievements 实现 M11 竞赛成就只读契约。
func (s *Service) ListStudentAchievements(ctx context.Context, tenantID, studentID int64) ([]contracts.ContestAchievement, error) {
	items, err := s.store.ListAchievements(ctx, tenantID, studentID)
	if err != nil {
		return nil, err
	}
	out := make([]contracts.ContestAchievement, 0, len(items))
	for _, item := range items {
		contestID, _ := ids.Parse(item.ContestID)
		teamID, _ := ids.Parse(item.TeamID)
		out = append(out, contracts.ContestAchievement{TenantID: tenantID, StudentID: studentID, ContestID: contestID, TeamID: teamID, Score: item.Score, Rank: item.Rank})
	}
	return out, nil
}

// transitionContest 执行竞赛状态流转并写审计。
func (s *Service) transitionContest(ctx context.Context, contestID int64, to int16, action string) (ContestDTO, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return ContestDTO{}, apperr.ErrUnauthorized
	}
	contest, err := s.store.GetContest(ctx, contestID)
	if err != nil {
		return ContestDTO{}, err
	}
	if err := s.ensureContestManager(ctx, contest); err != nil {
		return ContestDTO{}, err
	}
	if err := validateContestTransition(contest.Status, to); err != nil {
		return ContestDTO{}, err
	}
	out, err := s.store.UpdateContestStatus(ctx, contestID, to)
	if err != nil {
		return ContestDTO{}, err
	}
	return out, s.writeAudit(ctx, id.TenantID, action, auditTargetContest, contestID, map[string]any{"status": to})
}

// ensureContestManager 校验当前账号是竞赛组织者或学校管理员。
func (s *Service) ensureContestManager(ctx context.Context, contest ContestDTO) error {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return apperr.ErrUnauthorized
	}
	if id.IsPlatform {
		return nil
	}
	if ids.ParseOrZero(contest.OrganizerID) == id.AccountID {
		return nil
	}
	if s.identity != nil {
		allowed, err := s.identity.HasRole(ctx, id.AccountID, contracts.RoleSchoolAdmin)
		if err != nil {
			return apperr.ErrContestForbidden.WithCause(err)
		}
		if allowed {
			return nil
		}
	}
	return apperr.ErrContestForbidden
}

// ensureContestPlayable 校验选手侧比赛动作只发生在进行中或封榜期。
func (s *Service) ensureContestPlayable(contest ContestDTO) error {
	if contest.Status != ContestStatusRunning && contest.Status != ContestStatusFrozen {
		return apperr.ErrContestState
	}
	return nil
}

// ensureTeamAccess 校验学生只能访问自己所在队伍,教师和管理员按服务端角色契约放行。
func (s *Service) ensureTeamAccess(ctx context.Context, team TeamDTO) error {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return apperr.ErrUnauthorized
	}
	if id.IsPlatform {
		return nil
	}
	for _, member := range team.Members {
		if ids.ParseOrZero(member.AccountID) == id.AccountID {
			return nil
		}
	}
	if s.identity != nil {
		if allowed, err := s.identity.HasRole(ctx, id.AccountID, contracts.RoleTeacher); err != nil {
			return apperr.ErrContestForbidden.WithCause(err)
		} else if allowed {
			return nil
		}
		if allowed, err := s.identity.HasRole(ctx, id.AccountID, contracts.RoleSchoolAdmin); err != nil {
			return apperr.ErrContestForbidden.WithCause(err)
		} else if allowed {
			return nil
		}
	}
	return apperr.ErrContestForbidden
}

// ensureTeamMemberAccess 以服务端队伍成员关系校验当前账号是否可访问队伍资源。
func (s *Service) ensureTeamMemberAccess(ctx context.Context, teamID int64) error {
	if teamID == 0 {
		return apperr.ErrContestTeamInvalid
	}
	team, err := s.store.GetTeam(ctx, teamID)
	if err != nil {
		return err
	}
	return s.ensureTeamAccess(ctx, team)
}

// currentTeamForContest 从服务端成员关系推导当前账号在竞赛中的队伍。
func (s *Service) currentTeamForContest(ctx context.Context, contestID, accountID int64) (TeamDTO, error) {
	return s.store.GetTeamByContestAndAccount(ctx, contestID, accountID)
}

// ensureSubmissionAccess 校验提交记录只对提交者、队友或竞赛管理角色可见。
func (s *Service) ensureSubmissionAccess(ctx context.Context, submission SolveSubmissionDTO) error {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return apperr.ErrUnauthorized
	}
	if id.IsPlatform || ids.ParseOrZero(submission.SubmitterID) == id.AccountID {
		return nil
	}
	return s.ensureTeamMemberAccess(ctx, ids.ParseOrZero(submission.TeamID))
}

// ensureBattleMatchAccess 校验对局回放只对任一参赛队成员或竞赛管理角色可见。
func (s *Service) ensureBattleMatchAccess(ctx context.Context, match BattleMatchDTO) error {
	entryA, err := s.store.GetBattleEntry(ctx, ids.ParseOrZero(match.EntryAID))
	if err != nil {
		return err
	}
	if err := s.ensureTeamMemberAccess(ctx, ids.ParseOrZero(entryA.TeamID)); err == nil {
		return nil
	} else if !isContestForbidden(err) {
		return err
	}
	entryB, err := s.store.GetBattleEntry(ctx, ids.ParseOrZero(match.EntryBID))
	if err != nil {
		return err
	}
	if err := s.ensureTeamMemberAccess(ctx, ids.ParseOrZero(entryB.TeamID)); err == nil {
		return nil
	} else if !isContestForbidden(err) {
		return err
	}
	return apperr.ErrContestForbidden
}

// isContestForbidden 判断访问校验是否只是权限拒绝,用于对局双方队伍的权限检查。
func isContestForbidden(err error) bool {
	appErr, ok := apperr.As(err)
	return ok && appErr.Code == apperr.ErrContestForbidden.Code
}

// nextID 从雪花节点生成 ID。
func (s *Service) nextID() int64 {
	return s.idgen.Generate()
}

// sourceRefForContest 构造符合全局规范的竞赛来源引用。
func sourceRefForContest(contestID int64) string {
	return fmt.Sprintf("contest:%d:contest:%d", timex.Now().Year(), contestID)
}

// sourceRefForSolveSubmission 构造提交级来源引用,避免 M3 幂等把同场竞赛多次提交折叠。
func sourceRefForSolveSubmission(submissionID int64) string {
	return fmt.Sprintf("contest:%d:submission:%d", timex.Now().Year(), submissionID)
}

// tenantFromContext 读取当前请求租户身份。
func tenantFromContext(ctx context.Context) (tenant.Identity, bool) { return tenant.FromContext(ctx) }
