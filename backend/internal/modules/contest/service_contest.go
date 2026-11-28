// contest service_contest 文件实现竞赛定义、赛题编排、生命周期和归档快照。
package contest

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/background"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	pkgcrypto "chaimir/pkg/crypto"
)

// ListContests 查询当前租户竞赛列表。
func (s *Service) ListContests(ctx context.Context, status int16, keyword string, page, size int) ([]ContestDTO, int64, int, int, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	page, size = pagex.Normalize(page, size)
	keyword = strings.TrimSpace(keyword)
	var items []Contest
	var total int64
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		items, total, err = tx.ListContests(ctx, id.TenantID, status, keyword, page, size)
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

// GetContestForTeacher 读取教师可管理的单个竞赛定义,包含草稿态配置。
func (s *Service) GetContestForTeacher(ctx context.Context, contestID int64) (ContestDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ContestDTO{}, err
	}
	if contestID <= 0 {
		return ContestDTO{}, apperr.ErrContestNotFound
	}
	var item Contest
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		item, err = s.loadContestForManage(ctx, tx, id.TenantID, id.AccountID, contestID)
		return err
	}); err != nil {
		return ContestDTO{}, err
	}
	return contestDTOFromModel(item), nil
}

// ListStudentContests 返回本校及当前学生已入队的跨校正式赛事，status 为 0 时不过滤状态。
func (s *Service) ListStudentContests(ctx context.Context, status int16, keyword string, page, size int) ([]ContestDTO, int64, int, int, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	page, size = pagex.Normalize(page, size)
	keyword = strings.TrimSpace(keyword)
	var items []Contest
	var total int64
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		items, total, err = tx.ListStudentContests(ctx, id.TenantID, id.AccountID, status, keyword, page, size)
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

// GetStudentContest 读取本校或当前成员获准访问的跨校正式赛事，不依赖列表访问记录。
func (s *Service) GetStudentContest(ctx context.Context, contestID int64) (ContestDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ContestDTO{}, err
	}
	organizerTenantID, err := s.resolvePublishedContestTenant(ctx, contestID)
	if err != nil {
		return ContestDTO{}, err
	}
	var item Contest
	if err := s.store.TenantTx(ctx, organizerTenantID, func(ctx context.Context, tx TxStore) error {
		if err := s.ensurePublishedContestReadAccess(ctx, tx, organizerTenantID, contestID, id); err != nil {
			return err
		}
		item, err = tx.GetContest(ctx, organizerTenantID, contestID)
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
	item := Contest{ID: s.ids.Generate(), TenantID: id.TenantID, OrganizerID: id.AccountID, Name: req.Name, Mode: req.Mode, MatchMode: req.MatchMode, TeamMode: req.TeamMode, SignupStart: req.SignupStart, SignupEnd: req.SignupEnd, StartAt: req.StartAt, EndAt: req.EndAt, FreezeMinutes: req.FreezeMinutes}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		item, err = tx.CreateContest(ctx, item)
		if err != nil {
			return err
		}
		return s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "contest.create", auditTargetContest, item.ID, nil)
	}); err != nil {
		return ContestDTO{}, err
	}
	return contestDTOFromModel(item), nil
}

// UpdateContest 更新草稿或报名中的竞赛定义;赛制与参赛形式只在草稿态可改。
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
		current, err := s.lockContestForManage(ctx, tx, id.TenantID, id.AccountID, contestID)
		if err != nil {
			return err
		}
		if err := contestStructureEditable(current, req); err != nil {
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
		item, err = tx.UpdateContest(ctx, current)
		if err != nil {
			return err
		}
		return s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "contest.update", auditTargetContest, item.ID, nil)
	}); err != nil {
		return ContestDTO{}, err
	}
	return contestDTOFromModel(item), nil
}

