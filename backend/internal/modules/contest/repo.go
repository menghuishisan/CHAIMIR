// M8 数据访问层:封装 contest 自有租户表的 sqlc 查询与 RLS 注入。
package contest

import (
	"context"

	"chaimir/internal/modules/contest/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/crypto"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// contestStore 是服务层依赖的数据访问接口,便于服务逻辑测试。
type contestStore interface {
	ListContests(context.Context, int16, int, int) ([]ContestDTO, int64, error)
	CreateContest(context.Context, tenant.Identity, int64, ContestRequest) (ContestDTO, error)
	GetContest(context.Context, int64) (ContestDTO, error)
	UpdateContest(context.Context, int64, ContestRequest) (ContestDTO, error)
	UpdateContestStatus(context.Context, int64, int16) (ContestDTO, error)
	CreateProblem(context.Context, tenant.Identity, int64, int64, ContestProblemRequest) (ContestProblemDTO, error)
	ListProblems(context.Context, int64) ([]ContestProblemDTO, error)
	GetProblem(context.Context, int64) (ContestProblemDTO, error)
	CreateTeam(context.Context, tenant.Identity, int64, int64, string, bool) (TeamDTO, error)
	AddTeamMember(context.Context, tenant.Identity, int64, int64, int64, int64, bool) (TeamMemberDTO, error)
	GetTeam(context.Context, int64) (TeamDTO, error)
	GetTeamByContestAndAccount(context.Context, int64, int64) (TeamDTO, error)
	LockTeam(context.Context, int64) (TeamDTO, error)
	CreateSolveSubmission(context.Context, tenant.Identity, int64, int64, int64, int64, SolveSubmitRequest, string, string) (SolveSubmissionDTO, error)
	GetSolveSubmission(context.Context, int64) (SolveSubmissionDTO, error)
	PendingSubmissionByJudgeTask(context.Context, int64, int64) (pendingSolveSubmission, error)
	UpdateSubmissionResult(context.Context, int64, bool, int32) (SolveSubmissionDTO, error)
	CreateBattleEntry(context.Context, tenant.Identity, int64, int64, int64, BattleEntryRequest, int32) (BattleEntryDTO, error)
	ListBattleEntries(context.Context, int64, int64) ([]BattleEntryDTO, error)
	GetBattleEntry(context.Context, int64) (BattleEntryDTO, error)
	CreateBattleMatch(context.Context, int64, int64, BattleMatchResult, map[string]any) (BattleMatchDTO, error)
	GetBattleMatch(context.Context, int64) (BattleMatchDTO, error)
	ListBattleMatches(context.Context, int64, int64, int, int) ([]BattleMatchDTO, error)
	UpsertRank(context.Context, int64, int64, int64, int64, float64, int32) (LadderRankDTO, error)
	GetRankOrDefault(context.Context, int64, int64, int64) (LadderRankDTO, error)
	AddRankScore(context.Context, int64, int64, int64, int64, float64) error
	ListRanks(context.Context, int64, int, int) ([]LadderRankDTO, error)
	CreateSnapshot(context.Context, int64, int64, int64, []LadderRankDTO) (ResultSnapshotDTO, error)
	GetSnapshot(context.Context, int64) (ResultSnapshotDTO, error)
	ListAchievements(context.Context, int64, int64) ([]ContestAchievementDTO, error)
	CreateCheatRecord(context.Context, tenant.Identity, int64, int64, int64, CheatRecordRequest) (CheatRecordDTO, error)
	ListCheatRecords(context.Context, int64, int, int) ([]CheatRecordDTO, error)
	CreateVulnSource(context.Context, tenant.Identity, int64, VulnSourceRequest) (VulnSourceDTO, error)
	ListVulnSources(context.Context, int, int) ([]VulnSourceDTO, error)
	GetVulnSource(context.Context, int64) (VulnSourceDTO, error)
	MarkVulnSourceSynced(context.Context, int64) (VulnSourceDTO, error)
	CreateVulnProblem(context.Context, tenant.Identity, int64, VulnProblemImportRequest) (VulnProblemDTO, error)
	GetVulnProblem(context.Context, int64) (VulnProblemDTO, error)
	UpdateVulnPrevalidate(context.Context, int64, int16, map[string]any) (VulnProblemDTO, error)
	FinalizeVulnProblem(context.Context, int64, string, string) (VulnProblemDTO, error)
	Stats(context.Context, int64) (StatsDTO, error)
}

// repo 是 M8 模块数据库访问封装。
type repo struct {
	db *db.DB
}

// newRepo 构造 M8 repo。
func newRepo(database *db.DB) *repo { return &repo{db: database} }

// queryFunc 是 M8 sqlc 查询闭包。
type queryFunc func(q *sqlcgen.Queries) error

// inTenant 从请求上下文读取租户并注入 RLS。
func (r *repo) inTenant(ctx context.Context, fn queryFunc) error {
	return r.db.WithTenantTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// inTenantID 用显式租户 ID 注入 RLS,供事件与 contracts 内部入口使用。
func (r *repo) inTenantID(ctx context.Context, tenantID int64, fn queryFunc) error {
	return r.db.WithTenantTxID(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// ListContests 查询竞赛列表并返回同条件总数。
func (r *repo) ListContests(ctx context.Context, status int16, page, size int) ([]ContestDTO, int64, error) {
	page, size = pagex.Normalize(page, size)
	var rows []sqlcgen.Contest
	var total int64
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		rows, err = q.ListContests(ctx, sqlcgen.ListContestsParams{Status: pgInt2(status), OffsetCount: int32((page - 1) * size), LimitCount: int32(size)})
		if err != nil {
			return err
		}
		total, err = q.CountContests(ctx, pgInt2(status))
		return err
	}); err != nil {
		return nil, 0, apperr.ErrContestQueryFailed.WithCause(err)
	}
	out := make([]ContestDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, contestDTOFromRow(row))
	}
	return out, total, nil
}

// CreateContest 创建竞赛草稿。
func (r *repo) CreateContest(ctx context.Context, id tenant.Identity, contestID int64, req ContestRequest) (ContestDTO, error) {
	rules, err := jsonx.ObjectBytes(req.Rules, apperr.ErrContestInvalid)
	if err != nil {
		return ContestDTO{}, err
	}
	var row sqlcgen.Contest
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var createErr error
		row, createErr = q.CreateContest(ctx, sqlcgen.CreateContestParams{
			ID: contestID, TenantID: id.TenantID, OrganizerID: id.AccountID, Name: req.Name, Mode: req.Mode,
			MatchMode: pgInt2(req.MatchMode), TeamMode: req.TeamMode, SignupStart: timex.Timestamptz(req.SignupStart),
			SignupEnd: timex.Timestamptz(req.SignupEnd), StartAt: timex.Timestamptz(req.StartAt), EndAt: timex.Timestamptz(req.EndAt),
			FreezeMinutes: req.FreezeMinutes, Rules: rules, Status: ContestStatusDraft,
		})
		return createErr
	}); err != nil {
		return ContestDTO{}, apperr.ErrContestInvalid.WithCause(err)
	}
	return contestDTOFromRow(row), nil
}

