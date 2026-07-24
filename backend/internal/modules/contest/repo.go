// contest repo 文件定义 M8 持久化接口和事务边界,只操作竞赛模块自有表。
package contest

import (
	"context"
	"errors"
	"fmt"

	"chaimir/internal/modules/contest/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"

	"github.com/jackc/pgx/v5"
)

// Store 定义 contest service 所需的事务入口。
type Store interface {
	// TenantTx 在注入 RLS 租户变量后访问 M8 租户表。
	TenantTx(ctx context.Context, tenantID int64, fn func(context.Context, TxStore) error) error
	// PrivilegedTx 在受控后台任务中跨租户扫描 M8 自有表。
	PrivilegedTx(ctx context.Context, fn func(context.Context, TxStore) error) error
}

// TxStore 定义单个事务内可调用的数据访问能力,不暴露 sqlc 行类型。
type TxStore interface {
	CreateContest(context.Context, Contest) (Contest, error)
	GetContest(context.Context, int64, int64) (Contest, error)
	ListContests(context.Context, int64, int16, int, int) ([]Contest, int64, error)
	ListStudentContests(context.Context, int64, int, int) ([]Contest, int64, error)
	UpdateContest(context.Context, Contest) (Contest, error)
	SetContestStatus(context.Context, int64, int64, int16) (Contest, error)
	UpsertContestProblem(context.Context, ContestProblem) (ContestProblem, error)
	GetContestProblem(context.Context, int64, int64) (ContestProblem, error)
	ListContestProblems(context.Context, int64, int64) ([]ContestProblem, error)
	CreateTeam(context.Context, Team) (Team, error)
	GetTeam(context.Context, int64, int64) (Team, error)
	GetTeamByInviteCode(context.Context, int64, string) (Team, error)
	GetTeamForAccount(context.Context, int64, int64, int64, int64) (Team, error)
	LockTeam(context.Context, int64, int64) (Team, error)
	LockContestTeams(context.Context, int64, int64) error
	AddTeamMember(context.Context, TeamMember) (TeamMember, error)
	ListTeamMembers(context.Context, int64, int64) ([]TeamMember, error)
	AccountTeamIDs(context.Context, int64, int64, int64, int64) ([]int64, error)
	CreateSolveSubmission(context.Context, SolveSubmission) (SolveSubmission, error)
	GetSolveSubmission(context.Context, int64, int64) (SolveSubmission, error)
	GetSolveSubmissionByJudgeTask(context.Context, int64, string) (SolveSubmission, error)
	UpdateSolveSubmissionResult(context.Context, int64, int64, bool, int32) (SolveSubmission, error)
	RecentSolveCount(context.Context, int64, int64, int64, int64, int) (int64, error)
	RecentFailedSolveCount(context.Context, int64, int64, int64, int64, int) (int64, error)
	CountProblemSolvedTeams(context.Context, int64, int64, int64) (int64, error)
	SumTeamSolvedScore(context.Context, int64, int64, int64) (LadderRank, error)
	GetLadderByTeam(context.Context, int64, int64, int64) (LadderRank, error)
	UpsertLadder(context.Context, LadderRank) (LadderRank, error)
	RefreshContestRanks(context.Context, int64, int64) error
	ListLadder(context.Context, int64, int64, int, int) ([]LadderRank, int64, error)
	DeactivateBattleEntries(context.Context, int64, int64, int64, int64, int16) error
	NextBattleVersion(context.Context, int64, int64, int64, int64, int16) (int32, error)
	CreateBattleEntry(context.Context, BattleEntry) (BattleEntry, error)
	GetBattleEntry(context.Context, int64, int64) (BattleEntry, error)
	ListBattleEntriesForTeam(context.Context, int64, int64, int64) ([]BattleEntry, error)
	ListActiveBattleOpponents(context.Context, int64, int64, int64, int64, int64, int16, int, float64) ([]BattleEntry, error)
	CreateBattleMatch(context.Context, BattleMatch) (BattleMatch, error)
	ClaimPendingBattleMatches(context.Context, int) ([]BattleMatch, error)
	ListRunningBattleMatchesWithJudgeTask(context.Context, int) ([]BattleMatch, error)
	StartBattleMatch(context.Context, int64, int64, string, string) (BattleMatch, error)
	GetBattleMatch(context.Context, int64, int64) (BattleMatch, error)
	GetBattleMatchByJudgeTask(context.Context, int64, string) (BattleMatch, error)
	ListBattleMatchesForTeam(context.Context, int64, int64, int64, int, int) ([]BattleMatch, int64, error)
	ListActiveBattleSourceRefsForArchive(context.Context, int64, int64) ([]string, error)
	FinishBattleMatch(context.Context, BattleMatch) (BattleMatch, error)
	FailBattleMatch(context.Context, int64, int64) (BattleMatch, error)
	UpsertLadderSnapshot(context.Context, LadderSnapshot) (LadderSnapshot, error)
	GetLadderSnapshot(context.Context, int64, int64, int16) (LadderSnapshot, error)
	CreateCheatRecord(context.Context, CheatRecord) (CheatRecord, error)
	ListCheatRecords(context.Context, int64, int64, int, int) ([]CheatRecord, int64, error)
	UpsertVulnSource(context.Context, VulnSource) (VulnSource, error)
	ListVulnSources(context.Context, int64) ([]VulnSource, error)
	GetVulnSource(context.Context, int64, int64) (VulnSource, error)
	MarkVulnSourceSynced(context.Context, int64, int64) (VulnSource, error)
	UpsertVulnProblem(context.Context, VulnProblem) (VulnProblem, error)
	GetVulnProblem(context.Context, int64, int64) (VulnProblem, error)
	ListVulnProblems(context.Context, int64, int64, int16, int, int) ([]VulnProblem, int64, error)
	SetVulnProblemPrevalidate(context.Context, int64, int64, int16, map[string]any) (VulnProblem, error)
	FinalizeVulnProblem(context.Context, int64, int64, string, string) (VulnProblem, error)
	ListStudentContestRecords(context.Context, int64, int64) ([]StudentContestRecord, error)
	Stats(context.Context, int64) (ContestStatsSnapshot, error)
	ClaimAutoArchiveContests(context.Context, int) ([]Contest, error)
}

