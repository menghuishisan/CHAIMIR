// server grade 文件负责装配 M11 成绩中心模块。
package main

import (
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

// GradeModuleDeps 是 M11 装配所需的依赖。
type GradeModuleDeps struct {
	CleanupTimeout time.Duration
	Router         gin.IRouter
	Database       *db.DB
	IDs            snowflake.Generator
	Audit          audit.Writer
	Teaching       contracts.TeachingReadService
	EventBus       eventbus.Bus
	Redis          *redis.Client
	Storage        *storage.Storage
	FileService    storage.Service
	Config         config.GradeConfig
	Auth           *auth.Manager
	Roles          contracts.IdentityService
}

// RegisterGradeModule 构造成绩中心 store/service,注册路由、事件订阅和 outbox worker。
func RegisterGradeModule(tasks *background.Runner, deps GradeModuleDeps) (*grade.Service, error) {
	store := grade.NewStore(deps.Database)
	svc, err := grade.NewService(grade.ServiceDeps{
		CleanupTimeout: deps.CleanupTimeout,
		Store:          store,
		IDs:            deps.IDs,
		Audit:          deps.Audit,
		Roles:          deps.Roles,
		Teaching:       deps.Teaching,
		Bus:            deps.EventBus,
		Cache:          deps.Redis,
		Storage:        deps.Storage,
		FileService:    deps.FileService,
		Config:         deps.Config,
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
	if err := tasks.Start(
		background.Task{
			Name:     "grade.outbox",
			Interval: time.Duration(deps.Config.LockOutboxPollMs) * time.Millisecond,
			Run:      svc.RunOutboxesOnce,
		},
	); err != nil {
		return nil, err
	}
	return svc, nil
}
