// contest service_solve 文件实现解题赛环境、提交、判题事件回写和排行榜推送。
package contest

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/response"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
)

type contestEnvContext struct {
	OrganizerTenantID int64
	MemberTenantID    int64
	MemberAccountID   int64
	Contest           Contest
	Problem           ContestProblem
	Team              Team
	SourceRef         string
}

// contestEnvSourceRefPrefix 返回竞赛题目和队伍环境来源引用的稳定前缀。
func contestEnvSourceRefPrefix(year int, problemID, teamID int64) string {
	return fmt.Sprintf("contest:%04d:solve-env:%s-%s-", year, ids.Format(problemID), ids.Format(teamID))
}

// resolveContestEnvContext 统一解析竞赛环境的队伍、题目和稳定来源引用。
// 创建与刷新恢复必须使用同一来源规则,否则页面重开无法定位原有沙箱。
func (s *Service) resolveContestEnvContext(ctx context.Context, contestID, problemID int64, req EnvRequest) (contestEnvContext, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return contestEnvContext{}, err
	}
	organizerTenantID, err := s.resolvePublishedContestTenant(ctx, contestID)
	if err != nil {
		return contestEnvContext{}, err
	}
	requestRef := strings.TrimSpace(req.RequestRef)
	if requestRef == "" || len(requestRef) > 128 {
		return contestEnvContext{}, apperr.ErrContestSandboxUnavailable
	}
	out := contestEnvContext{OrganizerTenantID: organizerTenantID, MemberTenantID: id.TenantID, MemberAccountID: id.AccountID}
	if err := s.store.TenantTx(ctx, organizerTenantID, func(ctx context.Context, tx TxStore) error {
		out.Contest, err = tx.GetContest(ctx, organizerTenantID, contestID)
		if err != nil {
			return err
		}
		if err := validateContestRunning(out.Contest); err != nil {
			return err
		}
		out.Problem, err = tx.GetContestProblem(ctx, organizerTenantID, problemID)
		if err != nil {
			return err
		}
		if out.Problem.ContestID != contestID {
			return apperr.ErrContestProblemInvalid
		}
		out.Team, err = s.currentAccountTeam(ctx, tx, organizerTenantID, id.TenantID, contestID, id.AccountID)
		if err != nil {
			return err
		}
		hash := sha256.Sum256([]byte(requestRef))
		out.SourceRef = fmt.Sprintf("%s%x", contestEnvSourceRefPrefix(out.Contest.CreatedAt.Year(), problemID, out.Team.ID), hash[:8])
		return nil
	}); err != nil {
		return contestEnvContext{}, err
	}
	return out, nil
}