type store struct{ database *db.DB }
type txStore struct{ q *sqlcgen.Queries }

// NewStore 创建 contest 模块持久化入口,仅装配层应调用。
func NewStore(database *db.DB) Store { return &store{database: database} }

// TenantTx 在当前租户事务中执行 M8 自有表读写。
func (s *store) TenantTx(ctx context.Context, tenantID int64, fn func(context.Context, TxStore) error) error {
	if s == nil || s.database == nil {
		return fmt.Errorf("contest store 未初始化")
	}
	return s.database.WithTenantTxID(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return fn(ctx, &txStore{q: sqlcgen.New(tx)})
	})
}

// PrivilegedTx 在 contest 模块自有表内执行受控后台扫描事务。
func (s *store) PrivilegedTx(ctx context.Context, fn func(context.Context, TxStore) error) error {
	if s == nil || s.database == nil {
		return fmt.Errorf("contest store 未初始化")
	}
	return s.database.WithPrivilegedModuleTx(ctx, "contest", func(ctx context.Context, tx pgx.Tx) error {
		return fn(ctx, &txStore{q: sqlcgen.New(tx)})
	})
}

// isNoRows 统一识别未命中错误,让 service 不直接依赖 pgx。
func isNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

// CreateContest 创建竞赛草稿。
func (tx *txStore) CreateContest(ctx context.Context, item Contest) (Contest, error) {
	row, err := tx.q.CreateContest(ctx, sqlcgen.CreateContestParams{ID: item.ID, TenantID: item.TenantID, OrganizerID: item.OrganizerID, Name: item.Name, Mode: item.Mode, MatchMode: pgtypex.Int2(item.MatchMode), TeamMode: item.TeamMode, SignupStart: timex.Timestamptz(item.SignupStart), SignupEnd: timex.Timestamptz(item.SignupEnd), StartAt: timex.Timestamptz(item.StartAt), EndAt: timex.Timestamptz(item.EndAt), FreezeMinutes: item.FreezeMinutes})
	if err != nil {
		return Contest{}, apperr.ErrContestInvalid.WithCause(err)
	}
	return contestFromRow(row)
}

// GetContest 读取竞赛定义。
func (tx *txStore) GetContest(ctx context.Context, tenantID, id int64) (Contest, error) {
	row, err := tx.q.GetContest(ctx, sqlcgen.GetContestParams{TenantID: tenantID, ID: id})
	if err != nil {
		return Contest{}, apperr.ErrContestNotFound.WithCause(err)
	}
	return contestFromRow(row)
}

// ListContests 查询竞赛列表。
func (tx *txStore) ListContests(ctx context.Context, tenantID int64, status int16, page, size int) ([]Contest, int64, error) {
	rows, err := tx.q.ListContests(ctx, sqlcgen.ListContestsParams{TenantID: tenantID, Column2: status, Limit: int32(size), Offset: int32((page - 1) * size)})
	if err != nil {
		return nil, 0, apperr.ErrContestInvalid.WithCause(err)
	}
	total, err := tx.q.CountContests(ctx, sqlcgen.CountContestsParams{TenantID: tenantID, Column2: status})
	if err != nil {
		return nil, 0, apperr.ErrContestInvalid.WithCause(err)
	}
	out := make([]Contest, 0, len(rows))
	for _, row := range rows {
		item, err := contestFromRow(row)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, item)
	}
	return out, total, nil
}

