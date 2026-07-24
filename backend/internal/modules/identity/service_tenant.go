// identity service_tenant 文件实现学校管理员维护租户配置和统一认证配置的业务编排。
package identity

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/pkg/apperr"
)

// GetTenantConfig 读取当前租户配置,仅学校管理员可访问。
func (s *Service) GetTenantConfig(ctx context.Context) (TenantDTO, error) {
	id, err := requireTenantRole(ctx, s, contracts.RoleSchoolAdmin)
	if err != nil {
		return TenantDTO{}, err
	}
	var row Tenant
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		item, err := tx.GetTenantByID(ctx, id.TenantID)
		if err != nil {
			return err
		}
		row = item
		return nil
	}); err != nil {
		return TenantDTO{}, apperr.ErrInternal.WithCause(err)
	}
	return ToTenantDTO(row), nil
}

// UpdateTenantConfigByAdmin 更新当前租户展示和认证模式配置。
func (s *Service) UpdateTenantConfigByAdmin(ctx context.Context, req TenantConfigRequest) (TenantDTO, error) {
	id, err := requireTenantRole(ctx, s, contracts.RoleSchoolAdmin)
	if err != nil {
		return TenantDTO{}, err
	}
	if err := validateTenantConfigRequest(req); err != nil {
		return TenantDTO{}, err
	}
	var row Tenant
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		item, err := tx.UpdateTenantConfig(ctx, UpdateTenantConfigInput{
			TenantID:             id.TenantID,
			LogoURL:              req.LogoURL,
			DisplayName:          req.DisplayName,
			AuthMode:             req.AuthMode,
			EnableActivationCode: req.EnableActivationCode,
		})
		if err != nil {
			return err
		}
		row = item
		return nil
	}); err != nil {
		return TenantDTO{}, apperr.ErrInternal.WithCause(err)
	}
	if err := s.auditTenantOperation(ctx, id, "tenant.config.update", "identity.tenant", id.TenantID, map[string]any{"auth_mode": req.AuthMode, "enable_activation_code": req.EnableActivationCode}); err != nil {
		return TenantDTO{}, err
	}
	return ToTenantDTO(row), nil
}

// validateTenantConfigRequest 校验租户配置输入,避免非法认证模式或不可序列化开关写入配置。
func validateTenantConfigRequest(req TenantConfigRequest) error {
	return ValidateAuthMode(req.AuthMode)
}

// ListSSOConfigsByAdmin 读取当前租户的 CAS/LDAP 配置,敏感字段返回前必须脱敏。
func (s *Service) ListSSOConfigsByAdmin(ctx context.Context) ([]SSOConfigDTO, error) {
	id, err := requireTenantRole(ctx, s, contracts.RoleSchoolAdmin)
	if err != nil {
		return nil, err
	}
	var rows []SSOConfig
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		items, err := tx.ListSSOConfigs(ctx, id.TenantID)
		if err != nil {
			return err
		}
		rows = items
		return nil
	}); err != nil {
		return nil, apperr.ErrInternal.WithCause(err)
	}
	out := make([]SSOConfigDTO, 0, len(rows))
	for _, row := range rows {
		dto, err := ToSSOConfigDTO(row)
		if err != nil {
			return nil, err
		}
		out = append(out, dto)
	}
	return out, nil
}
