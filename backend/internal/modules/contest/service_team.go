// contest service_team 文件实现报名、组队、邀请码加入和名单锁定。
package contest

import (
	"context"
	"strings"

	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	pkgcrypto "chaimir/pkg/crypto"
)

const inviteCodeLength = 13

// Signup 创建当前学生在竞赛中的队伍,个人赛和团队赛都以 team 建模。
func (s *Service) Signup(ctx context.Context, contestID int64, req SignupRequest) (TeamDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return TeamDTO{}, err
	}
	var contest Contest
	var team Team
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		contest, err = tx.GetContest(ctx, id.TenantID, contestID)
		if err != nil {
			return err
		}
		if err := validateSignupWindow(contest, timex.Now()); err != nil {
			return err
		}
		if existing, err := tx.GetTeamForAccount(ctx, id.TenantID, contestID, id.TenantID, id.AccountID); err == nil {
			team, err = tx.GetTeam(ctx, id.TenantID, existing.ID)
			return err
		} else if !isNoRows(err) {
			return err
		}
		name := strings.TrimSpace(req.TeamName)
		if contest.TeamMode == TeamModeSolo {
			name = "个人参赛"
		}
		name, err = validateTeamName(name)
		if err != nil {
			return err
		}
		inviteCode, err := newInviteCode()
		if err != nil {
			return err
		}
		if contest.TeamMode == TeamModeSolo {
			inviteCode = ""
		}
		team, err = tx.CreateTeam(ctx, Team{ID: s.ids.Generate(), TenantID: id.TenantID, ContestID: contestID, Name: name, InviteCode: inviteCode})
		if err != nil {
			return err
		}
		if _, err = tx.AddTeamMember(ctx, TeamMember{ID: s.ids.Generate(), TenantID: id.TenantID, TeamID: team.ID, AccountID: id.AccountID, MemberTenantID: id.TenantID, IsLeader: true}); err != nil {
			return err
		}
		team, err = tx.GetTeam(ctx, id.TenantID, team.ID)
		return err
	}); err != nil {
		return TeamDTO{}, err
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, audit.ActorRoleStudent, "contest.signup", auditTargetContestTeam, team.ID, map[string]any{"contest_id": contestID}); err != nil {
		return TeamDTO{}, err
	}
	return teamDTOFromModel(team), nil
}

// JoinTeam 通过邀请码加入团队赛队伍。
func (s *Service) JoinTeam(ctx context.Context, contestID int64, req JoinTeamRequest) (TeamDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return TeamDTO{}, err
	}
	code := strings.ToUpper(strings.TrimSpace(req.InviteCode))
	if code == "" {
		return TeamDTO{}, apperr.ErrContestTeamInvalid
	}
	var team Team
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		contest, err := tx.GetContest(ctx, id.TenantID, contestID)
		if err != nil {
			return err
		}
		if contest.TeamMode != TeamModeGroup {
			return apperr.ErrContestTeamInvalid
		}
		if err := validateSignupWindow(contest, timex.Now()); err != nil {
			return err
		}
		if ids, err := tx.AccountTeamIDs(ctx, id.TenantID, contestID, id.TenantID, id.AccountID); err != nil {
			return err
		} else if len(ids) > 0 {
			return apperr.ErrContestTeamInvalid
		}
		team, err = tx.GetTeamByInviteCode(ctx, id.TenantID, code)
		if err != nil {
			return err
		}
		if team.ContestID != contestID || team.Status != TeamStatusBuilding {
			return apperr.ErrContestTeamInvalid
		}
		if _, err = tx.AddTeamMember(ctx, TeamMember{ID: s.ids.Generate(), TenantID: id.TenantID, TeamID: team.ID, AccountID: id.AccountID, MemberTenantID: id.TenantID, IsLeader: false}); err != nil {
			return err
		}
		team, err = tx.GetTeam(ctx, id.TenantID, team.ID)
		return err
	}); err != nil {
		return TeamDTO{}, err
	}
	return teamDTOFromModel(team), s.writeAudit(ctx, id.TenantID, id.AccountID, audit.ActorRoleStudent, "contest.team.join", auditTargetContestTeam, team.ID, map[string]any{"contest_id": contestID})
}

