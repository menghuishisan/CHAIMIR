// M1 初始化数据访问:集中处理私有化租户、首个学校管理员和平台管理员的幂等写入。
package identity

import (
	"context"

	"chaimir/internal/modules/identity/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/pkg/apperr"

	"github.com/jackc/pgx/v5/pgtype"
)

// bootstrapPrivateSchool 原子创建或复用私有化租户和首个学校管理员。
func (r *repo) bootstrapPrivateSchool(ctx context.Context, req BootstrapPrivateSchoolRequest, accountID int64, phoneEnc []byte, phoneHash, passwordHash string, nextID func() int64, auditBuilder func(adminID int64, created bool) (AuditLogCreate, error)) (int64, bool, error) {
	created := false
	if err := r.inAppTenantID(ctx, req.TenantID, func(q *sqlcgen.Queries) error {
		tenantCreated, err := ensureBootstrapTenantInTx(ctx, q, req)
		if err != nil {
			return err
		}
		adminID, adminCreated, err := ensureBootstrapSchoolAdminInTx(ctx, q, req, accountID, phoneEnc, phoneHash, passwordHash, nextID)
		if err != nil {
			return err
		}
		accountID = adminID
		created = tenantCreated || adminCreated
		auditLog, err := auditBuilder(adminID, created)
		if err != nil {
			return err
		}
		return q.CreateAuditLog(ctx, auditLogParamsFromCreate(auditLog))
	}); err != nil {
		return 0, false, err
	}
	return accountID, created, nil
}

// bootstrapPlatformAdmin 幂等创建 SaaS 首个平台管理员。
func (r *repo) bootstrapPlatformAdmin(ctx context.Context, req BootstrapPlatformAdminRequest, adminID int64, passwordHash string) (int64, bool, error) {
	created := false
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		existing, err := q.GetPlatformAdminByUsername(ctx, req.Username)
		if err == nil {
			if existing.Status != TenantActive {
				return apperr.ErrBootstrapConflict
			}
			adminID = existing.ID
			return nil
		}
		if !db.IsNoRows(err) {
			return err
		}
		row, err := q.CreatePlatformAdmin(ctx, sqlcgen.CreatePlatformAdminParams{
			ID: adminID, Username: req.Username, PasswordHash: passwordHash, Name: req.Name, Status: TenantActive,
		})
		if err != nil {
			if isUniqueViolation(err) {
				return apperr.ErrBootstrapConflict
			}
			return err
		}
		adminID = row.ID
		created = true
		return nil
	}); err != nil {
		return 0, false, err
	}
	return adminID, created, nil
}

// ensureBootstrapTenantInTx 创建私有化租户,或校验已有租户与当前配置完全一致。
func ensureBootstrapTenantInTx(ctx context.Context, q *sqlcgen.Queries, req BootstrapPrivateSchoolRequest) (bool, error) {
	existing, err := q.GetTenantByID(ctx, req.TenantID)
	if err == nil {
		if existing.Code != req.Code || existing.Name != req.Name || existing.Type != req.Type || existing.DeployMode != DeployModeSchool {
			return false, apperr.ErrBootstrapConflict
		}
		return false, nil
	}
	if !db.IsNoRows(err) {
		return false, err
	}
	if _, err := q.CreateTenant(ctx, sqlcgen.CreateTenantParams{
		ID: req.TenantID, Code: req.Code, Name: req.Name, Type: req.Type,
		Status: TenantActive, DeployMode: DeployModeSchool,
		ExpireAt: pgtype.Timestamptz{}, AuthMode: AuthModeLocal, EnableActivationCode: false,
	}); err != nil {
		if isUniqueViolation(err) {
			return false, apperr.ErrTenantCodeExists
		}
		return false, err
	}
	return true, nil
}

// ensureBootstrapSchoolAdminInTx 创建首个学校管理员账号,或复用同手机号教师账号并补齐管理员角色。
func ensureBootstrapSchoolAdminInTx(ctx context.Context, q *sqlcgen.Queries, req BootstrapPrivateSchoolRequest, accountID int64, phoneEnc []byte, phoneHash, passwordHash string, nextID func() int64) (int64, bool, error) {
	account, err := q.GetAccountByPhoneHash(ctx, phoneHash)
	if err == nil {
		if account.TenantID != req.TenantID || account.BaseIdentity != BaseIdentityTeacher {
			return 0, false, apperr.ErrBootstrapConflict
		}
		if err := ensureBootstrapRoleInTx(ctx, q, req.TenantID, account.ID, RoleTeacher, nextID); err != nil {
			return 0, false, err
		}
		if err := ensureBootstrapRoleInTx(ctx, q, req.TenantID, account.ID, RoleSchoolAdmin, nextID); err != nil {
			return 0, false, err
		}
		return account.ID, false, nil
	}
	if !db.IsNoRows(err) {
		return 0, false, err
	}
	if _, err := q.CreateAccount(ctx, sqlcgen.CreateAccountParams{
		ID: accountID, TenantID: req.TenantID, PhoneEnc: phoneEnc, PhoneHash: phoneHash,
		PasswordHash: pgtype.Text{String: passwordHash, Valid: true},
		Name:         req.AdminName, BaseIdentity: BaseIdentityTeacher, Status: AccountActive, MustChangePwd: true,
	}); err != nil {
		if isUniqueViolation(err) {
			return 0, false, apperr.ErrPhoneAlreadyExists
		}
		return 0, false, err
	}
	if err := ensureBootstrapRoleInTx(ctx, q, req.TenantID, accountID, RoleTeacher, nextID); err != nil {
		return 0, false, err
	}
	if err := ensureBootstrapRoleInTx(ctx, q, req.TenantID, accountID, RoleSchoolAdmin, nextID); err != nil {
		return 0, false, err
	}
	return accountID, true, nil
}

// ensureBootstrapRoleInTx 通过 account_role 的唯一约束实现幂等补角色。
func ensureBootstrapRoleInTx(ctx context.Context, q *sqlcgen.Queries, tenantID, accountID int64, role int16, nextID func() int64) error {
	return q.AddAccountRole(ctx, sqlcgen.AddAccountRoleParams{
		ID: nextID(), TenantID: tenantID, AccountID: accountID, Role: role,
	})
}
