// identity service_platform 文件实现 SaaS 入驻申请、租户管理和平台审核流程。
package identity

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/crypto"
	"chaimir/pkg/snowflake"

	"github.com/jackc/pgx/v5"
)

// CreateApplication 提交学校入驻申请,这是申请流程而非自助注册。
func (s *Service) CreateApplication(ctx context.Context, req CreateApplicationRequest) (TenantApplication, error) {
	if err := s.ensurePlatformLayerEnabled(); err != nil {
		return TenantApplication{}, err
	}
	if strings.TrimSpace(req.SchoolName) == "" || !validSchoolType(req.SchoolType) || strings.TrimSpace(req.ContactName) == "" {
		return TenantApplication{}, apperr.ErrIdentityApplicationInvalid
	}
	if err := ValidatePhone(req.ContactPhone); err != nil {
		return TenantApplication{}, err
	}
	if err := ValidateEmail(req.ContactEmail); err != nil {
		return TenantApplication{}, err
	}
	req.ContactPhone = strings.TrimSpace(req.ContactPhone)
	req.ContactEmail = strings.TrimSpace(req.ContactEmail)
	var app TenantApplication
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		duplicate, err := tx.HasPendingTenantApplication(ctx, req.ContactPhone, req.ContactEmail)
		if err != nil {
			return err
		}
		if duplicate {
			return apperr.ErrIdentityApplicationInvalid
		}
		row, err := tx.CreateTenantApplication(ctx, req, s.ids.Generate())
		if err != nil {
			return err
		}
		app = row
		return nil
	}); err != nil {
		if isUniqueViolation(err) {
			return TenantApplication{}, apperr.ErrIdentityApplicationInvalid
		}
		return TenantApplication{}, apperr.ErrInternal.WithCause(err)
	}
	return app, nil
}

// ApproveApplication 审核通过入驻申请,创建租户和首个学校管理员账号。
func (s *Service) ApproveApplication(ctx context.Context, appID int64, req ReviewApplicationRequest) (TenantDTO, string, error) {
	if err := s.ensurePlatformLayerEnabled(); err != nil {
		return TenantDTO{}, "", err
	}
	id, ok := tenant.FromContext(ctx)
	if !ok || !id.IsPlatform {
		return TenantDTO{}, "", apperr.ErrForbidden
	}
	if err := ValidateTenantCode(req.TenantCode); err != nil {
		return TenantDTO{}, "", err
	}
	if err := ValidatePhone(req.AdminPhone); err != nil {
		return TenantDTO{}, "", err
	}
	var out TenantDTO
	var activation string
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		// 平台审核在同一事务内创建租户、更新申请、注入新租户 RLS 并开通首个管理员。
		app, err := tx.GetTenantApplication(ctx, appID)
		if err != nil {
			return err
		}
		tenantID := s.ids.Generate()
		t, err := tx.CreateTenant(ctx, CreateTenantInput{
			ID:                   tenantID,
			Code:                 strings.TrimSpace(req.TenantCode),
			Name:                 app.SchoolName,
			Type:                 app.SchoolType,
			Status:               TenantStatusActive,
			DeployMode:           DeployModeSaaS,
			FeatureFlags:         []byte(`{}`),
			AuthMode:             1,
			EnableActivationCode: true,
		})
		if err != nil {
			return err
		}
		out, err = ToTenantDTO(t)
		if err != nil {
			return err
		}
		if _, err := tx.ApproveTenantApplication(ctx, appID, id.AccountID, tenantID); err != nil {
			return err
		}

		_, code, err := s.createBootstrapAdminInTx(ctx, tx, tenantID, req.AdminName, req.AdminPhone)
		if err != nil {
			return err
		}
		// 首管账号创建失败会回滚租户和申请状态,激活码明文只在审核结果中返回一次。
		activation = code
		if err := enqueueTenantProvision(ctx, tx, s.ids, t); err != nil {
			return err
		}
		entry, err := audit.BuildEntry(ctx, 0, id.AccountID, contracts.RoleNumPlatformAdmin, "tenant.application.approve", "identity.tenant_application", appID, map[string]any{"tenant_id": t.ID})
		if err != nil {
			return err
		}
		return s.auditWriter.writeInTx(ctx, tx, entry)
	}); err != nil {
		return TenantDTO{}, "", apperr.ErrInternal.WithCause(err)
	}
	s.drainTenantProvisionOutboxBestEffort(ctx)
	return out, activation, nil
}