// JoinTeamByID 通过队伍 ID 和邀请码加入团队赛队伍。
func (s *Service) JoinTeamByID(ctx context.Context, teamID int64, req JoinTeamRequest) (TeamDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return TeamDTO{}, err
	}
	code := strings.ToUpper(strings.TrimSpace(req.InviteCode))
	if code == "" {
		return TeamDTO{}, apperr.ErrContestTeamInvalid
	}
	var team Team
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		team, err = tx.GetTeam(ctx, id.TenantID, teamID)
		if err != nil {
			return err
		}
		contest, err := tx.GetContest(ctx, id.TenantID, team.ContestID)
		if err != nil {
			return err
		}
		if contest.TeamMode != TeamModeGroup || team.InviteCode != code || team.Status != TeamStatusBuilding {
			return apperr.ErrContestTeamInvalid
		}
		if err := validateSignupWindow(contest, timex.Now()); err != nil {
			return err
		}
		if ids, err := tx.AccountTeamIDs(ctx, id.TenantID, contest.ID, id.TenantID, id.AccountID); err != nil {
			return err
		} else if len(ids) > 0 {
			return apperr.ErrContestTeamInvalid
		}
		if _, err = tx.AddTeamMember(ctx, TeamMember{ID: s.ids.Generate(), TenantID: id.TenantID, TeamID: team.ID, AccountID: id.AccountID, MemberTenantID: id.TenantID, IsLeader: false}); err != nil {
			return err
		}
		team, err = tx.GetTeam(ctx, id.TenantID, team.ID)
		return err
	}); err != nil {
		return TeamDTO{}, err
	}
	return teamDTOFromModel(team), s.writeAudit(ctx, id.TenantID, id.AccountID, audit.ActorRoleStudent, "contest.team.join", auditTargetContestTeam, team.ID, map[string]any{"contest_id": team.ContestID})
}

// GetTeam 读取当前账号可访问的队伍。
func (s *Service) GetTeam(ctx context.Context, teamID int64) (TeamDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return TeamDTO{}, err
	}
	var team Team
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		team, err = tx.GetTeam(ctx, id.TenantID, teamID)
		if err != nil {
			return err
		}
		return ensureTeamAccess(id.TenantID, id.AccountID, team)
	}); err != nil {
		return TeamDTO{}, err
	}
	return teamDTOFromModel(team), nil
}

// LockTeam 由队长锁定团队名单,锁定后不再允许加入。
func (s *Service) LockTeam(ctx context.Context, teamID int64) (TeamDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return TeamDTO{}, err
	}
	var team Team
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetTeam(ctx, id.TenantID, teamID)
		if err != nil {
			return err
		}
		if !isTeamLeader(id.TenantID, id.AccountID, current) {
			return apperr.ErrContestTeamAccessDenied
		}
		team, err = tx.LockTeam(ctx, id.TenantID, teamID)
		if err != nil {
			return err
		}
		team.Members, err = tx.ListTeamMembers(ctx, id.TenantID, teamID)
		return err
	}); err != nil {
		return TeamDTO{}, err
	}
	return teamDTOFromModel(team), s.writeAudit(ctx, id.TenantID, id.AccountID, audit.ActorRoleStudent, "contest.team.lock", auditTargetContestTeam, team.ID, nil)
}

// currentAccountTeam 读取当前账号在竞赛内的队伍并带成员列表。
func (s *Service) currentAccountTeam(ctx context.Context, tx TxStore, tenantID, contestID, accountID int64) (Team, error) {
	team, err := tx.GetTeamForAccount(ctx, tenantID, contestID, tenantID, accountID)
	if err != nil {
		if isNoRows(err) {
			return Team{}, apperr.ErrContestTeamNotFound
		}
		return Team{}, err
	}
	return tx.GetTeam(ctx, tenantID, team.ID)
}

// ensureTeamAccess 校验当前账号是队伍成员。
func ensureTeamAccess(memberTenantID, accountID int64, team Team) error {
	for _, member := range team.Members {
		if member.MemberTenantID == memberTenantID && member.AccountID == accountID {
			return nil
		}
	}
	return apperr.ErrContestTeamAccessDenied
}

// isTeamLeader 判断当前账号是否为队长。
func isTeamLeader(memberTenantID, accountID int64, team Team) bool {
	for _, member := range team.Members {
		if member.MemberTenantID == memberTenantID && member.AccountID == accountID && member.IsLeader {
			return true
		}
	}
	return false
}

// newInviteCode 生成团队赛邀请码,避免依赖可预测 ID。
func newInviteCode() (string, error) {
	code, err := pkgcrypto.RandomToken(inviteCodeLength)
	if err != nil {
		return "", apperr.ErrContestTeamInvalid.WithCause(err)
	}
	return code, nil
}
