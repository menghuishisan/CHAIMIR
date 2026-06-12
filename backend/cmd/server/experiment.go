// server experiment 文件负责装配 M7 实验模块、事件订阅和实例生命周期后台回收任务。
package main

import (
	"context"
	"fmt"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/experiment"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/background"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/storage"
	"chaimir/pkg/snowflake"

	"github.com/gin-gonic/gin"
)

// ExperimentModuleDeps 汇总组合根装配 M7 需要的基础设施和跨模块契约。
type ExperimentModuleDeps struct {
	Router   gin.IRouter
	Database *db.DB
	IDs      snowflake.Generator
	Config   config.ExperimentConfig
	Content  contracts.ContentReadService
	Sandbox  contracts.SandboxService
	Judge    contracts.JudgeService
	Sim      contracts.SimService
	Audit    audit.Writer
	EventBus eventbus.Bus
	Storage  *storage.Storage
	Auth     *auth.Manager
	Roles    auth.RoleChecker
}

// RegisterExperimentModule 构造实验 store/service,注册路由、事件和生命周期后台任务。
func RegisterExperimentModule(ctx context.Context, deps ExperimentModuleDeps) (*experiment.Service, error) {
	if ctx == nil {
		return nil, fmt.Errorf("experiment module 缺少后台任务 context")
	}
	if deps.Router == nil {
		return nil, fmt.Errorf("experiment module 缺少 HTTP router")
	}
	if deps.Database == nil {
		return nil, fmt.Errorf("experiment module 缺少 database")
	}
	store := experiment.NewStore(deps.Database)
	svc, err := experiment.NewService(experiment.ServiceDeps{
		Store:   store,
		IDs:     deps.IDs,
		Config:  deps.Config,
		Audit:   deps.Audit,
		Roles:   deps.Roles,
		Content: deps.Content,
		Sandbox: deps.Sandbox,
		Judge:   deps.Judge,
		Sim:     deps.Sim,
		Bus:     deps.EventBus,
		Storage: deps.Storage,
	})
	if err != nil {
		return nil, err
	}
	if err := experiment.RegisterRoutes(deps.Router, svc, deps.Auth, deps.Roles); err != nil {
		return nil, err
	}
	if _, err := experiment.SubscribeEvents(deps.EventBus, svc); err != nil {
		return nil, err
	}
	task, err := experimentRecycleTask(deps.Config, svc)
	if err != nil {
		return nil, err
	}
	go background.Run(ctx, task)
	return svc, nil
}

// experimentRecycleTask 把 M7 实例生命周期回收接入统一后台任务运行器。
func experimentRecycleTask(cfg config.ExperimentConfig, svc *experiment.Service) (background.Task, error) {
	if svc == nil {
		return background.Task{}, fmt.Errorf("experiment recycle task 缺少 service")
	}
	if cfg.RecyclePollIntervalSeconds <= 0 {
		return background.Task{}, fmt.Errorf("EXPERIMENT_RECYCLE_POLL_INTERVAL_SECONDS 必须大于 0")
	}
	return background.Task{
		Name:     "experiment.recycle",
		Interval: time.Duration(cfg.RecyclePollIntervalSeconds) * time.Second,
		Run:      svc.RunRecycleOnce,
	}, nil
}
