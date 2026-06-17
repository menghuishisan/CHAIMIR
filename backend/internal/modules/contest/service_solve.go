// contest service_solve 文件实现解题赛环境、提交、判题事件回写和排行榜推送。
package contest

import (
	"context"
	"fmt"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
)

// CreateEnv 为竞赛题创建学生实操沙箱环境。
func (s *Service) CreateEnv(ctx context.Context, contestID, problemID int64, req EnvRequest) (EnvDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return EnvDTO{}, err
	}
	var sourceRef string
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		contest, err := tx.GetContest(ctx, id.TenantID, contestID)
		if err != nil {
			return err
		}
		if err := validateContestRunning(contest); err != nil {
			return err
		}
		problem, err := tx.GetContestProblem(ctx, id.TenantID, problemID)
		if err != nil {
			return err
		}
		if problem.ContestID != contestID {
			return apperr.ErrContestProblemInvalid
		}
		if _, err := s.currentAccountTeam(ctx, tx, id.TenantID, contestID, id.AccountID); err != nil {
			return err
		}
		sourceRef = contestSourceRef(contestID, contest.CreatedAt)
		return nil
	}); err != nil {
		return EnvDTO{}, err
	}
	info, err := s.sandbox.CreateSandbox(ctx, contracts.SandboxCreateRequest{TenantID: id.TenantID, RuntimeCode: req.RuntimeCode, RuntimeImageVersion: req.RuntimeImageVersion, ToolCodes: req.ToolCodes, InitCodeRef: req.InitCodeRef, InitScriptRef: req.InitScriptRef, OwnerAccountID: id.AccountID, SourceRef: sourceRef, KeepAlive: false, SnapshotEnabled: false})
	if err != nil {
		return EnvDTO{}, apperr.ErrContestSandboxUnavailable.WithCause(err)
	}
	return EnvDTO{SandboxID: info.SandboxID, SourceRef: info.SourceRef, Status: info.Status}, nil
}

// SubmitSolve 提交解题赛代码并创建 M3 判题任务。
func (s *Service) SubmitSolve(ctx context.Context, contestID, problemID int64, req SubmitRequest) (SubmissionDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return SubmissionDTO{}, err
	}
	req.CodeHash = strings.TrimSpace(req.CodeHash)
	if req.ContentRef == nil || req.CodeStorageKey == "" || !isSHA256Hex(req.CodeHash) {
		return SubmissionDTO{}, apperr.ErrContestSubmissionInvalid
	}
	var contest Contest
	var problem ContestProblem
	var team Team
	submissionID := s.ids.Generate()
	sourceRef := submissionSourceRef(submissionID, timex.Now())
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		contest, err = tx.GetContest(ctx, id.TenantID, contestID)
		if err != nil {
			return err
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
		team, err = s.currentAccountTeam(ctx, tx, id.TenantID, contestID, id.AccountID)
		if err != nil {
			return err
		}
		recent, err := tx.RecentSolveCount(ctx, id.TenantID, contestID, problemID, team.ID, s.cfg.SubmitRateLimitSeconds)
		if err != nil {
			return err
		}
		if recent > 0 {
			return apperr.ErrContestSubmitRateLimited
		}
		failed, err := tx.RecentFailedSolveCount(ctx, id.TenantID, contestID, problemID, team.ID, s.cfg.FailedCooldownSeconds)
		if err != nil {
			return err
		}
		if failed > 0 {
			return apperr.ErrContestSubmitRateLimited
		}
		return nil
	}); err != nil {
		return SubmissionDTO{}, err
	}
	task, err := s.judge.SubmitJudgeTask(ctx, contracts.JudgeSubmitRequest{TenantID: id.TenantID, JudgerCode: "contest-solve", ItemCode: problem.ItemCode, ItemVersion: problem.ItemVersion, CodeStorageKey: req.CodeStorageKey, CodeHash: req.CodeHash, SubmitterID: id.AccountID, SourceRef: sourceRef, SourceOwnerID: id.AccountID, SourceCourseID: 0, SourceScope: "contest", SandboxMode: sandboxModeForSolve(req), TargetSandboxRef: req.SandboxRef, ExtraInput: map[string]any{"contest_id": contestID, "problem_id": problemID, "content_ref": req.ContentRef}, Priority: 8})
	if err != nil {
		return SubmissionDTO{}, apperr.ErrContestJudgeUnavailable.WithCause(err)
	}
	item := SolveSubmission{ID: submissionID, TenantID: id.TenantID, ContestID: contest.ID, ProblemID: problem.ID, TeamID: team.ID, SubmitterID: id.AccountID, ContentRef: req.ContentRef, SourceRef: sourceRef, JudgeTaskRef: ids.Format(task.TaskID), SandboxRef: req.SandboxRef}
	if task.Status == contracts.JudgeTaskStatusDone {
		item.Passed = task.Result.Passed
		item.Score = scaledContestScore(problem.Score, task.Result.Score, task.Result.MaxScore)
	}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		if item.Passed {
			item.Score, err = s.dynamicSolveScore(ctx, tx, id.TenantID, contestID, problemID, problem)
			if err != nil {
				return err
			}
		}
		item, err = tx.CreateSolveSubmission(ctx, item)
		if err != nil {
			return err
		}
		if item.Passed {
			item, err = tx.UpdateSolveSubmissionResult(ctx, item.TenantID, item.ID, true, item.Score)
			if err != nil {
				return err
			}
			return s.refreshTeamRank(ctx, tx, item.TenantID, item.ContestID, item.TeamID)
		}
		return nil
	}); err != nil {
		return SubmissionDTO{}, err
	}
	if item.Passed {
		if err := s.pushLeaderboard(ctx, item.TenantID, item.ContestID); err != nil {
			return SubmissionDTO{}, err
		}
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumStudent, "contest.submit", auditTargetSolveSubmission, item.ID, map[string]any{"contest_id": contestID, "problem_id": problemID}); err != nil {
		return SubmissionDTO{}, err
	}
	return submissionDTOFromModel(item), nil
}

