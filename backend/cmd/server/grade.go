// server grade 文件负责装配 M11 成绩中心模块。
package main

import (
	"context"
	"fmt"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/grade"
	"chaimir/internal/platform/audit"
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

// GradeModuleDeps 汇总组合根装配 M11 需要的基础设施和跨模块契约。
type GradeModuleDeps struct {
	Router     gin.IRouter
	Database   *db.DB
	IDs        snowflake.Generator
	Audit      audit.Writer
	Teaching   contracts.TeachingReadService
	Notify     contracts.NotifyService
	EventBus   eventbus.Bus
	Redis      *redis.Client
	Storage    *storage.Storage
	Upload     config.UploadConfig
	MinIO      config.MinIOConfig
	AuthConfig config.AuthConfig
	Config     config.GradeConfig
	Auth       *auth.Manager
	Roles      contracts.IdentityService
}

// RegisterGradeModule 构造成绩中心 store/service,注册路由、事件订阅和 outbox worker。
func RegisterGradeModule(ctx context.Context, deps GradeModuleDeps) (*grade.Service, error) {
	if ctx == nil {
		return nil, fmt.Errorf("grade module 缺少后台任务 context")
	}
	if deps.Router == nil {
		return nil, fmt.Errorf("grade module 缺少 HTTP router")
	}
	if deps.Database == nil {
		return nil, fmt.Errorf("grade module 缺少 database")
	}
	fileService, err := storage.NewServiceFromConfig(deps.AuthConfig, deps.MinIO, deps.Upload)
	if err != nil {
		return nil, err
	}
	store := grade.NewStore(deps.Database)
	svc, err := grade.NewService(grade.ServiceDeps{
		Store:       store,
		IDs:         deps.IDs,
		Audit:       deps.Audit,
		Roles:       deps.Roles,
		Teaching:    deps.Teaching,
		Notify:      deps.Notify,
		Bus:         deps.EventBus,
		Cache:       deps.Redis,
		Storage:     deps.Storage,
		FileService: fileService,
		Config:      deps.Config,
	})
	if err != nil {
		return nil, err
	}
	if err := grade.RegisterRoutes(deps.Router, svc, deps.Auth, deps.Roles); err != nil {
		return nil, err
	}
	if _, err := grade.SubscribeEvents(deps.EventBus, svc); err != nil {
		return nil, err
	}
	task, err := gradeLockOutboxTask(deps.Config, svc)
	if err != nil {
		return nil, err
	}
	go background.Run(ctx, task)
	return svc, nil
}

// gradeLockOutboxTask 把 M11 锁定事件 outbox 投递接入统一后台任务运行器。
func gradeLockOutboxTask(cfg config.GradeConfig, svc *grade.Service) (background.Task, error) {
	if svc == nil {
		return background.Task{}, fmt.Errorf("grade lock outbox worker 缺少 service")
	}
	if cfg.LockOutboxPollMs <= 0 {
		return background.Task{}, fmt.Errorf("GRADE_LOCK_OUTBOX_POLL_INTERVAL_MS 必须大于 0")
	}
	return background.Task{
		Name:     "grade.lock_outbox",
		Interval: time.Duration(cfg.LockOutboxPollMs) * time.Millisecond,
		Run:      svc.RunLockOutboxOnce,
	}, nil
}
