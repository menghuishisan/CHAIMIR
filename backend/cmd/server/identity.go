// server identity 文件负责装配 M1 身份与租户模块及全平台审计写入器。
package main

import (
	"time"

	"chaimir/internal/modules/identity"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/background"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/redis"
	"chaimir/internal/platform/storage"
	"chaimir/pkg/snowflake"

	"github.com/gin-gonic/gin"
)

// IdentityModuleDeps 汇总组合根装配 M1 需要的基础设施。
type IdentityModuleDeps struct {
	Router      gin.IRouter
	Database    *db.DB
	Auth        *auth.Manager
	Redis       *redis.Client
	IDs         snowflake.Generator
	Config      config.Config
	Storage     *storage.Storage
	FileService storage.Service
	EventBus    eventbus.Bus
}

// RegisterIdentityModule 构造身份模块 store/service 并注册 HTTP 路由。
func RegisterIdentityModule(deps IdentityModuleDeps) (*identity.Service, error) {
	store := identity.NewStore(deps.Database)
	smsSender, err := identity.NewSMSSender(deps.Config.SMS)
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
		Scanner:        deps.FileService.Scanner,
		Objects:        deps.Storage,
		FileService:    deps.FileService,
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

// StartIdentityBackgroundTasks 在 M2 注册租户开通事件订阅后启动 outbox。
func StartIdentityBackgroundTasks(tasks *background.Runner, cfg config.IdentityConfig, svc *identity.Service) error {
	return tasks.Start(background.Task{
		Name:     "identity.tenant_provision_outbox",
		Interval: time.Duration(cfg.TenantProvisionOutboxPollMs) * time.Millisecond,
		Run:      svc.RunTenantProvisionOutboxOnce,
	})
}