// GetSubmission 读取当前账号队伍可见的解题提交。
func (s *Service) GetSubmission(ctx context.Context, submissionID int64) (SubmissionDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return SubmissionDTO{}, err
	}
	var item SolveSubmission
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		item, err = tx.GetSolveSubmission(ctx, id.TenantID, submissionID)
		if err != nil {
			return err
		}
		team, err := tx.GetTeam(ctx, id.TenantID, item.TeamID)
		if err != nil {
			return err
		}
		return ensureTeamAccess(id.TenantID, id.AccountID, team)
	}); err != nil {
		return SubmissionDTO{}, err
	}
	return submissionDTOFromModel(item), nil
}

// HandleSolveJudgeCompleted 消费 M3 判题完成事件并回写解题结果。
func (s *Service) HandleSolveJudgeCompleted(ctx context.Context, event contracts.JudgeCompletedEvent) error {
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
	var updated SolveSubmission
	if err := s.store.TenantTx(ctx, event.TenantID, func(ctx context.Context, tx TxStore) error {
		sub, err := tx.GetSolveSubmissionByJudgeTask(ctx, event.TenantID, ids.Format(event.TaskID))
		if err != nil {
			if isNoRows(err) {
				return apperr.ErrContestSubmissionNotFound
			}
			return err
		}
		if sub.SourceRef != event.SourceRef {
			return apperr.ErrContestEventSourceMismatch
		}
		problem, err := tx.GetContestProblem(ctx, event.TenantID, sub.ProblemID)
		if err != nil {
			return err
		}
		score := scaledContestScore(problem.Score, task.Result.Score, task.Result.MaxScore)
		if task.Result.Passed {
			score, err = s.dynamicSolveScore(ctx, tx, event.TenantID, sub.ContestID, sub.ProblemID, problem)
			if err != nil {
				return err
			}
		}
		updated, err = tx.UpdateSolveSubmissionResult(ctx, event.TenantID, sub.ID, task.Result.Passed, score)
		if err != nil {
			return err
		}
		if updated.Passed {
			return s.refreshTeamRank(ctx, tx, event.TenantID, updated.ContestID, updated.TeamID)
		}
		return nil
	}); err != nil {
		return err
	}
	if updated.Passed {
		return s.pushLeaderboard(ctx, updated.TenantID, updated.ContestID)
	}
	return nil
}

// HandleSolveJudgeFailed 消费 M3 判题失败事件并把提交记为失败零分。
func (s *Service) HandleSolveJudgeFailed(ctx context.Context, event contracts.JudgeFailedEvent) error {
	if event.TenantID <= 0 || event.TaskID <= 0 || !validContestSourceRef(event.SourceRef) {
		return apperr.ErrContestEventPayloadInvalid
	}
	return s.store.TenantTx(ctx, event.TenantID, func(ctx context.Context, tx TxStore) error {
		sub, err := tx.GetSolveSubmissionByJudgeTask(ctx, event.TenantID, ids.Format(event.TaskID))
		if err != nil {
			if isNoRows(err) {
				return apperr.ErrContestSubmissionNotFound
			}
			return err
		}
		if sub.SourceRef != event.SourceRef {
			return apperr.ErrContestEventSourceMismatch
		}
		_, err = tx.UpdateSolveSubmissionResult(ctx, event.TenantID, sub.ID, false, 0)
		return err
	})
}