// contestStructureEditable 赛制与参赛形式只在草稿态可改:发布后赛题按赛制锁定、报名按参赛形式组队,
// 改了会让已编排的赛题和已组好的队伍对不上;其余基本信息在报名中仍可改(状态门禁见更新语句)。
func contestStructureEditable(current Contest, req ContestRequest) error {
	if current.Status == ContestStatusDraft {
		return nil
	}
	if req.Mode != current.Mode || req.TeamMode != current.TeamMode {
		return apperr.ErrContestStateInvalid
	}
	return nil
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
		if err != nil {
			return err
		}
		if contest.Status != ContestStatusDraft {
			return apperr.ErrContestStateInvalid
		}
		return nil
	}); err != nil {
		return ProblemDTO{}, err
	}
	req, err = validateProblemRequest(req, contest.Mode)
	if err != nil {
		return ProblemDTO{}, err
	}
	full, err := s.content.GetContentFull(ctx, id.TenantID, contracts.ContentItemRef{ItemCode: req.ItemCode, ItemVersion: req.ItemVersion})
	if err != nil {
		return ProblemDTO{}, apperr.ErrContestContentUnavailable.WithCause(fmt.Errorf("读取赛题内容失败 tenant_id=%d contest_id=%d: %w", id.TenantID, contestID, err))
	}
	if _, err := validateContestContent(contest.Mode, full); err != nil {
		return ProblemDTO{}, err
	}
	var compositionSnapshot *contracts.SandboxCompositionSnapshot
	if rawComposition, declared := full.Body["composition"]; declared {
		profile, profileErr := sandboxProfileForContestMode(contest.Mode)
		if profileErr != nil {
			return ProblemDTO{}, profileErr
		}
		compiled, err := s.compileComposition(ctx, id.TenantID, rawComposition, profile)
		if err != nil {
			return ProblemDTO{}, apperr.ErrContestProblemInvalid.WithCause(fmt.Errorf("编译赛题组合失败 tenant_id=%d contest_id=%d: %w", id.TenantID, contestID, err))
		}
		compositionSnapshot = &compiled
	} else if contest.Mode == ContestModeBattle {
		return ProblemDTO{}, apperr.ErrContestProblemInvalid.WithCause(fmt.Errorf("对抗赛题缺少组合声明 tenant_id=%d contest_id=%d", id.TenantID, contestID))
	}
	if contest.Mode == ContestModeBattle {
		if _, err := s.judge.GetJudgeInputRequirements(ctx, id.TenantID, req.ItemCode, req.ItemVersion, contracts.JudgeInputContext{AccessProfile: contracts.SandboxAccessContestBattle, RequiresOutcome: req.BattleRule == BattleRuleGame}); err != nil {
			return ProblemDTO{}, apperr.ErrContestProblemInvalid.WithCause(fmt.Errorf("校验赛题判题模式失败 tenant_id=%d contest_id=%d: %w", id.TenantID, contestID, err))
		}
	}
	item := ContestProblem{ID: s.ids.Generate(), TenantID: id.TenantID, ContestID: contestID, ItemCode: req.ItemCode, ItemVersion: req.ItemVersion, Score: req.Score, DynamicScore: req.DynamicScore, BattleRule: req.BattleRule, Seq: req.Seq, CompositionSnapshot: compositionSnapshot}
	if compositionSnapshot != nil {
		item.CompositionDigest = compositionSnapshot.Digest
	}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		// 外部依赖校验结束后锁定父赛事,拒绝已发布或已切换赛制的陈旧请求。
		current, err := s.lockContestForManage(ctx, tx, id.TenantID, id.AccountID, contestID)
		if err != nil {
			return err
		}
		if current.Status != ContestStatusDraft || current.Mode != contest.Mode {
			return apperr.ErrContestStateInvalid
		}
		item, err = tx.UpsertContestProblem(ctx, item)
		if err != nil {
			return err
		}
		return s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "contest.problem.upsert", auditTargetContest, contestID, map[string]any{"problem_id": item.ID})
	}); err != nil {
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
	organizerTenantID, err := s.resolveContestTenantForProblemRead(ctx, id, contestID)
	if err != nil {
		return nil, err
	}
	var items []ContestProblem
	if err := s.store.TenantTx(ctx, organizerTenantID, func(ctx context.Context, tx TxStore) error {
		if err := s.ensurePublishedContestReadAccess(ctx, tx, organizerTenantID, contestID, id); err != nil {
			return err
		}
		if _, err := s.loadContestForRead(ctx, tx, organizerTenantID, id.AccountID, contestID); err != nil {
			return err
		}
		var err error
		items, err = tx.ListContestProblems(ctx, organizerTenantID, contestID)
		return err
	}); err != nil {
		return nil, err
	}
	out := make([]ProblemDTO, 0, len(items))
	for _, item := range items {
		dto := problemDTOFromModel(item)
		face, err := s.content.GetContentFace(ctx, organizerTenantID, contracts.ContentItemRef{ItemCode: item.ItemCode, ItemVersion: item.ItemVersion})
		if err != nil {
			return nil, apperr.ErrContestContentUnavailable.WithCause(err)
		}
		dto.Title = strings.TrimSpace(jsonx.StringFromAny(face.Body["title"]))
		out = append(out, dto)
	}
	return out, nil
}

