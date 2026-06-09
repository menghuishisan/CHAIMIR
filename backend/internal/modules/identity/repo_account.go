// M1 账号数据访问:集中处理账号、档案、角色、会话和账号审计的持久化事务。
package identity

import (
	"context"
	"time"

	"chaimir/internal/modules/identity/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
)

// createAccountWithAudit 原子创建账号、档案、基础角色、可选激活码和审计记录。
func (r *repo) createAccountWithAudit(ctx context.Context, tenantID, accountID, orgID int64, req CreateAccountRequest, phoneEnc []byte, phoneHash string, credential accountOpeningCredential, activationCodeHash string, activationExpireAt time.Time, actorID int64, nextID func() int64, auditLog AuditLogCreate) error {
	return r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		if err := ensureAccountOrgExists(ctx, q, req.BaseIdentity, orgID); err != nil {
			return err
		}
		// 账号、档案、角色和激活码必须同事务写入,避免管理员看到无法登录或无组织归属的半成品账号。
		if _, err := q.CreateAccount(ctx, sqlcgen.CreateAccountParams{
			ID: accountID, TenantID: tenantID, PhoneEnc: phoneEnc, PhoneHash: phoneHash,
			PasswordHash: credential.PasswordHash, Name: req.Name, BaseIdentity: req.BaseIdentity,
			Status: AccountPending, MustChangePwd: credential.MustChangePassword,
		}); err != nil {
			if isUniqueViolation(err) {
				return apperr.ErrPhoneAlreadyExists
			}
			return err
		}
		if _, err := q.CreateAccountProfile(ctx, sqlcgen.CreateAccountProfileParams{
			AccountID: accountID, TenantID: tenantID, No: req.No, OrgID: orgID,
			EnrollmentYear: pgtypex.Int2When(req.EnrollmentYear, req.BaseIdentity == BaseIdentityStudent),
			Title:          pgtypex.Text(req.Title),
		}); err != nil {
			if isUniqueViolation(err) {
				return apperr.ErrAccountNoAlreadyExists
			}
			return err
		}
		baseRole := RoleStudent
		if req.BaseIdentity == BaseIdentityTeacher {
			baseRole = RoleTeacher
		}
		if err := q.AddAccountRole(ctx, sqlcgen.AddAccountRoleParams{ID: nextID(), TenantID: tenantID, AccountID: accountID, Role: baseRole}); err != nil {
			return err
		}
		if credential.NeedsActivationCode {
			if err := r.createActivationCodeInTx(ctx, q, nextID(), tenantID, accountID, activationCodeHash, activationExpireAt, actorID); err != nil {
				return err
			}
		}
		return q.CreateAuditLog(ctx, auditLogParamsFromCreate(auditLog))
	})
}

// updateAccountNameWithAudit 确认账号存在后更新姓名并写审计。
func (r *repo) updateAccountNameWithAudit(ctx context.Context, accountID int64, name string, auditLog AuditLogCreate) error {
	return r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		if _, err := q.GetAccountByID(ctx, accountID); err != nil {
			return apperr.ErrAccountNotFound
		}
		if _, err := q.UpdateAccountName(ctx, sqlcgen.UpdateAccountNameParams{ID: accountID, Name: name}); err != nil {
			return err
		}
		return q.CreateAuditLog(ctx, auditLogParamsFromCreate(auditLog))
	})
}

// loadAccountForMutation 读取账号、角色和可选档案,供 service 执行业务规则判断。
func (r *repo) loadAccountForMutation(ctx context.Context, tenantID, accountID int64) (AccountMutationSnapshot, error) {
	var acc sqlcgen.Account
	var roles []int16
	var profile sqlcgen.AccountProfile
	hasProfile := true
	if err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		found, err := q.GetAccountByID(ctx, accountID)
		if err != nil {
			return apperr.ErrAccountNotFound
		}
		acc = found
		roles, err = q.ListAccountRoles(ctx, accountID)
		if err != nil {
			return err
		}
		prof, err := q.GetAccountProfile(ctx, accountID)
		if err != nil {
			if db.IsNoRows(err) {
				hasProfile = false
				return nil
			}
			return err
		}
		profile = prof
		return nil
	}); err != nil {
		return AccountMutationSnapshot{}, err
	}
	return accountMutationSnapshot(acc, roles, profile, hasProfile), nil
}

