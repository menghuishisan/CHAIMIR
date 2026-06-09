// M1 平台服务:入驻申请审核、租户管理、本校配置/SSO。
// 依据 docs/01 §3 接口、§5 §1 入驻状态机、§4 权限(平台层私有化关闭)。
package identity

import (
	"context"
	"time"

	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/secretmap"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/crypto"
)

// CreateApplication 访客提交入驻申请(公开,状态=申请中)。
func (s *Service) CreateApplication(ctx context.Context, req CreateApplicationRequest) (string, error) {
	if err := validateSchoolType(req.SchoolType); err != nil {
		return "", err
	}
	if !validCNPhone(req.ContactPhone) {
		return "", apperr.ErrPhoneInvalid
	}
	id := s.idgen.Generate()
	if err := s.repo.createTenantApplication(ctx, id, req); err != nil {
		return "", apperr.ErrTenantApplicationStoreFailed.WithCause(err)
	}
	return ids.Format(id), nil
}

// ListApplications 平台管理员列申请。
func (s *Service) ListApplications(ctx context.Context, status int16, page, size int) ([]map[string]any, int64, error) {
	if err := validateApplicationStatus(status); err != nil {
		return nil, 0, err
	}
	rows, total, err := s.repo.listTenantApplications(ctx, status, page, size)
	if err != nil {
		return nil, 0, apperr.ErrTenantQueryFailed.WithCause(err)
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, applicationToMap(row))
	}
	return out, total, nil
}

// ApproveApplication 通过申请:创建租户 + 分配短码 + 建首个学校管理员(激活码开通)。
// 全程一个事务(平台级表无 RLS;新租户的 account/profile/role 写入需在该租户上下文)。
func (s *Service) ApproveApplication(ctx context.Context, appID int64, reviewerID int64, tenantCode, adminPhone, adminName string) (*ApproveApplicationResult, error) {
	if tenantCode == "" || adminPhone == "" || adminName == "" {
		return nil, apperr.ErrApplicationApproveInvalid
	}
	if !validCNPhone(adminPhone) {
		return nil, apperr.ErrPhoneInvalid
	}
	tenantID := s.idgen.Generate()
	adminID := s.idgen.Generate()
	phoneEnc, err := s.encryptPhone(adminPhone)
	if err != nil {
		return nil, apperr.ErrAccountCredentialFailed.WithCause(err)
	}
	ph := s.phoneHash(adminPhone)

	activationCode, err := s.genActivationCode()
	if err != nil {
		return nil, apperr.ErrActivationCodeIssueFailed.WithCause(err)
	}
	if err := s.repo.approveApplication(ctx, appID, reviewerID, tenantID, adminID, tenantCode, adminName, phoneEnc, ph, s.activationCodeHash(activationCode), timex.Now().Add(s.activationCodeTTL), s.idgen.Generate); err != nil {
		return nil, toAppErr(err)
	}
	if err := s.writePlatformAudit(ctx, reviewerID, AuditActionTenantApprove, AuditTargetApplication, appID, map[string]any{
		"tenant_id":   ids.Format(tenantID),
		"tenant_code": tenantCode,
		"admin_id":    ids.Format(adminID),
	}); err != nil {
		return nil, err
	}

	return &ApproveApplicationResult{
		TenantID: ids.Format(tenantID), TenantCode: tenantCode,
		AdminPhone:     maskPhone(adminPhone),
		ActivationCode: activationCode,
		ActivationHint: "已创建学校管理员,请使用激活码自设密码后登录",
	}, nil
}

// RejectApplication 驳回申请。
func (s *Service) RejectApplication(ctx context.Context, appID, reviewerID int64, reason string) error {
	if err := s.repo.rejectApplication(ctx, appID, reviewerID, reason); err != nil {
		return err
	}
	return s.writePlatformAudit(ctx, reviewerID, AuditActionTenantReject, AuditTargetApplication, appID, map[string]any{
		"reason_recorded": reason != "",
	})
}

