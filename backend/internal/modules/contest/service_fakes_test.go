// M8 服务测试替身:为服务规则测试提供内存 store、固定 ID 和跨模块依赖。
package contest

import (
	"context"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/tenant"
)

// fixedIDGen 为测试返回固定雪花 ID。
type fixedIDGen int64

// Generate 返回固定 ID。
func (g fixedIDGen) Generate() int64 { return int64(g) }

// fakeContestStore 是服务测试使用的最小内存仓储。
type fakeContestStore struct {
	contest             ContestDTO
	pendingSubmission   pendingSolveSubmission
	updatedSubmission   SolveSubmissionDTO
	rankDelta           float64
	rankInitialized     bool
	entryA              BattleEntryDTO
	entryB              BattleEntryDTO
	eloA                float64
	eloB                float64
	ranks               []LadderRankDTO
	snapshotCreated     bool
	vulnSource          VulnSourceDTO
	sourceSynced        bool
	createdVulnProblems []VulnProblemImportRequest
}

// contestDTO 构造测试竞赛。
func contestDTO(status int16) ContestDTO {
	now := time.Now().UTC()
	return ContestDTO{
		ID: "8101", TenantID: "100", OrganizerID: "200", Name: "contest", Mode: ContestModeSolve, TeamMode: TeamModeSolo,
		SignupStart: now.Add(-time.Hour), SignupEnd: now.Add(time.Hour), StartAt: now.Add(2 * time.Hour),
		EndAt: now.Add(3 * time.Hour), Status: status, Rules: map[string]any{},
	}
}

// ListContests 返回测试竞赛列表。
func (s *fakeContestStore) ListContests(context.Context, int16, int, int) ([]ContestDTO, int64, error) {
	return []ContestDTO{s.contest}, 1, nil
}

// CreateContest 返回新建竞赛。
func (s *fakeContestStore) CreateContest(_ context.Context, id tenant.Identity, contestID int64, req ContestRequest) (ContestDTO, error) {
	return ContestDTO{ID: ids.Format(contestID), TenantID: ids.Format(id.TenantID), OrganizerID: ids.Format(id.AccountID), Name: req.Name, Mode: req.Mode, TeamMode: req.TeamMode, SignupStart: req.SignupStart, SignupEnd: req.SignupEnd, StartAt: req.StartAt, EndAt: req.EndAt, Status: ContestStatusDraft}, nil
}

// GetContest 返回当前测试竞赛。
func (s *fakeContestStore) GetContest(context.Context, int64) (ContestDTO, error) {
	return s.contest, nil
}

// UpdateContest 返回更新后的测试竞赛。
func (s *fakeContestStore) UpdateContest(_ context.Context, _ int64, req ContestRequest) (ContestDTO, error) {
	s.contest.Name = req.Name
	return s.contest, nil
}

// UpdateContestStatus 更新测试竞赛状态。
func (s *fakeContestStore) UpdateContestStatus(_ context.Context, _ int64, status int16) (ContestDTO, error) {
	s.contest.Status = status
	return s.contest, nil
}

// CreateProblem 返回竞赛题目引用。
func (s *fakeContestStore) CreateProblem(context.Context, tenant.Identity, int64, int64, ContestProblemRequest) (ContestProblemDTO, error) {
	return ContestProblemDTO{}, nil
}

// ListProblems 返回空题目列表。
func (s *fakeContestStore) ListProblems(context.Context, int64) ([]ContestProblemDTO, error) {
	return nil, nil
}

// GetProblem 返回测试题目。
func (s *fakeContestStore) GetProblem(context.Context, int64) (ContestProblemDTO, error) {
	return ContestProblemDTO{ID: "8301", ContestID: "8101", ItemCode: "p", ItemVersion: "v1", Score: 40}, nil
}

// CreateTeam 返回新建队伍。
func (s *fakeContestStore) CreateTeam(_ context.Context, _ tenant.Identity, teamID, contestID int64, name string, _ bool) (TeamDTO, error) {
	return TeamDTO{ID: ids.Format(teamID), ContestID: ids.Format(contestID), Name: name, Status: TeamStatusBuilding}, nil
}