// CreateEnv 为竞赛题创建学生实操沙箱环境。
func (s *Service) CreateEnv(ctx context.Context, contestID, problemID int64, req EnvRequest) (EnvDTO, error) {
	env, err := s.resolveContestEnvContext(ctx, contestID, problemID, req)
	if err != nil {
		return EnvDTO{}, err
	}
	profile, err := sandboxProfileForContestMode(env.Contest.Mode)
	if err != nil {
		return EnvDTO{}, err
	}
	snapshot, err := compositionFromProblem(env.Problem, profile)
	if err != nil {
		return EnvDTO{}, err
	}
	savedCodeRef, err := s.sandbox.FindSavedSandboxWorkspaceArchiveBySourceRef(ctx, env.OrganizerTenantID, env.SourceRef)
	if err != nil {
		return EnvDTO{}, apperr.ErrContestSandboxUnavailable.WithCause(err)
	}
	if err := s.recycleFailedContestEnv(ctx, env); err != nil {
		return EnvDTO{}, err
	}
	initCodeRef := snapshot.Spec.InitCodeRef
	if savedCodeRef != "" {
		initCodeRef = savedCodeRef
	}
	info, err := s.sandbox.CreateSandbox(ctx, contracts.SandboxCreateRequest{TenantID: env.OrganizerTenantID, CompositionSnapshot: snapshot, InitCodeRef: initCodeRef, InitScriptRef: snapshot.Spec.InitScriptRef, OwnerAccountID: env.Contest.OrganizerID, SourceRef: env.SourceRef, ScopeRef: env.SourceRef, KeepAlive: false, SnapshotEnabled: false})
	if err != nil {
		return EnvDTO{}, apperr.ErrContestSandboxUnavailable.WithCause(err)
	}
	capabilities := contestSandboxGrantCapabilities(info)
	if err := s.store.TenantTx(ctx, env.OrganizerTenantID, func(ctx context.Context, tx TxStore) error {
		contest, err := tx.GetContestForUpdate(ctx, env.OrganizerTenantID, env.Contest.ID)
		if err != nil {
			return err
		}
		if err := validateContestRunning(contest); err != nil {
			return err
		}
		team, err := s.currentAccountTeam(ctx, tx, env.OrganizerTenantID, env.MemberTenantID, env.Contest.ID, env.MemberAccountID)
		if err != nil {
			return err
		}
		if team.ID != env.Team.ID {
			return apperr.ErrContestTeamAccessDenied
		}
		for _, member := range team.Members {
			if _, err := tx.UpsertContestAccessGrant(ctx, ContestAccessGrant{
				ID: s.ids.Generate(), TenantID: env.OrganizerTenantID, ContestID: env.Contest.ID, TeamID: env.Team.ID, SandboxID: info.SandboxID,
				MemberTenantID: member.MemberTenantID, MemberAccountID: member.AccountID,
				Capabilities: capabilities,
				SourceRef:    info.SourceRef, ExpiresAt: contest.EndAt, Status: contestAccessGrantActive,
			}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		// 并发幂等请求可能已授权使用同一沙箱,不能因本次事务失败销毁共享环境。
		// 原来源键允许重试补齐授权;未继续使用的资源仍由M2正式生命周期回收。
		return EnvDTO{}, apperr.ErrContestSandboxUnavailable.WithCause(err)
	}
	return EnvDTO{SandboxID: ids.ID(info.SandboxID), SourceRef: info.SourceRef, Status: info.Status}, nil
}

// recycleFailedContestEnv 仅在用户重新准备时回收确切失败沙箱，避免稳定作用域误伤并发新环境。
func (s *Service) recycleFailedContestEnv(ctx context.Context, env contestEnvContext) error {
	info, err := s.sandbox.GetSandboxBySourceRef(ctx, env.OrganizerTenantID, env.SourceRef)
	if errors.Is(err, apperr.ErrSandboxNotFound) {
		return nil
	}
	if err != nil {
		return apperr.ErrContestSandboxUnavailable.WithCause(err)
	}
	if info.Status != contracts.SandboxStatusFailed {
		return nil
	}
	var revoked []ContestAccessGrant
	if err := s.store.TenantTx(ctx, env.OrganizerTenantID, func(ctx context.Context, tx TxStore) error {
		grants, err := tx.ListContestAccessGrantsForContest(ctx, env.OrganizerTenantID, env.Contest.ID)
		if err != nil {
			return err
		}
		for _, grant := range grants {
			if grant.SandboxID == info.SandboxID {
				revoked = append(revoked, grant)
			}
		}
		return tx.RevokeContestAccessGrantsForSandbox(ctx, env.OrganizerTenantID, info.SandboxID)
	}); err != nil {
		return apperr.ErrContestSandboxUnavailable.WithCause(err)
	}
	if err := s.closeRevokedContestSandboxSessions(ctx, env.OrganizerTenantID, revoked); err != nil {
		return apperr.ErrContestSandboxUnavailable.WithCause(err)
	}
	if err := s.sandbox.DestroySandbox(ctx, contracts.SandboxControlRequest{TenantID: env.OrganizerTenantID, SandboxID: info.SandboxID, SourceRef: info.SourceRef}); err != nil {
		return apperr.ErrContestSandboxUnavailable.WithCause(err)
	}
	return nil
}

// GetEnv 恢复当前队伍仍未销毁的竞赛实操环境,不创建新资源也不接受客户端运行时配置。
func (s *Service) GetEnv(ctx context.Context, contestID, problemID int64, req EnvRequest) (EnvDTO, error) {
	env, err := s.resolveContestEnvContext(ctx, contestID, problemID, req)
	if err != nil {
		return EnvDTO{}, err
	}
	info, err := s.sandbox.GetSandboxBySourceRef(ctx, env.OrganizerTenantID, env.SourceRef)
	if err != nil {
		if errors.Is(err, apperr.ErrSandboxNotFound) {
			return EnvDTO{}, apperr.ErrNotFound.WithCause(err)
		}
		return EnvDTO{}, apperr.ErrContestSandboxUnavailable.WithCause(err)
	}
	var grant ContestAccessGrant
	if err := s.store.TenantTx(ctx, env.OrganizerTenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		grant, err = tx.GetContestAccessGrantForSubject(ctx, env.OrganizerTenantID, info.SandboxID, env.MemberTenantID, env.MemberAccountID)
		return err
	}); err != nil {
		if isNoRows(err) {
			// 首次授权事务失败时仍显示准备入口,由同一POST和幂等键恢复授权。
			return EnvDTO{}, apperr.ErrNotFound.WithCause(err)
		}
		return EnvDTO{}, apperr.ErrContestSandboxUnavailable.WithCause(err)
	}
	if grant.ContestID != env.Contest.ID || grant.TeamID != env.Team.ID || grant.SourceRef != info.SourceRef || grant.Status != contestAccessGrantActive || !grant.ExpiresAt.After(timex.Now()) {
		return EnvDTO{}, apperr.ErrContestTeamAccessDenied
	}
	return EnvDTO{SandboxID: ids.ID(info.SandboxID), SourceRef: info.SourceRef, Status: info.Status}, nil
}

// SubmitSolve 提交解题赛答案或当前授权沙箱,并创建 M3 判题任务。
func (s *Service) SubmitSolve(ctx context.Context, contestID, problemID int64, req SubmitRequest) (SubmissionDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return SubmissionDTO{}, err
	}
	organizerTenantID, err := s.resolvePublishedContestTenant(ctx, contestID)
	if err != nil {
		return SubmissionDTO{}, err
	}
	req.SandboxRef = strings.TrimSpace(req.SandboxRef)
	if req.ContentRef == nil {
		return SubmissionDTO{}, apperr.ErrContestSubmissionInvalid
	}
	var contest Contest
	var problem ContestProblem
	var team Team
	var recentTaskRefs []string
	submissionID := s.ids.Generate()
	sourceRef := submissionSourceRef(submissionID, timex.Now())
	if err := s.store.TenantTx(ctx, organizerTenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		contest, err = tx.GetContest(ctx, organizerTenantID, contestID)
		if err != nil {
			return err
		}
		if err := validateContestRunning(contest); err != nil {
			return err
		}
		if contest.Mode != ContestModeSolve {
			return apperr.ErrContestSubmissionInvalid
		}
		problem, err = tx.GetContestProblem(ctx, organizerTenantID, problemID)
		if err != nil {
			return err
		}
		if problem.ContestID != contestID {
			return apperr.ErrContestProblemInvalid
		}
		team, err = s.currentAccountTeam(ctx, tx, organizerTenantID, id.TenantID, contestID, id.AccountID)
		if err != nil {
			return err
		}
		recent, err := tx.RecentSolveCount(ctx, organizerTenantID, contestID, problemID, team.ID, s.cfg.SubmitRateLimitSeconds)
		if err != nil {
			return err
		}
		if recent > 0 {
			return apperr.ErrContestSubmitRateLimited
		}
		recentTaskRefs, err = tx.ListRecentSolveTaskRefs(ctx, organizerTenantID, contestID, problemID, team.ID, s.cfg.FailedCooldownSeconds)
		return err
	}); err != nil {
		return SubmissionDTO{}, err
	}
	for _, ref := range recentTaskRefs {
		taskID, ok := ids.Parse(ref)
		if !ok {
			return SubmissionDTO{}, apperr.ErrContestJudgeUnavailable.WithCause(fmt.Errorf("invalid stored judge task reference %q", ref))
		}
		task, err := s.judge.GetJudgeTask(ctx, organizerTenantID, taskID)
		if err != nil {
			return SubmissionDTO{}, apperr.ErrContestJudgeUnavailable.WithCause(err)
		}
		if task.Status == contracts.JudgeTaskStatusDone && !task.Result.Passed {
			return SubmissionDTO{}, apperr.ErrContestSubmitRateLimited
		}
	}
	face, err := s.content.GetContentFace(ctx, organizerTenantID, contracts.ContentItemRef{ItemCode: problem.ItemCode, ItemVersion: problem.ItemVersion})
	if err != nil {
		return SubmissionDTO{}, apperr.ErrContestContentUnavailable.WithCause(err)
	}
	submitKey := strings.TrimSpace(jsonx.StringFromAny(face.Body["submit_key"]))
	input, err := s.judge.GetJudgeInputRequirements(ctx, organizerTenantID, problem.ItemCode, problem.ItemVersion, contracts.JudgeInputContext{AccessProfile: contracts.SandboxAccessContestSolve})
	if err != nil {
		return SubmissionDTO{}, apperr.ErrContestJudgeUnavailable.WithCause(err)
	}
	needsSandbox := input.RequiresCode || input.SandboxMode == contracts.JudgeSandboxModeReuse
	if err := validateContestSolveRequest(req, submitKey, needsSandbox); err != nil {
		return SubmissionDTO{}, apperr.ErrContestSubmissionInvalid
	}
	var savedCode contracts.SandboxWorkspaceSave
	var targetSandboxRef, sandboxSourceRef string
	if needsSandbox {
		sandboxID, ok := ids.Parse(req.SandboxRef)
		if !ok {
			return SubmissionDTO{}, apperr.ErrContestSubmissionInvalid
		}
		req.SandboxRef = ids.Format(sandboxID)
		if input.RequiresCode {
			savedCode, err = s.saveContestSubmissionWorkspace(ctx, contest, problem, team, sandboxID, id.TenantID, id.AccountID)
		} else {
			var info contracts.SandboxInfo
			_, info, err = s.contestSubmissionSandbox(ctx, contest, problem, team, sandboxID, id.TenantID, id.AccountID, contestSandboxCapabilityChain)
			targetSandboxRef, sandboxSourceRef = req.SandboxRef, info.SourceRef
		}
		if err != nil {
			return SubmissionDTO{}, err
		}
	}
	if err := s.store.TenantTx(ctx, organizerTenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetContest(ctx, organizerTenantID, contestID)
		if err != nil {
			return err
		}
		return s.validateSolveSubmissionContext(ctx, tx, current, problem, team, id.TenantID, id.AccountID)
	}); err != nil {
		return SubmissionDTO{}, err
	}
	task, err := s.judge.SubmitJudgeTask(ctx, contracts.JudgeSubmitRequest{TenantID: organizerTenantID, ItemCode: problem.ItemCode, ItemVersion: problem.ItemVersion, CodeStorageKey: savedCode.CodeStorageKey, CodeHash: savedCode.CodeHash, SubmitterTenantID: id.TenantID, SubmitterID: id.AccountID, SourceRef: sourceRef, SourceOwnerID: contest.OrganizerID, SourceCourseID: 0, SourceScope: "contest", SandboxMode: input.SandboxMode, TargetSandboxRef: targetSandboxRef, SandboxSourceRef: sandboxSourceRef, ExtraInput: req.ContentRef, Priority: 8})
	if err != nil {
		return SubmissionDTO{}, apperr.ErrContestJudgeUnavailable.WithCause(err)
	}
	item := SolveSubmission{ID: submissionID, TenantID: organizerTenantID, ContestID: contest.ID, ProblemID: problem.ID, TeamID: team.ID, SubmitterTenantID: id.TenantID, SubmitterID: id.AccountID, ContentRef: req.ContentRef, SourceRef: sourceRef, ScopeRef: sourceRef, JudgeTaskRef: ids.Format(task.TaskID), SandboxRef: req.SandboxRef}
	if task.Status == contracts.JudgeTaskStatusDone {
		item.Passed = task.Result.Passed
		item.Score = scaledContestScore(problem.Score, task.Result.Score, task.Result.MaxScore)
	}
	if err := s.store.TenantTx(ctx, organizerTenantID, func(ctx context.Context, tx TxStore) error {
		score := item.Score
		current, err := tx.GetContestForUpdate(ctx, organizerTenantID, contestID)
		if err != nil {
			return err
		}
		if err := s.validateSolveSubmissionContext(ctx, tx, current, problem, team, id.TenantID, id.AccountID); err != nil {
			return err
		}
		recent, err := tx.RecentSolveCount(ctx, organizerTenantID, contestID, problemID, team.ID, s.cfg.SubmitRateLimitSeconds)
		if err != nil {
			return err
		}
		if recent > 0 {
			return apperr.ErrContestSubmitRateLimited
		}
		if item.Passed {
			score, err = s.dynamicSolveScore(ctx, tx, organizerTenantID, contestID, problemID, problem)
			if err != nil {
				return err
			}
		}
		item, err = tx.CreateSolveSubmission(ctx, item)
		if err != nil {
			return err
		}
		if task.Status == contracts.JudgeTaskStatusDone {
			item, err = tx.UpdateSolveSubmissionResult(ctx, item.TenantID, item.ID, task.Result.Passed, score)
			if err != nil {
				return err
			}
			if item.Passed {
				if err := s.refreshTeamRank(ctx, tx, item.TenantID, item.ContestID, item.TeamID); err != nil {
					return err
				}
			}
		}
		return s.writeStudentAudit(ctx, organizerTenantID, id, "contest.submit", auditTargetSolveSubmission, item.ID, map[string]any{"contest_id": contestID, "problem_id": problemID})
	}); err != nil {
		return SubmissionDTO{}, err
	}
	return submissionDTOFromModel(item, task, problem.Score), nil
}