// GetContest 读取竞赛定义。
func (r *repo) GetContest(ctx context.Context, contestID int64) (ContestDTO, error) {
	var row sqlcgen.Contest
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.GetContestByID(ctx, contestID)
		return err
	}); err != nil {
		return ContestDTO{}, notFoundOrInternal(err, apperr.ErrContestNotFound)
	}
	return contestDTOFromRow(row), nil
}

// UpdateContest 更新竞赛定义。
func (r *repo) UpdateContest(ctx context.Context, contestID int64, req ContestRequest) (ContestDTO, error) {
	rules, err := jsonx.ObjectBytes(req.Rules, apperr.ErrContestInvalid)
	if err != nil {
		return ContestDTO{}, err
	}
	var row sqlcgen.Contest
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var updateErr error
		row, updateErr = q.UpdateContest(ctx, sqlcgen.UpdateContestParams{
			ID: contestID, Name: req.Name, Mode: req.Mode, MatchMode: pgInt2(req.MatchMode), TeamMode: req.TeamMode,
			SignupStart: timex.Timestamptz(req.SignupStart), SignupEnd: timex.Timestamptz(req.SignupEnd), StartAt: timex.Timestamptz(req.StartAt),
			EndAt: timex.Timestamptz(req.EndAt), FreezeMinutes: req.FreezeMinutes, Rules: rules,
		})
		return updateErr
	}); err != nil {
		return ContestDTO{}, notFoundOrInternal(err, apperr.ErrContestNotFound)
	}
	return contestDTOFromRow(row), nil
}

// UpdateContestStatus 更新竞赛状态。
func (r *repo) UpdateContestStatus(ctx context.Context, contestID int64, status int16) (ContestDTO, error) {
	var row sqlcgen.Contest
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.UpdateContestStatus(ctx, sqlcgen.UpdateContestStatusParams{ID: contestID, Status: status})
		return err
	}); err != nil {
		return ContestDTO{}, notFoundOrInternal(err, apperr.ErrContestNotFound)
	}
	return contestDTOFromRow(row), nil
}