// AddTeamMember 返回新增队员。
func (s *fakeContestStore) AddTeamMember(_ context.Context, _ tenant.Identity, memberID, teamID, accountID, memberTenantID int64, leader bool) (TeamMemberDTO, error) {
	return TeamMemberDTO{ID: ids.Format(memberID), TeamID: ids.Format(teamID), AccountID: ids.Format(accountID), MemberTenantID: ids.Format(memberTenantID), IsLeader: leader}, nil
}

// GetTeam 返回测试队伍。
func (s *fakeContestStore) GetTeam(context.Context, int64) (TeamDTO, error) {
	return TeamDTO{ID: "8201", ContestID: "8101", Name: "alice", Status: TeamStatusBuilding, Members: []TeamMemberDTO{{ID: "1", TeamID: "8201", AccountID: "200", MemberTenantID: "100", IsLeader: true}}}, nil
}

// GetTeamByContestAndAccount 返回当前账号所属队伍。
func (s *fakeContestStore) GetTeamByContestAndAccount(ctx context.Context, _ int64, _ int64) (TeamDTO, error) {
	return s.GetTeam(ctx, 8201)
}

// LockTeam 返回锁定队伍。
func (s *fakeContestStore) LockTeam(context.Context, int64) (TeamDTO, error) {
	team, _ := s.GetTeam(context.Background(), 0)
	team.Status = TeamStatusLocked
	return team, nil
}

// CreateSolveSubmission 返回提交记录。
func (s *fakeContestStore) CreateSolveSubmission(context.Context, tenant.Identity, int64, int64, int64, int64, SolveSubmitRequest, string, string) (SolveSubmissionDTO, error) {
	return SolveSubmissionDTO{}, nil
}

// GetSolveSubmission 返回提交记录。
func (s *fakeContestStore) GetSolveSubmission(context.Context, int64) (SolveSubmissionDTO, error) {
	return SolveSubmissionDTO{}, nil
}

// PendingSubmissionByJudgeTask 返回待回写提交。
func (s *fakeContestStore) PendingSubmissionByJudgeTask(context.Context, int64, int64) (pendingSolveSubmission, error) {
	return s.pendingSubmission, nil
}

// UpdateSubmissionResult 记录提交回写。
func (s *fakeContestStore) UpdateSubmissionResult(_ context.Context, id int64, passed bool, score int32) (SolveSubmissionDTO, error) {
	s.updatedSubmission = SolveSubmissionDTO{ID: ids.Format(id), Passed: passed, Score: score}
	return s.updatedSubmission, nil
}

// CreateBattleEntry 返回参战物。
func (s *fakeContestStore) CreateBattleEntry(context.Context, tenant.Identity, int64, int64, int64, BattleEntryRequest, int32) (BattleEntryDTO, error) {
	return BattleEntryDTO{}, nil
}

// ListBattleEntries 返回空参战物列表。
func (s *fakeContestStore) ListBattleEntries(context.Context, int64, int64) ([]BattleEntryDTO, error) {
	return nil, nil
}

// GetBattleEntry 返回测试参战物。
func (s *fakeContestStore) GetBattleEntry(_ context.Context, entryID int64) (BattleEntryDTO, error) {
	if entryID == ids.ParseOrZero(s.entryA.ID) {
		return s.entryA, nil
	}
	return s.entryB, nil
}

// CreateBattleMatch 返回对局记录。
func (s *fakeContestStore) CreateBattleMatch(_ context.Context, _ int64, matchID int64, result BattleMatchResult, delta map[string]any) (BattleMatchDTO, error) {
	return BattleMatchDTO{ID: ids.Format(matchID), ContestID: ids.Format(result.ContestID), EntryAID: ids.Format(result.EntryAID), EntryBID: ids.Format(result.EntryBID), Result: result.Result, ReplayRef: result.ReplayRef, ScoreDelta: delta}, nil
}