// ListLadder 查询排行榜。
func (s *Service) ListLadder(ctx context.Context, contestID int64, page, size int) ([]LadderDTO, int64, int, int, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	var ranks []LadderRank
	var total int64
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		ranks, total, err = tx.ListLadder(ctx, id.TenantID, contestID, page, size)
		return err
	}); err != nil {
		return nil, 0, 0, 0, err
	}
	out := make([]LadderDTO, 0, len(ranks))
	for _, rank := range ranks {
		out = append(out, ladderDTOFromModel(rank))
	}
	return out, total, page, size, nil
}

// refreshTeamRank 依据已通过提交重算单队成绩并刷新全榜排名。
func (s *Service) refreshTeamRank(ctx context.Context, tx TxStore, tenantID, contestID, teamID int64) error {
	rank, err := tx.SumTeamSolvedScore(ctx, tenantID, contestID, teamID)
	if err != nil {
		return err
	}
	rank.ID = s.ids.Generate()
	if _, err := tx.UpsertLadder(ctx, rank); err != nil {
		return err
	}
	return tx.RefreshContestRanks(ctx, tenantID, contestID)
}

// pushLeaderboard 通过 M10 统一实时推送排行榜变化。
func (s *Service) pushLeaderboard(ctx context.Context, tenantID, contestID int64) error {
	var ranks []LadderRank
	shouldPush := false
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		contest, err := tx.GetContest(ctx, tenantID, contestID)
		if err != nil {
			return err
		}
		if contest.Status != ContestStatusRunning {
			return nil
		}
		ranks, _, err = tx.ListLadder(ctx, tenantID, contestID, 1, 50)
		shouldPush = err == nil
		return err
	}); err != nil {
		return err
	}
	if !shouldPush {
		return nil
	}
	payload := map[string]any{"contest_id": contestID, "items": ranks}
	if err := s.notify.Push(ctx, contracts.NotifyPushRequest{TenantID: tenantID, Topic: fmt.Sprintf("tenant:%d:contest:%d:leaderboard", tenantID, contestID), Payload: payload}); err != nil {
		return apperr.ErrContestNotifyFailed.WithCause(err)
	}
	return nil
}

// sandboxModeForSolve 根据提交是否绑定现场沙箱决定判题模式。
func sandboxModeForSolve(req SubmitRequest) string {
	if strings.TrimSpace(req.SandboxRef) != "" {
		return contracts.JudgeSandboxModeReuse
	}
	return contracts.JudgeSandboxModeFresh
}

// scaledContestScore 将 M3 原始分按竞赛题配置分值归一。
func scaledContestScore(maxScore, score, judgeMax int32) int32 {
	if judgeMax <= 0 {
		if score > maxScore {
			return maxScore
		}
		return score
	}
	return int32((int64(score)*int64(maxScore) + int64(judgeMax)/2) / int64(judgeMax))
}

// dynamicSolveScore 按题目动态分配置计算通过提交得分。
func (s *Service) dynamicSolveScore(ctx context.Context, tx TxStore, tenantID, contestID, problemID int64, problem ContestProblem) (int32, error) {
	if len(problem.DynamicScore) == 0 {
		return problem.Score, nil
	}
	solved, err := tx.CountProblemSolvedTeams(ctx, tenantID, contestID, problemID)
	if err != nil {
		return 0, err
	}
	minScore := int32FromMap(problem.DynamicScore, "min_score", problem.Score)
	decay := int32FromMap(problem.DynamicScore, "decay_per_solve", 0)
	score := problem.Score - int32(solved)*decay
	if score < minScore {
		score = minScore
	}
	if score <= 0 {
		return 0, apperr.ErrContestProblemInvalid
	}
	return score, nil
}

// int32FromMap 从动态配置读取整数。
func int32FromMap(m map[string]any, key string, defaultValue int32) int32 {
	switch v := m[key].(type) {
	case float64:
		return int32(v)
	case int:
		return int32(v)
	case int32:
		return v
	default:
		return defaultValue
	}
}
