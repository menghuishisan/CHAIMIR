// contest service_battle 文件实现对抗赛参战物、撮合、执行、结算和回放读取。
package contest

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
)

// SubmitBattleEntry 提交对抗赛参战物并为可用对手创建待执行对局。
func (s *Service) SubmitBattleEntry(ctx context.Context, contestID int64, req BattleEntryRequest) (BattleEntryDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return BattleEntryDTO{}, err
	}
	req, err = validateBattleEntryRequest(req)
	if err != nil {
		return BattleEntryDTO{}, err
	}
	var entry BattleEntry
	var opponents []BattleEntry
	var problem ContestProblem
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		contest, err := tx.GetContest(ctx, id.TenantID, contestID)
		if err != nil {
			return err
		}
		if contest.Mode != ContestModeBattle {
			return apperr.ErrContestBattleEntryInvalid
		}
		if err := validateContestRunning(contest); err != nil {
			return err
		}
		problem, err = tx.GetContestProblem(ctx, id.TenantID, req.ProblemID)
		if err != nil {
			return err
		}
		if problem.ContestID != contestID {
			return apperr.ErrContestProblemInvalid
		}
		team, err := s.currentAccountTeam(ctx, tx, id.TenantID, contestID, id.AccountID)
		if err != nil {
			return err
		}
		version, err := tx.NextBattleVersion(ctx, id.TenantID, contestID, req.ProblemID, team.ID, req.Role)
		if err != nil {
			return err
		}
		if err := tx.DeactivateBattleEntries(ctx, id.TenantID, contestID, req.ProblemID, team.ID, req.Role); err != nil {
			return err
		}
		entry, err = tx.CreateBattleEntry(ctx, BattleEntry{ID: s.ids.Generate(), TenantID: id.TenantID, ContestID: contestID, ProblemID: req.ProblemID, TeamID: team.ID, Role: req.Role, ArtifactRef: req.ArtifactRef, VersionNo: version})
		if err != nil {
			return err
		}
		opponents, err = tx.ListActiveBattleOpponents(ctx, id.TenantID, contestID, req.ProblemID, entry.ID, team.ID, contest.MatchMode, s.cfg.MatchmakerBatchSize)
		if err != nil {
			return err
		}
		for _, opponent := range opponents {
			if !battleRolesCompatible(problem.BattleRule, entry.Role, opponent.Role) {
				continue
			}
			matchID := s.ids.Generate()
			if _, err := tx.CreateBattleMatch(ctx, BattleMatch{ID: matchID, TenantID: id.TenantID, ContestID: contestID, ProblemID: req.ProblemID, EntryAID: entry.ID, EntryBID: opponent.ID, SourceRef: battleSourceRef(matchID, timex.Now())}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return BattleEntryDTO{}, err
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, audit.ActorRoleStudent, "contest.battle.entry.submit", auditTargetBattleEntry, entry.ID, map[string]any{"contest_id": contestID, "problem_id": req.ProblemID}); err != nil {
		return BattleEntryDTO{}, err
	}
	return battleEntryDTOFromModel(entry), nil
}

// ListBattleEntries 查询当前账号队伍的参战物列表。
func (s *Service) ListBattleEntries(ctx context.Context, contestID int64) ([]BattleEntryDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var entries []BattleEntry
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		team, err := s.currentAccountTeam(ctx, tx, id.TenantID, contestID, id.AccountID)
		if err != nil {
			return err
		}
		entries, err = tx.ListBattleEntriesForTeam(ctx, id.TenantID, contestID, team.ID)
		return err
	}); err != nil {
		return nil, err
	}
	out := make([]BattleEntryDTO, 0, len(entries))
	for _, entry := range entries {
		out = append(out, battleEntryDTOFromModel(entry))
	}
	return out, nil
}

// ListBattleMatches 查询当前账号队伍的对局历史。
func (s *Service) ListBattleMatches(ctx context.Context, contestID int64, page, size int) ([]BattleMatchDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var matches []BattleMatch
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		team, err := s.currentAccountTeam(ctx, tx, id.TenantID, contestID, id.AccountID)
		if err != nil {
			return err
		}
		matches, err = tx.ListBattleMatchesForTeam(ctx, id.TenantID, contestID, team.ID, page, size)
		return err
	}); err != nil {
		return nil, err
	}
	out := make([]BattleMatchDTO, 0, len(matches))
	for _, match := range matches {
		out = append(out, battleMatchDTOFromModel(match))
	}
	return out, nil
}

