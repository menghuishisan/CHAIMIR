// identity service_account 文件实现学校管理员账号管理和状态流转。
package identity

import (
	"context"
	"log/slog"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
	"chaimir/pkg/crypto"
	"chaimir/pkg/logging"
)

// ListAccountsByAdmin 分页读取租户账号列表并做手机号脱敏。
func (s *Service) ListAccountsByAdmin(ctx context.Context, query AccountQuery) (AccountListResponse, error) {
	id, err := requireTenantRole(ctx, s, contracts.RoleSchoolAdmin)
	if err != nil {
		return AccountListResponse{}, err
	}
	page, size := pagex.Normalize(int(query.Page), int(query.Size))
	query.Page = int32(page)
	query.Size = int32(size)
	var rows []Account
	var total int64
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		items, count, err := tx.ListAccounts(ctx, query)
		if err != nil {
			return err
		}
		rows = items
		total = count
		return nil
	}); err != nil {
		return AccountListResponse{}, apperr.ErrInternal.WithCause(err)
	}
	list := make([]AccountDTO, 0, len(rows))
	for _, row := range rows {
		phonePlain, err := s.decryptPhone(row.PhoneEnc)
		if err != nil {
			return AccountListResponse{}, apperr.ErrInternal.WithCause(err)
		}
		list = append(list, ToAccountDTO(row, phonePlain))
	}
	return AccountListResponse{List: list, Total: total, Page: query.Page, Size: query.Size}, nil
}

// CreateAccountByAdmin 由学校管理员创建单个师生账号。
func (s *Service) CreateAccountByAdmin(ctx context.Context, req CreateAccountRequest) (AccountDTO, string, error) {
	id, err := requireTenantRole(ctx, s, contracts.RoleSchoolAdmin)
	if err != nil {
		return AccountDTO{}, "", err
	}
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.No) == "" || req.OrgID <= 0 {
		return AccountDTO{}, "", apperr.ErrIdentityAccountUpdateInvalid
	}
	if err := ValidatePhone(req.Phone); err != nil {
		return AccountDTO{}, "", err
	}
	role, err := BaseRole(req.BaseIdentity)
	if err != nil {
		return AccountDTO{}, "", err
	}
	phoneEnc, err := s.encryptPhone(req.Phone)
	if err != nil {
		return AccountDTO{}, "", apperr.ErrInternal.WithCause(err)
	}
	phoneHash, err := s.phoneHash(req.Phone)
	if err != nil {
		return AccountDTO{}, "", apperr.ErrInternal.WithCause(err)
	}
	var passwordHash string
	status := AccountStatusPending
	mustChange := false
	if !req.UseActivation {
		if err := ValidatePassword(req.InitialPassword); err != nil {
			return AccountDTO{}, "", err
		}
		passwordHash, err = crypto.HashPassword(req.InitialPassword)
		if err != nil {
			return AccountDTO{}, "", apperr.ErrInternal.WithCause(err)
		}
		mustChange = true
	}
	accountID := s.ids.Generate()
	var activationPlain string
	var account Account
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		// 单账号开通必须读取租户配置,避免前端用 use_activation 绕过学校开通策略。
		currentTenant, err := tx.GetTenantByID(ctx, id.TenantID)
		if err != nil {
			return err
		}
		if req.UseActivation && !currentTenant.EnableActivationCode {
			return apperr.ErrIdentityActivationDisabled
		}
		// 教师只能挂真实院系,学生只能挂真实班级;account_profile 没有外键,必须在 service 层显式防脏数据。
		if err := validateAccountOrgForProfile(ctx, tx, id.TenantID, req.BaseIdentity, req.OrgID, req.EnrollmentYear); err != nil {
			return err
		}
		row, err := tx.CreateAccount(ctx, CreateAccountInput{
			ID:            accountID,
			TenantID:      id.TenantID,
			PhoneEnc:      phoneEnc,
			PhoneHash:     phoneHash,
			PasswordHash:  passwordHash,
			Name:          strings.TrimSpace(req.Name),
			BaseIdentity:  req.BaseIdentity,
			Status:        status,
			MustChangePwd: mustChange,
			Roles:         []RoleCreateInput{{ID: s.ids.Generate(), Role: role}},
			Profile:       &CreateProfileInput{No: strings.TrimSpace(req.No), OrgID: req.OrgID, EnrollmentYear: req.EnrollmentYear, Title: strings.TrimSpace(req.Title)},
		})
		if err != nil {
			return err
		}
		account = row
		if req.UseActivation {
			code, err := crypto.RandomToken(16)
			if err != nil {
				return err
			}
			hash, err := s.hashSecret(code)
			if err != nil {
				return err
			}
			if _, err := tx.CreateActivationCode(ctx, CreateActivationInput{ID: s.ids.Generate(), TenantID: id.TenantID, AccountID: accountID, CodeHash: hash, ExpireAt: s.activationExpireAt(), CreatedBy: id.AccountID}); err != nil {
				return err
			}
			activationPlain = code
		}
		return nil
	}); err != nil {
		return AccountDTO{}, "", apperr.AsAppError(err)
	}
	if auditErr := s.auditAccount(ctx, id, "account.create", account.ID); auditErr != nil {
		logging.ErrorContext(ctx, "写入账号创建审计失败", auditErr.Error(), slog.Int64("tenant_id", id.TenantID), slog.Int64("account_id", account.ID))
	}
	return ToAccountDTO(account, req.Phone), activationPlain, nil
}