// GetProblemForStudent 读取已报名队伍成员可见的完整安全题面。
// 题目列表只返回摘要;完整题面必须经过队伍成员校验,并再次剥离执行期字段。
func (s *Service) GetProblemForStudent(ctx context.Context, contestID, problemID int64) (ProblemDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ProblemDTO{}, err
	}
	organizerTenantID, err := s.resolvePublishedContestTenant(ctx, contestID)
	if err != nil {
		return ProblemDTO{}, err
	}
	var item ContestProblem
	if err := s.store.TenantTx(ctx, organizerTenantID, func(ctx context.Context, tx TxStore) error {
		if _, err := s.currentAccountTeam(ctx, tx, organizerTenantID, id.TenantID, contestID, id.AccountID); err != nil {
			if errors.Is(err, apperr.ErrContestTeamNotFound) {
				return apperr.ErrContestNotFound
			}
			return err
		}
		var err error
		item, err = tx.GetContestProblem(ctx, organizerTenantID, problemID)
		if err != nil {
			return err
		}
		if item.ContestID != contestID {
			return apperr.ErrContestProblemInvalid
		}
		return nil
	}); err != nil {
		return ProblemDTO{}, err
	}
	face, err := s.content.GetContentFace(ctx, organizerTenantID, contracts.ContentItemRef{ItemCode: item.ItemCode, ItemVersion: item.ItemVersion})
	if err != nil {
		return ProblemDTO{}, apperr.ErrContestContentUnavailable.WithCause(err)
	}
	safeFace, err := contestStudentFace(face.Body)
	if err != nil {
		return ProblemDTO{}, apperr.ErrContestContentUnavailable.WithCause(err)
	}
	out := problemDTOFromModel(item)
	out.Title = strings.TrimSpace(jsonx.StringFromAny(safeFace["title"]))
	out.Face = safeFace
	return out, nil
}

// contestStudentFace 只投影学生答题所需的题面与组合摘要,不透传执行期配置。
func contestStudentFace(face map[string]any) (map[string]any, error) {
	out, err := jsonx.CloneObjectStrict(face)
	if err != nil {
		return nil, err
	}
	delete(out, "init_code_ref")
	delete(out, "init_script_ref")
	delete(out, "sim_package_ref")
	delete(out, "judge_config")
	if raw, ok := out["composition"]; ok {
		composition, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("题面组合声明格式不正确")
		}
		safeComposition := map[string]any{}
		if workspace := strings.TrimSpace(jsonx.StringFromAny(composition["workspace_runtime_instance"])); workspace != "" {
			safeComposition["workspace_runtime_instance"] = workspace
		}
		runtimes, ok := composition["runtimes"].([]any)
		if !ok || len(runtimes) == 0 {
			return nil, fmt.Errorf("题面组合声明缺少运行时")
		}
		safeRuntimes := make([]any, 0, len(runtimes))
		for _, rawRuntime := range runtimes {
			runtime, ok := rawRuntime.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("题面运行时声明格式不正确")
			}
			instanceCode := strings.TrimSpace(jsonx.StringFromAny(runtime["instance_code"]))
			runtimeCode := strings.TrimSpace(jsonx.StringFromAny(runtime["runtime_code"]))
			if instanceCode == "" || runtimeCode == "" {
				return nil, fmt.Errorf("题面运行时声明不完整")
			}
			safeRuntimes = append(safeRuntimes, map[string]any{"instance_code": instanceCode, "runtime_code": runtimeCode})
		}
		safeComposition["runtimes"] = safeRuntimes
		out["composition"] = safeComposition
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
	snapshot, err := s.archiveContest(ctx, claim, "contest_archive", id.AccountID)
	if err != nil {
		return ResultSnapshotDTO{}, err
	}
	// 快照只存 team_id,返回前按同一快照一次读取队名
	var names map[int64]string
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		names, err = tx.TeamNames(ctx, id.TenantID, contestID, snapshotTeamIDs(snapshot))
		return err
	}); err != nil {
		return ResultSnapshotDTO{}, err
	}
	return resultSnapshotDTOFromModel(snapshot, contest.Mode, names), nil
}

