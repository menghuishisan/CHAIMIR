// server sandbox 文件负责装配 M2 沙箱模块及其后台回收任务。
package main

import (
	"context"
	"fmt"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/sandbox"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/background"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/eventbus"
	platformk8s "chaimir/internal/platform/k8s"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/snowflake"

	"github.com/gin-gonic/gin"
)

// SandboxModuleDeps 汇总组合根装配 M2 需要的基础设施和跨模块契约。
type SandboxModuleDeps struct {
	Router       gin.IRouter
	Database     *db.DB
	IDs          snowflake.Generator
	Config       config.SandboxConfig
	Storage      *storage.Storage
	K8s          *platformk8s.Client
	Audit        audit.Writer
	EventBus     eventbus.Bus
	WSHub        *ws.Hub
	Auth         *auth.Manager
	Roles        contracts.IdentityService
	Capabilities map[string]sandbox.ChainCapability
}

// RegisterSandboxModule 构造沙箱 store/service/orchestrator,注册路由与后台回收任务。
func RegisterSandboxModule(ctx context.Context, deps SandboxModuleDeps) (*sandbox.Service, error) {
	if ctx == nil {
		return nil, fmt.Errorf("sandbox module 缺少后台任务 context")
	}
	if deps.Router == nil {
		return nil, fmt.Errorf("sandbox module 缺少 HTTP router")
	}
	if deps.Database == nil {
		return nil, fmt.Errorf("sandbox module 缺少 database")
	}
	if deps.K8s == nil {
		return nil, fmt.Errorf("sandbox module 缺少 K8s client")
	}
	store := sandbox.NewStore(deps.Database)
	orchestrator := sandbox.NewK8sOrchestrator(deps.K8s, deps.Config)
	svc, err := sandbox.NewService(sandbox.ServiceDeps{
		Store:        store,
		IDs:          deps.IDs,
		Config:       deps.Config,
		Storage:      deps.Storage,
		Orchestrator: orchestrator,
		Audit:        deps.Audit,
		EventBus:     deps.EventBus,
		WSHub:        deps.WSHub,
		Capabilities: deps.Capabilities,
	})
	if err != nil {
		return nil, err
	}
	if err := sandbox.RegisterRoutes(deps.Router, svc, deps.Auth, deps.Roles); err != nil {
		return nil, err
	}
	if err := sandbox.RegisterEventSubscriptions(deps.EventBus, svc); err != nil {
		return nil, err
	}
	task, err := sandboxRecycleTask(deps.Config, svc)
	if err != nil {
		return nil, err
	}
	go background.Run(ctx, task)
	outboxTask, err := sandboxRecycleOutboxTask(deps.Config, svc)
	if err != nil {
		return nil, err
	}
	go background.Run(ctx, outboxTask)
	return svc, nil
}

// sandboxRecycleTask 把 M2 回收调度接入统一后台任务运行器。
func sandboxRecycleTask(cfg config.SandboxConfig, svc *sandbox.Service) (background.Task, error) {
	if svc == nil {
		return background.Task{}, fmt.Errorf("sandbox recycle task 缺少 service")
	}
	if cfg.RecyclePollIntervalSeconds <= 0 {
		return background.Task{}, fmt.Errorf("SANDBOX_RECYCLE_POLL_INTERVAL_SECONDS 必须大于 0")
	}
	return background.Task{
		Name:     "sandbox.recycle",
		Interval: time.Duration(cfg.RecyclePollIntervalSeconds) * time.Second,
		Run:      svc.RunRecycleOnce,
	}, nil
}

// sandboxRecycleOutboxTask 把 M2 回收事件 outbox 接入统一后台任务运行器。
func sandboxRecycleOutboxTask(cfg config.SandboxConfig, svc *sandbox.Service) (background.Task, error) {
	if svc == nil {
		return background.Task{}, fmt.Errorf("sandbox recycle outbox task 缺少 service")
	}
	if cfg.RecycleOutboxPollMs <= 0 {
		return background.Task{}, fmt.Errorf("SANDBOX_RECYCLE_OUTBOX_POLL_INTERVAL_MS 必须大于 0")
	}
	return background.Task{
		Name:     "sandbox.recycle_outbox",
		Interval: time.Duration(cfg.RecycleOutboxPollMs) * time.Millisecond,
		Run:      svc.RunSandboxRecycleOutboxOnce,
	}, nil
}
