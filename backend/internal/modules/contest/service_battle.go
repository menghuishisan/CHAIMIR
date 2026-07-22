// contest service_battle 文件实现对抗赛参战物、撮合、执行、结算和回放读取。
package contest

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/pagex"
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
	problemID := req.ProblemID.Int64()
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
		problem, err = tx.GetContestProblem(ctx, id.TenantID, problemID)
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
		version, err := tx.NextBattleVersion(ctx, id.TenantID, contestID, problemID, team.ID, req.Role)
		if err != nil {
			return err
		}
		if err := tx.DeactivateBattleEntries(ctx, id.TenantID, contestID, problemID, team.ID, req.Role); err != nil {
			return err
		}
		entry, err = tx.CreateBattleEntry(ctx, BattleEntry{ID: s.ids.Generate(), TenantID: id.TenantID, ContestID: contestID, ProblemID: problemID, TeamID: team.ID, Role: req.Role, ArtifactRef: req.ArtifactRef, ArtifactHash: req.CodeHash, VersionNo: version})
		if err != nil {
			return err
		}
		opponents, err = tx.ListActiveBattleOpponents(ctx, id.TenantID, contestID, problemID, entry.ID, team.ID, contest.MatchMode, s.cfg.MatchmakerBatchSize, s.cfg.BattleELOInitialScore)
		if err != nil {
			return err
		}
		for _, opponent := range opponents {
			if !battleRolesCompatible(problem.BattleRule, entry.Role, opponent.Role) {
				continue
			}
			matchID := s.ids.Generate()
			if _, err := tx.CreateBattleMatch(ctx, BattleMatch{ID: matchID, TenantID: id.TenantID, ContestID: contestID, ProblemID: problemID, EntryAID: entry.ID, EntryBID: opponent.ID, SourceRef: battleSourceRef(matchID, timex.Now())}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return BattleEntryDTO{}, err
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumStudent, "contest.battle.entry.submit", auditTargetBattleEntry, entry.ID, map[string]any{"contest_id": contestID, "problem_id": req.ProblemID}); err != nil {
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

// ListBattleMatches 查询当前账号队伍的对局历史分页。
func (s *Service) ListBattleMatches(ctx context.Context, contestID int64, page, size int) ([]BattleMatchDTO, int64, int, int, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, 0, page, size, err
	}
	page, size = pagex.Normalize(page, size)
	var matches []BattleMatch
	var total int64
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		team, err := s.currentAccountTeam(ctx, tx, id.TenantID, contestID, id.AccountID)
		if err != nil {
			return err
		}
		matches, total, err = tx.ListBattleMatchesForTeam(ctx, id.TenantID, contestID, team.ID, page, size)
		return err
	}); err != nil {
		return nil, 0, page, size, err
	}
	out := make([]BattleMatchDTO, 0, len(matches))
	for _, match := range matches {
		out = append(out, battleMatchDTOFromModel(match))
	}
	return out, total, page, size, nil
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
	return map[string]any{"match_id": ids.Format(match.ID), "replay_ref": match.ReplayRef}, nil
}

// RunMatchmakerOnce 执行一次待对局认领和启动,供统一 background runner 调用。
func (s *Service) RunMatchmakerOnce(ctx context.Context) error {
	if err := s.reconcileRunningBattleMatches(ctx); err != nil {
		return err
	}
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

// reconcileRunningBattleMatches 补偿已发布但消费失败的 M3 终态事件,仍只通过 M3 contract 读取判题结果。
func (s *Service) reconcileRunningBattleMatches(ctx context.Context) error {
	var matches []BattleMatch
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		matches, err = tx.ListRunningBattleMatchesWithJudgeTask(ctx, s.cfg.MatchmakerBatchSize)
		return err
	}); err != nil {
		return err
	}
	for _, match := range matches {
		if err := s.reconcileBattleMatch(ctx, match); err != nil {
			return err
		}
	}
	return nil
}