// RunAutoArchiveOnce 执行一次竞赛自动收尾扫描,供统一 background runner 调用。
func (s *Service) RunAutoArchiveOnce(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
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
		return errors.Join(err, ctx.Err())
	}
	var resultErr error
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return errors.Join(resultErr, err)
		}
		if _, err := s.archiveContest(ctx, item, "contest_auto_archive", 0); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("自动归档竞赛失败 tenant_id=%d contest_id=%d: %w", item.TenantID, item.ID, err))
		}
	}
	return errors.Join(resultErr, ctx.Err())
}

// AdvanceContestStatusesOnce 按赛事时间窗口推进报名、进行中、封榜和结束状态。
// 各校状态与审计独立提交；一校失败不阻断其他学校，取消后停止启动后续事务。
func (s *Service) AdvanceContestStatusesOnce(ctx context.Context, now time.Time) error {
	if now.IsZero() {
		now = timex.Now()
	}
	var tenantIDs []int64
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		tenantIDs, err = tx.ListContestLifecycleTenantIDs(ctx, now)
		return err
	}); err != nil {
		return err
	}
	var failures []error
	for _, tenantID := range tenantIDs {
		if err := ctx.Err(); err != nil {
			return errors.Join(append(failures, err)...)
		}
		if err := s.advanceTenantContestStatuses(ctx, tenantID, now); err != nil {
			failures = append(failures, fmt.Errorf("推进竞赛阶段失败 tenant_id=%d: %w", tenantID, err))
		}
	}
	return errors.Join(failures...)
}