// ListTenants 平台列租户。
func (s *Service) ListTenants(ctx context.Context, status int16, page, size int) ([]map[string]any, int64, error) {
	if err := validateOptionalTenantStatus(status); err != nil {
		return nil, 0, err
	}
	rows, total, err := s.repo.listTenants(ctx, status, page, size)
	if err != nil {
		return nil, 0, apperr.ErrTenantQueryFailed.WithCause(err)
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, tenantToMap(row))
	}
	return out, total, nil
}

// GetTenant 平台取租户详情。
func (s *Service) GetTenant(ctx context.Context, id int64) (map[string]any, error) {
	row, err := s.repo.getTenant(ctx, id)
	if err != nil {
		return nil, toAppErr(err)
	}
	return tenantToMap(row), nil
}

// UpdateTenant 平台改租户状态/到期。
func (s *Service) UpdateTenant(ctx context.Context, id int64, req UpdateTenantRequest) error {
	if err := validateTenantStatus(req.Status); err != nil {
		return err
	}
	var expire *time.Time
	if req.ExpireAt != "" {
		t, err := time.Parse(time.RFC3339, req.ExpireAt)
		if err != nil {
			return apperr.ErrTenantExpireAtInvalid
		}
		expire = &t
	}
	if err := s.repo.updateTenantStatus(ctx, id, req.Status, expire); err != nil {
		return err
	}
	current, ok := CurrentIdentity(ctx)
	if !ok {
		return apperr.ErrUnauthorized
	}
	return s.writePlatformAudit(ctx, current.AccountID, AuditActionTenantUpdate, AuditTargetTenant, id, map[string]any{
		"status":            req.Status,
		"expire_at_updated": req.ExpireAt != "",
	})
}

// GetTenantConfig 学校管理员取本校配置。
func (s *Service) GetTenantConfig(ctx context.Context, tenantID int64) (map[string]any, error) {
	return s.GetTenant(ctx, tenantID)
}

// UpdateTenantConfig 学校管理员改本校配置。
func (s *Service) UpdateTenantConfig(ctx context.Context, tenantID int64, req TenantConfigRequest) error {
	if err := validateAuthMode(req.AuthMode); err != nil {
		return err
	}
	flags, err := jsonx.ObjectBytes(req.FeatureFlags, apperr.ErrTenantFeatureFlagsInvalid)
	if err != nil {
		return err
	}
	if err := s.repo.updateTenantConfig(ctx, tenantID, req, flags); err != nil {
		return err
	}
	return s.writeAudit(ctx, RoleSchoolAdmin, AuditActionTenantConfig, AuditTargetTenant, tenantID, map[string]any{
		"fields":                 []string{"logo_url", "display_name", "feature_flags", "auth_mode", "enable_activation_code"},
		"auth_mode":              req.AuthMode,
		"enable_activation_code": req.EnableActivationCode,
	})
}

// GetSsoConfig 读取本校启用的 SSO/LDAP 配置。
func (s *Service) GetSsoConfig(ctx context.Context, tenantID int64) (*SsoConfigView, error) {
	row, found, err := s.repo.getSsoConfigForView(ctx, tenantID)
	if err != nil {
		return nil, toAppErr(err)
	}
	if !found {
		return &SsoConfigView{Config: map[string]any{}, Enabled: false}, nil
	}
	return ssoConfigViewFromSnapshot(row)
}

