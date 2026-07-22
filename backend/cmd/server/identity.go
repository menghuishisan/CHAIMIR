// server identity 文件负责装配 M1 身份与租户模块及全平台审计写入器。
package main

import (
	"context"
	"fmt"
	"time"

	"chaimir/internal/modules/identity"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/background"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/redis"
	"chaimir/internal/platform/upload"
	"chaimir/pkg/snowflake"

	"github.com/gin-gonic/gin"
)

// IdentityModuleDeps 汇总组合根装配 M1 需要的基础设施。
type IdentityModuleDeps struct {
	Router   gin.IRouter
	Database *db.DB
	Auth     *auth.Manager
	Redis    *redis.Client
	IDs      snowflake.Generator
	Config   config.Config
	EventBus eventbus.Bus
}

// RegisterIdentityModule 构造身份模块 store/service 并注册 HTTP 路由。
func RegisterIdentityModule(deps IdentityModuleDeps) (*identity.Service, error) {
	if deps.Router == nil {
		return nil, fmt.Errorf("identity module 缺少 HTTP router")
	}
	if deps.Database == nil {
		return nil, fmt.Errorf("identity module 缺少 database")
	}
	if deps.Auth == nil {
		return nil, fmt.Errorf("identity module 缺少 auth manager")
	}
	if deps.EventBus == nil {
		return nil, fmt.Errorf("identity module 缺少事件总线")
	}
	store := identity.NewStore(deps.Database)
	smsSender, err := identity.NewSMSSender(deps.Config.SMS)
	if err != nil {
		return nil, err
	}
	scanner, err := upload.NewScannerFromConfig(deps.Config.Upload)
	if err != nil {
		return nil, err
	}
	svc, err := identity.NewService(identity.ServiceDeps{
		Store:          store,
		Auth:           deps.Auth,
		Redis:          deps.Redis,
		IDs:            deps.IDs,
		AuthConfig:     deps.Config.Auth,
		IdentityConfig: deps.Config.Identity,
		UploadConfig:   deps.Config.Upload,
		Scanner:        scanner,
		DeployConfig:   deps.Config.Deploy,
		SMSSender:      smsSender,
		EventBus:       deps.EventBus,
	})
	if err != nil {
		return nil, err
	}
	if err := identity.RegisterRoutes(deps.Router, svc, deps.Auth); err != nil {
		return nil, err
	}
	return svc, nil
}

// StartIdentityBackgroundTasks 在下游事件订阅完成后启动 M1 outbox，避免 Core NATS 无订阅时丢失初始化事件。
func StartIdentityBackgroundTasks(ctx context.Context, cfg config.IdentityConfig, svc *identity.Service) error {
	if ctx == nil || svc == nil {
		return fmt.Errorf("identity background task 缺少 context 或 service")
	}
	if cfg.TenantProvisionOutboxPollMs <= 0 {
		return fmt.Errorf("IDENTITY_TENANT_PROVISION_OUTBOX_POLL_INTERVAL_MS 必须大于 0")
	}
	go background.Run(ctx, background.Task{
		Name:     "identity.tenant_provision_outbox",
		Interval: time.Duration(cfg.TenantProvisionOutboxPollMs) * time.Millisecond,
		Run:      svc.RunTenantProvisionOutboxOnce,
	})
	return nil
}
