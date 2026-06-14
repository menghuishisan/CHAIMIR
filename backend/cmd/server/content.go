// server content 文件负责装配 M5 题库与模板中心模块。
package main

import (
	"fmt"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/content"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/upload"
	"chaimir/pkg/snowflake"

	"github.com/gin-gonic/gin"
)

// ContentModuleDeps 汇总组合根装配 M5 需要的基础设施和跨模块契约。
type ContentModuleDeps struct {
	Router   gin.IRouter
	Database *db.DB
	IDs      snowflake.Generator
	Upload   config.UploadConfig
	MinIO    config.MinIOConfig
	AuthCfg  config.AuthConfig
	Storage  *storage.Storage
	Audit    audit.Writer
	Auth     *auth.Manager
	Roles    contracts.IdentityService
}

// RegisterContentModule 构造题库 store/service 并注册 HTTP 路由。
func RegisterContentModule(deps ContentModuleDeps) (*content.Service, error) {
	if deps.Router == nil {
		return nil, fmt.Errorf("content module 缺少 HTTP router")
	}
	if deps.Database == nil {
		return nil, fmt.Errorf("content module 缺少 database")
	}
	if deps.Storage == nil {
		return nil, fmt.Errorf("content module 缺少统一对象存储")
	}
	scanner, err := upload.NewScannerFromConfig(deps.Upload)
	if err != nil {
		return nil, err
	}
	store := content.NewStore(deps.Database)
	svc, err := content.NewService(content.ServiceDeps{
		Store:   store,
		IDs:     deps.IDs,
		Audit:   deps.Audit,
		Storage: deps.Storage,
		FileService: storage.Service{
			Scanner:          scanner,
			SigningKey:       deps.AuthCfg.HMACKey,
			DownloadGrantTTL: time.Duration(deps.MinIO.DownloadGrantTTLSeconds) * time.Second,
		},
		ContentAttachmentMaxBytes: deps.Upload.ContentAttachmentMaxBytes,
	})
	if err != nil {
		return nil, err
	}
	if err := content.RegisterRoutes(deps.Router, svc, deps.Auth, deps.Roles); err != nil {
		return nil, err
	}
	return svc, nil
}