// 维护扫描只定位学校；租户事务重新锁定当前赛事，避免使用扫描时的陈旧状态。
func (s *Service) advanceTenantContestStatuses(ctx context.Context, tenantID int64, now time.Time) error {
	var revokedGrants []ContestAccessGrant
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		dueToEnd, err := tx.ListContestsDueToEnd(ctx, now)
		if err != nil {
			return err
		}
		for _, contest := range dueToEnd {
			grants, err := s.revokeContestSandboxGrants(ctx, tx, tenantID, contest.ID)
			if err != nil {
				return err
			}
			if _, err := tx.SetContestStatus(ctx, tenantID, contest.ID, ContestStatusEnded); err != nil {
				return err
			}
			if err := s.writeAudit(ctx, tenantID, 0, audit.ActorRoleSystem, "contest.lifecycle", auditTargetContest, contest.ID, map[string]any{"status": ContestStatusEnded}); err != nil {
				return err
			}
			revokedGrants = append(revokedGrants, grants...)
		}

		dueToRun, err := tx.ListContestsDueToRun(ctx, now)
		if err != nil {
			return err
		}
		for _, contest := range dueToRun {
			if err := tx.LockContestTeams(ctx, tenantID, contest.ID); err != nil {
				return err
			}
			if _, err := tx.SetContestStatus(ctx, tenantID, contest.ID, ContestStatusRunning); err != nil {
				return err
			}
			if err := s.writeAudit(ctx, tenantID, 0, audit.ActorRoleSystem, "contest.lifecycle", auditTargetContest, contest.ID, map[string]any{"status": ContestStatusRunning}); err != nil {
				return err
			}
		}

		dueToFreeze, err := tx.ListContestsDueToFreeze(ctx, now)
		if err != nil {
			return err
		}
		for _, contest := range dueToFreeze {
			ranks, err := s.listAllContestLadder(ctx, tx, tenantID, contest.ID)
			if err != nil {
				return err
			}
			snapshot, err := s.saveLadderSnapshot(ctx, tx, tenantID, contest.ID, ContestStatusFrozen, ranks)
			if err != nil {
				return err
			}
			if _, err := tx.SetContestStatus(ctx, tenantID, contest.ID, ContestStatusFrozen); err != nil {
				return err
			}
			if err := s.writeAudit(ctx, tenantID, 0, audit.ActorRoleSystem, "contest.lifecycle", auditTargetContest, contest.ID, map[string]any{"status": ContestStatusFrozen, "snapshot_id": snapshot.ID}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}

	if err := s.closeRevokedContestSandboxSessions(ctx, tenantID, revokedGrants); err != nil {
		return apperr.ErrContestSandboxUnavailable.WithCause(err)
	}
	return nil
}

// listAllContestLadder 按分页完整读取排行榜,保证封榜快照不因固定页大小截断。
func (s *Service) listAllContestLadder(ctx context.Context, tx TxStore, tenantID, contestID int64) ([]LadderRank, error) {
	pageSize := s.cfg.MatchmakerBatchSize
	if pageSize <= 0 {
		return nil, apperr.ErrContestStateInvalid
	}
	ranks := make([]LadderRank, 0)
	for page := 1; ; page++ {
		batch, total, err := tx.ListLadder(ctx, tenantID, contestID, page, pageSize)
		if err != nil {
			return nil, err
		}
		ranks = append(ranks, batch...)
		if int64(len(ranks)) >= total {
			return ranks, nil
		}
		if len(batch) == 0 {
			return nil, apperr.ErrContestStateInvalid
		}
	}
}

// archiveContest 共用归档收尾；actorID 为零时记录系统任务，否则记录实际教师。
func (s *Service) archiveContest(ctx context.Context, item ContestArchiveClaim, reason string, actorID int64) (LadderSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return LadderSnapshot{}, err
	}
	leaseDuration := time.Duration(s.cfg.AutoArchiveLeaseDurationMs) * time.Millisecond
	err := s.store.TenantTx(ctx, item.TenantID, func(ctx context.Context, tx TxStore) error {
		return renewContestArchiveLease(ctx, tx, item, leaseDuration)
	})
	if err != nil {
		return LadderSnapshot{}, err
	}
	_, err = background.WithRenewal(ctx, leaseDuration, func(ctx context.Context) (bool, error) {
		err := s.store.TenantTx(ctx, item.TenantID, func(ctx context.Context, tx TxStore) error {
			return renewContestArchiveLease(ctx, tx, item, leaseDuration)
		})
		return err == nil, err
	}, func(ctx context.Context) error {
		return s.cleanupContestForArchive(ctx, item, reason)
	})
	if err != nil {
		return LadderSnapshot{}, err
	}
	var snapshot LadderSnapshot
	if err := s.store.TenantTx(ctx, item.TenantID, func(ctx context.Context, tx TxStore) error {
		if err := renewContestArchiveLease(ctx, tx, item, leaseDuration); err != nil {
			return err
		}
		ranks, err := s.listAllContestLadder(ctx, tx, item.TenantID, item.ID)
		if err != nil {
			return err
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
		actorRole, action := audit.ActorRoleSystem, "contest.archive.auto"
		if actorID > 0 {
			actorRole, action = contracts.RoleNumTeacher, "contest.archive"
		}
		return s.writeAudit(ctx, item.TenantID, actorID, actorRole, action, auditTargetContest, item.ID, map[string]any{"snapshot_id": snapshot.ID})
	}); err != nil {
		return LadderSnapshot{}, err
	}
	return snapshot, nil
}

func renewContestArchiveLease(ctx context.Context, tx TxStore, item ContestArchiveClaim, duration time.Duration) error {
	renewed, err := tx.RenewContestArchiveLease(ctx, item.TenantID, item.ID, item.ArchiveLeaseToken, timex.Now().Add(duration))
	if err != nil {
		return err
	}
	if !renewed {
		return apperr.ErrContestStateInvalid
	}
	return nil
}

func (s *Service) cleanupContestForArchive(ctx context.Context, item ContestArchiveClaim, reason string) error {
	var revokedGrants []ContestAccessGrant
	if err := s.store.TenantTx(ctx, item.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		revokedGrants, err = s.revokeContestSandboxGrants(ctx, tx, item.TenantID, item.ID)
		return err
	}); err != nil {
		return err
	}
	if err := s.closeRevokedContestSandboxSessions(ctx, item.TenantID, revokedGrants); err != nil {
		return apperr.ErrContestSandboxUnavailable.WithCause(err)
	}
	if err := s.recycleContestSandboxes(ctx, item.TenantID, item.ID, reason); err != nil {
		return apperr.ErrContestSandboxUnavailable.WithCause(err)
	}
	return nil
}

