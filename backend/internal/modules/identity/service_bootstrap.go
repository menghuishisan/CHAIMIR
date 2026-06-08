// M1 初始化服务负责把部署期 bootstrap 输入落到 identity 自有表。
// 迁移脚本只做编排,首个租户、首个学校管理员和审计写入仍走本服务,避免 shell/SQL 复制一套账号规则。
package identity

import (
	"context"
	"strings"

	"chaimir/internal/modules/identity/internal/sqlcgen"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/ids"
	"chaimir/pkg/apperr"
	"chaimir/pkg/crypto"

	"github.com/jackc/pgx/v5/pgtype"
)

// BootstrapPrivateSchoolRequest 描述私有化单校部署首次启动所需的权威输入。
// TenantID 复用 SCHOOL_TENANT_ID,避免运行期租户与初始化租户出现两套 ID。
type BootstrapPrivateSchoolRequest struct {
	TenantID  int64
	Code      string
	Name      string
	Type      int16
	Phone     string
	AdminName string
	Password  string
}

// BootstrapPrivateSchoolResult 返回初始化最终绑定的租户和管理员账号。
// Created 只表示本次是否新建核心记录,不承诺重跑时会重置已有账号凭据。
type BootstrapPrivateSchoolResult struct {
	TenantID string
	AdminID  string
	Created  bool
}

// BootstrapPlatformAdminRequest 描述 SaaS 首个平台管理员初始化输入。
// 平台管理员不属于任何租户,因此只写 platform_admin 表,不会创建 account/account_role。
type BootstrapPlatformAdminRequest struct {
	Username string
	Name     string
	Password string
}

// BootstrapPlatformAdminResult 返回 SaaS 平台管理员初始化结果。
// Created=false 表示账号已存在且配置一致,初始化命令重跑不会重置密码。
type BootstrapPlatformAdminResult struct {
	AdminID string
	Created bool
}

// BootstrapPrivateSchool 幂等创建私有化租户和首个学校管理员。
// 重跑初始化只补齐缺失角色,不会覆盖已有管理员密码,避免升级/重放 Job 破坏真实凭据。
func (s *Service) BootstrapPrivateSchool(ctx context.Context, req BootstrapPrivateSchoolRequest) (*BootstrapPrivateSchoolResult, error) {
	if err := validateBootstrapPrivateSchoolRequest(req); err != nil {
		return nil, err
	}
	passwordHash, err := crypto.HashPassword(req.Password)
	if err != nil {
		return nil, apperr.ErrAccountCredentialFailed.WithCause(err)
	}
	phoneEnc, err := s.encryptPhone(req.Phone)
	if err != nil {
		return nil, apperr.ErrAccountCredentialFailed.WithCause(err)
	}
	phoneHash := s.phoneHash(req.Phone)
	accountID := s.idgen.Generate()
	created := false

	if err := s.repo.inAppTenantID(ctx, req.TenantID, func(q *sqlcgen.Queries) error {
		// 第一步确保全局租户存在;已有租户必须与配置一致,防止初始化串到错误学校。
		tenantCreated, err := s.ensureBootstrapTenant(ctx, q, req)
		if err != nil {
			return err
		}
		// 第二步创建或复用教师账号,并保证学校管理员角色完整。
		adminID, adminCreated, err := s.ensureBootstrapSchoolAdmin(ctx, q, req, accountID, phoneEnc, phoneHash, passwordHash)
		if err != nil {
			return err
		}
		accountID = adminID
		created = tenantCreated || adminCreated
		// 第三步把初始化纳入统一审计;审计失败必须回滚,不能留下无留痕的管理员创建。
		entry, err := buildBootstrapAuditEntry(ctx, req.TenantID, adminID, created)
		if err != nil {
			return err
		}
		return q.CreateAuditLog(ctx, buildAuditLogParams(s.idgen.Generate(), entry))
	}); err != nil {
		return nil, toAppErr(err)
	}
	return &BootstrapPrivateSchoolResult{
		TenantID: ids.Format(req.TenantID),
		AdminID:  ids.Format(accountID),
		Created:  created,
	}, nil
}