// CreateProblem 创建竞赛题目引用。
func (r *repo) CreateProblem(ctx context.Context, id tenant.Identity, problemID, contestID int64, req ContestProblemRequest) (ContestProblemDTO, error) {
	dynamicScore, err := jsonx.ObjectBytes(req.DynamicScore, apperr.ErrContestProblem)
	if err != nil {
		return ContestProblemDTO{}, err
	}
	var row sqlcgen.ContestProblem
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var createErr error
		row, createErr = q.CreateContestProblem(ctx, sqlcgen.CreateContestProblemParams{
			ID: problemID, TenantID: id.TenantID, ContestID: contestID, ItemCode: req.ItemCode, ItemVersion: req.ItemVersion,
			Score: req.Score, DynamicScore: dynamicScore, BattleRule: pgInt2(req.BattleRule), Seq: req.Seq,
		})
		return createErr
	}); err != nil {
		return ContestProblemDTO{}, apperr.ErrContestProblem.WithCause(err)
	}
	return contestProblemDTOFromRow(row), nil
}

// ListProblems 查询竞赛题目引用。
func (r *repo) ListProblems(ctx context.Context, contestID int64) ([]ContestProblemDTO, error) {
	var rows []sqlcgen.ContestProblem
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		rows, err = q.ListContestProblems(ctx, contestID)
		return err
	}); err != nil {
		return nil, apperr.ErrContestProblem.WithCause(err)
	}
	out := make([]ContestProblemDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, contestProblemDTOFromRow(row))
	}
	return out, nil
}

// GetProblem 读取竞赛题目引用。
func (r *repo) GetProblem(ctx context.Context, problemID int64) (ContestProblemDTO, error) {
	var row sqlcgen.ContestProblem
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.GetContestProblemByID(ctx, problemID)
		return err
	}); err != nil {
		return ContestProblemDTO{}, notFoundOrInternal(err, apperr.ErrContestProblem)
	}
	return contestProblemDTOFromRow(row), nil
}

// CreateTeam 创建队伍。
func (r *repo) CreateTeam(ctx context.Context, id tenant.Identity, teamID, contestID int64, name string, withInvite bool) (TeamDTO, error) {
	invite := ""
	if withInvite {
		var err error
		invite, err = crypto.RandomToken(8)
		if err != nil {
			return TeamDTO{}, apperr.ErrContestTeamInvalid.WithCause(err)
		}
	}
	var row sqlcgen.Team
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.CreateTeam(ctx, sqlcgen.CreateTeamParams{ID: teamID, TenantID: id.TenantID, ContestID: contestID, Name: name, InviteCode: pgText(invite), Status: TeamStatusBuilding})
		return err
	}); err != nil {
		return TeamDTO{}, apperr.ErrContestTeamInvalid.WithCause(err)
	}
	return teamDTOFromRows(row, nil), nil
}

// AddTeamMember 新增或更新队员。
func (r *repo) AddTeamMember(ctx context.Context, id tenant.Identity, memberID, teamID, accountID, memberTenantID int64, leader bool) (TeamMemberDTO, error) {
	var row sqlcgen.TeamMember
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.AddTeamMember(ctx, sqlcgen.AddTeamMemberParams{ID: memberID, TenantID: id.TenantID, TeamID: teamID, AccountID: accountID, MemberTenantID: memberTenantID, IsLeader: leader})
		return err
	}); err != nil {
		return TeamMemberDTO{}, apperr.ErrContestTeamInvalid.WithCause(err)
	}
	return teamMemberDTOFromRow(row), nil
}

// GetTeam 读取队伍及成员。
func (r *repo) GetTeam(ctx context.Context, teamID int64) (TeamDTO, error) {
	var team sqlcgen.Team
	var members []sqlcgen.TeamMember
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		team, err = q.GetTeamByID(ctx, teamID)
		if err != nil {
			return err
		}
		members, err = q.ListTeamMembers(ctx, teamID)
		return err
	}); err != nil {
		return TeamDTO{}, notFoundOrInternal(err, apperr.ErrContestTeamNotFound)
	}
	return teamDTOFromRows(team, members), nil
}