// reconcileBattleMatch 根据 M3 任务终态幂等结算对局,避免 NATS 短暂失败后对局永久停留在 running。
func (s *Service) reconcileBattleMatch(ctx context.Context, match BattleMatch) error {
	taskID, err := strconv.ParseInt(strings.TrimSpace(match.JudgeTaskRef), 10, 64)
	if err != nil {
		return apperr.ErrContestBattleMatchFailed.WithCause(err)
	}
	if taskID <= 0 {
		return apperr.ErrContestBattleMatchFailed.WithCause(fmt.Errorf("invalid judge_task_ref %q", match.JudgeTaskRef))
	}
	task, err := s.judge.GetJudgeTask(ctx, match.TenantID, taskID)
	if err != nil {
		return apperr.ErrContestJudgeUnavailable.WithCause(err)
	}
	switch task.Status {
	case contracts.JudgeTaskStatusDone:
		return s.HandleBattleJudgeCompleted(ctx, contracts.JudgeCompletedEvent{TenantID: match.TenantID, TaskID: taskID, SourceRef: match.SourceRef, Status: task.Status, Score: task.Result.Score, Passed: task.Result.Passed, FinishedAt: timex.Now()})
	case contracts.JudgeTaskStatusFailed, contracts.JudgeTaskStatusCanceled:
		return s.HandleBattleJudgeFailed(ctx, contracts.JudgeFailedEvent{TenantID: match.TenantID, TaskID: taskID, SourceRef: match.SourceRef, Reason: "judge_terminal_state", FailedAt: timex.Now()})
	default:
		return nil
	}
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
	info, err := s.sandbox.CreateSandbox(ctx, contracts.SandboxCreateRequest{TenantID: match.TenantID, RuntimeCode: spec.RuntimeCode, RuntimeImageVersion: spec.RuntimeImageVersion, ToolCodes: spec.ToolCodes, OwnerAccountID: ownerID, SourceRef: match.SourceRef, KeepAlive: false, SnapshotEnabled: false})
	if err != nil {
		if failErr := s.markBattleFailed(ctx, match); failErr != nil {
			return apperr.ErrContestBattleMatchFailed.WithCause(fmt.Errorf("创建对局沙箱失败: %w; 标记对局失败也失败: %v", err, failErr))
		}
		return apperr.ErrContestSandboxUnavailable.WithCause(err)
	}
	if err := s.prepareBattleSandbox(ctx, match, info.SandboxID, entryA, entryB); err != nil {
		if recycleErr := s.sandbox.RecycleBySourceRef(ctx, contracts.SandboxRecycleRequest{TenantID: match.TenantID, SourceRef: match.SourceRef, Reason: "battle_sandbox_prepare_failed"}); recycleErr != nil {
			return apperr.ErrContestBattleMatchFailed.WithCause(fmt.Errorf("准备对局沙箱失败: %w; 回收沙箱失败: %v", err, recycleErr))
		}
		if failErr := s.markBattleFailed(ctx, match); failErr != nil {
			return apperr.ErrContestBattleMatchFailed.WithCause(fmt.Errorf("准备对局沙箱失败: %w; 标记对局失败也失败: %v", err, failErr))
		}
		return apperr.ErrContestSandboxUnavailable.WithCause(err)
	}
	task, err := s.judge.SubmitJudgeTask(ctx, contracts.JudgeSubmitRequest{TenantID: match.TenantID, ItemCode: problem.ItemCode, ItemVersion: problem.ItemVersion, SubmitterID: ownerID, SourceRef: match.SourceRef, SourceOwnerID: ownerID, SourceCourseID: 0, SourceScope: "contest", SandboxMode: contracts.JudgeSandboxModeReuse, TargetSandboxRef: strconv.FormatInt(info.SandboxID, 10), ExtraInput: map[string]any{"entry_a": entryA.ArtifactRef, "entry_b": entryB.ArtifactRef, "entry_a_hash": entryA.ArtifactHash, "entry_b_hash": entryB.ArtifactHash, "role_a": entryA.Role, "role_b": entryB.Role}, Priority: 9})
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
		_, err := tx.StartBattleMatch(ctx, match.TenantID, match.ID, ids.Format(info.SandboxID), ids.Format(task.TaskID))
		return err
	})
}

