// experiment service_group 文件实现 M7 多人协作小组和角色绑定。
package experiment

import (
	"context"
	"errors"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/pkg/apperr"
)

// CreateGroup 创建实验协作小组。
func (s *Service) CreateGroup(ctx context.Context, experimentID int64, req CreateGroupRequest) (GroupDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return GroupDTO{}, err
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 128 {
		return GroupDTO{}, apperr.ErrExperimentGroupInvalid
	}
	var group ExperimentGroup
	var profiles map[int64]contracts.AccountInfo
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		exp, err := tx.GetExperiment(ctx, id.TenantID, experimentID)
		if err != nil {
			return err
		}
		if err := s.ensureTeacherCanManage(ctx, id.AccountID, exp); err != nil {
			return err
		}
		if exp.CollabMode != CollabModeGroup {
			return apperr.ErrExperimentGroupInvalid
		}
		group, err = tx.CreateGroup(ctx, ExperimentGroup{ID: s.ids.Generate(), TenantID: id.TenantID, ExperimentID: experimentID, Name: req.Name})
		if err != nil {
			return err
		}
		profiles, err = s.groupMemberProfiles(ctx, id.TenantID, []ExperimentGroup{group})
		if err != nil {
			return err
		}
		return s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "experiment.group.create", auditTargetGroup, group.ID, map[string]any{"experiment_id": experimentID})
	}); err != nil {
		return GroupDTO{}, err
	}
	return groupDTOFromModel(group, profiles), nil
}

// UpsertGroupMember 添加或调整协作小组成员角色。
func (s *Service) UpsertGroupMember(ctx context.Context, groupID int64, req UpsertGroupMemberRequest) (GroupDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return GroupDTO{}, err
	}
	req.Role = strings.TrimSpace(req.Role)
	if req.StudentID <= 0 || req.Role == "" || len(req.Role) > 64 {
		return GroupDTO{}, apperr.ErrExperimentGroupInvalid
	}
	var group ExperimentGroup
	var profiles map[int64]contracts.AccountInfo
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		currentGroup, err := tx.GetGroupForUpdate(ctx, id.TenantID, groupID)
		if err != nil {
			return err
		}
		exp, err := tx.GetExperiment(ctx, id.TenantID, currentGroup.ExperimentID)
		if err != nil {
			return err
		}
		if err := s.ensureTeacherCanManage(ctx, id.AccountID, exp); err != nil {
			return err
		}
		if !roleAllowed(exp.GroupConfig, req.Role) {
			return apperr.ErrExperimentRoleInvalid
		}
		if err := s.ensureStudentAccount(ctx, req.StudentID.Int64()); err != nil {
			return err
		}
		members, err := tx.ListGroupMembers(ctx, id.TenantID, groupID)
		if err != nil {
			return err
		}
		if exp.GroupConfig.Size > 0 && !memberAlreadyExists(members, req.StudentID.Int64()) && len(members) >= exp.GroupConfig.Size {
			return apperr.ErrExperimentGroupFull
		}
		activeInstance, instanceErr := tx.GetActiveGroupInstance(ctx, id.TenantID, currentGroup.ExperimentID, groupID)
		if instanceErr != nil && !isNoRows(instanceErr) {
			return instanceErr
		}
		if _, err := tx.UpsertGroupMember(ctx, GroupMember{ID: s.ids.Generate(), TenantID: id.TenantID, GroupID: groupID, StudentID: req.StudentID.Int64(), Role: req.Role}); err != nil {
			return err
		}
		group, err = tx.GetGroup(ctx, id.TenantID, groupID)
		if err != nil {
			return err
		}
		members, err = tx.ListGroupMembers(ctx, id.TenantID, groupID)
		if err != nil {
			return err
		}
		memberIDs := groupMemberAccountIDs(members)
		if instanceErr == nil {
			if err := s.syncActiveInstanceAuthorizations(ctx, activeInstance, memberIDs); err != nil {
				return err
			}
		}
		profiles, err = s.groupMemberProfiles(ctx, id.TenantID, []ExperimentGroup{group})
		if err != nil {
			return err
		}
		return s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "experiment.group.member.upsert", auditTargetGroup, groupID, map[string]any{"student_id": req.StudentID, "role": req.Role})
	}); err != nil {
		return GroupDTO{}, err
	}
	return groupDTOFromModel(group, profiles), nil
}

