// contest service_contest 文件实现竞赛定义、赛题编排、生命周期和归档快照。
package contest

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	pkgcrypto "chaimir/pkg/crypto"
)

// ListContests 查询当前租户竞赛列表。
func (s *Service) ListContests(ctx context.Context, status int16, page, size int) ([]ContestDTO, int64, int, int, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	page, size = pagex.Normalize(page, size)
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

// ListStudentContests 查询学生可发现的报名中、进行中和已结束竞赛。
// status 传 0 表示不按状态过滤;传具体状态时仍受「非草稿」可见区间约束。
func (s *Service) ListStudentContests(ctx context.Context, status int16, page, size int) ([]ContestDTO, int64, int, int, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	page, size = pagex.Normalize(page, size)
	var items []Contest
	var total int64
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		items, total, err = tx.ListStudentContests(ctx, id.TenantID, status, page, size)
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

// GetStudentContest 读取单个学生可发现的竞赛。
// 门槛与 ListStudentContests 一致(非草稿态):学生竞赛详情页需支持深链与刷新,
// 且竞赛答题与对局回放两条沉浸路由退出后都回落到该页,不能依赖列表态。
func (s *Service) GetStudentContest(ctx context.Context, contestID int64) (ContestDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ContestDTO{}, err
	}
	var item Contest
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		item, err = tx.GetContest(ctx, id.TenantID, contestID)
		if err != nil {
			return err
		}
		if item.Status == ContestStatusDraft {
			return apperr.ErrContestNotFound
		}
		return nil
	}); err != nil {
		return ContestDTO{}, err
	}
	return contestDTOFromModel(item), nil
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
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "contest.create", auditTargetContest, item.ID, nil); err != nil {
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
	return contestDTOFromModel(item), s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "contest.update", auditTargetContest, item.ID, nil)
}

// AddProblem 添加或更新竞赛题目引用,并校验 M5 题面可读取。
func (s *Service) AddProblem(ctx context.Context, contestID int64, req ProblemRequest) (ProblemDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ProblemDTO{}, err
	}
	var contest Contest
	var existingProblems []ContestProblem
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		contest, err = s.loadContestForManage(ctx, tx, id.TenantID, id.AccountID, contestID)
		if err != nil {
			return err
		}
		existingProblems, err = tx.ListContestProblems(ctx, id.TenantID, contestID)
		return err
	}); err != nil {
		return ProblemDTO{}, err
	}
	req, err = validateProblemRequest(req, contest.Mode)
	if err != nil {
		return ProblemDTO{}, err
	}
	for _, existing := range existingProblems {
		if existing.ItemCode == req.ItemCode && existing.ItemVersion == req.ItemVersion && existing.Seq != req.Seq {
			return ProblemDTO{}, apperr.ErrContestProblemInvalid
		}
	}
	if _, err := s.content.GetContentFace(ctx, id.TenantID, contracts.ContentItemRef{ItemCode: req.ItemCode, ItemVersion: req.ItemVersion}); err != nil {
		return ProblemDTO{}, apperr.ErrContestContentUnavailable.WithCause(err)
	}
	if contest.Mode == ContestModeBattle {
		if err := s.judge.ValidateJudgeMode(ctx, id.TenantID, req.ItemCode, req.ItemVersion, contracts.JudgeSandboxModeReuse); err != nil {
			return ProblemDTO{}, apperr.ErrContestProblemInvalid.WithCause(err)
		}
	}
	item := ContestProblem{ID: s.ids.Generate(), TenantID: id.TenantID, ContestID: contestID, ItemCode: req.ItemCode, ItemVersion: req.ItemVersion, Score: req.Score, DynamicScore: req.DynamicScore, BattleConfig: req.BattleConfig, BattleRule: req.BattleRule, Seq: req.Seq}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		item, err = tx.UpsertContestProblem(ctx, item)
		return err
	}); err != nil {
		return ProblemDTO{}, err
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "contest.problem.upsert", auditTargetContest, contestID, map[string]any{"problem_id": item.ID}); err != nil {
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
		if _, err := s.loadContestForRead(ctx, tx, id.TenantID, id.AccountID, contestID); err != nil {
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

// ArchiveContest 生成最终榜单快照,归档竞赛并回收竞赛关联沙箱资源。
func (s *Service) ArchiveContest(ctx context.Context, contestID int64) (ResultSnapshotDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ResultSnapshotDTO{}, err
	}
	var contest Contest
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		contest, err = s.loadContestForManage(ctx, tx, id.TenantID, id.AccountID, contestID)
		if err != nil {
			return err
		}
		return validateContestTransition(contest.Status, ContestStatusArchived)
	}); err != nil {
		return ResultSnapshotDTO{}, err
	}
	leaseToken, err := pkgcrypto.RandomToken(48)
	if err != nil {
		return ResultSnapshotDTO{}, apperr.ErrContestStateInvalid.WithCause(err)
	}
	leaseUntil := time.Now().Add(time.Duration(s.cfg.AutoArchiveLeaseDurationMs) * time.Millisecond)
	var claim ContestArchiveClaim
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		claim, err = tx.ClaimManualArchiveContest(ctx, id.TenantID, contestID, time.Now(), leaseUntil, leaseToken)
		return err
	}); err != nil {
		return ResultSnapshotDTO{}, err
	}
	snapshot, err := s.archiveContestSystem(ctx, claim, "contest_archive")
	if err != nil {
		return ResultSnapshotDTO{}, err
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "contest.archive", auditTargetContest, contestID, map[string]any{"snapshot_id": snapshot.ID}); err != nil {
		return ResultSnapshotDTO{}, err
	}
	return resultSnapshotDTOFromModel(snapshot), nil
}