// RejectApplication 驳回学校入驻申请。
func (s *Service) RejectApplication(ctx context.Context, appID int64, reason string) error {
	if err := s.ensurePlatformLayerEnabled(); err != nil {
		return err
	}
	id, ok := tenant.FromContext(ctx)
	if !ok || !id.IsPlatform {
		return apperr.ErrForbidden
	}
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		_, err := tx.RejectTenantApplication(ctx, appID, id.AccountID, strings.TrimSpace(reason))
		if err != nil {
			return err
		}
		return s.auditPlatformOperation(ctx, id.AccountID, "tenant.application.reject", "identity.tenant_application", appID, map[string]any{})
	}); err != nil {
		return apperr.ErrInternal.WithCause(err)
	}
	return nil
}

// ListApplicationsByPlatform 读取入驻申请列表,仅平台管理员可访问。
func (s *Service) ListApplicationsByPlatform(ctx context.Context, status int16) ([]TenantApplication, error) {
	if err := s.ensurePlatformLayerEnabled(); err != nil {
		return nil, err
	}
	id, ok := tenant.FromContext(ctx)
	if !ok || !id.IsPlatform {
		return nil, apperr.ErrForbidden
	}
	var out []TenantApplication
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		rows, err := tx.ListTenantApplications(ctx, status)
		if err != nil {
			return err
		}
		out = rows
		return nil
	}); err != nil {
		return nil, apperr.ErrInternal.WithCause(err)
	}
	return out, nil
}

// GetApplicationByPlatform 读取平台管理员可见的单个入驻申请,不返回任何凭据字段。
func (s *Service) GetApplicationByPlatform(ctx context.Context, appID int64) (TenantApplication, error) {
	if err := s.ensurePlatformLayerEnabled(); err != nil {
		return TenantApplication{}, err
	}
	id, ok := tenant.FromContext(ctx)
	if !ok || !id.IsPlatform {
		return TenantApplication{}, apperr.ErrForbidden
	}
	if appID <= 0 {
		return TenantApplication{}, apperr.ErrIdentityApplicationInvalid
	}
	var out TenantApplication
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.GetTenantApplication(ctx, appID)
		return err
	}); err != nil {
		return TenantApplication{}, apperr.ErrIdentityApplicationInvalid.WithCause(err)
	}
	return out, nil
}

// ListTenantsByPlatform 分页读取平台租户列表,用于 SaaS 平台学校管理页。
// status 传 0、keyword 传空串表示该维度不过滤;过滤与计数都在服务端完成,
// 前端不得再对当前页做二次筛选(否则分页与总数会自相矛盾)。
func (s *Service) ListTenantsByPlatform(ctx context.Context, status int16, keyword string, page, size int) ([]TenantDTO, int64, int, int, pagex.Facets, error) {
	if err := s.ensurePlatformLayerEnabled(); err != nil {
		return nil, 0, page, size, nil, err
	}
	id, ok := tenant.FromContext(ctx)
	if !ok || !id.IsPlatform {
		return nil, 0, page, size, nil, apperr.ErrForbidden
	}
	page, size = pagex.Normalize(page, size)
	var rows []Tenant
	var byStatus pagex.FacetGroup
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		items, err := tx.ListTenants(ctx, TenantListQuery{
			Status:  status,
			Keyword: strings.TrimSpace(keyword),
			Page:    page,
			Size:    size,
		})
		if err != nil {
			return err
		}
		rows = items
		// 状态分段计数:套用关键词,不套用状态筛选
		byStatus, err = tx.CountTenantsByStatus(ctx, strings.TrimSpace(keyword))
		return err
	}); err != nil {
		return nil, 0, page, size, nil, apperr.ErrInternal.WithCause(err)
	}
	out := make([]TenantDTO, 0, len(rows))
	for _, row := range rows {
		dto, err := ToTenantDTO(row)
		if err != nil {
			return nil, 0, page, size, nil, err
		}
		out = append(out, dto)
	}
	statusKey := ""
	if status != 0 {
		statusKey = strconv.Itoa(int(status))
	}
	return out, byStatus.Total(statusKey), page, size, pagex.Facets{"status": byStatus}, nil
}

// GetTenantByPlatform 读取单个租户详情,用于平台管理员管理学校状态。
func (s *Service) GetTenantByPlatform(ctx context.Context, tenantID int64) (TenantDTO, error) {
	if err := s.ensurePlatformLayerEnabled(); err != nil {
		return TenantDTO{}, err
	}
	id, ok := tenant.FromContext(ctx)
	if !ok || !id.IsPlatform {
		return TenantDTO{}, apperr.ErrForbidden
	}
	var row Tenant
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		item, err := tx.GetTenantByID(ctx, tenantID)
		if err != nil {
			return err
		}
		row = item
		return nil
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TenantDTO{}, apperr.ErrNotFound.WithCause(err)
		}
		return TenantDTO{}, apperr.ErrInternal.WithCause(err)
	}
	return ToTenantDTO(row)
}