// ensureStudentAccount 校验编组目标账号属于当前租户且具备学生角色。
// 成员表只保存账号编号，入口必须先经过 identity 契约，避免把教师、管理员或跨租户账号写入小组。
func (s *Service) ensureStudentAccount(ctx context.Context, accountID int64) error {
	ok, err := s.roles.HasRole(ctx, accountID, contracts.RoleStudent)
	if err != nil {
		return apperr.ErrExperimentGroupInvalid.WithCause(err)
	}
	if !ok {
		return apperr.ErrExperimentGroupInvalid
	}
	return nil
}

// RemoveGroupMember 在小组行锁下原子更新成员、资源授权与审计。
func (s *Service) RemoveGroupMember(ctx context.Context, groupID, studentID int64) (GroupDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return GroupDTO{}, err
	}
	if groupID <= 0 || studentID <= 0 {
		return GroupDTO{}, apperr.ErrExperimentGroupInvalid
	}
	var group ExperimentGroup
	var profiles map[int64]contracts.AccountInfo
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		currentGroup, err := tx.GetGroupForUpdate(ctx, id.TenantID, groupID)
		if err != nil {
			return err
		}
		exp, err := tx.GetExperiment(ctx, id.TenantID, currentGroup.ExperimentID)
		if err != nil {
			return err
		}
		if err := s.ensureTeacherCanManage(ctx, id.AccountID, exp); err != nil {
			return err
		}
		activeInstance, instanceErr := tx.GetActiveGroupInstance(ctx, id.TenantID, currentGroup.ExperimentID, groupID)
		if instanceErr != nil && !isNoRows(instanceErr) {
			return instanceErr
		}
		if instanceErr == nil {
			if activeInstance.OwnerAccountID == studentID {
				return apperr.ErrExperimentGroupOwnerLocked
			}
		}
		if err := tx.DeleteGroupMember(ctx, id.TenantID, groupID, studentID); err != nil {
			return err
		}
		group, err = tx.GetGroup(ctx, id.TenantID, groupID)
		if err != nil {
			return err
		}
		if instanceErr == nil {
			members, err := tx.ListGroupMembers(ctx, id.TenantID, groupID)
			if err != nil {
				return err
			}
			if err := s.syncActiveInstanceAuthorizations(ctx, activeInstance, groupMemberAccountIDs(members)); err != nil {
				return err
			}
		}
		profiles, err = s.groupMemberProfiles(ctx, id.TenantID, []ExperimentGroup{group})
		if err != nil {
			return err
		}
		return s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "experiment.group.member.remove", auditTargetGroup, groupID, map[string]any{"student_id": studentID})
	}); err != nil {
		return GroupDTO{}, err
	}
	return groupDTOFromModel(group, profiles), nil
}

// syncActiveInstanceAuthorizations 在当前租户事务内更新全部引擎授权，失败由持有者整体回滚。
func (s *Service) syncActiveInstanceAuthorizations(ctx context.Context, inst ExperimentInstance, accountIDs []int64) error {
	var syncErrs []error
	for _, ref := range inst.SandboxRefs {
		if s.sandbox == nil {
			syncErrs = append(syncErrs, apperr.ErrExperimentSandboxUnavailable)
			continue
		}
		if err := s.sandbox.UpdateSandboxAuthorizedAccounts(ctx, contracts.SandboxAuthorizedAccountsRequest{TenantID: inst.TenantID, SandboxID: ref.SandboxID.Int64(), SourceRef: inst.SourceRef, AuthorizedAccountIDs: accountIDs}); err != nil {
			syncErrs = append(syncErrs, apperr.ErrExperimentSandboxUnavailable.WithCause(err))
		}
	}
	for _, ref := range inst.SimSessionRefs {
		if s.sim == nil {
			syncErrs = append(syncErrs, apperr.ErrExperimentSimUnavailable)
			continue
		}
		if err := s.sim.UpdateSessionAuthorizedAccounts(ctx, contracts.SimAuthorizedAccountsRequest{TenantID: inst.TenantID, SessionID: ref.SessionID.Int64(), SourceRef: inst.SourceRef, AuthorizedAccountIDs: accountIDs}); err != nil {
			syncErrs = append(syncErrs, apperr.ErrExperimentSimUnavailable.WithCause(err))
		}
	}
	return errors.Join(syncErrs...)
}