// GetBattleReplay 读取对局回放引用,只向参赛队伍成员开放。
func (s *Service) GetBattleReplay(ctx context.Context, matchID int64) (map[string]any, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var match BattleMatch
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		match, err = tx.GetBattleMatch(ctx, id.TenantID, matchID)
		if err != nil {
			return err
		}
		a, err := tx.GetBattleEntry(ctx, id.TenantID, match.EntryAID)
		if err != nil {
			return err
		}
		b, err := tx.GetBattleEntry(ctx, id.TenantID, match.EntryBID)
		if err != nil {
			return err
		}
		teamA, err := tx.GetTeam(ctx, id.TenantID, a.TeamID)
		if err != nil {
			return err
		}
		teamB, err := tx.GetTeam(ctx, id.TenantID, b.TeamID)
		if err != nil {
			return err
		}
		if ensureTeamAccess(id.TenantID, id.AccountID, teamA) == nil || ensureTeamAccess(id.TenantID, id.AccountID, teamB) == nil {
			return nil
		}
		return apperr.ErrContestTeamAccessDenied
	}); err != nil {
		return nil, err
	}
	if match.ReplayRef == "" {
		return nil, apperr.ErrContestReplayUnavailable
	}
	return map[string]any{"match_id": match.ID, "replay_ref": match.ReplayRef}, nil
}

// RunMatchmakerOnce 执行一次待对局认领和启动,供统一 background runner 调用。
func (s *Service) RunMatchmakerOnce(ctx context.Context) error {
	var matches []BattleMatch
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		matches, err = tx.ClaimPendingBattleMatches(ctx, s.cfg.MatchmakerBatchSize)
		return err
	}); err != nil {
		return err
	}
	for _, match := range matches {
		if err := s.executeBattleMatch(ctx, match); err != nil {
			return err
		}
	}
	return nil
}

// executeBattleMatch 创建对局沙箱并提交 M3 判题任务。
func (s *Service) executeBattleMatch(ctx context.Context, match BattleMatch) error {
	var problem ContestProblem
	var entryA BattleEntry
	var entryB BattleEntry
	var ownerID int64
	if err := s.store.TenantTx(ctx, match.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		problem, err = tx.GetContestProblem(ctx, match.TenantID, match.ProblemID)
		if err != nil {
			return err
		}
		entryA, err = tx.GetBattleEntry(ctx, match.TenantID, match.EntryAID)
		if err != nil {
			return err
		}
		entryB, err = tx.GetBattleEntry(ctx, match.TenantID, match.EntryBID)
		if err != nil {
			return err
		}
		ownerID, err = teamLeaderID(ctx, tx, match.TenantID, entryA.TeamID)
		return err
	}); err != nil {
		return err
	}
	spec, err := battleRuntimeSpecFromProblem(problem)
	if err != nil {
		if failErr := s.markBattleFailed(ctx, match); failErr != nil {
			return apperr.ErrContestBattleMatchFailed.WithCause(fmt.Errorf("对局配置无效: %w; 标记对局失败也失败: %v", err, failErr))
		}
		return err
	}
	info, err := s.sandbox.CreateSandbox(ctx, contracts.SandboxCreateRequest{TenantID: match.TenantID, RuntimeCode: spec.RuntimeCode, RuntimeImageVersion: spec.RuntimeImageVersion, ToolCodes: spec.ToolCodes, OwnerAccountID: ownerID, SourceRef: match.SourceRef, KeepAlive: false, SnapshotEnabled: true})
	if err != nil {
		if failErr := s.markBattleFailed(ctx, match); failErr != nil {
			return apperr.ErrContestBattleMatchFailed.WithCause(fmt.Errorf("创建对局沙箱失败: %w; 标记对局失败也失败: %v", err, failErr))
		}
		return apperr.ErrContestSandboxUnavailable.WithCause(err)
	}
	task, err := s.judge.SubmitJudgeTask(ctx, contracts.JudgeSubmitRequest{TenantID: match.TenantID, JudgerCode: spec.JudgerCode, ItemCode: problem.ItemCode, ItemVersion: problem.ItemVersion, CodeStorageKey: entryA.ArtifactRef, SubmitterID: ownerID, SourceRef: match.SourceRef, SandboxMode: contracts.JudgeSandboxModeReuse, TargetSandboxRef: strconv.FormatInt(info.SandboxID, 10), ExtraInput: map[string]any{"entry_a": entryA.ArtifactRef, "entry_b": entryB.ArtifactRef, "role_a": entryA.Role, "role_b": entryB.Role}, Priority: 9})
	if err != nil {
		if recycleErr := s.sandbox.RecycleBySourceRef(ctx, contracts.SandboxRecycleRequest{TenantID: match.TenantID, SourceRef: match.SourceRef, Reason: "battle_judge_submit_failed"}); recycleErr != nil {
			return apperr.ErrContestBattleMatchFailed.WithCause(fmt.Errorf("提交对局判题失败: %w; 回收沙箱失败: %v", err, recycleErr))
		}
		if failErr := s.markBattleFailed(ctx, match); failErr != nil {
			return apperr.ErrContestBattleMatchFailed.WithCause(fmt.Errorf("提交对局判题失败: %w; 标记对局失败也失败: %v", err, failErr))
		}
		return apperr.ErrContestJudgeUnavailable.WithCause(err)
	}
	return s.store.TenantTx(ctx, match.TenantID, func(ctx context.Context, tx TxStore) error {
		_, err := tx.StartBattleMatch(ctx, match.TenantID, match.ID, strconv.FormatInt(info.SandboxID, 10), strconv.FormatInt(task.TaskID, 10))
		return err
	})
}