// GetBattleMatch 返回对局记录。
func (s *fakeContestStore) GetBattleMatch(context.Context, int64) (BattleMatchDTO, error) {
	return BattleMatchDTO{}, nil
}

// ListBattleMatches 返回空对局列表。
func (s *fakeContestStore) ListBattleMatches(context.Context, int64, int64, int, int) ([]BattleMatchDTO, error) {
	return nil, nil
}

// UpsertRank 记录排行更新。
func (s *fakeContestStore) UpsertRank(_ context.Context, _ int64, _ int64, _ int64, teamID int64, score float64, solved int32) (LadderRankDTO, error) {
	s.rankInitialized = true
	if teamID == ids.ParseOrZero(s.entryA.TeamID) {
		s.eloA = score
	}
	if teamID == ids.ParseOrZero(s.entryB.TeamID) {
		s.eloB = score
	}
	return LadderRankDTO{TeamID: ids.Format(teamID), Score: score, SolvedCount: solved}, nil
}

// GetRankOrDefault 返回默认 ELO。
func (s *fakeContestStore) GetRankOrDefault(_ context.Context, _ int64, _ int64, teamID int64) (LadderRankDTO, error) {
	return LadderRankDTO{TeamID: ids.Format(teamID), Score: 1000}, nil
}

// AddRankScore 记录解题排行增量。
func (s *fakeContestStore) AddRankScore(_ context.Context, _ int64, _ int64, _ int64, _ int64, delta float64) error {
	s.rankDelta = delta
	return nil
}

// ListRanks 返回测试排行榜。
func (s *fakeContestStore) ListRanks(context.Context, int64, int, int) ([]LadderRankDTO, error) {
	return s.ranks, nil
}

// CreateSnapshot 记录快照创建。
func (s *fakeContestStore) CreateSnapshot(_ context.Context, _ int64, snapshotID, contestID int64, ranks []LadderRankDTO) (ResultSnapshotDTO, error) {
	s.snapshotCreated = true
	return ResultSnapshotDTO{ID: ids.Format(snapshotID), ContestID: ids.Format(contestID), FinalRanking: ranks}, nil
}

// GetSnapshot 返回空快照。
func (s *fakeContestStore) GetSnapshot(context.Context, int64) (ResultSnapshotDTO, error) {
	return ResultSnapshotDTO{}, nil
}

// ListAchievements 返回空成就。
func (s *fakeContestStore) ListAchievements(context.Context, int64, int64) ([]ContestAchievementDTO, error) {
	return nil, nil
}

// CreateCheatRecord 返回作弊记录。
func (s *fakeContestStore) CreateCheatRecord(context.Context, tenant.Identity, int64, int64, int64, CheatRecordRequest) (CheatRecordDTO, error) {
	return CheatRecordDTO{}, nil
}

// ListCheatRecords 返回空作弊记录。
func (s *fakeContestStore) ListCheatRecords(context.Context, int64, int, int) ([]CheatRecordDTO, error) {
	return nil, nil
}

// CreateVulnSource 返回漏洞源。
func (s *fakeContestStore) CreateVulnSource(context.Context, tenant.Identity, int64, VulnSourceRequest) (VulnSourceDTO, error) {
	return VulnSourceDTO{}, nil
}

// ListVulnSources 返回空漏洞源列表。
func (s *fakeContestStore) ListVulnSources(context.Context, int, int) ([]VulnSourceDTO, error) {
	return nil, nil
}

// GetVulnSource 返回测试漏洞源。
func (s *fakeContestStore) GetVulnSource(context.Context, int64) (VulnSourceDTO, error) {
	return s.vulnSource, nil
}

// MarkVulnSourceSynced 记录漏洞源同步时间更新。
func (s *fakeContestStore) MarkVulnSourceSynced(context.Context, int64) (VulnSourceDTO, error) {
	s.sourceSynced = true
	return s.vulnSource, nil
}