// UpsertSsoConfig 保存本校 SSO/CAS/LDAP 配置,敏感字段加密后才进入 JSONB。
func (s *Service) UpsertSsoConfig(ctx context.Context, tenantID int64, req SsoConfigRequest) (*SsoConfigView, error) {
	if req.Type != SsoTypeCAS && req.Type != SsoTypeLDAP {
		return nil, apperr.ErrSsoTypeInvalid
	}
	if req.MatchField != 1 && req.MatchField != 2 {
		return nil, apperr.ErrSsoMatchFieldInvalid
	}
	// 先按类型校验配置完整性和外部地址安全边界,防止保存不可用或可 SSRF 的 SSO 配置。
	if err := validateSsoConfigForStorage(req.Type, req.Config); err != nil {
		return nil, err
	}
	// 再加密 bind_password/client_secret 等敏感字段,持久化层只接收受保护配置。
	protected, err := protectSsoConfig(s.cipher, req.Config)
	if err != nil {
		return nil, apperr.ErrSsoConfigProtectFailed.WithCause(err)
	}
	cfgBytes, err := jsonx.ObjectBytes(protected, apperr.ErrSsoConfigFormatInvalid)
	if err != nil {
		return nil, err
	}
	row, err := s.repo.upsertSsoConfigWithAudit(ctx, tenantID, s.idgen.Generate(), req, cfgBytes, func(rowID int64) (AuditLogCreate, error) {
		entry, err := buildAuditEntry(ctx, RoleSchoolAdmin, AuditActionTenantSSO, AuditTargetSSOConfig, rowID, map[string]any{
			"type":        req.Type,
			"match_field": req.MatchField,
			"enabled":     req.Enabled,
		})
		if err != nil {
			return AuditLogCreate{}, err
		}
		return buildAuditLogCreate(s.idgen.Generate(), entry), nil
	})
	if err != nil {
		return nil, toAppErrWith(err, apperr.ErrSsoConfigReadFailed)
	}
	return ssoConfigViewFromSnapshot(row)
}

// applicationToMap 把入驻申请投影转对外 map。
func applicationToMap(r TenantApplicationSnapshot) map[string]any {
	return map[string]any{
		"id": ids.Format(r.ID), "school_name": r.SchoolName, "school_type": r.SchoolType,
		"contact_name": r.ContactName, "contact_phone": r.ContactPhone, "contact_email": r.ContactEmail,
		"status": r.Status, "reject_reason": r.RejectReason, "created_at": r.CreatedAt,
	}
}

// tenantToMap 把租户行转对外 map。
func tenantToMap(r TenantSnapshot) map[string]any {
	m := map[string]any{
		"id": ids.Format(r.ID), "code": r.Code, "name": r.Name, "type": r.Type,
		"status": r.Status, "deploy_mode": r.DeployMode,
		"logo_url": r.LogoURL, "display_name": r.DisplayName,
		"auth_mode": r.AuthMode, "enable_activation_code": r.EnableActivationCode,
	}
	if r.HasExpireAt {
		m["expire_at"] = r.ExpireAt
	}
	return m
}

// ssoConfigViewFromSnapshot 把 SSO 配置投影转换为脱敏响应视图。
func ssoConfigViewFromSnapshot(row SsoConfigSnapshot) (*SsoConfigView, error) {
	cfg, err := jsonx.ObjectMapStrict(row.Config)
	if err != nil {
		return nil, err
	}
	return &SsoConfigView{
		ID: ids.Format(row.ID), Type: row.Type, Config: maskSsoConfig(cfg),
		MatchField: row.MatchField, Enabled: row.Enabled,
	}, nil
}

// protectSsoConfig 加密 SSO/LDAP 配置中的敏感字段。
func protectSsoConfig(cipher *crypto.Cipher, cfg map[string]any) (map[string]any, error) {
	return secretmap.Protect(cipher, cfg, "SSO 敏感配置")
}

// maskSsoConfig 脱敏 SSO/LDAP 配置响应。
func maskSsoConfig(cfg map[string]any) map[string]any {
	return secretmap.Mask(cfg)
}

// revealSsoConfig 还原服务端内部使用的 SSO/LDAP 配置,仅供协议适配器使用。
func revealSsoConfig(cipher *crypto.Cipher, cfg map[string]any) (map[string]any, error) {
	return secretmap.Reveal(cipher, cfg, "SSO 敏感配置")
}