// HandleBattleJudgeCompleted 消费 M3 判题完成事件并结算对局。
func (s *Service) HandleBattleJudgeCompleted(ctx context.Context, event contracts.JudgeCompletedEvent) error {
	if event.TenantID <= 0 || event.TaskID <= 0 || !validContestSourceRef(event.SourceRef) {
		return apperr.ErrContestEventPayloadInvalid
	}
	task, err := s.judge.GetJudgeTask(ctx, event.TenantID, event.TaskID)
	if err != nil {
		return apperr.ErrContestJudgeUnavailable.WithCause(err)
	}
	if task.SourceRef != event.SourceRef {
		return apperr.ErrContestEventSourceMismatch
	}
	var match BattleMatch
	if err := s.store.TenantTx(ctx, event.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetBattleMatchByJudgeTask(ctx, event.TenantID, strconv.FormatInt(event.TaskID, 10))
		if err != nil {
			if isNoRows(err) {
				return apperr.ErrContestBattleMatchNotFound
			}
			return err
		}
		if current.SourceRef != event.SourceRef {
			return apperr.ErrContestEventSourceMismatch
		}
		a, err := tx.GetBattleEntry(ctx, event.TenantID, current.EntryAID)
		if err != nil {
			return err
		}
		b, err := tx.GetBattleEntry(ctx, event.TenantID, current.EntryBID)
		if err != nil {
			return err
		}
		result := battleResultFromTask(task.Result)
		deltaA, deltaB := battleScoreDelta(result)
		current.Result = result
		current.ScoreDelta = map[string]any{"team_a": a.TeamID, "team_b": b.TeamID, "delta_a": deltaA, "delta_b": deltaB}
		current.ReplayRef = task.Result.SnapshotRef
		match, err = tx.FinishBattleMatch(ctx, current)
		if err != nil {
			return err
		}
		if err := s.applyBattleRankDelta(ctx, tx, event.TenantID, current.ContestID, a.TeamID, deltaA); err != nil {
			return err
		}
		if err := s.applyBattleRankDelta(ctx, tx, event.TenantID, current.ContestID, b.TeamID, deltaB); err != nil {
			return err
		}
		return tx.RefreshContestRanks(ctx, event.TenantID, current.ContestID)
	}); err != nil {
		return err
	}
	if err := s.sandbox.RecycleBySourceRef(ctx, contracts.SandboxRecycleRequest{TenantID: match.TenantID, SourceRef: match.SourceRef, Reason: "battle_finished"}); err != nil {
		return apperr.ErrContestSandboxUnavailable.WithCause(err)
	}
	if err := s.pushLeaderboard(ctx, match.TenantID, match.ContestID); err != nil {
		return err
	}
	return s.writeAudit(ctx, match.TenantID, 0, audit.ActorRoleSystem, "contest.battle.finish", auditTargetBattleMatch, match.ID, match.ScoreDelta)
}

// HandleBattleJudgeFailed 消费 M3 判题失败事件并标记对局失败。
func (s *Service) HandleBattleJudgeFailed(ctx context.Context, event contracts.JudgeFailedEvent) error {
	if event.TenantID <= 0 || event.TaskID <= 0 || !validContestSourceRef(event.SourceRef) {
		return apperr.ErrContestEventPayloadInvalid
	}
	var match BattleMatch
	if err := s.store.TenantTx(ctx, event.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetBattleMatchByJudgeTask(ctx, event.TenantID, strconv.FormatInt(event.TaskID, 10))
		if err != nil {
			if isNoRows(err) {
				return apperr.ErrContestBattleMatchNotFound
			}
			return err
		}
		if current.SourceRef != event.SourceRef {
			return apperr.ErrContestEventSourceMismatch
		}
		match, err = tx.FailBattleMatch(ctx, event.TenantID, current.ID)
		return err
	}); err != nil {
		return err
	}
	if err := s.sandbox.RecycleBySourceRef(ctx, contracts.SandboxRecycleRequest{TenantID: match.TenantID, SourceRef: match.SourceRef, Reason: "battle_failed"}); err != nil {
		return apperr.ErrContestSandboxUnavailable.WithCause(err)
	}
	return nil
}

