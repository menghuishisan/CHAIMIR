// contest repo 文件定义 M8 持久化接口和事务边界,只操作竞赛模块自有表。
package contest

import (
	"context"
	"fmt"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/contest/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/intx"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pagex"
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
	FindPublishedContestTenant(context.Context, int64) (int64, error)
	FindTeamTenant(context.Context, int64) (int64, error)
	FindBattleMatchTenant(context.Context, int64) (int64, error)
	ListContests(context.Context, int64, int16, int, int) ([]Contest, int64, error)
	ListStudentContests(context.Context, int64, int16, int, int) ([]Contest, int64, error)
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
	GetContestAccessGrant(context.Context, int64, int64) (ContestAccessGrant, error)
	GetContestAccessGrantForSubject(context.Context, int64, int64, int64, int64) (ContestAccessGrant, error)
	FindContestAccessGrantForSubject(context.Context, int64, int64, int64) (ContestAccessGrant, error)
	UpsertContestAccessGrant(context.Context, ContestAccessGrant) (ContestAccessGrant, error)
	RevokeContestAccessGrantsForSandbox(context.Context, int64, int64) error
	ListContestAccessGrantsForSandbox(context.Context, int64, int64) ([]ContestAccessGrant, error)
	ListContestAccessGrantsForContest(context.Context, int64, int64) ([]ContestAccessGrant, error)
	CreateSolveSubmission(context.Context, SolveSubmission) (SolveSubmission, error)
	GetSolveSubmission(context.Context, int64, int64) (SolveSubmission, error)
	FindSolveSubmissionTenant(context.Context, int64) (int64, error)
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
	ListBattleEntriesForTeam(context.Context, int64, int64, int64, int, int) ([]BattleEntry, int64, error)
	ListActiveBattleOpponents(context.Context, int64, int64, int64, int64, int64, int16, int, float64) ([]BattleEntry, error)
	CreateBattleMatch(context.Context, BattleMatch) (BattleMatch, error)
	ExhaustUnstartedBattleMatches(context.Context, int32, time.Time) ([]BattleMatch, error)
	ClaimPendingBattleMatches(context.Context, int, int32, time.Time, time.Time, string) ([]BattleMatch, error)
	ListRunningBattleMatchesWithJudgeTask(context.Context, int) ([]BattleMatch, error)
	StartBattleMatch(context.Context, int64, int64, string, string, string) (BattleMatch, bool, error)
	RenewBattleMatchStartLease(context.Context, int64, int64, string, time.Time) (bool, error)
	GetBattleMatch(context.Context, int64, int64) (BattleMatch, error)
	GetBattleMatchByJudgeTask(context.Context, int64, string) (BattleMatch, error)
	ListBattleMatchesForTeam(context.Context, int64, int64, int64, int, int) ([]BattleMatch, int64, error)
	ListBattleReplayMatchesForTeam(context.Context, int64, int64, int64, int, int) ([]BattleReplayRow, error)
	CountBattleReplayPendingForTeam(context.Context, int64, int64, int64) (int64, error)
	CountBattleReplayCompletedForTeam(context.Context, int64, int64, int64) (int64, error)
	GetBattleReplayCheckpointForTeam(context.Context, int64, int64, int64, int, int) (BattleReplayCheckpoint, error)
	ListActiveBattleSourceRefsForArchive(context.Context, int64, int64) ([]string, error)
	FinishBattleMatch(context.Context, BattleMatch) (BattleMatch, error)
	FailBattleMatchStart(context.Context, int64, int64, string) (BattleMatch, bool, error)
	FailBattleMatchByJudgeTask(context.Context, int64, int64, string) (BattleMatch, error)
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
	ListVulnProblems(context.Context, int64, int64, int16, int16, int, int) ([]VulnProblem, int64, error)
	SetVulnProblemPrevalidate(context.Context, int64, int64, int16, map[string]any, string, *contracts.SandboxCompositionSnapshot, string, string) (VulnProblem, error)
	FinalizeVulnProblem(context.Context, int64, int64, string, string) (VulnProblem, error)
	ListStudentContestRecords(context.Context, int64, int64) ([]StudentContestRecord, error)
	Stats(context.Context, int64) (ContestStatsSnapshot, error)
	ClaimManualArchiveContest(context.Context, int64, int64, time.Time, time.Time, string) (ContestArchiveClaim, error)
	ClaimAutoArchiveContests(context.Context, int, time.Time, time.Time, string) ([]ContestArchiveClaim, error)
	CompleteAutoArchiveContest(context.Context, int64, int64, string) (int64, error)
}

