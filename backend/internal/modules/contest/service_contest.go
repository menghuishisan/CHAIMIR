// contest service_contest 文件实现竞赛定义、赛题编排、生命周期和归档快照。
package contest

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/pkg/apperr"
)

// ListContests 查询当前租户竞赛列表。
func (s *Service) ListContests(ctx context.Context, status int16, page, size int) ([]ContestDTO, int64, int, int, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	var items []Contest
	var total int64
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		items, total, err = tx.ListContests(ctx, id.TenantID, status, page, size)
		return err
	}); err != nil {
		return nil, 0, 0, 0, err
	}
	out := make([]ContestDTO, 0, len(items))
	for _, item := range items {
		out = append(out, contestDTOFromModel(item))
	}
	return out, total, page, size, nil
}

// CreateContest 创建竞赛草稿并持久化完整赛程配置。
func (s *Service) CreateContest(ctx context.Context, req ContestRequest) (ContestDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ContestDTO{}, err
	}
	req, err = validateContestRequest(req)
	if err != nil {
		return ContestDTO{}, err
	}
	item := Contest{ID: s.ids.Generate(), TenantID: id.TenantID, OrganizerID: id.AccountID, Name: req.Name, Mode: req.Mode, MatchMode: req.MatchMode, TeamMode: req.TeamMode, SignupStart: req.SignupStart, SignupEnd: req.SignupEnd, StartAt: req.StartAt, EndAt: req.EndAt, FreezeMinutes: req.FreezeMinutes, Rules: req.Rules}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		item, err = tx.CreateContest(ctx, item)
		return err
	}); err != nil {
		return ContestDTO{}, err
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, audit.ActorRoleTeacher, "contest.create", auditTargetContest, item.ID, nil); err != nil {
		return ContestDTO{}, err
	}
	return contestDTOFromModel(item), nil
}

// UpdateContest 更新草稿竞赛定义。
func (s *Service) UpdateContest(ctx context.Context, contestID int64, req ContestRequest) (ContestDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ContestDTO{}, err
	}
	req, err = validateContestRequest(req)
	if err != nil {
		return ContestDTO{}, err
	}
	var item Contest
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := s.loadContestForManage(ctx, tx, id.TenantID, id.AccountID, contestID)
		if err != nil {
			return err
		}
		current.Name = req.Name
		current.Mode = req.Mode
		current.MatchMode = req.MatchMode
		current.TeamMode = req.TeamMode
		current.SignupStart = req.SignupStart
		current.SignupEnd = req.SignupEnd
		current.StartAt = req.StartAt
		current.EndAt = req.EndAt
		current.FreezeMinutes = req.FreezeMinutes
		current.Rules = req.Rules
		item, err = tx.UpdateContest(ctx, current)
		return err
	}); err != nil {
		return ContestDTO{}, err
	}
	return contestDTOFromModel(item), s.writeAudit(ctx, id.TenantID, id.AccountID, audit.ActorRoleTeacher, "contest.update", auditTargetContest, item.ID, nil)
}

// AddProblem 添加或更新竞赛题目引用,并校验 M5 题面可读取。
func (s *Service) AddProblem(ctx context.Context, contestID int64, req ProblemRequest) (ProblemDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ProblemDTO{}, err
	}
	var contest Contest
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		contest, err = s.loadContestForManage(ctx, tx, id.TenantID, id.AccountID, contestID)
		return err
	}); err != nil {
		return ProblemDTO{}, err
	}
	req, err = validateProblemRequest(req, contest.Mode)
	if err != nil {
		return ProblemDTO{}, err
	}
	if _, err := s.content.GetContentFace(ctx, id.TenantID, contracts.ContentItemRef{ItemCode: req.ItemCode, ItemVersion: req.ItemVersion}); err != nil {
		return ProblemDTO{}, apperr.ErrContestContentUnavailable.WithCause(err)
	}
	item := ContestProblem{ID: s.ids.Generate(), TenantID: id.TenantID, ContestID: contestID, ItemCode: req.ItemCode, ItemVersion: req.ItemVersion, Score: req.Score, DynamicScore: req.DynamicScore, BattleRule: req.BattleRule, Seq: req.Seq}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		item, err = tx.UpsertContestProblem(ctx, item)
		return err
	}); err != nil {
		return ProblemDTO{}, err
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, audit.ActorRoleTeacher, "contest.problem.upsert", auditTargetContest, contestID, map[string]any{"problem_id": item.ID}); err != nil {
		return ProblemDTO{}, err
	}
	return problemDTOFromModel(item), nil
}