// validateSolveSubmissionContext 在调用方事务中复核同一题目和队伍；最终写入方必须先锁定赛事。
func (s *Service) validateSolveSubmissionContext(ctx context.Context, tx TxStore, contest Contest, problem ContestProblem, team Team, memberTenantID, accountID int64) error {
	if contest.Mode != ContestModeSolve {
		return apperr.ErrContestSubmissionInvalid
	}
	if err := validateContestRunning(contest); err != nil {
		return err
	}
	currentProblem, err := tx.GetContestProblem(ctx, contest.TenantID, problem.ID)
	if err != nil {
		return err
	}
	if currentProblem.ContestID != contest.ID || currentProblem.ItemCode != problem.ItemCode ||
		currentProblem.ItemVersion != problem.ItemVersion || currentProblem.CompositionDigest != problem.CompositionDigest {
		return apperr.ErrContestProblemInvalid
	}
	currentTeam, err := s.currentAccountTeam(ctx, tx, contest.TenantID, memberTenantID, contest.ID, accountID)
	if err != nil {
		return err
	}
	if currentTeam.ID != team.ID {
		return apperr.ErrContestTeamAccessDenied
	}
	return nil
}

// GetSubmission 读取当前账号队伍可见的解题提交。
func (s *Service) GetSubmission(ctx context.Context, submissionID int64) (SubmissionDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return SubmissionDTO{}, err
	}
	organizerTenantID, err := s.resolveSubmissionTenant(ctx, submissionID)
	if err != nil {
		return SubmissionDTO{}, err
	}
	var item SolveSubmission
	var problem ContestProblem
	if err := s.store.TenantTx(ctx, organizerTenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		item, err = tx.GetSolveSubmission(ctx, organizerTenantID, submissionID)
		if err != nil {
			return err
		}
		team, err := tx.GetTeam(ctx, organizerTenantID, item.TeamID)
		if err != nil {
			return err
		}
		if err := ensureTeamAccess(id.TenantID, id.AccountID, team); err != nil {
			return err
		}
		problem, err = tx.GetContestProblem(ctx, organizerTenantID, item.ProblemID)
		return err
	}); err != nil {
		return SubmissionDTO{}, err
	}
	return s.submissionDTO(ctx, item, problem.Score)
}