// RunAutoArchiveOnce 执行一次竞赛自动收尾扫描,供统一 background runner 调用。
func (s *Service) RunAutoArchiveOnce(ctx context.Context) error {
	var items []ContestArchiveClaim
	leaseToken, err := pkgcrypto.RandomToken(48)
	if err != nil {
		return apperr.ErrContestStateInvalid.WithCause(err)
	}
	staleBefore := time.Now()
	leaseUntil := staleBefore.Add(time.Duration(s.cfg.AutoArchiveLeaseDurationMs) * time.Millisecond)
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		items, err = tx.ClaimAutoArchiveContests(ctx, s.cfg.MatchmakerBatchSize, staleBefore, leaseUntil, leaseToken)
		return err
	}); err != nil {
		return err
	}
	for _, item := range items {
		if _, err := s.archiveContestSystem(ctx, item, "contest_auto_archive"); err != nil {
			return err
		}
	}
	return nil
}

// archiveContestSystem 执行归档,复用人工与自动归档的快照、回收和租约屏障。
func (s *Service) archiveContestSystem(ctx context.Context, item ContestArchiveClaim, reason string) (LadderSnapshot, error) {
	if err := s.recycleContestSandboxes(ctx, item.TenantID, item.ID, item.CreatedAt, reason); err != nil {
		return LadderSnapshot{}, apperr.ErrContestSandboxUnavailable.WithCause(err)
	}
	var snapshot LadderSnapshot
	if err := s.store.TenantTx(ctx, item.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		pageSize := s.cfg.MatchmakerBatchSize
		if pageSize <= 0 {
			return apperr.ErrContestStateInvalid
		}
		page := 1
		var ranks []LadderRank
		for {
			batch, total, err := tx.ListLadder(ctx, item.TenantID, item.ID, page, pageSize)
			if err != nil {
				return err
			}
			ranks = append(ranks, batch...)
			if int64(len(ranks)) >= total {
				break
			}
			page++
		}
		snapshot, err = s.saveLadderSnapshot(ctx, tx, item.TenantID, item.ID, ContestStatusArchived, ranks)
		if err != nil {
			return err
		}
		affected, err := tx.CompleteAutoArchiveContest(ctx, item.TenantID, item.ID, item.ArchiveLeaseToken)
		if err != nil {
			return err
		}
		if affected != 1 {
			return apperr.ErrContestStateInvalid
		}
		return nil
	}); err != nil {
		return LadderSnapshot{}, err
	}
	if err := s.writeAudit(ctx, item.TenantID, 0, audit.ActorRoleSystem, "contest.archive.auto", auditTargetContest, item.ID, map[string]any{"snapshot_id": snapshot.ID}); err != nil {
		return LadderSnapshot{}, err
	}
	return snapshot, nil
}

// recycleContestSandboxes 回收竞赛级解题环境和未终态对抗对局环境。
func (s *Service) recycleContestSandboxes(ctx context.Context, tenantID, contestID int64, contestCreatedAt time.Time, reason string) error {
	var battleRefs []string
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		battleRefs, err = tx.ListActiveBattleSourceRefsForArchive(ctx, tenantID, contestID)
		return err
	}); err != nil {
		return err
	}
	for _, sourceRef := range contestArchiveSourceRefs(contestID, contestCreatedAt, battleRefs) {
		if err := s.sandbox.RecycleBySourceRef(ctx, contracts.SandboxRecycleRequest{TenantID: tenantID, SourceRef: sourceRef, Reason: reason}); err != nil {
			return err
		}
	}
	return nil
}