// GetTeamByContestAndAccount 通过竞赛和当前账号定位参赛队伍。
func (r *repo) GetTeamByContestAndAccount(ctx context.Context, contestID, accountID int64) (TeamDTO, error) {
	var team sqlcgen.Team
	var members []sqlcgen.TeamMember
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		team, err = q.GetTeamByContestAndAccount(ctx, sqlcgen.GetTeamByContestAndAccountParams{ContestID: contestID, AccountID: accountID})
		if err != nil {
			return err
		}
		members, err = q.ListTeamMembers(ctx, team.ID)
		return err
	}); err != nil {
		return TeamDTO{}, notFoundOrInternal(err, apperr.ErrContestTeamNotFound)
	}
	return teamDTOFromRows(team, members), nil
}

// LockTeam 锁定队伍。
func (r *repo) LockTeam(ctx context.Context, teamID int64) (TeamDTO, error) {
	var row sqlcgen.Team
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.LockTeam(ctx, sqlcgen.LockTeamParams{ID: teamID, Status: TeamStatusLocked})
		return err
	}); err != nil {
		return TeamDTO{}, notFoundOrInternal(err, apperr.ErrContestTeamNotFound)
	}
	return teamDTOFromRows(row, nil), nil
}

// CreateSolveSubmission 创建解题提交记录。
func (r *repo) CreateSolveSubmission(ctx context.Context, id tenant.Identity, submissionID, contestID, problemID, teamID int64, req SolveSubmitRequest, sourceRef, judgeTaskRef string) (SolveSubmissionDTO, error) {
	content, err := jsonx.ObjectBytes(req.ContentRef, apperr.ErrContestSubmissionInvalid)
	if err != nil {
		return SolveSubmissionDTO{}, err
	}
	var row sqlcgen.SolveSubmission
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var createErr error
		row, createErr = q.CreateSolveSubmission(ctx, sqlcgen.CreateSolveSubmissionParams{
			ID: submissionID, TenantID: id.TenantID, ContestID: contestID, ProblemID: problemID, TeamID: teamID,
			SubmitterID: id.AccountID, ContentRef: content, SourceRef: sourceRef, JudgeTaskRef: pgText(judgeTaskRef), SandboxRef: pgText(req.SandboxRef),
		})
		return createErr
	}); err != nil {
		return SolveSubmissionDTO{}, apperr.ErrContestSubmissionInvalid.WithCause(err)
	}
	return solveSubmissionDTOFromRow(row), nil
}

// GetSolveSubmission 读取提交结果。
func (r *repo) GetSolveSubmission(ctx context.Context, submissionID int64) (SolveSubmissionDTO, error) {
	var row sqlcgen.SolveSubmission
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.GetSolveSubmissionByID(ctx, submissionID)
		return err
	}); err != nil {
		return SolveSubmissionDTO{}, notFoundOrInternal(err, apperr.ErrContestSubmissionNotFound)
	}
	return solveSubmissionDTOFromRow(row), nil
}

// PendingSubmissionByJudgeTask 根据判题任务定位等待回写的提交。
func (r *repo) PendingSubmissionByJudgeTask(ctx context.Context, tenantID, taskID int64) (pendingSolveSubmission, error) {
	var row sqlcgen.GetSolveSubmissionByJudgeTaskRow
	if err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.GetSolveSubmissionByJudgeTask(ctx, pgText(ids.Format(taskID)))
		return err
	}); err != nil {
		return pendingSolveSubmission{}, notFoundOrInternal(err, apperr.ErrContestSubmissionNotFound)
	}
	return pendingSolveSubmission{ID: row.ID, TenantID: row.TenantID, ContestID: row.ContestID, TeamID: row.TeamID, ProblemID: row.ProblemID, SourceRef: row.SourceRef, MaxScore: row.MaxScore}, nil
}

// UpdateSubmissionResult 回写判题结果。
func (r *repo) UpdateSubmissionResult(ctx context.Context, submissionID int64, passed bool, score int32) (SolveSubmissionDTO, error) {
	var row sqlcgen.SolveSubmission
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.UpdateSolveSubmissionResult(ctx, sqlcgen.UpdateSolveSubmissionResultParams{ID: submissionID, Passed: passed, Score: score})
		return err
	}); err != nil {
		return SolveSubmissionDTO{}, notFoundOrInternal(err, apperr.ErrContestSubmissionNotFound)
	}
	return solveSubmissionDTOFromRow(row), nil
}