// ListProblems 展开竞赛题目列表,并按题面视角补充 M5 内容摘要。
func (s *Service) ListProblems(ctx context.Context, contestID int64) ([]ProblemDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var items []ContestProblem
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if _, err := tx.GetContest(ctx, id.TenantID, contestID); err != nil {
			return err
		}
		var err error
		items, err = tx.ListContestProblems(ctx, id.TenantID, contestID)
		return err
	}); err != nil {
		return nil, err
	}
	out := make([]ProblemDTO, 0, len(items))
	for _, item := range items {
		dto := problemDTOFromModel(item)
		face, err := s.content.GetContentFace(ctx, id.TenantID, contracts.ContentItemRef{ItemCode: item.ItemCode, ItemVersion: item.ItemVersion})
		if err != nil {
			return nil, apperr.ErrContestContentUnavailable.WithCause(err)
		}
		dto.Face = face.Body
		out = append(out, dto)
	}
	return out, nil
}

// PublishContest 发布竞赛到报名中,发布前必须至少配置一道题并登记内容引用。
func (s *Service) PublishContest(ctx context.Context, contestID int64) (ContestDTO, error) {
	return s.transitionContest(ctx, contestID, ContestStatusSignup, "contest.publish", true)
}

// StartContest 将报名中竞赛切换到进行中。
func (s *Service) StartContest(ctx context.Context, contestID int64) (ContestDTO, error) {
	return s.transitionContest(ctx, contestID, ContestStatusRunning, "contest.start", false)
}

// EndContest 将运行中竞赛切换到已结束。
func (s *Service) EndContest(ctx context.Context, contestID int64) (ContestDTO, error) {
	return s.transitionContest(ctx, contestID, ContestStatusEnded, "contest.end", false)
}

// FreezeContest 将运行中竞赛切换到封榜期。
func (s *Service) FreezeContest(ctx context.Context, contestID int64) (ContestDTO, error) {
	return s.transitionContest(ctx, contestID, ContestStatusFrozen, "contest.freeze", false)
}

// ArchiveContest 生成最终榜单快照,归档竞赛并回收竞赛级沙箱资源。
func (s *Service) ArchiveContest(ctx context.Context, contestID int64) (ResultSnapshot, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ResultSnapshot{}, err
	}
	var contest Contest
	var snapshot ResultSnapshot
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		contest, err = s.loadContestForManage(ctx, tx, id.TenantID, id.AccountID, contestID)
		if err != nil {
			return err
		}
		if err := validateContestTransition(contest.Status, ContestStatusArchived); err != nil {
			return err
		}
		ranks, _, err := tx.ListLadder(ctx, id.TenantID, contestID, 1, 1000)
		if err != nil {
			return err
		}
		final := make([]map[string]any, 0, len(ranks))
		for _, rank := range ranks {
			final = append(final, map[string]any{"team_id": rank.TeamID, "score": rank.Score, "solved_count": rank.SolvedCount, "rank": rank.Rank})
		}
		snapshot, err = tx.CreateResultSnapshot(ctx, ResultSnapshot{ID: s.ids.Generate(), TenantID: id.TenantID, ContestID: contestID, FinalRanking: final})
		if err != nil {
			return err
		}
		_, err = tx.SetContestStatus(ctx, id.TenantID, contestID, ContestStatusArchived)
		return err
	}); err != nil {
		return ResultSnapshot{}, err
	}
	if err := s.sandbox.RecycleBySourceRef(ctx, contracts.SandboxRecycleRequest{TenantID: id.TenantID, SourceRef: contestSourceRef(contest.ID, contest.CreatedAt), Reason: "contest_archive"}); err != nil {
		return ResultSnapshot{}, apperr.ErrContestSandboxUnavailable.WithCause(err)
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, audit.ActorRoleTeacher, "contest.archive", auditTargetContest, contestID, map[string]any{"snapshot_id": snapshot.ID}); err != nil {
		return ResultSnapshot{}, err
	}
	return snapshot, nil
}