// GetLatestSubmission 读取当前学生队伍在指定题目下的最近一次提交。
// 竞赛结束后仍允许参赛队成员查看自己的提交历史,用于刷新与关闭重开后的状态恢复。
func (s *Service) GetLatestSubmission(ctx context.Context, contestID, problemID int64) (SubmissionDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return SubmissionDTO{}, err
	}
	organizerTenantID, err := s.resolvePublishedContestTenant(ctx, contestID)
	if err != nil {
		return SubmissionDTO{}, err
	}
	var item SolveSubmission
	var problem ContestProblem
	if err := s.store.TenantTx(ctx, organizerTenantID, func(ctx context.Context, tx TxStore) error {
		contest, err := tx.GetContest(ctx, organizerTenantID, contestID)
		if err != nil {
			return err
		}
		problem, err = tx.GetContestProblem(ctx, organizerTenantID, problemID)
		if err != nil {
			return err
		}
		if problem.ContestID != contest.ID {
			return apperr.ErrContestProblemInvalid
		}
		team, err := s.currentAccountTeam(ctx, tx, organizerTenantID, id.TenantID, contestID, id.AccountID)
		if err != nil {
			return err
		}
		item, err = tx.GetLatestSolveSubmission(ctx, organizerTenantID, contestID, problemID, team.ID)
		return err
	}); err != nil {
		return SubmissionDTO{}, err
	}
	return s.submissionDTO(ctx, item, problem.Score)
}