// CreateBattleEntry 创建参战物版本。
func (r *repo) CreateBattleEntry(ctx context.Context, id tenant.Identity, entryID, contestID, teamID int64, req BattleEntryRequest, versionNo int32) (BattleEntryDTO, error) {
	var row sqlcgen.BattleEntry
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.CreateBattleEntry(ctx, sqlcgen.CreateBattleEntryParams{ID: entryID, TenantID: id.TenantID, ContestID: contestID, TeamID: teamID, Role: req.Role, ArtifactRef: req.ArtifactRef, VersionNo: versionNo})
		return err
	}); err != nil {
		return BattleEntryDTO{}, apperr.ErrContestBattleInvalid.WithCause(err)
	}
	return battleEntryDTOFromRow(row), nil
}

// ListBattleEntries 查询参战物版本历史。
func (r *repo) ListBattleEntries(ctx context.Context, contestID, teamID int64) ([]BattleEntryDTO, error) {
	var rows []sqlcgen.BattleEntry
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		rows, err = q.ListBattleEntries(ctx, sqlcgen.ListBattleEntriesParams{ContestID: contestID, TeamID: teamID})
		return err
	}); err != nil {
		return nil, apperr.ErrContestBattleInvalid.WithCause(err)
	}
	out := make([]BattleEntryDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, battleEntryDTOFromRow(row))
	}
	return out, nil
}

// GetBattleEntry 读取参战物。
func (r *repo) GetBattleEntry(ctx context.Context, entryID int64) (BattleEntryDTO, error) {
	var row sqlcgen.BattleEntry
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.GetBattleEntryByID(ctx, entryID)
		return err
	}); err != nil {
		return BattleEntryDTO{}, notFoundOrInternal(err, apperr.ErrContestBattleEntryNotFound)
	}
	return battleEntryDTOFromRow(row), nil
}

// CreateBattleMatch 创建对局记录。
func (r *repo) CreateBattleMatch(ctx context.Context, tenantID, matchID int64, result BattleMatchResult, scoreDelta map[string]any) (BattleMatchDTO, error) {
	data, err := jsonx.ObjectBytes(scoreDelta, apperr.ErrContestBattleInvalid)
	if err != nil {
		return BattleMatchDTO{}, err
	}
	var row sqlcgen.BattleMatch
	if err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var createErr error
		row, createErr = q.CreateBattleMatch(ctx, sqlcgen.CreateBattleMatchParams{
			ID: matchID, TenantID: tenantID, ContestID: result.ContestID, EntryAID: result.EntryAID, EntryBID: result.EntryBID,
			SandboxRef: result.SandboxRef, Result: result.Result, ScoreDelta: data, ReplayRef: result.ReplayRef,
		})
		return createErr
	}); err != nil {
		return BattleMatchDTO{}, apperr.ErrContestBattleFailed.WithCause(err)
	}
	return battleMatchDTOFromRow(row), nil
}

// GetBattleMatch 读取对局记录。
func (r *repo) GetBattleMatch(ctx context.Context, matchID int64) (BattleMatchDTO, error) {
	var row sqlcgen.BattleMatch
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.GetBattleMatchByID(ctx, matchID)
		return err
	}); err != nil {
		return BattleMatchDTO{}, notFoundOrInternal(err, apperr.ErrContestBattleInvalid)
	}
	return battleMatchDTOFromRow(row), nil
}

// ListBattleMatches 按队伍查询对局列表。
func (r *repo) ListBattleMatches(ctx context.Context, contestID, teamID int64, page, size int) ([]BattleMatchDTO, error) {
	page, size = pagex.Normalize(page, size)
	var rows []sqlcgen.BattleMatch
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		if teamID > 0 {
			rows, err = q.ListBattleMatchesByTeam(ctx, sqlcgen.ListBattleMatchesByTeamParams{ContestID: contestID, TeamID: teamID, OffsetCount: int32((page - 1) * size), LimitCount: int32(size)})
			return err
		}
		rows, err = q.ListBattleMatches(ctx, sqlcgen.ListBattleMatchesParams{ContestID: contestID, EntryID: pgInt8(0), OffsetCount: int32((page - 1) * size), LimitCount: int32(size)})
		return err
	}); err != nil {
		return nil, apperr.ErrContestBattleInvalid.WithCause(err)
	}
	out := make([]BattleMatchDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, battleMatchDTOFromRow(row))
	}
	return out, nil
}

