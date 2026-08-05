// experiment service_group 文件实现 M7 多人协作小组和角色绑定。
package experiment

import (
	"context"
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
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		exp, err := tx.GetExperiment(ctx, id.TenantID, experimentID)
		if err != nil {
			return err
		}
		if err := ensureTeacherCanManage(id.AccountID, s.isSchoolAdmin(ctx, id.AccountID), exp); err != nil {
			return err
		}
		if exp.CollabMode != CollabModeGroup {
			return apperr.ErrExperimentGroupInvalid
		}
		group, err = tx.CreateGroup(ctx, ExperimentGroup{ID: s.ids.Generate(), TenantID: id.TenantID, ExperimentID: experimentID, Name: req.Name})
		return err
	}); err != nil {
		return GroupDTO{}, err
	}
	profiles, err := s.groupMemberProfiles(ctx, []ExperimentGroup{group})
	if err != nil {
		return GroupDTO{}, err
	}
	return groupDTOFromModel(group, profiles), s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "experiment.group.create", auditTargetGroup, group.ID, map[string]any{"experiment_id": experimentID})
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
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		currentGroup, err := tx.GetGroup(ctx, id.TenantID, groupID)
		if err != nil {
			return err
		}
		exp, err := tx.GetExperiment(ctx, id.TenantID, currentGroup.ExperimentID)
		if err != nil {
			return err
		}
		if err := ensureTeacherCanManage(id.AccountID, s.isSchoolAdmin(ctx, id.AccountID), exp); err != nil {
			return err
		}
		if !roleAllowed(exp.GroupConfig, req.Role) {
			return apperr.ErrExperimentRoleInvalid
		}
		members, err := tx.ListGroupMembers(ctx, id.TenantID, groupID)
		if err != nil {
			return err
		}
		if exp.GroupConfig.Size > 0 && !memberAlreadyExists(members, req.StudentID.Int64()) && len(members) >= exp.GroupConfig.Size {
			return apperr.ErrExperimentGroupFull
		}
		if _, err := tx.UpsertGroupMember(ctx, GroupMember{ID: s.ids.Generate(), TenantID: id.TenantID, GroupID: groupID, StudentID: req.StudentID.Int64(), Role: req.Role}); err != nil {
			return err
		}
		group, err = tx.GetGroup(ctx, id.TenantID, groupID)
		return err
	}); err != nil {
		return GroupDTO{}, err
	}
	profiles, err := s.groupMemberProfiles(ctx, []ExperimentGroup{group})
	if err != nil {
		return GroupDTO{}, err
	}
	return groupDTOFromModel(group, profiles), s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "experiment.group.member.upsert", auditTargetGroup, groupID, map[string]any{"student_id": req.StudentID, "role": req.Role})
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
		if err := ensureTeacherCanManage(id.AccountID, s.isSchoolAdmin(ctx, id.AccountID), exp); err != nil {
			return err
		}
		groups, err = tx.ListGroupsByExperiment(ctx, id.TenantID, experimentID)
		return err
	}); err != nil {
		return nil, err
	}
	profiles, err := s.groupMemberProfiles(ctx, groups)
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
func (s *Service) groupMemberProfiles(ctx context.Context, groups []ExperimentGroup) (map[int64]contracts.AccountInfo, error) {
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
	accounts, err := s.roles.BatchGetAccounts(ctx, accountIDs)
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
		if exp.AuthorID != id.AccountID && !s.isSchoolAdmin(ctx, id.AccountID) {
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
	profiles, err := s.groupMemberProfiles(ctx, []ExperimentGroup{group})
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