func (s *Service) submissionDTO(ctx context.Context, item SolveSubmission, maxScore int32) (SubmissionDTO, error) {
	taskID, ok := ids.Parse(item.JudgeTaskRef)
	if !ok {
		return SubmissionDTO{}, apperr.ErrContestJudgeUnavailable.WithCause(fmt.Errorf("invalid stored judge task reference %q", item.JudgeTaskRef))
	}
	task, err := s.judge.GetJudgeTask(ctx, item.TenantID, taskID)
	if err != nil {
		return SubmissionDTO{}, apperr.ErrContestJudgeUnavailable.WithCause(err)
	}
	if task.SourceRef != item.SourceRef {
		return SubmissionDTO{}, apperr.ErrContestEventSourceMismatch
	}
	return submissionDTOFromModel(item, task, maxScore), nil
}

// HandleSolveJudgeCompleted 消费 M3 判题完成事件并回写解题结果。
func (s *Service) HandleSolveJudgeCompleted(ctx context.Context, event contracts.JudgeCompletedEvent) error {
	return s.recordSolveJudgement(ctx, event.TenantID, event.TaskID, event.SourceRef, contracts.JudgeTaskStatusDone)
}

// HandleSolveJudgeFailed 撤回执行失败提交的得分并重算榜单。
func (s *Service) HandleSolveJudgeFailed(ctx context.Context, event contracts.JudgeFailedEvent) error {
	return s.recordSolveJudgement(ctx, event.TenantID, event.TaskID, event.SourceRef, contracts.JudgeTaskStatusFailed)
}

