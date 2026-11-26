// contest service_team 文件实现报名、组队、邀请码加入和名单锁定。
package contest

import (
	"context"
	"strings"

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
	organizerTenantID, err := s.resolvePublishedContestTenant(ctx, contestID)
	if err != nil {
		return TeamDTO{}, err
	}
	var team Team
	if err := s.store.TenantTx(ctx, organizerTenantID, func(ctx context.Context, tx TxStore) error {
		contest, err := tx.GetContestForUpdate(ctx, organizerTenantID, contestID)
		if err != nil {
			return err
		}
		if err := validateSignupWindow(contest, timex.Now()); err != nil {
			return err
		}
		team, err = tx.GetTeamForAccount(ctx, organizerTenantID, contestID, id.TenantID, id.AccountID)
		if err != nil && !isNoRows(err) {
			return err
		}
		if isNoRows(err) {
			name, err := validateTeamName(req.TeamName)
			if err != nil {
				return err
			}
			var inviteCode string
			if contest.TeamMode == TeamModeGroup {
				inviteCode, err = newInviteCode()
				if err != nil {
					return err
				}
			}
			team, err = tx.CreateTeam(ctx, Team{ID: s.ids.Generate(), TenantID: organizerTenantID, ContestID: contestID, Name: name, InviteCode: inviteCode})
			if err != nil {
				return err
			}
			if _, err = tx.AddTeamMember(ctx, TeamMember{ID: s.ids.Generate(), TenantID: organizerTenantID, TeamID: team.ID, AccountID: id.AccountID, MemberTenantID: id.TenantID, IsLeader: true}); err != nil {
				return err
			}
		}
		team, err = tx.GetTeam(ctx, organizerTenantID, team.ID)
		if err != nil {
			return err
		}
		return s.writeStudentAudit(ctx, organizerTenantID, id, "contest.signup", auditTargetContestTeam, team.ID, map[string]any{"contest_id": contestID})
	}); err != nil {
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
	organizerTenantID, err := s.resolvePublishedContestTenant(ctx, contestID)
	if err != nil {
		return TeamDTO{}, err
	}
	var team Team
	if err := s.store.TenantTx(ctx, organizerTenantID, func(ctx context.Context, tx TxStore) error {
		contest, err := tx.GetContestForUpdate(ctx, organizerTenantID, contestID)
		if err != nil {
			return err
		}
		if contest.TeamMode != TeamModeGroup {
			return apperr.ErrContestTeamInvalid
		}
		if err := validateSignupWindow(contest, timex.Now()); err != nil {
			return err
		}
		if _, err := tx.GetTeamForAccount(ctx, organizerTenantID, contestID, id.TenantID, id.AccountID); err == nil {
			return apperr.ErrContestTeamInvalid
		} else if !isNoRows(err) {
			return err
		}
		team, err = tx.GetTeamByInviteCode(ctx, organizerTenantID, contestID, code)
		if err != nil {
			return err
		}
		if team.Status != TeamStatusBuilding {
			return apperr.ErrContestTeamInvalid
		}
		if _, err = tx.AddTeamMember(ctx, TeamMember{ID: s.ids.Generate(), TenantID: organizerTenantID, TeamID: team.ID, AccountID: id.AccountID, MemberTenantID: id.TenantID, IsLeader: false}); err != nil {
			return err
		}
		team, err = tx.GetTeam(ctx, organizerTenantID, team.ID)
		if err != nil {
			return err
		}
		return s.writeStudentAudit(ctx, organizerTenantID, id, "contest.team.join", auditTargetContestTeam, team.ID, map[string]any{"contest_id": contestID})
	}); err != nil {
		return TeamDTO{}, err
	}
	return teamDTOFromModel(team), nil
}

// GetTeam 读取当前账号可访问的队伍。
func (s *Service) GetTeam(ctx context.Context, teamID int64) (TeamDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return TeamDTO{}, err
	}
	organizerTenantID, err := s.resolveTeamTenant(ctx, teamID)
	if err != nil {
		return TeamDTO{}, err
	}
	var team Team
	if err := s.store.TenantTx(ctx, organizerTenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		team, err = tx.GetTeam(ctx, organizerTenantID, teamID)
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
	organizerTenantID, err := s.resolveTeamTenant(ctx, teamID)
	if err != nil {
		return TeamDTO{}, err
	}
	var team Team
	if err := s.store.TenantTx(ctx, organizerTenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetTeam(ctx, organizerTenantID, teamID)
		if err != nil {
			return err
		}
		if _, err := tx.GetContestForUpdate(ctx, organizerTenantID, current.ContestID); err != nil {
			return err
		}
		if !isTeamLeader(id.TenantID, id.AccountID, current) {
			return apperr.ErrContestTeamAccessDenied
		}
		team, err = tx.LockTeam(ctx, organizerTenantID, teamID)
		if err != nil {
			return err
		}
		team.Members, err = tx.ListTeamMembers(ctx, organizerTenantID, teamID)
		if err != nil {
			return err
		}
		return s.writeStudentAudit(ctx, organizerTenantID, id, "contest.team.lock", auditTargetContestTeam, team.ID, nil)
	}); err != nil {
		return TeamDTO{}, err
	}
	return teamDTOFromModel(team), nil
}

// currentAccountTeam 读取当前账号在竞赛内的队伍并带成员列表。
func (s *Service) currentAccountTeam(ctx context.Context, tx TxStore, organizerTenantID, memberTenantID, contestID, accountID int64) (Team, error) {
	team, err := tx.GetTeamForAccount(ctx, organizerTenantID, contestID, memberTenantID, accountID)
	if err != nil {
		if isNoRows(err) {
			return Team{}, apperr.ErrContestTeamNotFound
		}
		return Team{}, err
	}
	return tx.GetTeam(ctx, organizerTenantID, team.ID)
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