// UpsertRank 新增或更新排行榜分数。
func (r *repo) UpsertRank(ctx context.Context, tenantID, rankID, contestID, teamID int64, score float64, solvedCount int32) (LadderRankDTO, error) {
	numeric, err := pgNumeric(score)
	if err != nil {
		return LadderRankDTO{}, apperr.ErrContestInvalid.WithCause(err)
	}
	var row sqlcgen.LadderRank
	if err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var upsertErr error
		var lastSolveAt pgtype.Timestamptz
		if solvedCount > 0 {
			lastSolveAt = timex.RequiredTimestamptz(timex.Now())
		}
		row, upsertErr = q.UpsertLadderRank(ctx, sqlcgen.UpsertLadderRankParams{
			ID: rankID, TenantID: tenantID, ContestID: contestID, TeamID: teamID, Score: numeric, SolvedCount: solvedCount,
			LastSolveAt: lastSolveAt, Rank: 1,
		})
		return upsertErr
	}); err != nil {
		return LadderRankDTO{}, apperr.ErrContestInvalid.WithCause(err)
	}
	return ladderRankDTOFromRow(row), nil
}

// GetRankOrDefault 读取排行,未初始化时返回默认 ELO 1000。
func (r *repo) GetRankOrDefault(ctx context.Context, tenantID, contestID, teamID int64) (LadderRankDTO, error) {
	var row sqlcgen.LadderRank
	if err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.GetLadderRank(ctx, sqlcgen.GetLadderRankParams{ContestID: contestID, TeamID: teamID})
		return err
	}); err != nil {
		if db.IsNoRows(err) {
			return LadderRankDTO{ContestID: ids.Format(contestID), TeamID: ids.Format(teamID), Score: 1000, Rank: 1}, nil
		}
		return LadderRankDTO{}, apperr.ErrContestQueryFailed.WithCause(err)
	}
	return ladderRankDTOFromRow(row), nil
}

// AddRankScore 追加解题赛分数并增加解题数。
func (r *repo) AddRankScore(ctx context.Context, tenantID, rankID, contestID, teamID int64, delta float64) error {
	current, err := r.GetRankOrDefault(ctx, tenantID, contestID, teamID)
	if err != nil {
		return err
	}
	_, err = r.UpsertRank(ctx, tenantID, rankID, contestID, teamID, current.Score+delta, current.SolvedCount+1)
	return err
}

// ListRanks 查询排行榜。
func (r *repo) ListRanks(ctx context.Context, contestID int64, page, size int) ([]LadderRankDTO, error) {
	page, size = pagex.Normalize(page, size)
	var rows []sqlcgen.LadderRank
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		rows, err = q.ListLadderRanks(ctx, sqlcgen.ListLadderRanksParams{ContestID: contestID, OffsetCount: int32((page - 1) * size), LimitCount: int32(size)})
		return err
	}); err != nil {
		return nil, apperr.ErrContestInvalid.WithCause(err)
	}
	out := make([]LadderRankDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, ladderRankDTOFromRow(row))
	}
	return out, nil
}

// CreateSnapshot 生成或覆盖归档成绩快照。
func (r *repo) CreateSnapshot(ctx context.Context, tenantID, snapshotID, contestID int64, ranks []LadderRankDTO) (ResultSnapshotDTO, error) {
	data, err := ladderRanksBytes(ranks)
	if err != nil {
		return ResultSnapshotDTO{}, err
	}
	var row sqlcgen.ContestResultSnapshot
	if err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var createErr error
		row, createErr = q.CreateResultSnapshot(ctx, sqlcgen.CreateResultSnapshotParams{ID: snapshotID, TenantID: tenantID, ContestID: contestID, FinalRanking: data})
		return createErr
	}); err != nil {
		return ResultSnapshotDTO{}, apperr.ErrContestInvalid.WithCause(err)
	}
	return snapshotDTOFromRow(row), nil
}

// GetSnapshot 读取归档成绩快照。
func (r *repo) GetSnapshot(ctx context.Context, contestID int64) (ResultSnapshotDTO, error) {
	var row sqlcgen.ContestResultSnapshot
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.GetResultSnapshot(ctx, contestID)
		return err
	}); err != nil {
		return ResultSnapshotDTO{}, notFoundOrInternal(err, apperr.ErrContestNotFound)
	}
	return snapshotDTOFromRow(row), nil
}

// ListAchievements 通过队伍成员关系派生学生竞赛成就。
func (r *repo) ListAchievements(ctx context.Context, tenantID, studentID int64) ([]ContestAchievementDTO, error) {
	var rows []sqlcgen.LadderRank
	if err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var err error
		rows, err = q.ListStudentAchievements(ctx, sqlcgen.ListStudentAchievementsParams{StudentID: studentID, OffsetCount: 0, LimitCount: 100})
		return err
	}); err != nil {
		return nil, apperr.ErrContestInvalid.WithCause(err)
	}
	out := make([]ContestAchievementDTO, 0, len(rows))
	for _, row := range rows {
		rank := ladderRankDTOFromRow(row)
		out = append(out, ContestAchievementDTO{ContestID: rank.ContestID, TeamID: rank.TeamID, Score: rank.Score, Rank: rank.Rank})
	}
	return out, nil
}

