// M1 平台服务:入驻申请审核、租户管理、本校配置/SSO。
// 依据 docs/01 §3 接口、§5 §1 入驻状态机、§4 权限(平台层私有化关闭)。
package identity

import (
	"context"
	"time"

	"chaimir/internal/modules/identity/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/secretmap"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/crypto"

	"github.com/jackc/pgx/v5/pgtype"
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
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		_, e := q.CreateTenantApplication(ctx, sqlcgen.CreateTenantApplicationParams{
			ID: id, SchoolName: req.SchoolName, SchoolType: req.SchoolType,
			ContactName: req.ContactName, ContactPhone: req.ContactPhone, ContactEmail: req.ContactEmail,
		})
		return e
	}); err != nil {
		return "", apperr.ErrTenantApplicationStoreFailed.WithCause(err)
	}
	return ids.Format(id), nil
}

// ListApplications 平台管理员列申请。
func (s *Service) ListApplications(ctx context.Context, status int16, page, size int) ([]map[string]any, int64, error) {
	if err := validateApplicationStatus(status); err != nil {
		return nil, 0, err
	}
	var out []map[string]any
	var total int64
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		cnt, e := q.CountTenantApplications(ctx, pgInt2(status, status != 0))
		if e != nil {
			return e
		}
		total = cnt
		rows, e := q.ListTenantApplications(ctx, sqlcgen.ListTenantApplicationsParams{
			Status: pgInt2(status, status != 0), Limit: int32(size), Offset: int32((page - 1) * size),
		})
		if e != nil {
			return e
		}
		for _, r := range rows {
			out = append(out, map[string]any{
				"id": ids.Format(r.ID), "school_name": r.SchoolName, "school_type": r.SchoolType,
				"contact_name": r.ContactName, "contact_phone": r.ContactPhone, "contact_email": r.ContactEmail,
				"status": r.Status, "reject_reason": textVal(r.RejectReason), "created_at": timex.FromTimestamptz(r.CreatedAt),
			})
		}
		return nil
	}); err != nil {
		return nil, 0, apperr.ErrTenantQueryFailed.WithCause(err)
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

	var activationCode string
	if err := s.repo.inAppTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		app, e := q.GetTenantApplicationByID(ctx, appID)
		if e != nil {
			return apperr.ErrApplicationNotFound
		}
		if app.Status != ApplicationPending {
			return apperr.ErrApplicationHandled
		}
		// 建租户(SaaS 模式)。
		if _, e := q.CreateTenant(ctx, sqlcgen.CreateTenantParams{
			ID: tenantID, Code: tenantCode, Name: app.SchoolName, Type: app.SchoolType,
			Status: TenantActive, DeployMode: DeployModeSaaS,
			ExpireAt: pgtype.Timestamptz{}, AuthMode: AuthModeLocal, EnableActivationCode: true,
		}); e != nil {
			if isUniqueViolation(e) {
				return apperr.ErrTenantCodeExists
			}
			return e
		}
		// 回填申请为通过。
		if _, e := q.ApproveTenantApplication(ctx, sqlcgen.ApproveTenantApplicationParams{
			ID: appID, ReviewedBy: pgInt8(reviewerID, true), TenantID: pgInt8(tenantID, true),
		}); e != nil {
			return e
		}
		// 同一事务内已注入新租户 RLS,继续创建首个学校管理员,避免半成品租户。
		if _, e := q.CreateAccount(ctx, sqlcgen.CreateAccountParams{
			ID: adminID, TenantID: tenantID, PhoneEnc: phoneEnc, PhoneHash: ph,
			PasswordHash: pgtype.Text{}, Name: adminName, BaseIdentity: BaseIdentityTeacher,
			Status: AccountPending, MustChangePwd: false,
		}); e != nil {
			return e
		}
		// 加教师角色 + 学校管理员角色。
		if e := q.AddAccountRole(ctx, sqlcgen.AddAccountRoleParams{
			ID: s.idgen.Generate(), TenantID: tenantID, AccountID: adminID, Role: RoleTeacher,
		}); e != nil {
			return e
		}
		if e := q.AddAccountRole(ctx, sqlcgen.AddAccountRoleParams{
			ID: s.idgen.Generate(), TenantID: tenantID, AccountID: adminID, Role: RoleSchoolAdmin,
		}); e != nil {
			return e
		}
		code, e := s.CreateActivationCode(ctx, q, tenantID, adminID, reviewerID)
		if e != nil {
			return e
		}
		activationCode = code
		return nil
	}); err != nil {
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
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		r, e := q.RejectTenantApplication(ctx, sqlcgen.RejectTenantApplicationParams{
			ID: appID, ReviewedBy: pgInt8(reviewerID, true), RejectReason: pgText(reason),
		})
		if e != nil {
			return apperr.ErrTenantMutationFailed.WithCause(e)
		}
		if r.ID == 0 {
			return apperr.ErrApplicationHandled
		}
		return nil
	}); err != nil {
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
	var out []map[string]any
	var total int64
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		cnt, e := q.CountTenants(ctx, pgInt2(status, status != 0))
		if e != nil {
			return e
		}
		total = cnt
		rows, e := q.ListTenants(ctx, sqlcgen.ListTenantsParams{
			Status: pgInt2(status, status != 0), Limit: int32(size), Offset: int32((page - 1) * size),
		})
		if e != nil {
			return e
		}
		for _, r := range rows {
			out = append(out, tenantToMap(r))
		}
		return nil
	}); err != nil {
		return nil, 0, apperr.ErrTenantQueryFailed.WithCause(err)
	}
	return out, total, nil
}