func (s *Service) recordSolveJudgement(ctx context.Context, tenantID, taskID int64, sourceRef string, status int16) error {
	if tenantID <= 0 || taskID <= 0 || !validContestSourceRef(sourceRef) {
		return apperr.ErrContestEventPayloadInvalid
	}
	task, err := s.judge.GetJudgeTask(ctx, tenantID, taskID)
	if err != nil {
		return apperr.ErrContestJudgeUnavailable.WithCause(err)
	}
	if task.SourceRef != sourceRef {
		return apperr.ErrContestEventSourceMismatch
	}
	if task.Status != status {
		return nil
	}
	return s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		sub, err := tx.GetSolveSubmissionByJudgeTask(ctx, tenantID, ids.Format(taskID))
		if err != nil {
			if isNoRows(err) {
				return apperr.ErrContestSubmissionNotFound
			}
			return err
		}
		if sub.SourceRef != sourceRef {
			return apperr.ErrContestEventSourceMismatch
		}
		passed := status == contracts.JudgeTaskStatusDone && task.Result.Passed
		var score int32
		if status == contracts.JudgeTaskStatusDone {
			problem, err := tx.GetContestProblem(ctx, tenantID, sub.ProblemID)
			if err != nil {
				return err
			}
			score = scaledContestScore(problem.Score, task.Result.Score, task.Result.MaxScore)
			if passed {
				// 已结算的同一提交不因重复完成事件再次衰减。
				if sub.Passed {
					score = sub.Score
				} else {
					score, err = s.dynamicSolveScore(ctx, tx, tenantID, sub.ContestID, sub.ProblemID, problem)
					if err != nil {
						return err
					}
				}
			}
		}
		updated, err := tx.UpdateSolveSubmissionResult(ctx, tenantID, sub.ID, passed, score)
		if err != nil {
			return err
		}
		return s.refreshTeamRank(ctx, tx, tenantID, updated.ContestID, updated.TeamID)
	})
}

