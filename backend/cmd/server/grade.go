// server grade 文件负责装配 M11 成绩中心模块。
package main

import (
	"fmt"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/grade"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/upload"
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
	Storage    *storage.Storage
	Upload     config.UploadConfig
	MinIO      config.MinIOConfig
	AuthConfig config.AuthConfig
	Config     config.GradeConfig
	Auth       *auth.Manager
	Roles      contracts.IdentityService
}

// RegisterGradeModule 构造成绩中心 store/service,注册路由和事件订阅。
func RegisterGradeModule(deps GradeModuleDeps) (*grade.Service, error) {
	if deps.Router == nil {
		return nil, fmt.Errorf("grade module 缺少 HTTP router")
	}
	if deps.Database == nil {
		return nil, fmt.Errorf("grade module 缺少 database")
	}
	scanner, err := upload.NewScannerFromConfig(deps.Upload)
	if err != nil {
		return nil, err
	}
	store := grade.NewStore(deps.Database)
	svc, err := grade.NewService(grade.ServiceDeps{
		Store:    store,
		IDs:      deps.IDs,
		Audit:    deps.Audit,
		Roles:    deps.Roles,
		Teaching: deps.Teaching,
		Notify:   deps.Notify,
		Bus:      deps.EventBus,
		Storage:  deps.Storage,
		FileService: storage.Service{
			Scanner:          scanner,
			SigningKey:       deps.AuthConfig.HMACKey,
			DownloadGrantTTL: time.Duration(deps.MinIO.DownloadGrantTTLSeconds) * time.Second,
		},
		Config: deps.Config,
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
	return svc, nil
}