// ListGroups 按实验列出全部协作小组,供教师编组视角使用。
// 与 GetGroup 的差别:这里不附带共享实例(编组时实例尚不存在),
// 且门槛是「能管理该实验」而不是「是本组成员」。
func (s *Service) ListGroups(ctx context.Context, experimentID int64) ([]GroupDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var groups []ExperimentGroup
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		exp, err := tx.GetExperiment(ctx, id.TenantID, experimentID)
		if err != nil {
			return err
		}
		if err := s.ensureTeacherCanManage(ctx, id.AccountID, exp); err != nil {
			return err
		}
		groups, err = tx.ListGroupsByExperiment(ctx, id.TenantID, experimentID)
		return err
	}); err != nil {
		return nil, err
	}
	profiles, err := s.groupMemberProfiles(ctx, id.TenantID, groups)
	if err != nil {
		return nil, err
	}
	out := make([]GroupDTO, 0, len(groups))
	for _, group := range groups {
		out = append(out, groupDTOFromModel(group, profiles))
	}
	return out, nil
}

// groupMemberProfiles 把给定小组的全部成员一次批量解析为账号档案。
// 成员表只存 student_id,而编组界面必须显示人名;经 M1 契约批量取回,
// 既不跨模块查表(铁律 3),也不逐组逐人调用形成 N+1。
func (s *Service) groupMemberProfiles(ctx context.Context, tenantID int64, groups []ExperimentGroup) (map[int64]contracts.AccountInfo, error) {
	seen := make(map[int64]struct{})
	accountIDs := make([]int64, 0)
	for _, group := range groups {
		for _, member := range group.Members {
			if _, ok := seen[member.StudentID]; ok {
				continue
			}
			seen[member.StudentID] = struct{}{}
			accountIDs = append(accountIDs, member.StudentID)
		}
	}
	if len(accountIDs) == 0 {
		return map[int64]contracts.AccountInfo{}, nil
	}
	accounts, err := s.roles.BatchGetAccounts(ctx, tenantID, accountIDs)
	if err != nil {
		return nil, apperr.ErrExperimentGroupInvalid.WithCause(err)
	}
	profiles := make(map[int64]contracts.AccountInfo, len(accounts))
	for _, account := range accounts {
		profiles[account.AccountID] = account
	}
	return profiles, nil
}

// GetGroup 读取协作小组成员和角色。
func (s *Service) GetGroup(ctx context.Context, groupID int64) (GroupDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return GroupDTO{}, err
	}
	var group ExperimentGroup
	var shared *ExperimentInstance
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		group, err = tx.GetGroup(ctx, id.TenantID, groupID)
		if err != nil {
			return err
		}
		exp, err := tx.GetExperiment(ctx, id.TenantID, group.ExperimentID)
		if err != nil {
			return err
		}
		isSchoolAdmin, err := s.isSchoolAdmin(ctx, id.AccountID)
		if err != nil {
			return err
		}
		if exp.AuthorID != id.AccountID && !isSchoolAdmin {
			if _, err := tx.GetGroupMember(ctx, id.TenantID, groupID, id.AccountID); err != nil {
				return err
			}
		}
		inst, err := tx.GetActiveGroupInstance(ctx, id.TenantID, group.ExperimentID, groupID)
		if err == nil {
			shared = &inst
			return nil
		}
		if isNoRows(err) {
			return nil
		}
		return err
	}); err != nil {
		return GroupDTO{}, err
	}
	profiles, err := s.groupMemberProfiles(ctx, id.TenantID, []ExperimentGroup{group})
	if err != nil {
		return GroupDTO{}, err
	}
	return groupDTOWithSharedInstance(group, profiles, shared), nil
}

// roleAllowed 校验角色必须来自实验定义的角色集合。
func roleAllowed(group GroupConfig, role string) bool {
	if len(group.Roles) == 0 {
		return true
	}
	for _, item := range group.Roles {
		if item == role {
			return true
		}
	}
	return false
}

// memberAlreadyExists 判断小组成员是否已存在。
func memberAlreadyExists(members []GroupMember, studentID int64) bool {
	for _, member := range members {
		if member.StudentID == studentID {
			return true
		}
	}
	return false
}
