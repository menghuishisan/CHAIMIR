// M1 初始化服务负责把部署期 bootstrap 输入落到 identity 自有表。
// 迁移脚本只做编排,首个租户、首个学校管理员和审计写入仍走本服务,避免 shell/SQL 复制一套账号规则。
package identity

import (
	"context"
	"strings"

	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/ids"
	"chaimir/pkg/apperr"
	"chaimir/pkg/crypto"
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

	accountID, created, err = s.repo.bootstrapPrivateSchool(ctx, req, accountID, phoneEnc, phoneHash, passwordHash, s.idgen.Generate, func(adminID int64, created bool) (AuditLogCreate, error) {
		// 初始化审计必须参与同一事务,避免留下无留痕的管理员创建。
		entry, err := buildBootstrapAuditEntry(ctx, req.TenantID, adminID, created)
		if err != nil {
			return AuditLogCreate{}, err
		}
		return buildAuditLogCreate(s.idgen.Generate(), entry), nil
	})
	if err != nil {
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
	adminID, created, err := s.repo.bootstrapPlatformAdmin(ctx, req, adminID, passwordHash)
	if err != nil {
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

// buildBootstrapAuditEntry 构造初始化审计条目。
// detail 只记录账号 ID 和是否新建,不写手机号明文、初始密码或其他 Secret。
func buildBootstrapAuditEntry(ctx context.Context, tenantID, adminID int64, created bool) (audit.Entry, error) {
	return buildAccountAuditEntry(ctx, tenantID, adminID, RoleSchoolAdmin, AuditActionTenantBootstrap, AuditTargetTenant, tenantID, map[string]any{
		"admin_id": ids.Format(adminID),
		"created":  created,
	})
}
