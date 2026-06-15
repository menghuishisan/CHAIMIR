// server sim 文件负责装配 M4 仿真可视化引擎模块。
package main

import (
	"fmt"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/sim"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/auth"
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
	Storage         *storage.Storage
	Audit           audit.Writer
	WSHub           *ws.Hub
	Auth            *auth.Manager
	Roles           contracts.IdentityService
	BackendAdapters sim.BackendRegistry
}

// RegisterSimModule 构造仿真 store/service 并注册 HTTP/WS 路由。
func RegisterSimModule(deps SimModuleDeps) (*sim.Service, error) {
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
		WSHub:           deps.WSHub,
		BackendAdapters: deps.BackendAdapters,
	})
	if err != nil {
		return nil, err
	}
	if err := sim.RegisterRoutes(deps.Router, svc, deps.Auth, deps.Roles); err != nil {
		return nil, err
	}
	return svc, nil
}