// UpdateAccountByAdmin 更新账号可编辑档案字段,不允许修改学号工号和角色。
func (s *Service) UpdateAccountByAdmin(ctx context.Context, accountID int64, req UpdateAccountRequest) (AccountDTO, error) {
	id, err := requireTenantRole(ctx, s, contracts.RoleSchoolAdmin)
	if err != nil {
		return AccountDTO{}, err
	}
	if strings.TrimSpace(req.Name) == "" || req.OrgID <= 0 {
		return AccountDTO{}, apperr.ErrIdentityAccountUpdateInvalid
	}
	var account Account
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetAccount(ctx, accountID)
		if err != nil {
			return err
		}
		// 账号基础身份不可编辑,因此组织类型校验必须以数据库中的身份为准,不能信任前端字段。
		if err := validateAccountOrgForProfile(ctx, tx, id.TenantID, current.BaseIdentity, req.OrgID, req.EnrollmentYear); err != nil {
			return err
		}
		row, err := tx.UpdateAccountEditable(ctx, id.TenantID, accountID, UpdateAccountRequest{Name: strings.TrimSpace(req.Name), OrgID: req.OrgID, EnrollmentYear: req.EnrollmentYear, Title: strings.TrimSpace(req.Title)})
		if err != nil {
			return err
		}
		account = row
		return nil
	}); err != nil {
		return AccountDTO{}, apperr.AsAppError(err)
	}
	phonePlain, err := s.decryptPhone(account.PhoneEnc)
	if err != nil {
		return AccountDTO{}, apperr.ErrInternal.WithCause(err)
	}
	if auditErr := s.auditAccount(ctx, id, "account.update", accountID); auditErr != nil {
		logging.ErrorContext(ctx, "写入账号更新审计失败", auditErr.Error(), slog.Int64("tenant_id", id.TenantID), slog.Int64("account_id", accountID))
	}
	return ToAccountDTO(account, phonePlain), nil
}

// ResetAccountPasswordByAdmin 重置指定账号密码并按请求设置是否强制首登改密。
func (s *Service) ResetAccountPasswordByAdmin(ctx context.Context, accountID int64, req AdminResetPasswordRequest) error {
	id, err := requireTenantRole(ctx, s, contracts.RoleSchoolAdmin)
	if err != nil {
		return err
	}
	if err := ValidatePassword(req.NewPassword); err != nil {
		return err
	}
	hash, err := crypto.HashPassword(req.NewPassword)
	if err != nil {
		return apperr.ErrInternal.WithCause(err)
	}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if _, err := tx.UpdateAccountPassword(ctx, accountID, id.TenantID, hash, req.MustChange, AccountStatusActive); err != nil {
			return err
		}
		return tx.RevokeAccountSessions(ctx, id.TenantID, accountID)
	}); err != nil {
		return apperr.ErrInternal.WithCause(err)
	}
	return s.auditAccount(ctx, id, "account.password.reset", accountID)
}

// GrantSchoolAdmin 授予教师学校管理员角色。
func (s *Service) GrantSchoolAdmin(ctx context.Context, accountID int64) error {
	id, err := requireTenantRole(ctx, s, contracts.RoleSchoolAdmin)
	if err != nil {
		return err
	}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		account, err := tx.GetAccount(ctx, accountID)
		if err != nil {
			return err
		}
		if account.BaseIdentity != BaseIdentityTeacher {
			return apperr.ErrIdentityTeacherAdminRequired
		}
		return tx.GrantRole(ctx, id.TenantID, accountID, RoleSchoolAdmin, s.ids.Generate())
	}); err != nil {
		return apperr.AsAppError(err)
	}
	return s.auditAccount(ctx, id, "account.admin.grant", accountID)
}

// RevokeSchoolAdmin 撤销账号学校管理员角色。
func (s *Service) RevokeSchoolAdmin(ctx context.Context, accountID int64) error {
	id, err := requireTenantRole(ctx, s, contracts.RoleSchoolAdmin)
	if err != nil {
		return err
	}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		return tx.RevokeRole(ctx, id.TenantID, accountID, RoleSchoolAdmin)
	}); err != nil {
		return apperr.ErrInternal.WithCause(err)
	}
	return s.auditAccount(ctx, id, "account.admin.revoke", accountID)
}