// recycleContestSandboxes 回收竞赛关联的全部解题与对局环境。
func (s *Service) recycleContestSandboxes(ctx context.Context, tenantID, contestID int64, reason string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var scopeRefs []string
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		scopeRefs, err = tx.ListContestSandboxScopeRefsForArchive(ctx, tenantID, contestID)
		return err
	}); err != nil {
		return errors.Join(err, ctx.Err())
	}
	var resultErr error
	for _, scopeRef := range scopeRefs {
		if err := ctx.Err(); err != nil {
			return errors.Join(resultErr, err)
		}
		if err := s.sandbox.RecycleByScopeRef(ctx, contracts.SandboxRecycleRequest{TenantID: tenantID, ScopeRef: scopeRef, SourceRef: scopeRef, Reason: reason}); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("回收竞赛环境失败 scope_ref=%s: %w", scopeRef, err))
		}
	}
	return errors.Join(resultErr, ctx.Err())
}

// GetSnapshot 按赛事管理范围读取最终榜单快照。
func (s *Service) GetSnapshot(ctx context.Context, contestID int64) (ResultSnapshotDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ResultSnapshotDTO{}, err
	}
	var snapshot LadderSnapshot
	var mode int16
	var names map[int64]string
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		contest, err := s.loadContestForManage(ctx, tx, id.TenantID, id.AccountID, contestID)
		if err != nil {
			return err
		}
		mode = contest.Mode
		snapshot, err = tx.GetLadderSnapshot(ctx, id.TenantID, contestID, ContestStatusArchived)
		if err != nil {
			return err
		}
		names, err = tx.TeamNames(ctx, id.TenantID, contestID, snapshotTeamIDs(snapshot))
		return err
	}); err != nil {
		return ResultSnapshotDTO{}, err
	}
	return resultSnapshotDTOFromModel(snapshot, mode, names), nil
}

