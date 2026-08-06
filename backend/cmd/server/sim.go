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
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/snowflake"

	"github.com/gin-gonic/gin"
)

// SimModuleDeps 汇总组合根装配 M4 需要的基础设施和跨模块契约。
type SimModuleDeps struct {
	Router          gin.IRouter
	Database        *db.DB
	IDs             snowflake.Generator
	Upload          config.UploadConfig
	MinIO           config.MinIOConfig
	AuthConfig      config.AuthConfig
	SimBackend      config.SimBackendConfig
	Storage         *storage.Storage
	Audit           audit.Writer
	WSHub           *ws.Hub
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
	fileService, err := storage.NewServiceFromConfig(deps.AuthConfig, deps.MinIO, deps.Upload)
	if err != nil {
		return nil, err
	}
	store := sim.NewStore(deps.Database)
	svc, err := sim.NewService(sim.ServiceDeps{
		Store:           store,
		IDs:             deps.IDs,
		Upload:          deps.Upload,
		Storage:         deps.Storage,
		FileService:     fileService,
		Audit:           deps.Audit,
		Identity:        deps.Roles,
		WSHub:           deps.WSHub,
		BackendAdapters: deps.BackendAdapters,
		SimBackend:      deps.SimBackend,
	})
	if err != nil {
		return nil, err
	}
	if err := sim.RegisterRoutes(deps.Router, svc, deps.Auth, deps.Roles); err != nil {
		return nil, err
	}
	previewTask, err := simPackagePreviewTask(deps.SimBackend, svc)
	if err != nil {
		return nil, err
	}
	go background.Run(ctx, previewTask)
	return svc, nil
}

// simPackagePreviewTask 把上架前隔离预览接入统一后台任务运行器。
// 它是 determinism_check / worker_preview 两项审核门禁的唯一生产者,缺了扩展包永远无法上架。
func simPackagePreviewTask(cfg config.SimBackendConfig, svc *sim.Service) (background.Task, error) {
	if svc == nil {
		return background.Task{}, fmt.Errorf("sim package preview task 缺少 service")
	}
	if cfg.PreviewPollIntervalSeconds <= 0 {
		return background.Task{}, fmt.Errorf("SIM_PREVIEW_POLL_INTERVAL_SECONDS 必须大于 0")
	}
	return background.Task{Name: "sim.package_preview", Interval: time.Duration(cfg.PreviewPollIntervalSeconds) * time.Second, Run: svc.RunPackagePreviewOnce}, nil
}