// contestArchiveSourceRefs 汇总归档需要回收的来源引用,保持竞赛级来源优先便于审计。
func contestArchiveSourceRefs(contestID int64, contestCreatedAt time.Time, battleRefs []string) []string {
	refs := []string{contestSourceRef(contestID, contestCreatedAt)}
	seen := map[string]bool{refs[0]: true}
	for _, sourceRef := range battleRefs {
		sourceRef = strings.TrimSpace(sourceRef)
		if sourceRef == "" || seen[sourceRef] {
			continue
		}
		seen[sourceRef] = true
		refs = append(refs, sourceRef)
	}
	return refs
}

// GetSnapshot 读取归档最终榜单快照。
func (s *Service) GetSnapshot(ctx context.Context, contestID int64) (ResultSnapshotDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ResultSnapshotDTO{}, err
	}
	var snapshot LadderSnapshot
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		snapshot, err = tx.GetLadderSnapshot(ctx, id.TenantID, contestID, ContestStatusArchived)
		return err
	}); err != nil {
		return ResultSnapshotDTO{}, err
	}
	return resultSnapshotDTOFromModel(snapshot), nil
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
		if err := validateContestTransitionWindow(current, next, timex.Now()); err != nil {
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
		if err := s.refreshProblemUsageRefs(ctx, item.TenantID, contestID, problems); err != nil {
			return ContestDTO{}, err
		}
	}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		if next == ContestStatusFrozen {
			ranks, _, err := tx.ListLadder(ctx, id.TenantID, contestID, 1, 1000)
			if err != nil {
				return err
			}
			if _, err := s.saveLadderSnapshot(ctx, tx, id.TenantID, contestID, ContestStatusFrozen, ranks); err != nil {
				return err
			}
		}
		item, err = tx.SetContestStatus(ctx, id.TenantID, contestID, next)
		return err
	}); err != nil {
		return ContestDTO{}, err
	}
	return contestDTOFromModel(item), s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, action, auditTargetContest, item.ID, nil)
}

// saveLadderSnapshot 把当前榜单固化为封榜或归档阶段的单一权威快照。
func (s *Service) saveLadderSnapshot(ctx context.Context, tx TxStore, tenantID, contestID int64, snapshotStatus int16, ranks []LadderRank) (LadderSnapshot, error) {
	if snapshotStatus != ContestStatusFrozen && snapshotStatus != ContestStatusArchived {
		return LadderSnapshot{}, apperr.ErrContestInvalid
	}
	ranking := make([]LadderSnapshotEntry, 0, len(ranks))
	for _, rank := range ranks {
		ranking = append(ranking, LadderSnapshotEntry{TeamID: rank.TeamID, Score: rank.Score, SolvedCount: rank.SolvedCount, LastSolveAt: rank.LastSolveAt, Rank: rank.Rank, UpdatedAt: rank.UpdatedAt})
	}
	return tx.UpsertLadderSnapshot(ctx, LadderSnapshot{ID: s.ids.Generate(), TenantID: tenantID, ContestID: contestID, SnapshotStatus: snapshotStatus, Ranking: ranking})
}

// refreshProblemUsageRefs 在发布时登记 M5 内容引用,用于删除保护和复用统计。
func (s *Service) refreshProblemUsageRefs(ctx context.Context, tenantID, contestID int64, problems []ContestProblem) error {
	seen := map[string]bool{}
	refs := make([]contracts.ContentItemRef, 0, len(problems))
	for _, problem := range problems {
		key := problem.ItemCode + ":" + problem.ItemVersion
		if seen[key] {
			continue
		}
		seen[key] = true
		refs = append(refs, contracts.ContentItemRef{ItemCode: problem.ItemCode, ItemVersion: problem.ItemVersion})
	}
	var createdAt time.Time
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		contest, err := tx.GetContest(ctx, tenantID, contestID)
		if err != nil {
			return err
		}
		createdAt = contest.CreatedAt
		return nil
	}); err != nil {
		return err
	}
	sourceRef := fmt.Sprintf("contest:%d:contest:%s", createdAt.Year(), ids.Format(contestID))
	if err := s.content.ReplaceUsageRefs(ctx, tenantID, "contest.contest", sourceRef, refs); err != nil {
		return apperr.ErrContestContentUnavailable.WithCause(err)
	}
	return nil
}
