// server admin 文件负责装配 M9 管理后台模块。
package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/admin"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/background"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/transfer"
	"chaimir/pkg/crypto"
	"chaimir/pkg/snowflake"

	"github.com/gin-gonic/gin"
)

// AdminModuleDeps 汇总组合根装配 M9 需要的基础设施和跨模块契约。
type AdminModuleDeps struct {
	Router     gin.IRouter
	Database   *db.DB
	IDs        snowflake.Generator
	Audit      audit.Writer
	Identity   contracts.IdentityTenantReadService
	Stats      contracts.IdentityStatsService
	AuditRead  contracts.IdentityAuditReadService
	Teaching   contracts.TeachingReadService
	Sandbox    contracts.SandboxService
	Experiment contracts.ExperimentReadService
	Contest    contracts.ContestReadService
	Notify     contracts.NotifyService
	Monitoring config.MonitoringConfig
	Config     config.AdminConfig
	Upload     config.UploadConfig
	MinIO      config.MinIOConfig
	AuthConfig config.AuthConfig
	Transfer   *transfer.Service
	Storage    *storage.Storage
	Auth       *auth.Manager
	Roles      contracts.IdentityService
}

// RegisterAdminModule 构造管理后台 store/service 并注册 HTTP 路由。
func RegisterAdminModule(ctx context.Context, deps AdminModuleDeps) (*admin.Service, error) {
	if ctx == nil {
		return nil, fmt.Errorf("admin module 缺少后台任务 context")
	}
	if deps.Router == nil {
		return nil, fmt.Errorf("admin module 缺少 HTTP router")
	}
	if deps.Database == nil {
		return nil, fmt.Errorf("admin module 缺少 database")
	}
	if deps.Transfer == nil {
		return nil, fmt.Errorf("admin module 缺少 transfer service")
	}
	if deps.Storage == nil {
		return nil, fmt.Errorf("admin module 缺少统一对象存储")
	}
	key, err := base64.StdEncoding.DecodeString(deps.AuthConfig.EncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("admin module 解析 APP_ENCRYPTION_KEY 失败: %w", err)
	}
	cipher, err := crypto.NewCipher(key)
	if err != nil {
		return nil, err
	}
	fileService, err := storage.NewServiceFromConfig(deps.AuthConfig, deps.MinIO, deps.Upload)
	if err != nil {
		return nil, err
	}
	store := admin.NewStore(deps.Database)
	svc, err := admin.NewService(admin.ServiceDeps{
		Store:       store,
		IDs:         deps.IDs,
		Audit:       deps.Audit,
		Roles:       deps.Roles,
		Identity:    deps.Identity,
		Stats:       deps.Stats,
		AuditRead:   deps.AuditRead,
		Teaching:    deps.Teaching,
		Sandbox:     deps.Sandbox,
		Experiment:  deps.Experiment,
		Contest:     deps.Contest,
		Notify:      deps.Notify,
		Monitoring:  deps.Monitoring,
		Cipher:      cipher,
		Transfers:   deps.Transfer,
		Storage:     deps.Storage,
		FileService: fileService,
	})
	if err != nil {
		return nil, err
	}
	if err := admin.RegisterRoutes(deps.Router, svc, deps.Auth, deps.Roles); err != nil {
		return nil, err
	}
	task, err := adminStatisticsSnapshotTask(deps.Config, svc)
	if err != nil {
		return nil, err
	}
	go background.Run(ctx, task)
	return svc, nil
}

// adminStatisticsSnapshotTask 把 M9 周期统计快照接入统一后台任务运行器。
func adminStatisticsSnapshotTask(cfg config.AdminConfig, svc *admin.Service) (background.Task, error) {
	if svc == nil {
		return background.Task{}, fmt.Errorf("admin statistics snapshot task 缺少 service")
	}
	if cfg.StatisticsSnapshotIntervalSeconds <= 0 {
		return background.Task{}, fmt.Errorf("ADMIN_STATISTICS_SNAPSHOT_INTERVAL_SECONDS 必须大于 0")
	}
	return background.Task{Name: "admin.statistics_snapshot", Interval: time.Duration(cfg.StatisticsSnapshotIntervalSeconds) * time.Second, Run: svc.RunStatisticsSnapshotOnce}, nil
}