// CreateCheatRecord 写入作弊判定记录。
func (r *repo) CreateCheatRecord(ctx context.Context, id tenant.Identity, recordID, contestID, teamID int64, req CheatRecordRequest) (CheatRecordDTO, error) {
	evidence, err := jsonx.ObjectBytes(req.Evidence, apperr.ErrContestInvalid)
	if err != nil {
		return CheatRecordDTO{}, err
	}
	var row sqlcgen.CheatRecord
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var createErr error
		row, createErr = q.CreateCheatRecord(ctx, sqlcgen.CreateCheatRecordParams{ID: recordID, TenantID: id.TenantID, ContestID: contestID, TeamID: teamID, Type: req.Type, Evidence: evidence, Action: req.Action, OperatorID: pgInt8(id.AccountID)})
		return createErr
	}); err != nil {
		return CheatRecordDTO{}, apperr.ErrContestInvalid.WithCause(err)
	}
	return cheatRecordDTOFromRow(row), nil
}

// ListCheatRecords 查询作弊记录。
func (r *repo) ListCheatRecords(ctx context.Context, contestID int64, page, size int) ([]CheatRecordDTO, error) {
	page, size = pagex.Normalize(page, size)
	var rows []sqlcgen.CheatRecord
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		rows, err = q.ListCheatRecordsByContest(ctx, sqlcgen.ListCheatRecordsByContestParams{ContestID: contestID, OffsetCount: int32((page - 1) * size), LimitCount: int32(size)})
		return err
	}); err != nil {
		return nil, apperr.ErrContestInvalid.WithCause(err)
	}
	out := make([]CheatRecordDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, cheatRecordDTOFromRow(row))
	}
	return out, nil
}

// CreateVulnSource 创建漏洞源配置。
func (r *repo) CreateVulnSource(ctx context.Context, id tenant.Identity, sourceID int64, req VulnSourceRequest) (VulnSourceDTO, error) {
	config, err := jsonx.ObjectBytes(req.Config, apperr.ErrContestVulnSourceInvalid)
	if err != nil {
		return VulnSourceDTO{}, err
	}
	var row sqlcgen.VulnSource
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var createErr error
		row, createErr = q.CreateVulnSource(ctx, sqlcgen.CreateVulnSourceParams{ID: sourceID, TenantID: pgInt8(id.TenantID), Type: req.Type, Name: req.Name, Config: config, DefaultLevel: req.DefaultLevel, Enabled: req.Enabled})
		return createErr
	}); err != nil {
		return VulnSourceDTO{}, apperr.ErrContestVulnSourceInvalid.WithCause(err)
	}
	return vulnSourceDTOFromRow(row), nil
}

// ListVulnSources 查询漏洞源配置。
func (r *repo) ListVulnSources(ctx context.Context, page, size int) ([]VulnSourceDTO, error) {
	page, size = pagex.Normalize(page, size)
	var rows []sqlcgen.VulnSource
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		rows, err = q.ListVulnSources(ctx, sqlcgen.ListVulnSourcesParams{OffsetCount: int32((page - 1) * size), LimitCount: int32(size)})
		return err
	}); err != nil {
		return nil, apperr.ErrContestVulnSourceInvalid.WithCause(err)
	}
	out := make([]VulnSourceDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, vulnSourceDTOFromRow(row))
	}
	return out, nil
}

// GetVulnSource 读取漏洞源配置。
func (r *repo) GetVulnSource(ctx context.Context, sourceID int64) (VulnSourceDTO, error) {
	var row sqlcgen.VulnSource
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.GetVulnSourceByID(ctx, sourceID)
		return err
	}); err != nil {
		return VulnSourceDTO{}, notFoundOrInternal(err, apperr.ErrContestVulnSourceInvalid)
	}
	return vulnSourceDTOFromRow(row), nil
}

// MarkVulnSourceSynced 更新漏洞源末次同步时间。
func (r *repo) MarkVulnSourceSynced(ctx context.Context, sourceID int64) (VulnSourceDTO, error) {
	var row sqlcgen.VulnSource
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.MarkVulnSourceSynced(ctx, sourceID)
		return err
	}); err != nil {
		return VulnSourceDTO{}, notFoundOrInternal(err, apperr.ErrContestVulnSourceInvalid)
	}
	return vulnSourceDTOFromRow(row), nil
}

