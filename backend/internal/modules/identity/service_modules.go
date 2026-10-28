// identity service_modules 统一解析租户开关并提供会话及跨模块只读授权。
package identity

import (
	"context"
	"encoding/json"
	"slices"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
)

// enabledTenantModules 按规范顺序返回启用模块；非法存量配置也拒绝，不能默认放行。
func enabledTenantModules(flags []byte) ([]string, error) {
	var config map[string]any
	if len(flags) > 0 {
		if err := json.Unmarshal(flags, &config); err != nil {
			return nil, apperr.ErrIdentityTenantConfigInvalid.WithCause(err)
		}
	}
	if err := ValidateTenantFeatureFlags(config); err != nil {
		return nil, err
	}
	known := contracts.TenantModules()
	raw, configured := config["modules"]
	if !configured {
		return known, nil
	}
	enabled := make([]string, 0, len(known))
	for _, module := range known {
		for _, item := range raw.([]any) {
			if item == module {
				enabled = append(enabled, module)
				break
			}
		}
	}
	return enabled, nil
}

// requireTenantModules 使用同一份租户快照判定开关，不重复读取会话已取出的租户。
func requireTenantModules(current Tenant, modules ...string) error {
	if len(modules) == 0 {
		return nil
	}
	enabled, err := enabledTenantModules(current.FeatureFlags)
	if err != nil {
		return err
	}
	return requireEnabledModules(enabled, modules...)
}

// requireEnabledModules 统一判定所需集合，供会话快照和只读契约共用。
func requireEnabledModules(enabled []string, modules ...string) error {
	for _, module := range modules {
		if !slices.Contains(enabled, module) {
			return apperr.ErrIdentityModuleDisabled
		}
	}
	return nil
}

// RequireTenantModules 供引擎按已验证资源归属和匿名分享所属租户检查当前授权。
// 收尾和清理不调用本契约，避免关闭模块后遗留运行资源。
func (s *Service) RequireTenantModules(ctx context.Context, tenantID int64, modules ...string) error {
	enabled, err := s.EnabledTenantModules(ctx, tenantID)
	if err != nil {
		return err
	}
	return requireEnabledModules(enabled, modules...)
}

// EnabledTenantModules 只暴露模块集合，不向调用方返回可变租户配置或敏感字段。
func (s *Service) EnabledTenantModules(ctx context.Context, tenantID int64) ([]string, error) {
	if tenantID <= 0 {
		return nil, apperr.ErrForbidden
	}
	var current Tenant
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		row, err := tx.GetTenantByID(ctx, tenantID)
		if err != nil {
			return err
		}
		current = row
		return nil
	}); err != nil {
		return nil, apperr.ErrInternal.WithCause(err)
	}
	if current.ID != tenantID {
		return nil, apperr.ErrForbidden
	}
	if err := EnsureTenantCanLogin(current, timex.Now()); err != nil {
		return nil, err
	}
	return enabledTenantModules(current.FeatureFlags)
}