// prepareBattleSandbox 等待对局沙箱就绪后恢复双方参战物到工作区。
func (s *Service) prepareBattleSandbox(ctx context.Context, match BattleMatch, sandboxID int64, entryA, entryB BattleEntry) error {
	if err := s.waitBattleSandboxReady(ctx, match.TenantID, sandboxID); err != nil {
		return err
	}
	for _, item := range []struct {
		entry BattleEntry
		dir   string
	}{
		{entry: entryA, dir: "battle/entry_a"},
		{entry: entryB, dir: "battle/entry_b"},
	} {
		if err := s.sandbox.RestoreSandboxArchive(ctx, contracts.SandboxArchiveRestoreRequest{TenantID: match.TenantID, SandboxID: sandboxID, SourceRef: match.SourceRef, ObjectRef: item.entry.ArtifactRef, ExpectedHash: item.entry.ArtifactHash, TargetDir: item.dir}); err != nil {
			return err
		}
	}
	return nil
}

// waitBattleSandboxReady 轮询 M2 状态,确保工作区恢复和判题复用发生在沙箱可执行后。
func (s *Service) waitBattleSandboxReady(ctx context.Context, tenantID, sandboxID int64) error {
	timeout := time.Duration(s.cfg.BattleSandboxReadyTimeoutSeconds) * time.Second
	interval := time.Duration(s.cfg.BattleSandboxReadyPollIntervalMs) * time.Millisecond
	if timeout <= 0 || interval <= 0 {
		return apperr.ErrContestSandboxUnavailable
	}
	deadline := timex.Now().Add(timeout)
	for {
		info, err := s.sandbox.GetSandbox(ctx, tenantID, sandboxID)
		if err != nil {
			return apperr.ErrContestSandboxUnavailable.WithCause(err)
		}
		if info.Status == contracts.SandboxStatusFailed || info.Status == contracts.SandboxStatusDestroyed {
			return apperr.ErrContestSandboxUnavailable
		}
		if (info.Status == contracts.SandboxStatusReady || info.Status == contracts.SandboxStatusRunning || info.Status == contracts.SandboxStatusIdle) && info.Phase >= contracts.SandboxPhaseReady {
			return nil
		}
		if timex.Now().After(deadline) {
			return apperr.ErrContestSandboxUnavailable
		}
		select {
		case <-ctx.Done():
			return apperr.ErrContestSandboxUnavailable.WithCause(ctx.Err())
		case <-time.After(interval):
		}
	}
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
		current, err := tx.GetBattleMatchByJudgeTask(ctx, event.TenantID, ids.Format(event.TaskID))
		if err != nil {
			if isNoRows(err) {
				return apperr.ErrContestBattleMatchNotFound
			}
			return err
		}
		if current.SourceRef != event.SourceRef {
			return apperr.ErrContestEventSourceMismatch
		}
		problem, err := tx.GetContestProblem(ctx, event.TenantID, current.ProblemID)
		if err != nil {
			return err
		}
		a, err := tx.GetBattleEntry(ctx, event.TenantID, current.EntryAID)
		if err != nil {
			return err
		}
		b, err := tx.GetBattleEntry(ctx, event.TenantID, current.EntryBID)
		if err != nil {
			return err
		}
		result, err := battleResultFromTask(task.Result, problem.BattleRule, a.Role, b.Role)
		if err != nil {
			return err
		}
		ratingA, err := s.battleRatingForTeam(ctx, tx, event.TenantID, current.ContestID, a.TeamID)
		if err != nil {
			return err
		}
		ratingB, err := s.battleRatingForTeam(ctx, tx, event.TenantID, current.ContestID, b.TeamID)
		if err != nil {
			return err
		}
		deltaA, deltaB := battleScoreDelta(result, ratingA, ratingB, s.cfg.BattleELOKFactor)
		current.Result = result
		current.ScoreDelta = map[string]any{
			"team_a":          a.TeamID,
			"team_b":          b.TeamID,
			"rating_a_before": ratingA,
			"rating_b_before": ratingB,
			"rating_a_after":  ratingA + deltaA,
			"rating_b_after":  ratingB + deltaB,
			"delta_a":         deltaA,
			"delta_b":         deltaB,
			"k_factor":        s.cfg.BattleELOKFactor,
			"result":          result,
		}
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
		current, err := tx.GetBattleMatchByJudgeTask(ctx, event.TenantID, ids.Format(event.TaskID))
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
			rank = LadderRank{ID: s.ids.Generate(), TenantID: tenantID, ContestID: contestID, TeamID: teamID, Score: s.cfg.BattleELOInitialScore, SolvedCount: 0}
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

// battleRatingForTeam 读取队伍当前 ELO,首次参战时使用统一配置的初始分。
func (s *Service) battleRatingForTeam(ctx context.Context, tx TxStore, tenantID, contestID, teamID int64) (float64, error) {
	rank, err := tx.GetLadderByTeam(ctx, tenantID, contestID, teamID)
	if err != nil {
		if isNoRows(err) {
			return s.cfg.BattleELOInitialScore, nil
		}
		return 0, err
	}
	return rank.Score, nil
}

// markBattleFailed 标记对局失败,用于启动阶段补偿。
func (s *Service) markBattleFailed(ctx context.Context, match BattleMatch) error {
	return s.store.TenantTx(ctx, match.TenantID, func(ctx context.Context, tx TxStore) error {
		_, err := tx.FailBattleMatch(ctx, match.TenantID, match.ID)
		return err
	})
}

type battleRuntimeSpec struct {
	RuntimeCode         string
	RuntimeImageVersion string
	ToolCodes           []string
}

// battleRuntimeSpecFromProblem 从题目配置读取对抗执行所需运行时,配置缺失时显式失败。
func battleRuntimeSpecFromProblem(problem ContestProblem) (battleRuntimeSpec, error) {
	get := func(key string) string {
		if v, ok := problem.BattleConfig[key].(string); ok {
			return strings.TrimSpace(v)
		}
		return ""
	}
	spec := battleRuntimeSpec{RuntimeCode: get("runtime_code"), RuntimeImageVersion: get("runtime_image_version")}
	if raw, ok := problem.BattleConfig["tool_codes"].([]any); ok {
		for _, item := range raw {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				spec.ToolCodes = append(spec.ToolCodes, strings.TrimSpace(s))
			}
		}
	}
	if spec.RuntimeCode == "" || spec.RuntimeImageVersion == "" {
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

// battleResultFromTask 从判题结果和对抗规则提取胜负,攻防型按断言是否攻破映射到攻击/防守角色。
func battleResultFromTask(result contracts.JudgeTaskResult, rule, roleA, roleB int16) (int16, error) {
	for _, detail := range result.Details {
		value := strings.ToLower(strings.TrimSpace(detail.Actual))
		switch value {
		case "a_win", "a", "attack_win":
			return BattleResultAWin, nil
		case "b_win", "b", "defense_win":
			return BattleResultBWin, nil
		case "draw", "tie":
			return BattleResultDraw, nil
		}
	}
	if rule == BattleRuleAttackDefense {
		if roleA == BattleRoleAttack && roleB == BattleRoleDefense {
			if result.Passed {
				return BattleResultAWin, nil
			}
			return BattleResultBWin, nil
		}
		if roleA == BattleRoleDefense && roleB == BattleRoleAttack {
			if result.Passed {
				return BattleResultBWin, nil
			}
			return BattleResultAWin, nil
		}
	}
	return 0, apperr.ErrContestBattleMatchFailed
}

// battleScoreDelta 按标准 ELO 公式计算双方积分增量。
func battleScoreDelta(result int16, ratingA, ratingB, kFactor float64) (float64, float64) {
	expectedA := 1 / (1 + math.Pow(10, (ratingB-ratingA)/400))
	actualA := 0.5
	switch result {
	case BattleResultAWin:
		actualA = 1
	case BattleResultBWin:
		actualA = 0
	}
	deltaA := kFactor * (actualA - expectedA)
	return deltaA, -deltaA
}