// applyBattleRankDelta 把对抗赛积分增量写入排行榜投影。
func (s *Service) applyBattleRankDelta(ctx context.Context, tx TxStore, tenantID, contestID, teamID int64, delta float64) error {
	rank, err := tx.GetLadderByTeam(ctx, tenantID, contestID, teamID)
	if err != nil {
		if isNoRows(err) {
			rank = LadderRank{ID: s.ids.Generate(), TenantID: tenantID, ContestID: contestID, TeamID: teamID, Score: 1000, SolvedCount: 0}
		} else {
			return err
		}
	}
	rank.ID = s.ids.Generate()
	rank.Score += delta
	rank.LastSolveAt = timex.Now()
	_, err = tx.UpsertLadder(ctx, rank)
	return err
}

// markBattleFailed 标记对局失败,用于启动阶段补偿。
func (s *Service) markBattleFailed(ctx context.Context, match BattleMatch) error {
	return s.store.TenantTx(ctx, match.TenantID, func(ctx context.Context, tx TxStore) error {
		_, err := tx.FailBattleMatch(ctx, match.TenantID, match.ID)
		return err
	})
}

type battleRuntimeSpec struct {
	JudgerCode          string
	RuntimeCode         string
	RuntimeImageVersion string
	ToolCodes           []string
}

// battleRuntimeSpecFromProblem 从题目配置读取对抗执行所需运行时,配置缺失时显式失败。
func battleRuntimeSpecFromProblem(problem ContestProblem) (battleRuntimeSpec, error) {
	get := func(key string) string {
		if v, ok := problem.DynamicScore[key].(string); ok {
			return strings.TrimSpace(v)
		}
		return ""
	}
	spec := battleRuntimeSpec{JudgerCode: get("judger_code"), RuntimeCode: get("runtime_code"), RuntimeImageVersion: get("runtime_image_version")}
	if raw, ok := problem.DynamicScore["tool_codes"].([]any); ok {
		for _, item := range raw {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				spec.ToolCodes = append(spec.ToolCodes, strings.TrimSpace(s))
			}
		}
	}
	if spec.JudgerCode == "" || spec.RuntimeCode == "" || spec.RuntimeImageVersion == "" {
		return battleRuntimeSpec{}, apperr.ErrContestProblemInvalid
	}
	return spec, nil
}

// battleRolesCompatible 判断两份参战物是否可组成对局。
func battleRolesCompatible(rule, a, b int16) bool {
	if rule == BattleRuleGame {
		return a == BattleRoleStrategy && b == BattleRoleStrategy
	}
	return (a == BattleRoleAttack && b == BattleRoleDefense) || (a == BattleRoleDefense && b == BattleRoleAttack)
}

// teamLeaderID 读取队伍负责人账号,后台创建对局沙箱时作为归属账号。
func teamLeaderID(ctx context.Context, tx TxStore, tenantID, teamID int64) (int64, error) {
	team, err := tx.GetTeam(ctx, tenantID, teamID)
	if err != nil {
		return 0, err
	}
	for _, member := range team.Members {
		if member.IsLeader {
			return member.AccountID, nil
		}
	}
	if len(team.Members) == 0 {
		return 0, apperr.ErrContestTeamInvalid
	}
	return team.Members[0].AccountID, nil
}

// battleResultFromTask 从判题结果中提取胜负,没有显式结果时使用通过状态兜底判定。
func battleResultFromTask(result contracts.JudgeTaskResult) int16 {
	for _, detail := range result.Details {
		value := strings.ToLower(strings.TrimSpace(detail.Actual))
		switch value {
		case "a_win", "a", "attack_win":
			return BattleResultAWin
		case "b_win", "b", "defense_win":
			return BattleResultBWin
		case "draw", "tie":
			return BattleResultDraw
		}
	}
	if result.Passed {
		return BattleResultAWin
	}
	return BattleResultBWin
}

// battleScoreDelta 计算对抗赛 ELO 简化增量。
func battleScoreDelta(result int16) (float64, float64) {
	switch result {
	case BattleResultAWin:
		return 16, -16
	case BattleResultBWin:
		return -16, 16
	default:
		return 0, 0
	}
}