// BootstrapPlatformAdmin 幂等创建 SaaS 首个平台管理员。
// 已存在同用户名账号时只校验可用状态,不覆盖密码,避免部署 Job 重跑导致平台管理员凭据被重置。
func (s *Service) BootstrapPlatformAdmin(ctx context.Context, req BootstrapPlatformAdminRequest) (*BootstrapPlatformAdminResult, error) {
	if err := validateBootstrapPlatformAdminRequest(req); err != nil {
		return nil, err
	}
	passwordHash, err := crypto.HashPassword(req.Password)
	if err != nil {
		return nil, apperr.ErrAccountCredentialFailed.WithCause(err)
	}
	adminID := s.idgen.Generate()
	created := false
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		existing, err := q.GetPlatformAdminByUsername(ctx, req.Username)
		if err == nil {
			if existing.Status != 1 {
				return apperr.ErrBootstrapConflict
			}
			adminID = existing.ID
			return nil
		}
		if !db.IsNoRows(err) {
			return err
		}
		row, err := q.CreatePlatformAdmin(ctx, sqlcgen.CreatePlatformAdminParams{
			ID: adminID, Username: req.Username, PasswordHash: passwordHash, Name: req.Name, Status: 1,
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
		return nil, toAppErr(err)
	}
	return &BootstrapPlatformAdminResult{AdminID: ids.Format(adminID), Created: created}, nil
}

// validateBootstrapPrivateSchoolRequest 在进入数据库前校验初始化边界。
// 这里复用学校类型、手机号和密码策略,保证 bootstrap 与普通账号开通没有两套规则。
func validateBootstrapPrivateSchoolRequest(req BootstrapPrivateSchoolRequest) error {
	if req.TenantID == 0 ||
		strings.TrimSpace(req.Code) == "" ||
		strings.TrimSpace(req.Name) == "" ||
		strings.TrimSpace(req.AdminName) == "" ||
		strings.TrimSpace(req.Password) == "" {
		return apperr.ErrBootstrapConfigInvalid
	}
	if err := validateSchoolType(req.Type); err != nil {
		return err
	}
	if !validCNPhone(req.Phone) {
		return apperr.ErrPhoneInvalid
	}
	if !validPassword(req.Password) {
		return apperr.ErrWeakPassword
	}
	return nil
}

// validateBootstrapPlatformAdminRequest 校验 SaaS 平台管理员初始化边界。
// 平台层是全局高权限入口,同样必须复用统一密码策略并拒绝空白用户名。
func validateBootstrapPlatformAdminRequest(req BootstrapPlatformAdminRequest) error {
	if strings.TrimSpace(req.Username) == "" ||
		strings.TrimSpace(req.Name) == "" ||
		strings.TrimSpace(req.Password) == "" {
		return apperr.ErrBootstrapConfigInvalid
	}
	if !validPassword(req.Password) {
		return apperr.ErrWeakPassword
	}
	return nil
}

// ensureBootstrapTenant 创建私有化租户,或校验已有租户与当前配置完全一致。
// 初始化是部署期高权限动作,遇到不一致必须失败,不能猜测是否应改写已有学校资料。
func (s *Service) ensureBootstrapTenant(ctx context.Context, q *sqlcgen.Queries, req BootstrapPrivateSchoolRequest) (bool, error) {
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

// ensureBootstrapSchoolAdmin 创建首个学校管理员账号,或复用同手机号教师账号并补齐管理员角色。
// 首个管理员允许暂缺 account_profile,因为真实组织架构必须由管理员登录后维护。
func (s *Service) ensureBootstrapSchoolAdmin(ctx context.Context, q *sqlcgen.Queries, req BootstrapPrivateSchoolRequest, accountID int64, phoneEnc []byte, phoneHash, passwordHash string) (int64, bool, error) {
	account, err := q.GetAccountByPhoneHash(ctx, phoneHash)
	if err == nil {
		if account.TenantID != req.TenantID || account.BaseIdentity != BaseIdentityTeacher {
			return 0, false, apperr.ErrBootstrapConflict
		}
		if err := s.ensureBootstrapRole(ctx, q, req.TenantID, account.ID, RoleTeacher); err != nil {
			return 0, false, err
		}
		if err := s.ensureBootstrapRole(ctx, q, req.TenantID, account.ID, RoleSchoolAdmin); err != nil {
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
	if err := s.ensureBootstrapRole(ctx, q, req.TenantID, accountID, RoleTeacher); err != nil {
		return 0, false, err
	}
	if err := s.ensureBootstrapRole(ctx, q, req.TenantID, accountID, RoleSchoolAdmin); err != nil {
		return 0, false, err
	}
	return accountID, true, nil
}

// ensureBootstrapRole 通过 account_role 的唯一约束实现幂等补角色。
func (s *Service) ensureBootstrapRole(ctx context.Context, q *sqlcgen.Queries, tenantID, accountID int64, role int16) error {
	return q.AddAccountRole(ctx, sqlcgen.AddAccountRoleParams{
		ID: s.idgen.Generate(), TenantID: tenantID, AccountID: accountID, Role: role,
	})
}

// buildBootstrapAuditEntry 构造初始化审计条目。
// detail 只记录账号 ID 和是否新建,不写手机号明文、初始密码或其他 Secret。
func buildBootstrapAuditEntry(ctx context.Context, tenantID, adminID int64, created bool) (audit.Entry, error) {
	return buildAccountAuditEntry(ctx, tenantID, adminID, RoleSchoolAdmin, AuditActionTenantBootstrap, AuditTargetTenant, tenantID, map[string]any{
		"admin_id": ids.Format(adminID),
		"created":  created,
	})
}