// updateAccountStatusWithAudit 写入账号状态迁移并记录迁移前后状态。
func (r *repo) updateAccountStatusWithAudit(ctx context.Context, accountID, tenantID int64, target int16, auditLog AuditLogCreate) error {
	return r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		if _, err := q.UpdateAccountStatus(ctx, sqlcgen.UpdateAccountStatusParams{ID: accountID, Status: target}); err != nil {
			return err
		}
		return q.CreateAuditLog(ctx, auditLogParamsFromCreate(auditLog))
	})
}

// archiveStudentAccountsByEnrollmentYearWithAudit 按学年归档学生账号并写批量审计。
func (r *repo) archiveStudentAccountsByEnrollmentYearWithAudit(ctx context.Context, enrollmentYear int16, auditLog func(archived []int64) (AuditLogCreate, error)) ([]int64, error) {
	var archived []int64
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		rows, err := q.ArchiveStudentAccountsByEnrollmentYear(ctx, pgtypex.Int2When(enrollmentYear, true))
		if err != nil {
			return err
		}
		archived = rows
		auditRow, err := auditLog(rows)
		if err != nil {
			return err
		}
		return q.CreateAuditLog(ctx, auditLogParamsFromCreate(auditRow))
	}); err != nil {
		return nil, err
	}
	return archived, nil
}

// forceLogoutWithAudit 确认账号存在后吊销全部会话并写审计。
func (r *repo) forceLogoutWithAudit(ctx context.Context, tenantID, accountID int64, auditLog AuditLogCreate) error {
	return r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		if _, err := q.GetAccountByID(ctx, accountID); err != nil {
			return apperr.ErrAccountNotFound
		}
		if err := q.RevokeAllAccountSessions(ctx, accountID); err != nil {
			return err
		}
		return q.CreateAuditLog(ctx, auditLogParamsFromCreate(auditLog))
	})
}

// resetAccountPasswordWithAudit 重置密码、清失败计数、吊销会话并写审计。
func (r *repo) resetAccountPasswordWithAudit(ctx context.Context, tenantID, accountID int64, passwordHash string, auditLog AuditLogCreate) error {
	return r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		if _, err := q.GetAccountByID(ctx, accountID); err != nil {
			return apperr.ErrAccountNotFound
		}
		if err := q.UpdateAccountPassword(ctx, sqlcgen.UpdateAccountPasswordParams{ID: accountID, PasswordHash: pgtypex.Text(passwordHash), MustChangePwd: true}); err != nil {
			return err
		}
		if err := q.ResetAccountPwdFailed(ctx, accountID); err != nil {
			return err
		}
		if err := q.RevokeAllAccountSessions(ctx, accountID); err != nil {
			return err
		}
		return q.CreateAuditLog(ctx, auditLogParamsFromCreate(auditLog))
	})
}

// grantAdminWithAudit 确认教师账号后授予学校管理员角色并写审计。
func (r *repo) grantAdminWithAudit(ctx context.Context, accountID, roleID int64, auditLog AuditLogCreate) error {
	return r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		acc, err := q.GetAccountByID(ctx, accountID)
		if err != nil {
			return apperr.ErrAccountNotFound
		}
		if acc.BaseIdentity != BaseIdentityTeacher {
			return apperr.ErrGrantAdminNonTeacher
		}
		if err := q.AddAccountRole(ctx, sqlcgen.AddAccountRoleParams{ID: roleID, TenantID: acc.TenantID, AccountID: accountID, Role: RoleSchoolAdmin}); err != nil {
			return err
		}
		return q.CreateAuditLog(ctx, auditLogParamsFromCreate(auditLog))
	})
}

// revokeAdminWithAudit 确认账号存在后撤销学校管理员角色并写审计。
func (r *repo) revokeAdminWithAudit(ctx context.Context, tenantID, accountID int64, auditLog AuditLogCreate) error {
	return r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		if _, err := q.GetAccountByID(ctx, accountID); err != nil {
			return apperr.ErrAccountNotFound
		}
		if err := q.RemoveAccountRole(ctx, sqlcgen.RemoveAccountRoleParams{AccountID: accountID, Role: RoleSchoolAdmin}); err != nil {
			return err
		}
		return q.CreateAuditLog(ctx, auditLogParamsFromCreate(auditLog))
	})
}

