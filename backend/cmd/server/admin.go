// server admin 文件负责装配 M9 管理后台模块。
package main

import (
	"context"
	"fmt"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/admin"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/background"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/transfer"
	"chaimir/pkg/snowflake"

	"github.com/gin-gonic/gin"
)

// AdminModuleDeps 是 M9 装配所需的依赖。
type AdminModuleDeps struct {
	CleanupTimeout time.Duration
	Metrics        []contracts.AlertMetricReader
	Recipients     contracts.AlertRecipientReader
	Router         gin.IRouter
	Database       *db.DB
	IDs            snowflake.Generator
	Audit          audit.Writer
	Identity       contracts.IdentityTenantReadService
	Stats          contracts.IdentityStatsService
	AuditRead      contracts.IdentityAuditReadService
	Teaching       contracts.TeachingReadService
	Sandbox        contracts.SandboxReadService
	Experiment     contracts.ExperimentReadService
	Contest        contracts.ContestReadService
	EventBus       eventbus.Bus
	Monitoring     config.MonitoringConfig
	Config         config.AdminConfig
	FileService    storage.Service
	Transfer       *transfer.Service
	Storage        *storage.Storage
	Auth           *auth.Manager
	Roles          contracts.IdentityService
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
	store := admin.NewStore(deps.Database)
	svc, err := admin.NewService(admin.ServiceDeps{
		CleanupTimeout: deps.CleanupTimeout,
		Store:          store,
		Metrics:        deps.Metrics,
		Recipients:     deps.Recipients,
		IDs:            deps.IDs,
		Audit:          deps.Audit,
		Roles:          deps.Roles,
		Identity:       deps.Identity,
		Accounts:       deps.Roles,
		Stats:          deps.Stats,
		AuditRead:      deps.AuditRead,
		Teaching:       deps.Teaching,
		Sandbox:        deps.Sandbox,
		Experiment:     deps.Experiment,
		Contest:        deps.Contest,
		Bus:            deps.EventBus,
		Monitoring:     deps.Monitoring,
		Config:         deps.Config,
		Transfers:      deps.Transfer,
		Storage:        deps.Storage,
		FileService:    deps.FileService,
	})
	if err != nil {
		return nil, err
	}
	if err := admin.RegisterRoutes(deps.Router, svc, deps.Auth, deps.Roles); err != nil {
		return nil, err
	}
	if err := background.Start(ctx,
		background.Task{Name: "admin.alerts", Interval: time.Duration(deps.Config.AlertPollSeconds) * time.Second, Run: svc.RunAlertsOnce},
		background.Task{
			Name:     "admin.statistics_snapshot",
			Interval: time.Duration(deps.Config.StatisticsSnapshotIntervalSeconds) * time.Second,
			Run:      svc.RunStatisticsSnapshotOnce,
		},
		background.Task{
			Name:     "admin.audit_export",
			Interval: time.Duration(deps.Config.AuditExportWorkerPollMs) * time.Millisecond,
			Run:      svc.RunAuditExportOnce,
		},
	); err != nil {
		return nil, err
	}
	return svc, nil
}
