// server notify 文件负责装配 M10 通知与实时推送模块。
package main

import (
	"fmt"

	"chaimir/internal/modules/notify"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/redis"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/snowflake"

	"github.com/gin-gonic/gin"
)

// NotifyModuleDeps 汇总组合根装配 M10 需要的基础设施。
type NotifyModuleDeps struct {
	Router   gin.IRouter
	Database *db.DB
	IDs      snowflake.Generator
	Redis    *redis.Client
	Hub      *ws.Hub
	Config   config.NotifyConfig
	EventBus eventbus.Bus
	Auth     *auth.Manager
	Roles    auth.RoleChecker
}

// RegisterNotifyModule 构造通知 store/service,注册路由和事件订阅。
func RegisterNotifyModule(deps NotifyModuleDeps) (*notify.Service, error) {
	if deps.Router == nil {
		return nil, fmt.Errorf("notify module 缺少 HTTP router")
	}
	if deps.Database == nil {
		return nil, fmt.Errorf("notify module 缺少 database")
	}
	store := notify.NewStore(deps.Database)
	svc, err := notify.NewService(notify.ServiceDeps{Store: store, IDs: deps.IDs, Redis: deps.Redis, Hub: deps.Hub, Config: deps.Config})
	if err != nil {
		return nil, err
	}
	if err := notify.RegisterRoutes(deps.Router, svc, deps.Auth, deps.Roles); err != nil {
		return nil, err
	}
	if _, err := notify.SubscribeEvents(deps.EventBus, svc); err != nil {
		return nil, err
	}
	return svc, nil
}