// RunAutoArchiveOnce 执行一次竞赛自动收尾扫描,供统一 background runner 调用。
func (s *Service) RunAutoArchiveOnce(ctx context.Context) error {
	var items []Contest
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		items, err = tx.ClaimAutoArchiveContests(ctx, s.cfg.MatchmakerBatchSize)
		return err
	}); err != nil {
		return err
	}
	for _, item := range items {
		if _, err := s.archiveContestSystem(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// archiveContestSystem 执行后台归档,复用人工归档的快照与回收规则。
func (s *Service) archiveContestSystem(ctx context.Context, item Contest) (ResultSnapshot, error) {
	var snapshot ResultSnapshot
	if err := s.store.TenantTx(ctx, item.TenantID, func(ctx context.Context, tx TxStore) error {
		ranks, _, err := tx.ListLadder(ctx, item.TenantID, item.ID, 1, 1000)
		if err != nil {
			return err
		}
		final := make([]map[string]any, 0, len(ranks))
		for _, rank := range ranks {
			final = append(final, map[string]any{"team_id": rank.TeamID, "score": rank.Score, "solved_count": rank.SolvedCount, "rank": rank.Rank})
		}
		snapshot, err = tx.CreateResultSnapshot(ctx, ResultSnapshot{ID: s.ids.Generate(), TenantID: item.TenantID, ContestID: item.ID, FinalRanking: final})
		if err != nil {
			return err
		}
		_, err = tx.SetContestStatus(ctx, item.TenantID, item.ID, ContestStatusArchived)
		return err
	}); err != nil {
		return ResultSnapshot{}, err
	}
	if err := s.sandbox.RecycleBySourceRef(ctx, contracts.SandboxRecycleRequest{TenantID: item.TenantID, SourceRef: contestSourceRef(item.ID, item.CreatedAt), Reason: "contest_auto_archive"}); err != nil {
		return ResultSnapshot{}, apperr.ErrContestSandboxUnavailable.WithCause(err)
	}
	if err := s.writeAudit(ctx, item.TenantID, 0, audit.ActorRoleSystem, "contest.archive.auto", auditTargetContest, item.ID, map[string]any{"snapshot_id": snapshot.ID}); err != nil {
		return ResultSnapshot{}, err
	}
	return snapshot, nil
}

// GetSnapshot 读取归档最终榜单快照。
func (s *Service) GetSnapshot(ctx context.Context, contestID int64) (ResultSnapshot, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ResultSnapshot{}, err
	}
	var snapshot ResultSnapshot
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		snapshot, err = tx.GetResultSnapshot(ctx, id.TenantID, contestID)
		return err
	}); err != nil {
		return ResultSnapshot{}, err
	}
	return snapshot, nil
}

// transitionContest 封装竞赛状态流转、内容引用登记和审计。
func (s *Service) transitionContest(ctx context.Context, contestID int64, next int16, action string, requireProblems bool) (ContestDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ContestDTO{}, err
	}
	var item Contest
	var problems []ContestProblem
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := s.loadContestForManage(ctx, tx, id.TenantID, id.AccountID, contestID)
		if err != nil {
			return err
		}
		if err := validateContestTransition(current.Status, next); err != nil {
			return err
		}
		problems, err = tx.ListContestProblems(ctx, id.TenantID, contestID)
		if err != nil {
			return err
		}
		if requireProblems && len(problems) == 0 {
			return apperr.ErrContestProblemInvalid
		}
		if next == ContestStatusRunning {
			if err := tx.LockContestTeams(ctx, id.TenantID, contestID); err != nil {
				return err
			}
		}
		item = current
		return nil
	}); err != nil {
		return ContestDTO{}, err
	}
	if requireProblems {
		if err := s.incrementProblemUsage(ctx, item.TenantID, problems); err != nil {
			return ContestDTO{}, err
		}
	}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		item, err = tx.SetContestStatus(ctx, id.TenantID, contestID, next)
		return err
	}); err != nil {
		return ContestDTO{}, err
	}
	return contestDTOFromModel(item), s.writeAudit(ctx, id.TenantID, id.AccountID, audit.ActorRoleTeacher, action, auditTargetContest, item.ID, nil)
}

// incrementProblemUsage 在发布时登记 M5 内容引用,用于删除保护和复用统计。
func (s *Service) incrementProblemUsage(ctx context.Context, tenantID int64, problems []ContestProblem) error {
	seen := map[string]bool{}
	for _, problem := range problems {
		key := problem.ItemCode + ":" + problem.ItemVersion
		if seen[key] {
			continue
		}
		seen[key] = true
		if err := s.content.IncrementUsage(ctx, tenantID, contracts.ContentItemRef{ItemCode: problem.ItemCode, ItemVersion: problem.ItemVersion}); err != nil {
			return apperr.ErrContestContentUnavailable.WithCause(err)
		}
	}
	return nil
}