// ListLadder 按已发布竞赛读取权限返回榜单；封榜和归档阶段只返回对应快照。
func (s *Service) ListLadder(ctx context.Context, contestID int64, page, size int) ([]LadderDTO, int64, int, int, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	organizerTenantID, err := s.resolvePublishedContestTenant(ctx, contestID)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	page, size = pagex.Normalize(page, size)
	var out []LadderDTO
	var total int64
	if err := s.store.TenantTx(ctx, organizerTenantID, func(ctx context.Context, tx TxStore) error {
		if err := s.ensurePublishedContestReadAccess(ctx, tx, organizerTenantID, contestID, id); err != nil {
			return err
		}
		contest, err := s.loadContestForRead(ctx, tx, organizerTenantID, id.AccountID, contestID)
		if err != nil {
			return err
		}
		if contest.Status == ContestStatusFrozen || contest.Status == ContestStatusArchived {
			snapshot, err := tx.GetLadderSnapshot(ctx, organizerTenantID, contestID, contest.Status)
			if err != nil {
				return err
			}
			all := ladderDTOsFromSnapshot(snapshot, contest.Mode)
			total = int64(len(all))
			start := (page - 1) * size
			if start >= len(all) {
				out = []LadderDTO{}
				return nil
			}
			end := min(start+size, len(all))
			out, err = fillLadderTeamNames(ctx, tx, organizerTenantID, contestID, all[start:end])
			return err
		}
		ranks, count, err := tx.ListLadder(ctx, organizerTenantID, contestID, page, size)
		if err != nil {
			return err
		}
		total = count
		out = make([]LadderDTO, 0, len(ranks))
		for _, rank := range ranks {
			out = append(out, ladderDTOFromModel(rank))
		}
		out, err = fillLadderTeamNames(ctx, tx, organizerTenantID, contestID, out)
		return err
	}); err != nil {
		return nil, 0, 0, 0, err
	}
	return out, total, page, size, nil
}

// fillLadderTeamNames 给榜单行填队名:按这一页的队伍编号一次读取(榜单与快照只存 team_id)。
func fillLadderTeamNames(ctx context.Context, tx TxStore, tenantID, contestID int64, items []LadderDTO) ([]LadderDTO, error) {
	teamIDs := make([]int64, 0, len(items))
	for _, item := range items {
		teamIDs = append(teamIDs, item.TeamID.Int64())
	}
	names, err := tx.TeamNames(ctx, tenantID, contestID, teamIDs)
	if err != nil {
		return nil, err
	}
	return withLadderTeamNames(items, names), nil
}