// CreateVulnProblem 创建漏洞题草稿。
func (r *repo) CreateVulnProblem(ctx context.Context, id tenant.Identity, problemID int64, req VulnProblemImportRequest) (VulnProblemDTO, error) {
	body, err := jsonx.ObjectBytes(req.DraftBody, apperr.ErrContestVulnProblemInvalid)
	if err != nil {
		return VulnProblemDTO{}, err
	}
	var row sqlcgen.VulnProblem
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var createErr error
		row, createErr = q.CreateVulnProblem(ctx, sqlcgen.CreateVulnProblemParams{
			ID: problemID, TenantID: id.TenantID, SourceID: pgInt8(ids.ParseOrZero(req.SourceID)), ExternalRef: pgText(req.ExternalRef),
			Title: req.Title, Level: req.Level, RuntimeMode: req.RuntimeMode, DraftBody: body, PrevalidateStatus: VulnPrevalidatePending,
			PrevalidateDetail: []byte("{}"), Status: VulnProblemDraft,
		})
		return createErr
	}); err != nil {
		return VulnProblemDTO{}, apperr.ErrContestVulnProblemInvalid.WithCause(err)
	}
	return vulnProblemDTOFromRow(row), nil
}

// GetVulnProblem 读取漏洞题草稿。
func (r *repo) GetVulnProblem(ctx context.Context, problemID int64) (VulnProblemDTO, error) {
	var row sqlcgen.VulnProblem
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.GetVulnProblemByID(ctx, problemID)
		return err
	}); err != nil {
		return VulnProblemDTO{}, notFoundOrInternal(err, apperr.ErrContestVulnProblemInvalid)
	}
	return vulnProblemDTOFromRow(row), nil
}

// UpdateVulnPrevalidate 写入预验证结果。
func (r *repo) UpdateVulnPrevalidate(ctx context.Context, problemID int64, status int16, detail map[string]any) (VulnProblemDTO, error) {
	data, err := jsonx.ObjectBytes(detail, apperr.ErrContestVulnProblemInvalid)
	if err != nil {
		return VulnProblemDTO{}, err
	}
	var row sqlcgen.VulnProblem
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var updateErr error
		row, updateErr = q.UpdateVulnProblemPrevalidate(ctx, sqlcgen.UpdateVulnProblemPrevalidateParams{ID: problemID, PrevalidateStatus: status, PrevalidateDetail: data})
		return updateErr
	}); err != nil {
		return VulnProblemDTO{}, notFoundOrInternal(err, apperr.ErrContestVulnProblemInvalid)
	}
	return vulnProblemDTOFromRow(row), nil
}

// FinalizeVulnProblem 标记漏洞题已经固化到 M5。
func (r *repo) FinalizeVulnProblem(ctx context.Context, problemID int64, code, version string) (VulnProblemDTO, error) {
	var row sqlcgen.VulnProblem
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		var updateErr error
		row, updateErr = q.FinalizeVulnProblem(ctx, sqlcgen.FinalizeVulnProblemParams{ID: problemID, Status: VulnProblemFinalized, ContentItemCode: pgText(code), ContentItemVersion: pgText(version)})
		return updateErr
	}); err != nil {
		return VulnProblemDTO{}, notFoundOrInternal(err, apperr.ErrContestVulnProblemInvalid)
	}
	return vulnProblemDTOFromRow(row), nil
}

// Stats 返回竞赛模块内部统计。
func (r *repo) Stats(ctx context.Context, tenantID int64) (StatsDTO, error) {
	var out StatsDTO
	if err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var err error
		out.ContestCount, err = q.CountContests(ctx, pgInt2(0))
		if err != nil {
			return err
		}
		out.ActiveContestCount, err = q.CountActiveContests(ctx)
		if err != nil {
			return err
		}
		out.TeamCount, err = q.CountContestTeams(ctx)
		return err
	}); err != nil {
		return StatsDTO{}, apperr.ErrContestQueryFailed.WithCause(err)
	}
	out.TenantID = ids.Format(tenantID)
	return out, nil
}

// notFoundOrInternal 把 pgx 未命中转换为模块错误码。
func notFoundOrInternal(err error, notFound *apperr.Error) error {
	if db.IsNoRows(err) {
		return notFound.WithCause(err)
	}
	return apperr.ErrContestQueryFailed.WithCause(err)
}
