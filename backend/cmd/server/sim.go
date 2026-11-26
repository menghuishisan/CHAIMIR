// server sim 文件负责装配 M4 仿真可视化引擎模块。
package main

import (
	"context"
	"fmt"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/sim"
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

// SimModuleDeps 是 M4 装配所需的依赖。
type SimModuleDeps struct {
	Router          gin.IRouter
	Database        *db.DB
	IDs             snowflake.Generator
	Upload          config.UploadConfig
	FileService     storage.Service
	SimBackend      config.SimBackendConfig
	Storage         *storage.Storage
	Audit           audit.Writer
	WSHub           *ws.Hub
	EventBus        eventbus.Bus
	Auth            *auth.Manager
	Roles           contracts.IdentityService
	BackendAdapters sim.BackendRegistry
}

// RegisterSimModule 构造仿真 store/service、注册 HTTP/WS 路由并启动隔离预览任务。
func RegisterSimModule(ctx context.Context, deps SimModuleDeps) (*sim.Service, error) {
	if deps.Router == nil {
		return nil, fmt.Errorf("sim module 缺少 HTTP router")
	}
	if deps.Database == nil {
		return nil, fmt.Errorf("sim module 缺少 database")
	}
	if deps.Storage == nil {
		return nil, fmt.Errorf("sim module 缺少统一对象存储")
	}
	store := sim.NewStore(deps.Database)
	fanout, ok := deps.EventBus.(eventbus.FanoutBus)
	if !ok {
		return nil, fmt.Errorf("sim module 缺少事件广播能力")
	}
	svc, err := sim.NewService(sim.ServiceDeps{
		Store:           store,
		IDs:             deps.IDs,
		Upload:          deps.Upload,
		Storage:         deps.Storage,
		FileService:     deps.FileService,
		Audit:           deps.Audit,
		Identity:        deps.Roles,
		WSHub:           deps.WSHub,
		EventBus:        fanout,
		BackendAdapters: deps.BackendAdapters,
		SimBackend:      deps.SimBackend,
	})
	if err != nil {
		return nil, err
	}
	if err := sim.RegisterRoutes(deps.Router, svc, deps.Auth, deps.Roles); err != nil {
		return nil, err
	}
	if err := sim.RegisterEventSubscriptions(svc); err != nil {
		return nil, err
	}
	// 隔离预览产生扩展包上架所需的确定性检查和 Worker 预览。
	if err := background.Start(ctx,
		background.Task{
			Name:     "sim.package_preview",
			Interval: time.Duration(deps.SimBackend.PreviewPollIntervalSeconds) * time.Second,
			Run:      svc.RunPackagePreviewOnce,
		},
	); err != nil {
		return nil, err
	}
	return svc, nil
}