// UpdateTenantStatusByPlatform 修改租户启停和到期时间,平台管理员边界由 service 再次校验。
func (s *Service) UpdateTenantStatusByPlatform(ctx context.Context, tenantID int64, req UpdateTenantStatusRequest) (TenantDTO, error) {
	if err := s.ensurePlatformLayerEnabled(); err != nil {
		return TenantDTO{}, err
	}
	id, ok := tenant.FromContext(ctx)
	if !ok || !id.IsPlatform {
		return TenantDTO{}, apperr.ErrForbidden
	}
	if req.Status != TenantStatusActive && req.Status != TenantStatusDisabled && req.Status != TenantStatusExpired {
		return TenantDTO{}, apperr.ErrIdentityTenantStatusInvalid
	}
	var out TenantDTO
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		item, err := tx.UpdateTenantStatus(ctx, UpdateTenantStatusInput{TenantID: tenantID, Status: req.Status, ExpireAt: req.ExpireAt})
		if err != nil {
			return err
		}
		out, err = ToTenantDTO(item)
		if err != nil {
			return err
		}
		return s.auditPlatformOperation(ctx, id.AccountID, "tenant.status.update", "identity.tenant", tenantID, map[string]any{"status": req.Status})
	}); err != nil {
		return TenantDTO{}, apperr.ErrInternal.WithCause(err)
	}
	return out, nil
}

// Bootstrap 初始化当前部署的首个管理员，学校资源开通交由在线 outbox worker。
func Bootstrap(ctx context.Context, store Store, ids snowflake.Generator, deploy config.DeployConfig, authCfg config.AuthConfig, cfg config.BootstrapConfig) error {
	if store == nil || ids == nil {
		return fmt.Errorf("identity bootstrap 缺少 store 或 ID 生成器")
	}
	if deploy.IsSaaS() {
		return bootstrapPlatformAdmin(ctx, store, ids, cfg)
	}
	if !deploy.IsSchool() {
		return apperr.ErrIdentityPlatformLayerDisabled
	}
	if err := validateBootstrapConfig(cfg); err != nil {
		return err
	}
	tenantID := deploy.SchoolTenantID
	if tenantID <= 0 {
		return fmt.Errorf("SCHOOL_TENANT_ID 必须为正整数")
	}
	passwordHash, err := crypto.HashPassword(cfg.AdminPassword)
	if err != nil {
		return apperr.ErrInternal.WithCause(err)
	}
	cipher, err := newPhoneCipher(authCfg.EncryptionKey)
	if err != nil {
		return err
	}
	phoneEnc, phoneHash, err := crypto.ProtectPhone(cipher, []byte(authCfg.HMACKey), cfg.AdminPhone)
	if err != nil {
		return err
	}
	writer := AuditWriter{store: store, ids: ids}
	return store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		code := strings.TrimSpace(cfg.SchoolTenantCode)
		existing, err := tx.GetTenantByID(ctx, tenantID)
		if err == nil {
			if existing.DeployMode != DeployModeSchool || existing.Code != code {
				return apperr.ErrIdentityBootstrapInvalid
			}
			return nil
		}
		if !isNoRows(err) {
			return err
		}
		// 私有化初始化直接创建学校租户,不经过 SaaS 入驻申请状态机。
		row, err := tx.CreateTenant(ctx, CreateTenantInput{
			ID:                   tenantID,
			Code:                 code,
			Name:                 strings.TrimSpace(cfg.SchoolName),
			Type:                 cfg.SchoolType,
			Status:               TenantStatusActive,
			DeployMode:           DeployModeSchool,
			FeatureFlags:         []byte(`{}`),
			AuthMode:             AuthModeLocal,
			EnableActivationCode: false,
		})
		if err != nil {
			return err
		}
		// 首个学校管理员仍不写虚假组织档案,真实院系由其登录后维护。
		adminID := ids.Generate()
		if _, err := tx.CreateAccount(ctx, CreateAccountInput{
			ID:            adminID,
			TenantID:      tenantID,
			PhoneEnc:      phoneEnc,
			PhoneHash:     phoneHash,
			PasswordHash:  passwordHash,
			Name:          strings.TrimSpace(cfg.AdminName),
			BaseIdentity:  BaseIdentityTeacher,
			Status:        AccountStatusActive,
			MustChangePwd: true,
			ActivatedAt:   ptrTime(timex.Now()),
			Roles: []RoleCreateInput{
				{ID: ids.Generate(), Role: contracts.RoleNumTeacher},
				{ID: ids.Generate(), Role: contracts.RoleNumSchoolAdmin},
			},
		}); err != nil {
			return err
		}
		if err := enqueueTenantProvision(ctx, tx, ids, row); err != nil {
			return err
		}
		// 初始化由系统任务触发,审计 actor 使用系统角色而不是伪造平台管理员。
		entry, err := audit.BuildEntry(ctx, tenantID, 0, audit.ActorRoleSystem, "tenant.bootstrap", "identity.tenant", tenantID, map[string]any{"deploy_mode": DeployModeSchool})
		if err != nil {
			return err
		}
		return writer.writeInTx(ctx, tx, entry)
	})
}