// CreateVulnProblem 返回漏洞题草稿。
func (s *fakeContestStore) CreateVulnProblem(_ context.Context, _ tenant.Identity, problemID int64, req VulnProblemImportRequest) (VulnProblemDTO, error) {
	s.createdVulnProblems = append(s.createdVulnProblems, req)
	return VulnProblemDTO{
		ID: ids.Format(problemID), SourceID: req.SourceID, ExternalRef: req.ExternalRef, Title: req.Title,
		Level: req.Level, RuntimeMode: req.RuntimeMode, DraftBody: req.DraftBody, Status: VulnProblemDraft,
		PrevalidateStatus: VulnPrevalidatePending,
	}, nil
}

// GetVulnProblem 返回通过预验证的漏洞题。
func (s *fakeContestStore) GetVulnProblem(context.Context, int64) (VulnProblemDTO, error) {
	return VulnProblemDTO{ID: "1", Title: "vuln", Status: VulnProblemDraft, PrevalidateStatus: VulnPrevalidatePassed, DraftBody: map[string]any{}}, nil
}

// UpdateVulnPrevalidate 返回漏洞题草稿。
func (s *fakeContestStore) UpdateVulnPrevalidate(context.Context, int64, int16, map[string]any) (VulnProblemDTO, error) {
	return VulnProblemDTO{}, nil
}

// FinalizeVulnProblem 返回固化后的漏洞题。
func (s *fakeContestStore) FinalizeVulnProblem(context.Context, int64, string, string) (VulnProblemDTO, error) {
	return VulnProblemDTO{Status: VulnProblemFinalized}, nil
}

// Stats 返回测试统计。
func (s *fakeContestStore) Stats(context.Context, int64) (StatsDTO, error) {
	return StatsDTO{}, nil
}

// fakeSandboxService 记录回收来源。
type fakeSandboxService struct {
	recycledSource string
}

// CreateSandbox 返回测试沙箱。
func (s *fakeSandboxService) CreateSandbox(context.Context, contracts.SandboxCreateRequest) (contracts.SandboxInfo, error) {
	return contracts.SandboxInfo{}, nil
}

// RecycleBySourceRef 记录级联回收 source_ref。
func (s *fakeSandboxService) RecycleBySourceRef(_ context.Context, _ int64, sourceRef, _ string) error {
	s.recycledSource = sourceRef
	return nil
}

// GetSandbox 返回测试沙箱摘要。
func (s *fakeSandboxService) GetSandbox(context.Context, int64) (contracts.SandboxInfo, error) {
	return contracts.SandboxInfo{}, nil
}

// PutSandboxFile 是测试替身的文件写入入口。
func (s *fakeSandboxService) PutSandboxFile(context.Context, contracts.SandboxFileWrite) error {
	return nil
}

// SaveSandboxFiles 是测试替身的文件持久化入口。
func (s *fakeSandboxService) SaveSandboxFiles(context.Context, int64) (string, error) {
	return "", nil
}

// ExecSandboxCommand 是测试替身的命令执行入口。
func (s *fakeSandboxService) ExecSandboxCommand(context.Context, contracts.SandboxExecRequest) (contracts.SandboxExecResult, error) {
	return contracts.SandboxExecResult{}, nil
}

// ChainDeploy 是测试替身的链上部署入口。
func (s *fakeSandboxService) ChainDeploy(context.Context, int64, map[string]any) (map[string]any, error) {
	return map[string]any{}, nil
}

// ChainSendTx 是测试替身的链上交易入口。
func (s *fakeSandboxService) ChainSendTx(context.Context, int64, map[string]any) (map[string]any, error) {
	return map[string]any{}, nil
}

// ChainQuery 是测试替身的链上查询入口。
func (s *fakeSandboxService) ChainQuery(context.Context, int64, string) (map[string]any, error) {
	return map[string]any{}, nil
}

// ChainReset 是测试替身的链上重置入口。
func (s *fakeSandboxService) ChainReset(context.Context, int64) error {
	return nil
}

// Stats 返回测试沙箱统计。
func (s *fakeSandboxService) Stats(context.Context, int64) (contracts.SandboxStats, error) {
	return contracts.SandboxStats{}, nil
}