// ListStudentContests 查询学生可发现的非草稿竞赛分页。
func (tx *txStore) ListStudentContests(ctx context.Context, tenantID int64, page, size int) ([]Contest, int64, error) {
	rows, err := tx.q.ListStudentContests(ctx, sqlcgen.ListStudentContestsParams{TenantID: tenantID, Limit: int32(size), Offset: int32((page - 1) * size)})
	if err != nil {
		return nil, 0, err
	}
	items := make([]Contest, 0, len(rows))
	for _, row := range rows {
		item, err := contestFromRow(row)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	total, err := tx.q.CountStudentContests(ctx, tenantID)
	return items, total, err
}

// UpdateContest 更新草稿竞赛。
func (tx *txStore) UpdateContest(ctx context.Context, item Contest) (Contest, error) {
	row, err := tx.q.UpdateContest(ctx, sqlcgen.UpdateContestParams{TenantID: item.TenantID, ID: item.ID, Name: item.Name, Mode: item.Mode, MatchMode: pgtypex.Int2(item.MatchMode), TeamMode: item.TeamMode, SignupStart: timex.Timestamptz(item.SignupStart), SignupEnd: timex.Timestamptz(item.SignupEnd), StartAt: timex.Timestamptz(item.StartAt), EndAt: timex.Timestamptz(item.EndAt), FreezeMinutes: item.FreezeMinutes})
	if err != nil {
		return Contest{}, apperr.ErrContestStateInvalid.WithCause(err)
	}
	return contestFromRow(row)
}

// SetContestStatus 更新竞赛生命周期状态。
func (tx *txStore) SetContestStatus(ctx context.Context, tenantID, id int64, status int16) (Contest, error) {
	row, err := tx.q.SetContestStatus(ctx, sqlcgen.SetContestStatusParams{TenantID: tenantID, ID: id, Status: status})
	if err != nil {
		return Contest{}, apperr.ErrContestStateInvalid.WithCause(err)
	}
	return contestFromRow(row)
}

// UpsertContestProblem 新增或更新赛题配置。
func (tx *txStore) UpsertContestProblem(ctx context.Context, item ContestProblem) (ContestProblem, error) {
	dynamic, err := encodeJSON(item.DynamicScore, apperr.ErrContestProblemInvalid)
	if err != nil {
		return ContestProblem{}, err
	}
	battleConfig, err := encodeJSON(item.BattleConfig, apperr.ErrContestProblemInvalid)
	if err != nil {
		return ContestProblem{}, err
	}
	row, err := tx.q.UpsertContestProblem(ctx, sqlcgen.UpsertContestProblemParams{ID: item.ID, TenantID: item.TenantID, ContestID: item.ContestID, ItemCode: item.ItemCode, ItemVersion: item.ItemVersion, Score: item.Score, DynamicScore: dynamic, BattleConfig: battleConfig, BattleRule: pgtypex.Int2(item.BattleRule), Seq: item.Seq})
	if err != nil {
		return ContestProblem{}, apperr.ErrContestProblemInvalid.WithCause(err)
	}
	return problemFromRow(row)
}

// GetContestProblem 读取单道赛题配置。
func (tx *txStore) GetContestProblem(ctx context.Context, tenantID, id int64) (ContestProblem, error) {
	row, err := tx.q.GetContestProblem(ctx, sqlcgen.GetContestProblemParams{TenantID: tenantID, ID: id})
	if err != nil {
		return ContestProblem{}, apperr.ErrContestProblemInvalid.WithCause(err)
	}
	return problemFromRow(row)
}

// ListContestProblems 查询竞赛题目配置。
func (tx *txStore) ListContestProblems(ctx context.Context, tenantID, contestID int64) ([]ContestProblem, error) {
	rows, err := tx.q.ListContestProblems(ctx, sqlcgen.ListContestProblemsParams{TenantID: tenantID, ContestID: contestID})
	if err != nil {
		return nil, apperr.ErrContestProblemInvalid.WithCause(err)
	}
	out := make([]ContestProblem, 0, len(rows))
	for _, row := range rows {
		item, err := problemFromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// CreateTeam 创建参赛队伍。
func (tx *txStore) CreateTeam(ctx context.Context, item Team) (Team, error) {
	row, err := tx.q.CreateTeam(ctx, sqlcgen.CreateTeamParams{ID: item.ID, TenantID: item.TenantID, ContestID: item.ContestID, Name: item.Name, InviteCode: pgtypex.Text(item.InviteCode)})
	if err != nil {
		return Team{}, apperr.ErrContestTeamInvalid.WithCause(err)
	}
	return teamFromRows(row, nil), nil
}

// GetTeam 读取队伍和成员。
func (tx *txStore) GetTeam(ctx context.Context, tenantID, id int64) (Team, error) {
	row, err := tx.q.GetTeam(ctx, sqlcgen.GetTeamParams{TenantID: tenantID, ID: id})
	if err != nil {
		return Team{}, apperr.ErrContestTeamNotFound.WithCause(err)
	}
	members, err := tx.q.ListTeamMembers(ctx, sqlcgen.ListTeamMembersParams{TenantID: tenantID, TeamID: id})
	if err != nil {
		return Team{}, apperr.ErrContestTeamInvalid.WithCause(err)
	}
	return teamFromRows(row, members), nil
}

// GetTeamByInviteCode 按邀请码读取队伍。
func (tx *txStore) GetTeamByInviteCode(ctx context.Context, tenantID int64, inviteCode string) (Team, error) {
	row, err := tx.q.GetTeamByInviteCode(ctx, sqlcgen.GetTeamByInviteCodeParams{TenantID: tenantID, InviteCode: pgtypex.Text(inviteCode)})
	if err != nil {
		return Team{}, apperr.ErrContestTeamNotFound.WithCause(err)
	}
	return teamFromRows(row, nil), nil
}

// GetTeamForAccount 读取账号在某竞赛中的队伍。
func (tx *txStore) GetTeamForAccount(ctx context.Context, tenantID, contestID, memberTenantID, accountID int64) (Team, error) {
	row, err := tx.q.GetTeamForAccount(ctx, sqlcgen.GetTeamForAccountParams{TenantID: tenantID, ContestID: contestID, MemberTenantID: memberTenantID, AccountID: accountID})
	if err != nil {
		return Team{}, err
	}
	return teamFromRows(row, nil), nil
}

// LockTeam 锁定参赛名单。
func (tx *txStore) LockTeam(ctx context.Context, tenantID, id int64) (Team, error) {
	row, err := tx.q.LockTeam(ctx, sqlcgen.LockTeamParams{TenantID: tenantID, ID: id})
	if err != nil {
		return Team{}, apperr.ErrContestTeamInvalid.WithCause(err)
	}
	return teamFromRows(row, nil), nil
}

// LockContestTeams 锁定竞赛全部组建中的队伍。
func (tx *txStore) LockContestTeams(ctx context.Context, tenantID, contestID int64) error {
	if err := tx.q.LockContestTeams(ctx, sqlcgen.LockContestTeamsParams{TenantID: tenantID, ContestID: contestID}); err != nil {
		return apperr.ErrContestTeamInvalid.WithCause(err)
	}
	return nil
}

// AddTeamMember 新增或提升队伍成员。
func (tx *txStore) AddTeamMember(ctx context.Context, item TeamMember) (TeamMember, error) {
	row, err := tx.q.AddTeamMember(ctx, sqlcgen.AddTeamMemberParams{ID: item.ID, TenantID: item.TenantID, TeamID: item.TeamID, AccountID: item.AccountID, MemberTenantID: item.MemberTenantID, IsLeader: item.IsLeader})
	if err != nil {
		return TeamMember{}, apperr.ErrContestTeamInvalid.WithCause(err)
	}
	return teamMemberFromRow(row), nil
}

// ListTeamMembers 查询队伍成员。
func (tx *txStore) ListTeamMembers(ctx context.Context, tenantID, teamID int64) ([]TeamMember, error) {
	rows, err := tx.q.ListTeamMembers(ctx, sqlcgen.ListTeamMembersParams{TenantID: tenantID, TeamID: teamID})
	if err != nil {
		return nil, apperr.ErrContestTeamInvalid.WithCause(err)
	}
	out := make([]TeamMember, 0, len(rows))
	for _, row := range rows {
		out = append(out, teamMemberFromRow(row))
	}
	return out, nil
}

// AccountTeamIDs 查询账号在竞赛中的队伍 ID。
func (tx *txStore) AccountTeamIDs(ctx context.Context, tenantID, contestID, memberTenantID, accountID int64) ([]int64, error) {
	return tx.q.AccountTeamIDs(ctx, sqlcgen.AccountTeamIDsParams{TenantID: tenantID, ContestID: contestID, MemberTenantID: memberTenantID, AccountID: accountID})
}

// CreateSolveSubmission 创建解题提交记录。
func (tx *txStore) CreateSolveSubmission(ctx context.Context, item SolveSubmission) (SolveSubmission, error) {
	content, err := encodeJSON(item.ContentRef, apperr.ErrContestSubmissionInvalid)
	if err != nil {
		return SolveSubmission{}, err
	}
	row, err := tx.q.CreateSolveSubmission(ctx, sqlcgen.CreateSolveSubmissionParams{ID: item.ID, TenantID: item.TenantID, ContestID: item.ContestID, ProblemID: item.ProblemID, TeamID: item.TeamID, SubmitterID: item.SubmitterID, ContentRef: content, SourceRef: item.SourceRef, JudgeTaskRef: pgtypex.Text(item.JudgeTaskRef), SandboxRef: pgtypex.Text(item.SandboxRef)})
	if err != nil {
		return SolveSubmission{}, apperr.ErrContestSubmissionInvalid.WithCause(err)
	}
	return submissionFromRow(row)
}

// GetSolveSubmission 读取解题提交。
func (tx *txStore) GetSolveSubmission(ctx context.Context, tenantID, id int64) (SolveSubmission, error) {
	row, err := tx.q.GetSolveSubmission(ctx, sqlcgen.GetSolveSubmissionParams{TenantID: tenantID, ID: id})
	if err != nil {
		return SolveSubmission{}, apperr.ErrContestSubmissionNotFound.WithCause(err)
	}
	return submissionFromRow(row)
}

// GetSolveSubmissionByJudgeTask 按判题任务读取解题提交。
func (tx *txStore) GetSolveSubmissionByJudgeTask(ctx context.Context, tenantID int64, judgeTaskRef string) (SolveSubmission, error) {
	row, err := tx.q.GetSolveSubmissionByJudgeTask(ctx, sqlcgen.GetSolveSubmissionByJudgeTaskParams{TenantID: tenantID, JudgeTaskRef: pgtypex.Text(judgeTaskRef)})
	if err != nil {
		return SolveSubmission{}, err
	}
	return submissionFromRow(row)
}

// UpdateSolveSubmissionResult 回写解题判题结果。
func (tx *txStore) UpdateSolveSubmissionResult(ctx context.Context, tenantID, id int64, passed bool, score int32) (SolveSubmission, error) {
	row, err := tx.q.UpdateSolveSubmissionResult(ctx, sqlcgen.UpdateSolveSubmissionResultParams{TenantID: tenantID, ID: id, Passed: passed, Score: score})
	if err != nil {
		return SolveSubmission{}, apperr.ErrContestSubmissionInvalid.WithCause(err)
	}
	return submissionFromRow(row)
}

// RecentFailedSolveCount 统计冷却期内失败提交数。
func (tx *txStore) RecentFailedSolveCount(ctx context.Context, tenantID, contestID, problemID, teamID int64, seconds int) (int64, error) {
	count, err := tx.q.RecentFailedSolveCount(ctx, sqlcgen.RecentFailedSolveCountParams{TenantID: tenantID, ContestID: contestID, ProblemID: problemID, TeamID: teamID, Column5: fmt.Sprintf("%d", seconds)})
	if err != nil {
		return 0, apperr.ErrContestSubmitRateLimited.WithCause(err)
	}
	return count, nil
}

// RecentSolveCount 统计限频窗口内全部提交数。
func (tx *txStore) RecentSolveCount(ctx context.Context, tenantID, contestID, problemID, teamID int64, seconds int) (int64, error) {
	count, err := tx.q.RecentSolveCount(ctx, sqlcgen.RecentSolveCountParams{TenantID: tenantID, ContestID: contestID, ProblemID: problemID, TeamID: teamID, Column5: fmt.Sprintf("%d", seconds)})
	if err != nil {
		return 0, apperr.ErrContestSubmitRateLimited.WithCause(err)
	}
	return count, nil
}

// CountProblemSolvedTeams 统计已经解出某题的队伍数。
func (tx *txStore) CountProblemSolvedTeams(ctx context.Context, tenantID, contestID, problemID int64) (int64, error) {
	count, err := tx.q.CountProblemSolvedTeams(ctx, sqlcgen.CountProblemSolvedTeamsParams{TenantID: tenantID, ContestID: contestID, ProblemID: problemID})
	if err != nil {
		return 0, apperr.ErrContestSubmissionInvalid.WithCause(err)
	}
	return count, nil
}

// SumTeamSolvedScore 汇总队伍解题赛最好成绩。
func (tx *txStore) SumTeamSolvedScore(ctx context.Context, tenantID, contestID, teamID int64) (LadderRank, error) {
	row, err := tx.q.SumTeamSolvedScore(ctx, sqlcgen.SumTeamSolvedScoreParams{TenantID: tenantID, ContestID: contestID, TeamID: teamID})
	if err != nil {
		return LadderRank{}, apperr.ErrContestSubmissionInvalid.WithCause(err)
	}
	return LadderRank{TenantID: tenantID, ContestID: contestID, TeamID: teamID, Score: row.Score, SolvedCount: row.SolvedCount, LastSolveAt: timex.FromTimestamptz(row.LastSolveAt)}, nil
}

// UpsertLadder 新增或更新排行榜投影。
func (tx *txStore) UpsertLadder(ctx context.Context, item LadderRank) (LadderRank, error) {
	row, err := tx.q.CreateOrUpdateLadderRank(ctx, sqlcgen.CreateOrUpdateLadderRankParams{ID: item.ID, TenantID: item.TenantID, ContestID: item.ContestID, TeamID: item.TeamID, Column5: fmt.Sprintf("%.4f", item.Score), SolvedCount: item.SolvedCount, LastSolveAt: timex.Timestamptz(item.LastSolveAt)})
	if err != nil {
		return LadderRank{}, apperr.ErrContestSubmissionInvalid.WithCause(err)
	}
	return ladderFromUpsertRow(row), nil
}

// GetLadderByTeam 读取单队当前天梯投影。
func (tx *txStore) GetLadderByTeam(ctx context.Context, tenantID, contestID, teamID int64) (LadderRank, error) {
	row, err := tx.q.GetLadderByTeam(ctx, sqlcgen.GetLadderByTeamParams{TenantID: tenantID, ContestID: contestID, TeamID: teamID})
	if err != nil {
		return LadderRank{}, err
	}
	return ladderFromGetRow(row), nil
}

// RefreshContestRanks 重算竞赛排名序号。
func (tx *txStore) RefreshContestRanks(ctx context.Context, tenantID, contestID int64) error {
	if err := tx.q.RefreshContestRanks(ctx, sqlcgen.RefreshContestRanksParams{TenantID: tenantID, ContestID: contestID}); err != nil {
		return apperr.ErrContestSubmissionInvalid.WithCause(err)
	}
	return nil
}

// ListLadder 查询排行榜。
func (tx *txStore) ListLadder(ctx context.Context, tenantID, contestID int64, page, size int) ([]LadderRank, int64, error) {
	rows, err := tx.q.ListLadder(ctx, sqlcgen.ListLadderParams{TenantID: tenantID, ContestID: contestID, Limit: int32(size), Offset: int32((page - 1) * size)})
	if err != nil {
		return nil, 0, apperr.ErrContestInvalid.WithCause(err)
	}
	total, err := tx.q.CountLadder(ctx, sqlcgen.CountLadderParams{TenantID: tenantID, ContestID: contestID})
	if err != nil {
		return nil, 0, apperr.ErrContestInvalid.WithCause(err)
	}
	out := make([]LadderRank, 0, len(rows))
	for _, row := range rows {
		out = append(out, ladderFromRow(row))
	}
	return out, total, nil
}

// DeactivateBattleEntries 停用同队同角色旧参战物。
func (tx *txStore) DeactivateBattleEntries(ctx context.Context, tenantID, contestID, problemID, teamID int64, role int16) error {
	if err := tx.q.DeactivateBattleEntries(ctx, sqlcgen.DeactivateBattleEntriesParams{TenantID: tenantID, ContestID: contestID, ProblemID: problemID, TeamID: teamID, Role: role}); err != nil {
		return apperr.ErrContestBattleEntryInvalid.WithCause(err)
	}
	return nil
}

// NextBattleVersion 计算参战物版本号。
func (tx *txStore) NextBattleVersion(ctx context.Context, tenantID, contestID, problemID, teamID int64, role int16) (int32, error) {
	v, err := tx.q.NextBattleVersion(ctx, sqlcgen.NextBattleVersionParams{TenantID: tenantID, ContestID: contestID, ProblemID: problemID, TeamID: teamID, Role: role})
	if err != nil {
		return 0, apperr.ErrContestBattleEntryInvalid.WithCause(err)
	}
	return v, nil
}

// CreateBattleEntry 创建参战物。
func (tx *txStore) CreateBattleEntry(ctx context.Context, item BattleEntry) (BattleEntry, error) {
	row, err := tx.q.CreateBattleEntry(ctx, sqlcgen.CreateBattleEntryParams{ID: item.ID, TenantID: item.TenantID, ContestID: item.ContestID, ProblemID: item.ProblemID, TeamID: item.TeamID, Role: item.Role, ArtifactRef: item.ArtifactRef, ArtifactHash: item.ArtifactHash, VersionNo: item.VersionNo})
	if err != nil {
		return BattleEntry{}, apperr.ErrContestBattleEntryInvalid.WithCause(err)
	}
	return battleEntryFromRow(row), nil
}

// GetBattleEntry 读取单个参战物。
func (tx *txStore) GetBattleEntry(ctx context.Context, tenantID, id int64) (BattleEntry, error) {
	row, err := tx.q.GetBattleEntry(ctx, sqlcgen.GetBattleEntryParams{TenantID: tenantID, ID: id})
	if err != nil {
		return BattleEntry{}, apperr.ErrContestBattleEntryInvalid.WithCause(err)
	}
	return battleEntryFromRow(row), nil
}

// ListBattleEntriesForTeam 查询队伍参战物。
func (tx *txStore) ListBattleEntriesForTeam(ctx context.Context, tenantID, contestID, teamID int64) ([]BattleEntry, error) {
	rows, err := tx.q.ListBattleEntriesForTeam(ctx, sqlcgen.ListBattleEntriesForTeamParams{TenantID: tenantID, ContestID: contestID, TeamID: teamID})
	if err != nil {
		return nil, apperr.ErrContestBattleEntryInvalid.WithCause(err)
	}
	out := make([]BattleEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, battleEntryFromRow(row))
	}
	return out, nil
}

// ListActiveBattleOpponents 查询可撮合的活跃对手。
func (tx *txStore) ListActiveBattleOpponents(ctx context.Context, tenantID, contestID, problemID, excludeEntryID, excludeTeamID int64, matchMode int16, limit int, initialScore float64) ([]BattleEntry, error) {
	initialScoreValue, err := pgtypex.NumericScale(initialScore, 2)
	if err != nil {
		return nil, apperr.ErrContestBattleEntryInvalid.WithCause(err)
	}
	rows, err := tx.q.ListActiveBattleOpponents(ctx, sqlcgen.ListActiveBattleOpponentsParams{TenantID: tenantID, ContestID: contestID, ProblemID: problemID, ID: excludeEntryID, TeamID: excludeTeamID, Column6: matchMode, Limit: int32(limit), Column8: initialScoreValue})
	if err != nil {
		return nil, apperr.ErrContestBattleEntryInvalid.WithCause(err)
	}
	out := make([]BattleEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, battleEntryFromRow(row))
	}
	return out, nil
}

// CreateBattleMatch 创建待执行对局。
func (tx *txStore) CreateBattleMatch(ctx context.Context, item BattleMatch) (BattleMatch, error) {
	row, err := tx.q.CreateBattleMatch(ctx, sqlcgen.CreateBattleMatchParams{ID: item.ID, TenantID: item.TenantID, ContestID: item.ContestID, ProblemID: item.ProblemID, EntryAID: item.EntryAID, EntryBID: item.EntryBID, SourceRef: item.SourceRef})
	if err != nil {
		return BattleMatch{}, apperr.ErrContestBattleMatchFailed.WithCause(err)
	}
	return battleMatchFromRow(row)
}

// ClaimPendingBattleMatches 跨租户认领待执行对局。
func (tx *txStore) ClaimPendingBattleMatches(ctx context.Context, limit int) ([]BattleMatch, error) {
	rows, err := tx.q.ClaimPendingBattleMatchesAcrossTenants(ctx, int32(limit))
	if err != nil {
		return nil, apperr.ErrContestBattleMatchFailed.WithCause(err)
	}
	out := make([]BattleMatch, 0, len(rows))
	for _, row := range rows {
		item, err := battleMatchFromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// ListRunningBattleMatchesWithJudgeTask 查询已启动但尚未结算的对局,用于补偿死信或短暂消费失败的判题完成事件。
func (tx *txStore) ListRunningBattleMatchesWithJudgeTask(ctx context.Context, limit int) ([]BattleMatch, error) {
	rows, err := tx.q.ListRunningBattleMatchesWithJudgeTask(ctx, int32(limit))
	if err != nil {
		return nil, apperr.ErrContestBattleMatchFailed.WithCause(err)
	}
	out := make([]BattleMatch, 0, len(rows))
	for _, row := range rows {
		item, err := battleMatchFromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// StartBattleMatch 保存对局沙箱和判题任务引用。
func (tx *txStore) StartBattleMatch(ctx context.Context, tenantID, id int64, sandboxRef, judgeTaskRef string) (BattleMatch, error) {
	row, err := tx.q.StartBattleMatch(ctx, sqlcgen.StartBattleMatchParams{TenantID: tenantID, ID: id, SandboxRef: pgtypex.Text(sandboxRef), JudgeTaskRef: pgtypex.Text(judgeTaskRef)})
	if err != nil {
		return BattleMatch{}, apperr.ErrContestBattleMatchFailed.WithCause(err)
	}
	return battleMatchFromRow(row)
}

// GetBattleMatch 读取对局。
func (tx *txStore) GetBattleMatch(ctx context.Context, tenantID, id int64) (BattleMatch, error) {
	row, err := tx.q.GetBattleMatch(ctx, sqlcgen.GetBattleMatchParams{TenantID: tenantID, ID: id})
	if err != nil {
		return BattleMatch{}, apperr.ErrContestBattleMatchNotFound.WithCause(err)
	}
	return battleMatchFromRow(row)
}

// GetBattleMatchByJudgeTask 按判题任务读取对局。
func (tx *txStore) GetBattleMatchByJudgeTask(ctx context.Context, tenantID int64, judgeTaskRef string) (BattleMatch, error) {
	row, err := tx.q.GetBattleMatchByJudgeTask(ctx, sqlcgen.GetBattleMatchByJudgeTaskParams{TenantID: tenantID, JudgeTaskRef: pgtypex.Text(judgeTaskRef)})
	if err != nil {
		return BattleMatch{}, err
	}
	return battleMatchFromRow(row)
}

// ListBattleMatchesForTeam 查询队伍对局历史和总数。
func (tx *txStore) ListBattleMatchesForTeam(ctx context.Context, tenantID, contestID, teamID int64, page, size int) ([]BattleMatch, int64, error) {
	rows, err := tx.q.ListBattleMatchesForTeam(ctx, sqlcgen.ListBattleMatchesForTeamParams{TenantID: tenantID, ContestID: contestID, TeamID: teamID, Limit: int32(size), Offset: int32((page - 1) * size)})
	if err != nil {
		return nil, 0, apperr.ErrContestBattleMatchFailed.WithCause(err)
	}
	out := make([]BattleMatch, 0, len(rows))
	for _, row := range rows {
		item, err := battleMatchFromRow(row)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, item)
	}
	total, err := tx.q.CountBattleMatchesForTeam(ctx, sqlcgen.CountBattleMatchesForTeamParams{TenantID: tenantID, ContestID: contestID, TeamID: teamID})
	if err != nil {
		return nil, 0, apperr.ErrContestBattleMatchFailed.WithCause(err)
	}
	return out, total, nil
}

// ListActiveBattleSourceRefsForArchive 查询归档时仍需回收的对抗对局沙箱来源。
func (tx *txStore) ListActiveBattleSourceRefsForArchive(ctx context.Context, tenantID, contestID int64) ([]string, error) {
	refs, err := tx.q.ListActiveBattleSourceRefsForArchive(ctx, sqlcgen.ListActiveBattleSourceRefsForArchiveParams{TenantID: tenantID, ContestID: contestID})
	if err != nil {
		return nil, apperr.ErrContestBattleMatchFailed.WithCause(err)
	}
	return refs, nil
}

// FinishBattleMatch 保存对局终态结果。
func (tx *txStore) FinishBattleMatch(ctx context.Context, item BattleMatch) (BattleMatch, error) {
	delta, err := encodeJSON(item.ScoreDelta, apperr.ErrContestBattleMatchFailed)
	if err != nil {
		return BattleMatch{}, err
	}
	replay, err := encodeJSON(item.Replay, apperr.ErrContestBattleMatchFailed)
	if err != nil {
		return BattleMatch{}, err
	}
	row, err := tx.q.FinishBattleMatch(ctx, sqlcgen.FinishBattleMatchParams{TenantID: item.TenantID, ID: item.ID, SandboxRef: pgtypex.Text(item.SandboxRef), JudgeTaskRef: pgtypex.Text(item.JudgeTaskRef), Result: pgtypex.Int2(item.Result), ScoreDelta: delta, ReplayData: replay})
	if err != nil {
		return BattleMatch{}, apperr.ErrContestBattleMatchFailed.WithCause(err)
	}
	return battleMatchFromRow(row)
}

// FailBattleMatch 标记对局失败终态。
func (tx *txStore) FailBattleMatch(ctx context.Context, tenantID, id int64) (BattleMatch, error) {
	row, err := tx.q.FailBattleMatch(ctx, sqlcgen.FailBattleMatchParams{TenantID: tenantID, ID: id})
	if err != nil {
		return BattleMatch{}, apperr.ErrContestBattleMatchFailed.WithCause(err)
	}
	return battleMatchFromRow(row)
}

// UpsertLadderSnapshot 保存封榜或归档阶段的权威榜单快照。
func (tx *txStore) UpsertLadderSnapshot(ctx context.Context, item LadderSnapshot) (LadderSnapshot, error) {
	raw, err := encodeJSON(item.Ranking, apperr.ErrContestInvalid)
	if err != nil {
		return LadderSnapshot{}, err
	}
	row, err := tx.q.UpsertLadderSnapshot(ctx, sqlcgen.UpsertLadderSnapshotParams{ID: item.ID, TenantID: item.TenantID, ContestID: item.ContestID, SnapshotStatus: item.SnapshotStatus, Ranking: raw})
	if err != nil {
		return LadderSnapshot{}, apperr.ErrContestInvalid.WithCause(err)
	}
	return ladderSnapshotFromRow(row)
}

// GetLadderSnapshot 按竞赛状态读取封榜或归档榜单快照。
func (tx *txStore) GetLadderSnapshot(ctx context.Context, tenantID, contestID int64, snapshotStatus int16) (LadderSnapshot, error) {
	row, err := tx.q.GetLadderSnapshot(ctx, sqlcgen.GetLadderSnapshotParams{TenantID: tenantID, ContestID: contestID, SnapshotStatus: snapshotStatus})
	if err != nil {
		return LadderSnapshot{}, apperr.ErrContestNotFound.WithCause(err)
	}
	return ladderSnapshotFromRow(row)
}

// CreateCheatRecord 创建违规处理记录。
func (tx *txStore) CreateCheatRecord(ctx context.Context, item CheatRecord) (CheatRecord, error) {
	evidence, err := encodeJSON(item.Evidence, apperr.ErrContestCheatInvalid)
	if err != nil {
		return CheatRecord{}, err
	}
	row, err := tx.q.CreateCheatRecord(ctx, sqlcgen.CreateCheatRecordParams{ID: item.ID, TenantID: item.TenantID, ContestID: item.ContestID, TeamID: item.TeamID, Type: item.Type, Evidence: evidence, Action: item.Action, OperatorID: pgtypex.Int8(item.OperatorID)})
	if err != nil {
		return CheatRecord{}, apperr.ErrContestCheatInvalid.WithCause(err)
	}
	return cheatFromRow(row)
}

// ListCheatRecords 查询违规记录和总数。
func (tx *txStore) ListCheatRecords(ctx context.Context, tenantID, contestID int64, page, size int) ([]CheatRecord, int64, error) {
	rows, err := tx.q.ListCheatRecords(ctx, sqlcgen.ListCheatRecordsParams{TenantID: tenantID, ContestID: contestID, Limit: int32(size), Offset: int32((page - 1) * size)})
	if err != nil {
		return nil, 0, apperr.ErrContestCheatInvalid.WithCause(err)
	}
	out := make([]CheatRecord, 0, len(rows))
	for _, row := range rows {
		item, err := cheatFromRow(row)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, item)
	}
	total, err := tx.q.CountCheatRecords(ctx, sqlcgen.CountCheatRecordsParams{TenantID: tenantID, ContestID: contestID})
	if err != nil {
		return nil, 0, apperr.ErrContestCheatInvalid.WithCause(err)
	}
	return out, total, nil
}

// UpsertVulnSource 新增或更新租户漏洞源配置。
func (tx *txStore) UpsertVulnSource(ctx context.Context, item VulnSource) (VulnSource, error) {
	cfg, err := encodeJSON(item.Config, apperr.ErrContestVulnSourceInvalid)
	if err != nil {
		return VulnSource{}, err
	}
	row, err := tx.q.UpsertVulnSource(ctx, sqlcgen.UpsertVulnSourceParams{ID: item.ID, TenantID: pgtypex.Int8(item.TenantID), Type: item.Type, Name: item.Name, Config: cfg, DefaultLevel: item.DefaultLevel, Enabled: item.Enabled})
	if err != nil {
		return VulnSource{}, apperr.ErrContestVulnSourceInvalid.WithCause(err)
	}
	return vulnSourceFromRow(row)
}

// ListVulnSources 查询平台源和本租户源。
func (tx *txStore) ListVulnSources(ctx context.Context, tenantID int64) ([]VulnSource, error) {
	rows, err := tx.q.ListVulnSources(ctx, pgtypex.Int8(tenantID))
	if err != nil {
		return nil, apperr.ErrContestVulnSourceInvalid.WithCause(err)
	}
	out := make([]VulnSource, 0, len(rows))
	for _, row := range rows {
		item, err := vulnSourceFromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// GetVulnSource 读取漏洞源。
func (tx *txStore) GetVulnSource(ctx context.Context, tenantID, id int64) (VulnSource, error) {
	row, err := tx.q.GetVulnSource(ctx, sqlcgen.GetVulnSourceParams{TenantID: pgtypex.Int8(tenantID), ID: id})
	if err != nil {
		return VulnSource{}, apperr.ErrContestVulnSourceInvalid.WithCause(err)
	}
	return vulnSourceFromRow(row)
}

// MarkVulnSourceSynced 更新时间同步标记。
func (tx *txStore) MarkVulnSourceSynced(ctx context.Context, tenantID, id int64) (VulnSource, error) {
	row, err := tx.q.MarkVulnSourceSynced(ctx, sqlcgen.MarkVulnSourceSyncedParams{TenantID: pgtypex.Int8(tenantID), ID: id})
	if err != nil {
		return VulnSource{}, apperr.ErrContestVulnSourceSyncMarkFailed.WithCause(err)
	}
	return vulnSourceFromRow(row)
}

// UpsertVulnProblem 新增或更新漏洞题草稿。
func (tx *txStore) UpsertVulnProblem(ctx context.Context, item VulnProblem) (VulnProblem, error) {
	body, err := encodeJSON(item.DraftBody, apperr.ErrContestVulnProblemInvalid)
	if err != nil {
		return VulnProblem{}, err
	}
	row, err := tx.q.UpsertVulnProblem(ctx, sqlcgen.UpsertVulnProblemParams{ID: item.ID, TenantID: item.TenantID, SourceID: pgtypex.Int8(item.SourceID), ExternalRef: pgtypex.Text(item.ExternalRef), Title: item.Title, Level: item.Level, RuntimeMode: item.RuntimeMode, DraftBody: body})
	if err != nil {
		return VulnProblem{}, apperr.ErrContestVulnProblemInvalid.WithCause(err)
	}
	return vulnProblemFromRow(row)
}

// GetVulnProblem 读取漏洞题草稿。
func (tx *txStore) GetVulnProblem(ctx context.Context, tenantID, id int64) (VulnProblem, error) {
	row, err := tx.q.GetVulnProblem(ctx, sqlcgen.GetVulnProblemParams{TenantID: tenantID, ID: id})
	if err != nil {
		return VulnProblem{}, apperr.ErrContestVulnProblemInvalid.WithCause(err)
	}
	return vulnProblemFromRow(row)
}

// ListVulnProblems 查询漏洞题草稿和总数。
func (tx *txStore) ListVulnProblems(ctx context.Context, tenantID, sourceID int64, status int16, page, size int) ([]VulnProblem, int64, error) {
	rows, err := tx.q.ListVulnProblems(ctx, sqlcgen.ListVulnProblemsParams{TenantID: tenantID, Column2: sourceID, Column3: status, Limit: int32(size), Offset: int32((page - 1) * size)})
	if err != nil {
		return nil, 0, apperr.ErrContestVulnProblemInvalid.WithCause(err)
	}
	out := make([]VulnProblem, 0, len(rows))
	for _, row := range rows {
		item, err := vulnProblemFromRow(row)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, item)
	}
	total, err := tx.q.CountVulnProblems(ctx, sqlcgen.CountVulnProblemsParams{TenantID: tenantID, Column2: sourceID, Column3: status})
	if err != nil {
		return nil, 0, apperr.ErrContestVulnProblemInvalid.WithCause(err)
	}
	return out, total, nil
}

// SetVulnProblemPrevalidate 保存预验证结论。
func (tx *txStore) SetVulnProblemPrevalidate(ctx context.Context, tenantID, id int64, status int16, detail map[string]any) (VulnProblem, error) {
	raw, err := encodeJSON(detail, apperr.ErrContestVulnProblemInvalid)
	if err != nil {
		return VulnProblem{}, err
	}
	row, err := tx.q.SetVulnProblemPrevalidate(ctx, sqlcgen.SetVulnProblemPrevalidateParams{TenantID: tenantID, ID: id, PrevalidateStatus: status, PrevalidateDetail: raw})
	if err != nil {
		return VulnProblem{}, apperr.ErrContestVulnPrevalidateFailed.WithCause(err)
	}
	return vulnProblemFromRow(row)
}

// FinalizeVulnProblem 保存漏洞题固化后的 M5 内容引用。
func (tx *txStore) FinalizeVulnProblem(ctx context.Context, tenantID, id int64, code, version string) (VulnProblem, error) {
	row, err := tx.q.FinalizeVulnProblem(ctx, sqlcgen.FinalizeVulnProblemParams{TenantID: tenantID, ID: id, ContentItemCode: pgtypex.Text(code), ContentItemVersion: pgtypex.Text(version)})
	if err != nil {
		return VulnProblem{}, apperr.ErrContestVulnFinalizeFailed.WithCause(err)
	}
	return vulnProblemFromRow(row)
}

// ListStudentContestRecords 查询学生竞赛战绩。
func (tx *txStore) ListStudentContestRecords(ctx context.Context, tenantID, accountID int64) ([]StudentContestRecord, error) {
	rows, err := tx.q.ListStudentContestRecords(ctx, sqlcgen.ListStudentContestRecordsParams{TenantID: tenantID, AccountID: accountID})
	if err != nil {
		return nil, apperr.ErrContestInvalid.WithCause(err)
	}
	out := make([]StudentContestRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, recordFromRow(row))
	}
	return out, nil
}

// Stats 返回租户维度竞赛统计。
func (tx *txStore) Stats(ctx context.Context, tenantID int64) (ContestStatsSnapshot, error) {
	row, err := tx.q.ContestStats(ctx, tenantID)
	if err != nil {
		return ContestStatsSnapshot{}, apperr.ErrContestInvalid.WithCause(err)
	}
	return ContestStatsSnapshot{ContestCount: row.ContestCount, ActiveContestCount: row.ActiveContestCount, ParticipantCount: row.ParticipantCount}, nil
}

// ClaimAutoArchiveContests 跨租户认领已到结束时间的竞赛并标记为已结束。
func (tx *txStore) ClaimAutoArchiveContests(ctx context.Context, limit int) ([]Contest, error) {
	rows, err := tx.q.ClaimAutoArchiveContestsAcrossTenants(ctx, int32(limit))
	if err != nil {
		return nil, apperr.ErrContestStateInvalid.WithCause(err)
	}
	out := make([]Contest, 0, len(rows))
	for _, row := range rows {
		item, err := contestFromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}