// bootstrapPlatformAdmin 为 SaaS 初始化首个平台管理员,已存在同名账号时保持原密码不变。
func bootstrapPlatformAdmin(ctx context.Context, store Store, ids snowflake.Generator, cfg config.BootstrapConfig) error {
	username := strings.TrimSpace(cfg.PlatformAdminUser)
	name := strings.TrimSpace(cfg.PlatformAdminName)
	if username == "" || name == "" {
		return apperr.ErrIdentityBootstrapInvalid
	}
	if err := ValidatePassword(cfg.PlatformAdminPassword); err != nil {
		return err
	}
	passwordHash, err := crypto.HashPassword(cfg.PlatformAdminPassword)
	if err != nil {
		return apperr.ErrInternal.WithCause(err)
	}
	return store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		// SaaS bootstrap 只在账号不存在时写入,避免重复运行迁移覆盖生产管理员口令。
		return tx.CreatePlatformAdminIfNotExists(ctx, CreatePlatformAdminInput{
			ID:           ids.Generate(),
			Username:     username,
			PasswordHash: passwordHash,
			Name:         name,
			Status:       TenantStatusActive,
		})
	})
}

// createBootstrapAdminInTx 创建首个学校管理员账号、教师/学校管理员角色和一次性激活码。
func (s *Service) createBootstrapAdminInTx(ctx context.Context, tx TxStore, tenantID int64, name, phone string) (Account, string, error) {
	phoneEnc, phoneHash, err := s.protectPhone(phone)
	if err != nil {
		return Account{}, "", err
	}
	accountID := s.ids.Generate()
	// Bootstrap 管理员只建账号和教师/学校管理员角色,不臆造默认院系或虚假档案。
	account, err := tx.CreateAccount(ctx, CreateAccountInput{
		ID:           accountID,
		TenantID:     tenantID,
		PhoneEnc:     phoneEnc,
		PhoneHash:    phoneHash,
		Name:         strings.TrimSpace(name),
		BaseIdentity: BaseIdentityTeacher,
		Status:       AccountStatusPending,
		Roles: []RoleCreateInput{
			{ID: s.ids.Generate(), Role: contracts.RoleNumTeacher},
			{ID: s.ids.Generate(), Role: contracts.RoleNumSchoolAdmin},
		},
	})
	if err != nil {
		return Account{}, "", err
	}
	code, err := s.createActivationCodeInTx(ctx, tx, tenantID, accountID, 0)
	return account, code, err
}

// ensurePlatformLayerEnabled 校验 SaaS 平台层是否启用,私有化部署所有平台能力都必须关闭。
func (s *Service) ensurePlatformLayerEnabled() error {
	if !s.deploy.IsSaaS() {
		return apperr.ErrIdentityPlatformLayerDisabled
	}
	return nil
}

// validateBootstrapConfig 校验私有化初始化配置,避免 seed 脚本写入不可登录的半成品租户。
func validateBootstrapConfig(cfg config.BootstrapConfig) error {
	if err := ValidateTenantCode(cfg.SchoolTenantCode); err != nil {
		return err
	}
	if strings.TrimSpace(cfg.SchoolName) == "" || !validSchoolType(cfg.SchoolType) || strings.TrimSpace(cfg.AdminName) == "" {
		return apperr.ErrIdentityBootstrapInvalid
	}
	if err := ValidatePhone(cfg.AdminPhone); err != nil {
		return err
	}
	if err := ValidatePassword(cfg.AdminPassword); err != nil {
		return err
	}
	return nil
}

// ptrTime 返回时间指针,用于明确表达账号已激活时间字段。
func ptrTime(t time.Time) *time.Time {
	return &t
}

// activationExpireAt 计算激活码配置化过期时间。
func (s *Service) activationExpireAt() time.Time {
	return timex.Now().Add(time.Duration(s.cfg.ActivationCodeTTLHours) * time.Hour)
}

// auditPlatformOperation 写入平台管理员敏感操作审计,平台级记录 tenant_id 为空。
func (s *Service) auditPlatformOperation(ctx context.Context, actorID int64, action, targetType string, targetID int64, detail map[string]any) error {
	entry, err := audit.BuildEntry(ctx, 0, actorID, contracts.RoleNumPlatformAdmin, action, targetType, targetID, detail)
	if err != nil {
		return err
	}
	return s.auditWriter.Write(ctx, entry)
}
