// server judge 文件负责装配 M3 评测引擎模块及其后台队列任务。
package main

import (
	"context"
	"fmt"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/judge"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/background"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/snowflake"

	"github.com/gin-gonic/gin"
)

// JudgeModuleDeps 是 M3 装配所需的依赖。
type JudgeModuleDeps struct {
	Router   gin.IRouter
	Database *db.DB
	IDs      snowflake.Generator
	Config   config.JudgeConfig
	AuthCfg  config.AuthConfig
	Storage  *storage.Storage
	Sandbox  contracts.SandboxService
	Content  contracts.ContentJudgeReadService
	Audit    audit.Writer
	EventBus eventbus.Bus
	WSHub    *ws.Hub
	Auth     *auth.Manager
	Roles    contracts.IdentityService
}

// RegisterJudgeModule 构造评测 store/service,注册路由并启动队列 worker。
func RegisterJudgeModule(ctx context.Context, deps JudgeModuleDeps) (*judge.Service, error) {
	if ctx == nil {
		return nil, fmt.Errorf("judge module 缺少后台任务 context")
	}
	if deps.Router == nil {
		return nil, fmt.Errorf("judge module 缺少 HTTP router")
	}
	if deps.Database == nil {
		return nil, fmt.Errorf("judge module 缺少 database")
	}
	store := judge.NewStore(deps.Database)
	svc, err := judge.NewService(judge.ServiceDeps{
		Store:    store,
		IDs:      deps.IDs,
		Config:   deps.Config,
		Auth:     deps.AuthCfg,
		Storage:  deps.Storage,
		Sandbox:  deps.Sandbox,
		Content:  deps.Content,
		Audit:    deps.Audit,
		Identity: deps.Roles,
		EventBus: deps.EventBus,
		WSHub:    deps.WSHub,
	})
	if err != nil {
		return nil, err
	}
	if err := judge.RegisterRoutes(deps.Router, svc, deps.Auth, deps.Roles); err != nil {
		return nil, err
	}
	if err := judge.RegisterEventSubscriptions(deps.EventBus, svc); err != nil {
		return nil, err
	}
	if err := background.Start(ctx,
		background.Task{
			Name:     "judge.worker",
			Interval: time.Duration(deps.Config.QueuePollIntervalMs) * time.Millisecond,
			Run:      svc.RunWorkerOnce,
		},
		background.Task{
			Name: "judge.catalog", Interval: time.Duration(deps.Config.CatalogReconcileIntervalMs) * time.Millisecond,
			Run: svc.ReconcileJudgerCatalogOnce,
		},
	); err != nil {
		return nil, err
	}
	return svc, nil
}