// transitionContest 封装竞赛状态流转、内容引用登记和审计。
func (s *Service) transitionContest(ctx context.Context, contestID int64, next int16, action string, requireProblems bool) (ContestDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return ContestDTO{}, err
	}
	var item Contest
	var problems []ContestProblem
	var revokedGrants []ContestAccessGrant
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
		if requireProblems {
			for _, problem := range problems {
				if problem.CompositionSnapshot == nil || problem.CompositionDigest == "" {
					if current.Mode == ContestModeBattle {
						return apperr.ErrContestProblemInvalid
					}
					continue
				}
				canonical, err := contracts.CanonicalSnapshotDigest(*problem.CompositionSnapshot)
				if err != nil || canonical != problem.CompositionDigest {
					return apperr.ErrContestProblemInvalid
				}
			}
		}
		item = current
		return nil
	}); err != nil {
		return ContestDTO{}, err
	}
	if requireProblems {
		digests := make([]string, 0, len(problems))
		for _, problem := range problems {
			face, err := s.content.GetContentFace(ctx, id.TenantID, contracts.ContentItemRef{ItemCode: problem.ItemCode, ItemVersion: problem.ItemVersion})
			if err != nil {
				return ContestDTO{}, apperr.ErrContestContentUnavailable.WithCause(err)
			}
			if _, err := validateContestContent(item.Mode, face); err != nil {
				return ContestDTO{}, err
			}
			if problem.CompositionSnapshot != nil {
				digests = append(digests, problem.CompositionDigest)
			}
		}
		if len(digests) > 0 {
			if s.sandbox == nil {
				return ContestDTO{}, apperr.ErrContestSandboxUnavailable
			}
			if err := s.sandbox.PrepareCompositions(ctx, digests); err != nil {
				return ContestDTO{}, apperr.ErrContestSandboxUnavailable.WithCause(err)
			}
		}
	}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := s.lockContestForManage(ctx, tx, id.TenantID, id.AccountID, contestID)
		if err != nil {
			return err
		}
		if err := validateContestTransitionWindow(current, next, timex.Now()); err != nil {
			return err
		}
		if requireProblems {
			latestProblems, err := tx.ListContestProblems(ctx, id.TenantID, contestID)
			if err != nil {
				return err
			}
			// 事务外准备过的题目必须仍是当前集合，才可登记引用并发布。
			if current.Mode != item.Mode || !reflect.DeepEqual(problems, latestProblems) {
				return apperr.ErrContestStateInvalid
			}
			if err := s.refreshProblemUsageRefs(ctx, current, latestProblems); err != nil {
				return err
			}
		}
		if next == ContestStatusRunning {
			if err := tx.LockContestTeams(ctx, id.TenantID, contestID); err != nil {
				return err
			}
		}
		if next == ContestStatusFrozen {
			ranks, err := s.listAllContestLadder(ctx, tx, id.TenantID, contestID)
			if err != nil {
				return err
			}
			if _, err := s.saveLadderSnapshot(ctx, tx, id.TenantID, contestID, ContestStatusFrozen, ranks); err != nil {
				return err
			}
		}
		if next == ContestStatusEnded {
			var err error
			revokedGrants, err = s.revokeContestSandboxGrants(ctx, tx, id.TenantID, contestID)
			if err != nil {
				return err
			}
		}
		item, err = tx.SetContestStatus(ctx, id.TenantID, contestID, next)
		if err != nil {
			return err
		}
		return s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, action, auditTargetContest, item.ID, nil)
	}); err != nil {
		return ContestDTO{}, err
	}
	if next == ContestStatusEnded {
		if err := s.closeRevokedContestSandboxSessions(ctx, id.TenantID, revokedGrants); err != nil {
			return ContestDTO{}, apperr.ErrContestSandboxUnavailable.WithCause(err)
		}
	}
	return contestDTOFromModel(item), nil
}

// saveLadderSnapshot 把当前榜单固化为封榜或归档阶段的单一权威快照。
func (s *Service) saveLadderSnapshot(ctx context.Context, tx TxStore, tenantID, contestID int64, snapshotStatus int16, ranks []LadderRank) (LadderSnapshot, error) {
	if snapshotStatus != ContestStatusFrozen && snapshotStatus != ContestStatusArchived {
		return LadderSnapshot{}, apperr.ErrContestInvalid
	}
	ranking := make([]LadderSnapshotEntry, 0, len(ranks))
	for _, rank := range ranks {
		ranking = append(ranking, LadderSnapshotEntry{TeamID: rank.TeamID, Score: rank.Score, SolvedCount: rank.SolvedCount, LastSolveAt: optionalTime(rank.LastSolveAt), Rank: rank.Rank, UpdatedAt: rank.UpdatedAt})
	}
	return tx.UpsertLadderSnapshot(ctx, LadderSnapshot{ID: s.ids.Generate(), TenantID: tenantID, ContestID: contestID, SnapshotStatus: snapshotStatus, Ranking: ranking})
}

// refreshProblemUsageRefs 在发布时登记 M5 内容引用,用于删除保护和复用统计。
func (s *Service) refreshProblemUsageRefs(ctx context.Context, contest Contest, problems []ContestProblem) error {
	refs := make([]contracts.ContentItemRef, 0, len(problems))
	for _, problem := range problems {
		refs = append(refs, contracts.ContentItemRef{ItemCode: problem.ItemCode, ItemVersion: problem.ItemVersion})
	}
	sourceRef := fmt.Sprintf("contest:%d:contest:%s", contest.CreatedAt.Year(), ids.Format(contest.ID))
	if err := s.content.ReplaceUsageRefs(ctx, contest.TenantID, "contest.contest", sourceRef, refs); err != nil {
		return apperr.ErrContestContentUnavailable.WithCause(err)
	}
	return nil
}
