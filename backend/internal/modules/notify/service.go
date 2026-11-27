// 组装通知服务依赖并校验身份。
package notify

import (
	"context"
	"fmt"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"
	"chaimir/pkg/snowflake"
)

// Service 承载 M10 通知业务编排。
type Service struct {
	store       Store
	ids         snowflake.Generator
	redis       Cache
	hub         *ws.Hub
	roles       RoleReader
	contests    contracts.ContestReadService
	experiments contracts.ExperimentProgressReadService
	audit       audit.Writer
	cfg         config.NotifyConfig
}

// Cache 定义 M10 使用的 Redis 缓存和限频能力。
type Cache interface {
	GetInt64(ctx context.Context, key string) (int64, bool, error)
	SetInt64(ctx context.Context, key string, value int64, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	IncrWithTTL(ctx context.Context, key string, ttl time.Duration) (int64, error)
}

// RoleReader 定义 M10 使用的账号角色只读契约。
type RoleReader interface {
	HasRole(ctx context.Context, accountID int64, role string) (bool, error)
}

// ServiceDeps 是 M10 服务装配依赖。
type ServiceDeps struct {
	Store       Store
	IDs         snowflake.Generator
	Redis       Cache
	Hub         *ws.Hub
	Roles       RoleReader
	Contests    contracts.ContestReadService
	Experiments contracts.ExperimentProgressReadService
	Audit       audit.Writer
	Config      config.NotifyConfig
}

// NewService 构造 M10 服务。
func NewService(deps ServiceDeps) (*Service, error) {
	if deps.Store == nil || deps.IDs == nil || deps.Redis == nil || deps.Hub == nil || deps.Roles == nil || deps.Contests == nil || deps.Experiments == nil || deps.Audit == nil {
		return nil, fmt.Errorf("notify service 依赖不完整")
	}
	if deps.Config.UnreadTTLHours <= 0 || deps.Config.SendRateWindowSeconds <= 0 || deps.Config.SendRateMax <= 0 {
		return nil, fmt.Errorf("notify service 配置不完整")
	}
	return &Service{store: deps.Store, ids: deps.IDs, redis: deps.Redis, hub: deps.Hub, roles: deps.Roles, contests: deps.Contests, experiments: deps.Experiments, audit: deps.Audit, cfg: deps.Config}, nil
}

// currentIdentity 读取通知模块要求的租户用户身份。
func currentIdentity(ctx context.Context) (tenant.Identity, error) {
	id, ok := tenant.FromContext(ctx)
	if !ok || id.TenantID <= 0 || id.AccountID <= 0 || id.IsPlatform {
		return tenant.Identity{}, apperr.ErrUnauthorized
	}
	return id, nil
}

// currentIdentityAllowPlatform 读取公告发布需要的租户或平台身份。
func currentIdentityAllowPlatform(ctx context.Context) (tenant.Identity, error) {
	id, ok := tenant.FromContext(ctx)
	if !ok || id.AccountID <= 0 {
		return tenant.Identity{}, apperr.ErrUnauthorized
	}
	if !id.IsPlatform && id.TenantID <= 0 {
		return tenant.Identity{}, apperr.ErrUnauthorized
	}
	return id, nil
}