// changeAccountPasswordWithAudit 写入本人新密码、必要时激活账号并写审计。
func (r *repo) changeAccountPasswordWithAudit(ctx context.Context, tenantID, accountID int64, passwordHash string, activate bool, auditLog AuditLogCreate) error {
	return r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		if err := q.UpdateAccountPassword(ctx, sqlcgen.UpdateAccountPasswordParams{ID: accountID, PasswordHash: pgtypex.Text(passwordHash), MustChangePwd: false}); err != nil {
			return err
		}
		if activate {
			if err := q.SetAccountActivated(ctx, accountID); err != nil {
				return err
			}
		}
		return q.CreateAuditLog(ctx, auditLogParamsFromCreate(auditLog))
	})
}

// changeAccountPhoneWithAudit 换绑手机号并写审计。
func (r *repo) changeAccountPhoneWithAudit(ctx context.Context, tenantID, accountID int64, phoneEnc []byte, phoneHash string, auditLog AuditLogCreate) error {
	return r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		if _, err := q.GetAccountByID(ctx, accountID); err != nil {
			return apperr.ErrAccountNotFound
		}
		err := q.UpdateAccountPhone(ctx, sqlcgen.UpdateAccountPhoneParams{ID: accountID, PhoneEnc: phoneEnc, PhoneHash: phoneHash})
		if isUniqueViolation(err) {
			return apperr.ErrPhoneAlreadyExists
		}
		if err != nil {
			return err
		}
		return q.CreateAuditLog(ctx, auditLogParamsFromCreate(auditLog))
	})
}

// listActiveSessions 读取当前账号有效会话。
func (r *repo) listActiveSessions(ctx context.Context, tenantID, accountID int64) ([]SessionSnapshot, error) {
	var rows []sqlcgen.AuthSession
	if err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		found, err := q.ListActiveSessions(ctx, accountID)
		if err != nil {
			return err
		}
		rows = found
		return nil
	}); err != nil {
		return nil, err
	}
	out := make([]SessionSnapshot, 0, len(rows))
	for _, row := range rows {
		out = append(out, SessionSnapshot{
			ID: row.ID, DeviceInfo: textVal(row.DeviceInfo), IP: textVal(row.Ip),
			ExpireAt: timex.FromTimestamptz(row.ExpireAt), CreatedAt: timex.FromTimestamptz(row.CreatedAt),
		})
	}
	return out, nil
}

// ensureAccountOrgExists 按账号身份确认所属组织存在。
func ensureAccountOrgExists(ctx context.Context, q *sqlcgen.Queries, baseIdentity int16, orgID int64) error {
	if baseIdentity == BaseIdentityStudent {
		if _, err := q.GetClassByID(ctx, orgID); err != nil {
			return apperr.ErrClassNotFound
		}
		return nil
	}
	if _, err := q.GetDepartmentByID(ctx, orgID); err != nil {
		return apperr.ErrDepartmentNotFound
	}
	return nil
}

// accountMutationSnapshot 把账号行、角色和可选档案合并为业务判断投影。
func accountMutationSnapshot(row sqlcgen.Account, roles []int16, profile sqlcgen.AccountProfile, hasProfile bool) AccountMutationSnapshot {
	out := AccountMutationSnapshot{
		ID: row.ID, TenantID: row.TenantID, PhoneEnc: row.PhoneEnc,
		PasswordHash: textVal(row.PasswordHash), HasPassword: row.PasswordHash.Valid,
		Name: row.Name, BaseIdentity: row.BaseIdentity, Status: row.Status,
		MustChangePwd: row.MustChangePwd, Roles: roleCodesOf(roles),
	}
	if hasProfile {
		out.No = profile.No
		out.OrgID = profile.OrgID
		if profile.EnrollmentYear.Valid {
			year := profile.EnrollmentYear.Int16
			out.EnrollmentYear = &year
		}
		out.Title = textVal(profile.Title)
	}
	return out
}
