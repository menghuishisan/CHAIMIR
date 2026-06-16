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

// JudgeModuleDeps 汇总组合根装配 M3 需要的基础设施和跨模块契约。
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
	task, err := judgeWorkerTask(deps.Config, svc)
	if err != nil {
		return nil, err
	}
	go background.Run(ctx, task)
	return svc, nil
}

// judgeWorkerTask 把 M3 队列消费接入统一后台任务运行器。
func judgeWorkerTask(cfg config.JudgeConfig, svc *judge.Service) (background.Task, error) {
	if svc == nil {
		return background.Task{}, fmt.Errorf("judge worker task 缺少 service")
	}
	if cfg.QueuePollIntervalMs <= 0 {
		return background.Task{}, fmt.Errorf("JUDGE_QUEUE_POLL_INTERVAL_MS 必须大于 0")
	}
	return background.Task{
		Name:     "judge.worker",
		Interval: time.Duration(cfg.QueuePollIntervalMs) * time.Millisecond,
		Run:      svc.RunWorkerOnce,
	}, nil
}