// GetTenant 平台取租户详情。
func (s *Service) GetTenant(ctx context.Context, id int64) (map[string]any, error) {
	var m map[string]any
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		r, e := q.GetTenantByID(ctx, id)
		if e != nil {
			return apperr.ErrTenantNotFound
		}
		m = tenantToMap(r)
		return nil
	}); err != nil {
		return nil, toAppErr(err)
	}
	return m, nil
}

// UpdateTenant 平台改租户状态/到期。
func (s *Service) UpdateTenant(ctx context.Context, id int64, req UpdateTenantRequest) error {
	if err := validateTenantStatus(req.Status); err != nil {
		return err
	}
	var expire pgtype.Timestamptz
	if req.ExpireAt != "" {
		t, err := time.Parse(time.RFC3339, req.ExpireAt)
		if err != nil {
			return apperr.ErrTenantExpireAtInvalid
		}
		expire = timex.RequiredTimestamptz(t)
	}
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		if _, e := q.GetTenantByID(ctx, id); e != nil {
			return apperr.ErrTenantNotFound
		}
		if _, e := q.UpdateTenantStatus(ctx, sqlcgen.UpdateTenantStatusParams{
			ID: id, Status: req.Status, ExpireAt: expire,
		}); e != nil {
			return apperr.ErrTenantMutationFailed.WithCause(e)
		}
		return nil
	}); err != nil {
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
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		if _, e := q.GetTenantByID(ctx, tenantID); e != nil {
			return apperr.ErrTenantNotFound
		}
		if _, e := q.UpdateTenantConfig(ctx, sqlcgen.UpdateTenantConfigParams{
			ID: tenantID, LogoUrl: pgText(req.LogoURL), DisplayName: pgText(req.DisplayName),
			FeatureFlags: flags, AuthMode: req.AuthMode, EnableActivationCode: req.EnableActivationCode,
		}); e != nil {
			return apperr.ErrTenantMutationFailed.WithCause(e)
		}
		return nil
	}); err != nil {
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
	var view *SsoConfigView
	if err := s.repo.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		row, e := q.GetSsoConfig(ctx, tenantID)
		if e != nil {
			if db.IsNoRows(e) {
				view = &SsoConfigView{Config: map[string]any{}, Enabled: false}
				return nil
			}
			return e
		}
		cfg, e := jsonx.ObjectMapStrict(row.Config)
		if e != nil {
			return e
		}
		view = &SsoConfigView{
			ID: ids.Format(row.ID), Type: row.Type, Config: maskSsoConfig(cfg),
			MatchField: row.MatchField, Enabled: row.Enabled,
		}
		return nil
	}); err != nil {
		return nil, toAppErr(err)
	}
	return view, nil
}

// UpsertSsoConfig 保存本校 SSO/LDAP 配置。
func (s *Service) UpsertSsoConfig(ctx context.Context, tenantID int64, req SsoConfigRequest) (*SsoConfigView, error) {
	if req.Type != SsoTypeCAS && req.Type != SsoTypeLDAP {
		return nil, apperr.ErrSsoTypeInvalid
	}
	if req.MatchField != 1 && req.MatchField != 2 {
		return nil, apperr.ErrSsoMatchFieldInvalid
	}
	if err := validateSsoConfigForStorage(req.Type, req.Config); err != nil {
		return nil, err
	}
	protected, err := protectSsoConfig(s.cipher, req.Config)
	if err != nil {
		return nil, apperr.ErrSsoConfigProtectFailed.WithCause(err)
	}
	cfgBytes, err := jsonx.ObjectBytes(protected, apperr.ErrSsoConfigFormatInvalid)
	if err != nil {
		return nil, err
	}
	var view SsoConfigView
	if err := s.repo.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		row, e := q.UpsertSsoConfig(ctx, sqlcgen.UpsertSsoConfigParams{
			ID: s.idgen.Generate(), TenantID: tenantID, Type: req.Type,
			Config: cfgBytes, MatchField: req.MatchField, Enabled: req.Enabled,
		})
		if e != nil {
			return e
		}
		cfg, e := jsonx.ObjectMapStrict(row.Config)
		if e != nil {
			return e
		}
		view = SsoConfigView{
			ID: ids.Format(row.ID), Type: row.Type, Config: maskSsoConfig(cfg),
			MatchField: row.MatchField, Enabled: row.Enabled,
		}
		return s.writeAuditInTx(ctx, q, RoleSchoolAdmin, AuditActionTenantSSO, AuditTargetSSOConfig, row.ID, map[string]any{
			"type":        req.Type,
			"match_field": req.MatchField,
			"enabled":     req.Enabled,
		})
	}); err != nil {
		return nil, toAppErrWith(err, apperr.ErrSsoConfigReadFailed)
	}
	return &view, nil
}

// tenantToMap 把租户行转对外 map。
func tenantToMap(r sqlcgen.Tenant) map[string]any {
	m := map[string]any{
		"id": ids.Format(r.ID), "code": r.Code, "name": r.Name, "type": r.Type,
		"status": r.Status, "deploy_mode": r.DeployMode,
		"logo_url": textVal(r.LogoUrl), "display_name": textVal(r.DisplayName),
		"auth_mode": r.AuthMode, "enable_activation_code": r.EnableActivationCode,
	}
	if r.ExpireAt.Valid {
		m["expire_at"] = r.ExpireAt.Time
	}
	return m
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
