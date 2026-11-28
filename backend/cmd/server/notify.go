// server notify 文件负责装配 M10 通知与实时推送模块。
package main

import (
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/notify"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/background"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/redis"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/snowflake"

	"github.com/gin-gonic/gin"
)

// NotifyModuleDeps 是 M10 装配所需的依赖。
type NotifyModuleDeps struct {
	Router      gin.IRouter
	Database    *db.DB
	IDs         snowflake.Generator
	Redis       *redis.Client
	Hub         *ws.Hub
	Config      config.NotifyConfig
	EventBus    eventbus.Bus
	Auth        *auth.Manager
	Roles       contracts.IdentityService
	Contests    contracts.ContestReadService
	Experiments contracts.ExperimentProgressReadService
	Audit       audit.Writer
}

// RegisterNotifyModule 构造通知 store/service,注册路由和事件订阅。
func RegisterNotifyModule(tasks *background.Runner, deps NotifyModuleDeps) (*notify.Service, error) {
	store := notify.NewStore(deps.Database)
	svc, err := notify.NewService(notify.ServiceDeps{Store: store, IDs: deps.IDs, Redis: deps.Redis, Hub: deps.Hub, Roles: deps.Roles, Contests: deps.Contests, Experiments: deps.Experiments, Audit: deps.Audit, Config: deps.Config})
	if err != nil {
		return nil, err
	}
	if err := notify.RegisterRoutes(deps.Router, svc, deps.Auth, deps.Roles); err != nil {
		return nil, err
	}
	if _, err := notify.SubscribeEvents(deps.EventBus, svc); err != nil {
		return nil, err
	}
	if err := tasks.Start(
		background.Task{
			Name:     "notify.cleanup",
			Interval: time.Duration(deps.Config.CleanupIntervalSeconds) * time.Second,
			Run:      svc.RunCleanupOnce,
		},
	); err != nil {
		return nil, err
	}
	return svc, nil
}