// UpdateAccountStatusByAdmin 执行学校管理员账号状态流转。
func (s *Service) UpdateAccountStatusByAdmin(ctx context.Context, accountID int64, status int16) error {
	id, err := requireTenantRole(ctx, s, contracts.RoleSchoolAdmin)
	if err != nil {
		return err
	}
	deleted := status == AccountStatusCancelled
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		_, err := tx.UpdateAccountStatus(ctx, accountID, id.TenantID, status, deleted)
		if err != nil {
			return err
		}
		if status == AccountStatusDisabled || status == AccountStatusCancelled || status == AccountStatusArchived {
			return tx.RevokeAccountSessions(ctx, id.TenantID, accountID)
		}
		return nil
	}); err != nil {
		return apperr.ErrInternal.WithCause(err)
	}
	return s.auditAccount(ctx, id, "account.status.update", accountID)
}

// ForceLogoutAccountByAdmin 吊销指定账号全部会话。
func (s *Service) ForceLogoutAccountByAdmin(ctx context.Context, accountID int64) error {
	id, err := requireTenantRole(ctx, s, contracts.RoleSchoolAdmin)
	if err != nil {
		return err
	}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		return tx.RevokeAccountSessions(ctx, id.TenantID, accountID)
	}); err != nil {
		return apperr.ErrInternal.WithCause(err)
	}
	return s.auditAccount(ctx, id, "account.force_logout", accountID)
}

// BatchUpdateAccountStatusByAdmin 批量更新账号状态,逐个复用同一状态机路径。
func (s *Service) BatchUpdateAccountStatusByAdmin(ctx context.Context, req BatchAccountIDsRequest, status int16) error {
	if len(req.AccountIDs) == 0 {
		return apperr.ErrIdentityAccountBatchEmpty
	}
	for _, accountID := range req.AccountIDs {
		if accountID <= 0 {
			return apperr.ErrIdentityAccountBatchInvalid
		}
		if err := s.UpdateAccountStatusByAdmin(ctx, accountID, status); err != nil {
			return err
		}
	}
	return nil
}

// ListImportBatchesByAdmin 读取账号导入批次历史。
func (s *Service) ListImportBatchesByAdmin(ctx context.Context) ([]ImportBatch, error) {
	id, err := requireTenantRole(ctx, s, contracts.RoleSchoolAdmin)
	if err != nil {
		return nil, err
	}
	var rows []ImportBatch
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		items, err := tx.ListImportBatches(ctx, id.TenantID)
		if err != nil {
			return err
		}
		rows = items
		return nil
	}); err != nil {
		return nil, apperr.ErrInternal.WithCause(err)
	}
	return rows, nil
}

// requireTenantRole 校验当前租户账号具备指定角色。
func requireTenantRole(ctx context.Context, s *Service, role string) (tenant.Identity, error) {
	id, ok := tenant.FromContext(ctx)
	if !ok || id.IsPlatform || id.TenantID <= 0 {
		return tenant.Identity{}, apperr.ErrUnauthorized
	}
	has, err := s.HasRole(ctx, id.AccountID, role)
	if err != nil {
		return tenant.Identity{}, err
	}
	if !has {
		return tenant.Identity{}, apperr.ErrForbidden
	}
	return id, nil
}

// auditAccount 写账号管理类审计。
func (s *Service) auditAccount(ctx context.Context, id tenant.Identity, action string, targetID int64) error {
	entry, err := audit.BuildEntry(ctx, id.TenantID, id.AccountID, audit.ActorRoleSchoolAdmin, action, "identity.account", targetID, map[string]any{})
	if err != nil {
		return err
	}
	return s.auditWriter.Write(ctx, entry)
}

// auditTenantOperation 写租户内敏感配置和组织操作审计。
func (s *Service) auditTenantOperation(ctx context.Context, id tenant.Identity, action, targetType string, targetID int64, detail map[string]any) error {
	entry, err := audit.BuildEntry(ctx, id.TenantID, id.AccountID, audit.ActorRoleSchoolAdmin, action, targetType, targetID, detail)
	if err != nil {
		return err
	}
	return s.auditWriter.Write(ctx, entry)
}

// validateAccountOrgForProfile 校验账号档案组织挂靠,防止无外键 profile 写入不存在或类型错误的组织。
func validateAccountOrgForProfile(ctx context.Context, tx TxStore, tenantID int64, baseIdentity int16, orgID int64, enrollmentYear int16) error {
	if orgID <= 0 {
		return apperr.ErrIdentityAccountUpdateInvalid
	}
	switch baseIdentity {
	case BaseIdentityTeacher:
		ok, err := tx.DepartmentExists(ctx, tenantID, orgID)
		if err != nil {
			return err
		}
		if !ok {
			return apperr.ErrIdentityOrgInvalidInput
		}
	case BaseIdentityStudent:
		if enrollmentYear <= 0 {
			return apperr.ErrIdentityAccountUpdateInvalid
		}
		ok, err := tx.ClassExists(ctx, tenantID, orgID)
		if err != nil {
			return err
		}
		if !ok {
			return apperr.ErrIdentityOrgInvalidInput
		}
	default:
		return apperr.ErrIdentityBaseRoleInvalid
	}
	return nil
}