// BattleReplayRow 是回放时间窗查询的内部行,包含服务端计算所需的队伍视角。
type BattleReplayRow struct {
	Match              BattleMatch
	SequenceNo         int64
	MySide             string
	ActiveEntryID      int64
	ActiveEntryRole    int16
	ActiveEntryVersion int32
	ActiveEntryAt      time.Time
}

// BattleReplayCheckpoint 是窗口之前的服务端聚合状态。
type BattleReplayCheckpoint struct {
	Wins        int32
	Losses      int32
	Draws       int32
	RatingDelta float64
	Rating      float64
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
func isNoRows(err error) bool { return db.IsNoRows(err) }

// CreateContest 创建竞赛草稿。
func (tx *txStore) CreateContest(ctx context.Context, item Contest) (Contest, error) {
	rules, err := jsonx.AnyBytes(item.Rules, apperr.ErrContestInvalid)
	if err != nil {
		return Contest{}, err
	}
	row, err := tx.q.CreateContest(ctx, sqlcgen.CreateContestParams{ID: item.ID, TenantID: item.TenantID, OrganizerID: item.OrganizerID, Name: item.Name, Mode: item.Mode, MatchMode: pgtypex.Int2(item.MatchMode), TeamMode: item.TeamMode, SignupStart: timex.Timestamptz(item.SignupStart), SignupEnd: timex.Timestamptz(item.SignupEnd), StartAt: timex.Timestamptz(item.StartAt), EndAt: timex.Timestamptz(item.EndAt), FreezeMinutes: item.FreezeMinutes, Rules: rules})
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

// FindPublishedContestTenant 在受控特权事务中解析公开竞赛所属租户,不暴露跨租户读写能力。
func (tx *txStore) FindPublishedContestTenant(ctx context.Context, contestID int64) (int64, error) {
	row, err := tx.q.FindPublishedContestTenant(ctx, contestID)
	if err != nil {
		return 0, apperr.ErrContestNotFound.WithCause(err)
	}
	return row, nil
}

// FindTeamTenant 在受控特权事务中解析队伍所属租户。
func (tx *txStore) FindTeamTenant(ctx context.Context, teamID int64) (int64, error) {
	row, err := tx.q.FindTeamTenant(ctx, teamID)
	if err != nil {
		return 0, apperr.ErrContestTeamNotFound.WithCause(err)
	}
	return row, nil
}

// FindBattleMatchTenant 在受控特权事务中解析对局所属组织租户。
func (tx *txStore) FindBattleMatchTenant(ctx context.Context, matchID int64) (int64, error) {
	row, err := tx.q.FindBattleMatchTenant(ctx, matchID)
	if err != nil {
		return 0, apperr.ErrContestBattleMatchNotFound.WithCause(err)
	}
	return row, nil
}

// ListContests 查询竞赛列表。
func (tx *txStore) ListContests(ctx context.Context, tenantID int64, status int16, page, size int) ([]Contest, int64, error) {
	limit, offset := pagex.LimitOffset(page, size)
	rows, err := tx.q.ListContests(ctx, sqlcgen.ListContestsParams{TenantID: tenantID, Column2: status, Limit: limit, Offset: offset})
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
// status 传 0 表示不按状态过滤;总数按同一条件计,与列表同口径。
func (tx *txStore) ListStudentContests(ctx context.Context, tenantID int64, status int16, page, size int) ([]Contest, int64, error) {
	limit, offset := pagex.LimitOffset(page, size)
	rows, err := tx.q.ListStudentContests(ctx, sqlcgen.ListStudentContestsParams{TenantID: tenantID, Status: status, PageLimit: limit, PageOffset: offset})
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
	total, err := tx.q.CountStudentContests(ctx, sqlcgen.CountStudentContestsParams{TenantID: tenantID, Status: status})
	return items, total, err
}

// UpdateContest 更新草稿竞赛。
func (tx *txStore) UpdateContest(ctx context.Context, item Contest) (Contest, error) {
	rules, err := jsonx.AnyBytes(item.Rules, apperr.ErrContestInvalid)
	if err != nil {
		return Contest{}, err
	}
	row, err := tx.q.UpdateContest(ctx, sqlcgen.UpdateContestParams{TenantID: item.TenantID, ID: item.ID, Name: item.Name, Mode: item.Mode, MatchMode: pgtypex.Int2(item.MatchMode), TeamMode: item.TeamMode, SignupStart: timex.Timestamptz(item.SignupStart), SignupEnd: timex.Timestamptz(item.SignupEnd), StartAt: timex.Timestamptz(item.StartAt), EndAt: timex.Timestamptz(item.EndAt), FreezeMinutes: item.FreezeMinutes, Rules: rules})
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
	dynamic, err := encodeOptionalJSON(item.DynamicScore, apperr.ErrContestProblemInvalid)
	if err != nil {
		return ContestProblem{}, err
	}
	battleConfig, err := encodeOptionalJSON(item.BattleConfig, apperr.ErrContestProblemInvalid)
	if err != nil {
		return ContestProblem{}, err
	}
	snapshot, err := encodeOptionalJSON(item.CompositionSnapshot, apperr.ErrContestProblemInvalid)
	if err != nil {
		return ContestProblem{}, err
	}
	row, err := tx.q.UpsertContestProblem(ctx, sqlcgen.UpsertContestProblemParams{ID: item.ID, TenantID: item.TenantID, ContestID: item.ContestID, ItemCode: item.ItemCode, ItemVersion: item.ItemVersion, Score: item.Score, DynamicScore: dynamic, BattleConfig: battleConfig, BattleRule: pgtypex.Int2(item.BattleRule), Seq: item.Seq, CompositionDigest: pgtypex.Text(item.CompositionDigest), CompositionSnapshot: snapshot})
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

// GetContestAccessGrant 读取单条跨校竞赛授权。
func (tx *txStore) GetContestAccessGrant(ctx context.Context, tenantID, id int64) (ContestAccessGrant, error) {
	row, err := tx.q.GetContestAccessGrant(ctx, sqlcgen.GetContestAccessGrantParams{TenantID: tenantID, ID: id})
	if err != nil {
		return ContestAccessGrant{}, apperr.ErrContestTeamAccessDenied.WithCause(err)
	}
	return contestAccessGrantFromRow(row)
}

// GetContestAccessGrantForSubject 按沙箱与受权主体读取跨校授权。
func (tx *txStore) GetContestAccessGrantForSubject(ctx context.Context, tenantID, sandboxID, memberTenantID, memberAccountID int64) (ContestAccessGrant, error) {
	row, err := tx.q.GetContestAccessGrantForSubject(ctx, sqlcgen.GetContestAccessGrantForSubjectParams{TenantID: tenantID, SandboxID: sandboxID, MemberTenantID: memberTenantID, MemberAccountID: memberAccountID})
	if err != nil {
		return ContestAccessGrant{}, apperr.ErrContestTeamAccessDenied.WithCause(err)
	}
	return contestAccessGrantFromRow(row)
}

// FindContestAccessGrantForSubject 在受控特权事务中按成员身份查找唯一的跨校沙箱授权。
func (tx *txStore) FindContestAccessGrantForSubject(ctx context.Context, sandboxID, memberTenantID, memberAccountID int64) (ContestAccessGrant, error) {
	row, err := tx.q.FindContestAccessGrantForSubject(ctx, sqlcgen.FindContestAccessGrantForSubjectParams{SandboxID: sandboxID, MemberTenantID: memberTenantID, MemberAccountID: memberAccountID})
	if err != nil {
		return ContestAccessGrant{}, apperr.ErrContestTeamAccessDenied.WithCause(err)
	}
	return contestAccessGrantFromRow(row)
}

// UpsertContestAccessGrant 签发或刷新一个沙箱范围的跨校授权。
func (tx *txStore) UpsertContestAccessGrant(ctx context.Context, item ContestAccessGrant) (ContestAccessGrant, error) {
	capabilities, err := jsonx.AnyBytes(item.Capabilities, apperr.ErrContestTeamAccessDenied)
	if err != nil {
		return ContestAccessGrant{}, err
	}
	row, err := tx.q.UpsertContestAccessGrant(ctx, sqlcgen.UpsertContestAccessGrantParams{ID: item.ID, TenantID: item.TenantID, ContestID: item.ContestID, TeamID: item.TeamID, SandboxID: item.SandboxID, MemberTenantID: item.MemberTenantID, MemberAccountID: item.MemberAccountID, Capabilities: capabilities, SourceRef: item.SourceRef, ExpiresAt: timex.Timestamptz(item.ExpiresAt)})
	if err != nil {
		return ContestAccessGrant{}, apperr.ErrContestTeamAccessDenied.WithCause(err)
	}
	return contestAccessGrantFromRow(row)
}

// RevokeContestAccessGrantsForSandbox 使指定沙箱的全部跨校授权立即失效。
func (tx *txStore) RevokeContestAccessGrantsForSandbox(ctx context.Context, tenantID, sandboxID int64) error {
	if err := tx.q.RevokeContestAccessGrantsForSandbox(ctx, sqlcgen.RevokeContestAccessGrantsForSandboxParams{TenantID: tenantID, SandboxID: sandboxID}); err != nil {
		return apperr.ErrContestTeamAccessDenied.WithCause(err)
	}
	return nil
}

// ListContestAccessGrantsForSandbox 列出沙箱当前和历史授权,供撤销通知与审计使用。
func (tx *txStore) ListContestAccessGrantsForSandbox(ctx context.Context, tenantID, sandboxID int64) ([]ContestAccessGrant, error) {
	rows, err := tx.q.ListContestAccessGrantsForSandbox(ctx, sqlcgen.ListContestAccessGrantsForSandboxParams{TenantID: tenantID, SandboxID: sandboxID})
	if err != nil {
		return nil, apperr.ErrContestTeamAccessDenied.WithCause(err)
	}
	out := make([]ContestAccessGrant, 0, len(rows))
	for _, row := range rows {
		item, err := contestAccessGrantFromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// ListContestAccessGrantsForContest 读取竞赛仍有效的全部沙箱授权，供归档时一次性撤销。
func (tx *txStore) ListContestAccessGrantsForContest(ctx context.Context, tenantID, contestID int64) ([]ContestAccessGrant, error) {
	rows, err := tx.q.ListContestAccessGrantsForContest(ctx, sqlcgen.ListContestAccessGrantsForContestParams{TenantID: tenantID, ContestID: contestID})
	if err != nil {
		return nil, apperr.ErrContestTeamAccessDenied.WithCause(err)
	}
	out := make([]ContestAccessGrant, 0, len(rows))
	for _, row := range rows {
		item, err := contestAccessGrantFromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// CreateSolveSubmission 创建解题提交记录。
func (tx *txStore) CreateSolveSubmission(ctx context.Context, item SolveSubmission) (SolveSubmission, error) {
	content, err := jsonx.AnyBytes(item.ContentRef, apperr.ErrContestSubmissionInvalid)
	if err != nil {
		return SolveSubmission{}, err
	}
	row, err := tx.q.CreateSolveSubmission(ctx, sqlcgen.CreateSolveSubmissionParams{ID: item.ID, TenantID: item.TenantID, ContestID: item.ContestID, ProblemID: item.ProblemID, TeamID: item.TeamID, SubmitterTenantID: item.SubmitterTenantID, SubmitterID: item.SubmitterID, ContentRef: content, SourceRef: item.SourceRef, JudgeTaskRef: pgtypex.Text(item.JudgeTaskRef), SandboxRef: pgtypex.Text(item.SandboxRef)})
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

// FindSolveSubmissionTenant 解析提交所属竞赛租户,供跨校学生读取提交。
func (tx *txStore) FindSolveSubmissionTenant(ctx context.Context, id int64) (int64, error) {
	row, err := tx.q.FindSolveSubmissionTenant(ctx, id)
	if err != nil {
		return 0, apperr.ErrContestSubmissionNotFound.WithCause(err)
	}
	return row, nil
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
	limit, offset := pagex.LimitOffset(page, size)
	rows, err := tx.q.ListLadder(ctx, sqlcgen.ListLadderParams{TenantID: tenantID, ContestID: contestID, Limit: limit, Offset: offset})
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
func (tx *txStore) ListBattleEntriesForTeam(ctx context.Context, tenantID, contestID, teamID int64, page, size int) ([]BattleEntry, int64, error) {
	limit, offset := pagex.LimitOffset(page, size)
	rows, err := tx.q.ListBattleEntriesForTeam(ctx, sqlcgen.ListBattleEntriesForTeamParams{TenantID: tenantID, ContestID: contestID, TeamID: teamID, PageLimit: limit, PageOffset: offset})
	if err != nil {
		return nil, 0, apperr.ErrContestBattleEntryInvalid.WithCause(err)
	}
	total, err := tx.q.CountBattleEntriesForTeam(ctx, sqlcgen.CountBattleEntriesForTeamParams{TenantID: tenantID, ContestID: contestID, TeamID: teamID})
	if err != nil {
		return nil, 0, apperr.ErrContestBattleEntryInvalid.WithCause(err)
	}
	out := make([]BattleEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, battleEntryFromRow(row))
	}
	return out, total, nil
}

// ListActiveBattleOpponents 查询可撮合的活跃对手。
func (tx *txStore) ListActiveBattleOpponents(ctx context.Context, tenantID, contestID, problemID, excludeEntryID, excludeTeamID int64, matchMode int16, limit int, initialScore float64) ([]BattleEntry, error) {
	initialScoreValue, err := pgtypex.NumericScale(initialScore, 2)
	if err != nil {
		return nil, apperr.ErrContestBattleEntryInvalid.WithCause(err)
	}
	limit32, ok := intx.Int32(limit)
	if !ok || limit32 <= 0 {
		return nil, apperr.ErrContestBattleEntryInvalid
	}
	rows, err := tx.q.ListActiveBattleOpponents(ctx, sqlcgen.ListActiveBattleOpponentsParams{TenantID: tenantID, ContestID: contestID, ProblemID: problemID, ID: excludeEntryID, TeamID: excludeTeamID, Column6: matchMode, Limit: limit32, Column8: initialScoreValue})
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
func (tx *txStore) ExhaustUnstartedBattleMatches(ctx context.Context, maxAttempts int32, staleBefore time.Time) ([]BattleMatch, error) {
	rows, err := tx.q.ExhaustUnstartedBattleMatches(ctx, sqlcgen.ExhaustUnstartedBattleMatchesParams{MaxAttempts: maxAttempts, StaleBefore: timex.RequiredTimestamptz(staleBefore)})
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

func (tx *txStore) ClaimPendingBattleMatches(ctx context.Context, limit int, maxAttempts int32, staleBefore, leaseUntil time.Time, leaseToken string) ([]BattleMatch, error) {
	limit32, ok := intx.Int32(limit)
	if !ok || limit32 <= 0 {
		return nil, apperr.ErrContestBattleMatchFailed
	}
	rows, err := tx.q.ClaimPendingBattleMatchesAcrossTenants(ctx, sqlcgen.ClaimPendingBattleMatchesAcrossTenantsParams{PageLimit: limit32, MaxAttempts: maxAttempts, StaleBefore: timex.RequiredTimestamptz(staleBefore), LeaseUntil: timex.RequiredTimestamptz(leaseUntil), LeaseToken: leaseToken})
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
	limit32, ok := intx.Int32(limit)
	if !ok || limit32 <= 0 {
		return nil, apperr.ErrContestBattleMatchFailed
	}
	rows, err := tx.q.ListRunningBattleMatchesWithJudgeTask(ctx, limit32)
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

// StartBattleMatch 保存对局沙箱和判题任务引用,未命中表示启动租约已经失效。
func (tx *txStore) StartBattleMatch(ctx context.Context, tenantID, id int64, sandboxRef, judgeTaskRef, leaseToken string) (BattleMatch, bool, error) {
	row, err := tx.q.StartBattleMatch(ctx, sqlcgen.StartBattleMatchParams{TenantID: tenantID, ID: id, SandboxRef: pgtypex.Text(sandboxRef), JudgeTaskRef: pgtypex.Text(judgeTaskRef), LeaseToken: leaseToken})
	if db.IsNoRows(err) {
		return BattleMatch{}, false, nil
	}
	if err != nil {
		return BattleMatch{}, false, apperr.ErrContestBattleMatchFailed.WithCause(err)
	}
	item, err := battleMatchFromRow(row)
	if err != nil {
		return BattleMatch{}, false, err
	}
	return item, true, nil
}

// RenewBattleMatchStartLease 延长当前 worker 的启动租约,未命中表示租约已失效或对局已进入 M3 生命周期。
func (tx *txStore) RenewBattleMatchStartLease(ctx context.Context, tenantID, id int64, leaseToken string, leaseUntil time.Time) (bool, error) {
	updated, err := tx.q.RenewBattleMatchStartLease(ctx, sqlcgen.RenewBattleMatchStartLeaseParams{TenantID: tenantID, ID: id, LeaseToken: leaseToken, LeaseUntil: timex.RequiredTimestamptz(leaseUntil)})
	if err != nil {
		return false, apperr.ErrContestBattleMatchFailed.WithCause(err)
	}
	return updated == 1, nil
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

// ListBattleMatchesForTeam 查询对局历史和总数。
// teamID 传 0 表示不按队伍过滤(组织者视角看本赛事全部对局)。
func (tx *txStore) ListBattleMatchesForTeam(ctx context.Context, tenantID, contestID, teamID int64, page, size int) ([]BattleMatch, int64, error) {
	limit, offset := pagex.LimitOffset(page, size)
	rows, err := tx.q.ListBattleMatchesForTeam(ctx, sqlcgen.ListBattleMatchesForTeamParams{TenantID: tenantID, ContestID: contestID, TeamID: teamID, PageLimit: limit, PageOffset: offset})
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

// ListBattleReplayMatchesForTeam 查询按完成时间排序的回放时间窗。
func (tx *txStore) ListBattleReplayMatchesForTeam(ctx context.Context, tenantID, contestID, teamID int64, page, size int) ([]BattleReplayRow, error) {
	limit, offset := pagex.LimitOffset(page, size)
	rows, err := tx.q.ListBattleReplayMatchesForTeam(ctx, sqlcgen.ListBattleReplayMatchesForTeamParams{
		TenantID: tenantID, ContestID: contestID, TeamID: teamID, PageLimit: int64(limit), PageOffset: int64(offset),
	})
	if err != nil {
		return nil, apperr.ErrContestBattleMatchFailed.WithCause(err)
	}
	out := make([]BattleReplayRow, 0, len(rows))
	for _, row := range rows {
		match, err := battleMatchFromReplayRow(row)
		if err != nil {
			return nil, err
		}
		item := BattleReplayRow{
			Match: match, SequenceNo: row.SequenceNo, MySide: row.MySide,
			ActiveEntryID: row.ActiveEntryID, ActiveEntryRole: row.ActiveEntryRole,
			ActiveEntryVersion: row.ActiveEntryVersionNo, ActiveEntryAt: timex.FromTimestamptz(row.ActiveEntrySubmittedAt),
		}
		out = append(out, item)
	}
	return out, nil
}

// CountBattleReplayPendingForTeam 统计当前队伍尚未完成的对局数量。
func (tx *txStore) CountBattleReplayPendingForTeam(ctx context.Context, tenantID, contestID, teamID int64) (int64, error) {
	count, err := tx.q.CountBattleReplayPendingForTeam(ctx, sqlcgen.CountBattleReplayPendingForTeamParams{TenantID: tenantID, ContestID: contestID, TeamID: teamID})
	if err != nil {
		return 0, apperr.ErrContestBattleMatchFailed.WithCause(err)
	}
	return count, nil
}

// CountBattleReplayCompletedForTeam 统计当前队伍已完成的对局总量。
func (tx *txStore) CountBattleReplayCompletedForTeam(ctx context.Context, tenantID, contestID, teamID int64) (int64, error) {
	count, err := tx.q.CountBattleReplayCompletedForTeam(ctx, sqlcgen.CountBattleReplayCompletedForTeamParams{TenantID: tenantID, ContestID: contestID, TeamID: teamID})
	if err != nil {
		return 0, apperr.ErrContestBattleMatchFailed.WithCause(err)
	}
	return count, nil
}

// GetBattleReplayCheckpointForTeam 获取时间窗之前的服务端战绩检查点。
func (tx *txStore) GetBattleReplayCheckpointForTeam(ctx context.Context, tenantID, contestID, teamID int64, page, size int) (BattleReplayCheckpoint, error) {
	_, offset := pagex.LimitOffset(page, size)
	row, err := tx.q.GetBattleReplayCheckpointForTeam(ctx, sqlcgen.GetBattleReplayCheckpointForTeamParams{TenantID: tenantID, ContestID: contestID, TeamID: teamID, PageOffset: int64(offset)})
	if err != nil {
		return BattleReplayCheckpoint{}, apperr.ErrContestBattleMatchFailed.WithCause(err)
	}
	return BattleReplayCheckpoint{Wins: row.Wins, Losses: row.Losses, Draws: row.Draws, RatingDelta: row.RatingDelta, Rating: row.Rating}, nil
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
	delta, err := jsonx.AnyBytes(battleScoreDeltaForJSON(item.ScoreDelta), apperr.ErrContestBattleMatchFailed)
	if err != nil {
		return BattleMatch{}, err
	}
	row, err := tx.q.FinishBattleMatch(ctx, sqlcgen.FinishBattleMatchParams{TenantID: item.TenantID, ID: item.ID, SandboxRef: pgtypex.Text(item.SandboxRef), JudgeTaskRef: pgtypex.Text(item.JudgeTaskRef), Result: pgtypex.Int2(item.Result), ScoreDelta: delta, ReplayRef: pgtypex.Text(item.ReplayRef)})
	if err != nil {
		return BattleMatch{}, apperr.ErrContestBattleMatchFailed.WithCause(err)
	}
	return battleMatchFromRow(row)
}

// FailBattleMatchStart 仅由仍持有启动租约的 worker 标记失败,未命中表示租约已失效。
func (tx *txStore) FailBattleMatchStart(ctx context.Context, tenantID, id int64, leaseToken string) (BattleMatch, bool, error) {
	row, err := tx.q.FailBattleMatchStart(ctx, sqlcgen.FailBattleMatchStartParams{TenantID: tenantID, ID: id, LeaseToken: leaseToken})
	if db.IsNoRows(err) {
		return BattleMatch{}, false, nil
	}
	if err != nil {
		return BattleMatch{}, false, apperr.ErrContestBattleMatchFailed.WithCause(err)
	}
	item, err := battleMatchFromRow(row)
	if err != nil {
		return BattleMatch{}, false, err
	}
	return item, true, nil
}

// FailBattleMatchByJudgeTask 只允许已持久化判题引用的终态事件结算对局。
func (tx *txStore) FailBattleMatchByJudgeTask(ctx context.Context, tenantID, id int64, judgeTaskRef string) (BattleMatch, error) {
	row, err := tx.q.FailBattleMatchByJudgeTask(ctx, sqlcgen.FailBattleMatchByJudgeTaskParams{TenantID: tenantID, ID: id, JudgeTaskRef: pgtypex.Text(judgeTaskRef)})
	if err != nil {
		return BattleMatch{}, apperr.ErrContestBattleMatchFailed.WithCause(err)
	}
	return battleMatchFromRow(row)
}

// UpsertLadderSnapshot 保存封榜或归档阶段的权威榜单快照。
func (tx *txStore) UpsertLadderSnapshot(ctx context.Context, item LadderSnapshot) (LadderSnapshot, error) {
	raw, err := jsonx.AnyBytes(ladderSnapshotEntriesJSON(item.Ranking), apperr.ErrContestInvalid)
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
	evidence, err := jsonx.AnyBytes(item.Evidence, apperr.ErrContestCheatInvalid)
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
	limit, offset := pagex.LimitOffset(page, size)
	rows, err := tx.q.ListCheatRecords(ctx, sqlcgen.ListCheatRecordsParams{TenantID: tenantID, ContestID: contestID, Limit: limit, Offset: offset})
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
	cfg, err := jsonx.AnyBytes(item.Config, apperr.ErrContestVulnSourceInvalid)
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
	body, err := jsonx.AnyBytes(item.DraftBody, apperr.ErrContestVulnProblemInvalid)
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
func (tx *txStore) ListVulnProblems(ctx context.Context, tenantID, sourceID int64, status, prevalidateStatus int16, page, size int) ([]VulnProblem, int64, error) {
	limit, offset := pagex.LimitOffset(page, size)
	rows, err := tx.q.ListVulnProblems(ctx, sqlcgen.ListVulnProblemsParams{TenantID: tenantID, SourceID: sourceID, Status: status, PrevalidateStatus: prevalidateStatus, PageOffset: offset, PageLimit: limit})
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
	total, err := tx.q.CountVulnProblems(ctx, sqlcgen.CountVulnProblemsParams{TenantID: tenantID, SourceID: sourceID, Status: status, PrevalidateStatus: prevalidateStatus})
	if err != nil {
		return nil, 0, apperr.ErrContestVulnProblemInvalid.WithCause(err)
	}
	return out, total, nil
}

// SetVulnProblemPrevalidate 保存预验证结论。
func (tx *txStore) SetVulnProblemPrevalidate(ctx context.Context, tenantID, id int64, status int16, detail map[string]any, digest string, snapshot *contracts.SandboxCompositionSnapshot, initCodeRef, initScriptRef string) (VulnProblem, error) {
	raw, err := jsonx.AnyBytes(detail, apperr.ErrContestVulnProblemInvalid)
	if err != nil {
		return VulnProblem{}, err
	}
	snapshotRaw, err := encodeOptionalJSON(snapshot, apperr.ErrContestVulnProblemInvalid)
	if err != nil {
		return VulnProblem{}, err
	}
	row, err := tx.q.SetVulnProblemPrevalidate(ctx, sqlcgen.SetVulnProblemPrevalidateParams{TenantID: tenantID, ID: id, PrevalidateStatus: status, PrevalidateDetail: raw, CompositionDigest: pgtypex.Text(digest), CompositionSnapshot: snapshotRaw, InitCodeRef: pgtypex.Text(initCodeRef), InitScriptRef: pgtypex.Text(initScriptRef)})
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
	rows, err := tx.q.ListStudentContestRecords(ctx, sqlcgen.ListStudentContestRecordsParams{MemberTenantID: tenantID, AccountID: accountID})
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
func (tx *txStore) ClaimManualArchiveContest(ctx context.Context, tenantID, contestID int64, staleBefore, leaseUntil time.Time, leaseToken string) (ContestArchiveClaim, error) {
	row, err := tx.q.ClaimManualArchiveContest(ctx, sqlcgen.ClaimManualArchiveContestParams{TenantID: tenantID, ContestID: contestID, StaleBefore: timex.RequiredTimestamptz(staleBefore), LeaseUntil: timex.RequiredTimestamptz(leaseUntil), LeaseToken: leaseToken})
	if err != nil {
		return ContestArchiveClaim{}, err
	}
	return contestArchiveClaimFromManualRow(row), nil
}

func (tx *txStore) ClaimAutoArchiveContests(ctx context.Context, limit int, staleBefore, leaseUntil time.Time, leaseToken string) ([]ContestArchiveClaim, error) {
	limit32, ok := intx.Int32(limit)
	if !ok || limit32 <= 0 {
		return nil, apperr.ErrContestStateInvalid
	}
	rows, err := tx.q.ClaimAutoArchiveContestsAcrossTenants(ctx, sqlcgen.ClaimAutoArchiveContestsAcrossTenantsParams{PageLimit: limit32, StaleBefore: timex.RequiredTimestamptz(staleBefore), LeaseUntil: timex.RequiredTimestamptz(leaseUntil), LeaseToken: leaseToken})
	if err != nil {
		return nil, apperr.ErrContestStateInvalid.WithCause(err)
	}
	out := make([]ContestArchiveClaim, 0, len(rows))
	for _, row := range rows {
		out = append(out, contestArchiveClaimFromAutoRow(row))
	}
	return out, nil
}

// CompleteAutoArchiveContest 仅允许仍持有领取更新时间的 worker 完成归档,防止过期 worker 覆盖新 worker。
func (tx *txStore) CompleteAutoArchiveContest(ctx context.Context, tenantID, contestID int64, leaseToken string) (int64, error) {
	return tx.q.CompleteAutoArchiveContest(ctx, sqlcgen.CompleteAutoArchiveContestParams{TenantID: tenantID, ContestID: contestID, LeaseToken: leaseToken})
}

func contestArchiveClaimFromAutoRow(row sqlcgen.ClaimAutoArchiveContestsAcrossTenantsRow) ContestArchiveClaim {
	return ContestArchiveClaim{ID: row.ID, TenantID: row.TenantID, CreatedAt: timex.FromTimestamptz(row.CreatedAt), ArchiveLeaseToken: row.ArchiveLeaseToken, ArchiveLeaseUntil: timex.FromTimestamptz(row.ArchiveLeaseUntil)}
}

func contestArchiveClaimFromManualRow(row sqlcgen.ClaimManualArchiveContestRow) ContestArchiveClaim {
	return ContestArchiveClaim{ID: row.ID, TenantID: row.TenantID, CreatedAt: timex.FromTimestamptz(row.CreatedAt), ArchiveLeaseToken: row.ArchiveLeaseToken, ArchiveLeaseUntil: timex.FromTimestamptz(row.ArchiveLeaseUntil)}
}