// refreshTeamRank 在调用方持有赛事锁时重算通过得分、累计处罚和解题进度。
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

// RunLeaderboardPushOnce 分页交付待推送榜单；单场失败不阻断其他赛事。
func (s *Service) RunLeaderboardPushOnce(ctx context.Context) error {
	if s.cfg.MatchmakerBatchSize <= 0 {
		return apperr.ErrContestNotifyFailed
	}
	afterID := int64(0)
	var deliveryErr error
	for {
		if err := ctx.Err(); err != nil {
			return errors.Join(deliveryErr, err)
		}
		var contests []Contest
		if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
			var err error
			contests, err = tx.ListPendingLeaderboardContests(ctx, s.cfg.MatchmakerBatchSize, afterID)
			return err
		}); err != nil {
			return errors.Join(deliveryErr, err)
		}
		for _, contest := range contests {
			if err := ctx.Err(); err != nil {
				return errors.Join(deliveryErr, err)
			}
			afterID = contest.ID
			if err := s.pushLeaderboard(ctx, contest.TenantID, contest.ID); err != nil {
				deliveryErr = errors.Join(deliveryErr, fmt.Errorf("推送榜单失败 tenant_id=%d contest_id=%d: %w", contest.TenantID, contest.ID, err))
			}
		}
		if len(contests) < s.cfg.MatchmakerBatchSize {
			return deliveryErr
		}
	}
}

// pushLeaderboard 持锁核验赛态，持久投递成功才清除标记，防止并发计分丢失更新。
func (s *Service) pushLeaderboard(ctx context.Context, tenantID, contestID int64) error {
	return s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		contest, err := tx.GetPendingLeaderboardContest(ctx, tenantID, contestID)
		if err != nil {
			if isNoRows(err) {
				return nil
			}
			return err
		}
		if contest.Status != ContestStatusRunning {
			return tx.CompleteLeaderboardPush(ctx, tenantID, contestID)
		}
		payload, err := json.Marshal(LeaderboardPushDTO{ContestID: ids.ID(contestID)})
		if err != nil {
			return apperr.ErrContestNotifyFailed.WithCause(err)
		}
		evt := contracts.NotifyPushRequestedEvent{TenantID: tenantID, TraceID: response.TraceFromContext(ctx), Topic: leaderboardTopic(tenantID, contestID), Payload: payload}
		if err := s.bus.Publish(ctx, contracts.SubjectNotifyPushRequested, evt); err != nil {
			return apperr.ErrContestNotifyFailed.WithCause(err)
		}
		return tx.CompleteLeaderboardPush(ctx, tenantID, contestID)
	})
}

// scaledContestScore 将 M3 原始分按竞赛题配置分值归一。
func scaledContestScore(maxScore, score, judgeMax int32) int32 {
	if judgeMax <= 0 {
		if score > maxScore {
			return maxScore
		}
		return score
	}
	scaled := (int64(score)*int64(maxScore) + int64(judgeMax)/2) / int64(judgeMax)
	if scaled < math.MinInt32 {
		return math.MinInt32
	}
	if scaled > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(scaled)
}

// dynamicSolveScore 按题目动态分配置计算通过提交得分。
func (s *Service) dynamicSolveScore(ctx context.Context, tx TxStore, tenantID, contestID, problemID int64, problem ContestProblem) (int32, error) {
	if problem.DynamicScore == nil {
		return problem.Score, nil
	}
	solved, err := tx.CountProblemSolvedTeams(ctx, tenantID, contestID, problemID)
	if err != nil {
		return 0, err
	}
	minScore := problem.DynamicScore.MinScore
	decay := problem.DynamicScore.DecayPerSolve
	if solved < 0 || minScore <= 0 || minScore > problem.Score || decay <= 0 {
		return 0, apperr.ErrContestProblemInvalid
	}
	// 先比较达到下限所需的队伍数，剩余乘积不会超过题目分值。
	if solved > int64((problem.Score-minScore)/decay) {
		return minScore, nil
	}
	return problem.Score - int32(solved)*decay, nil
}
