// server experiment 文件负责装配 M7 实验模块、事件订阅和实例生命周期后台回收任务。
package main

import (
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/experiment"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/background"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/upload"
	"chaimir/pkg/snowflake"

	"github.com/gin-gonic/gin"
)

// ExperimentModuleDeps 是 M7 装配所需的依赖。
type ExperimentModuleDeps struct {
	Router       gin.IRouter
	Database     *db.DB
	IDs          snowflake.Generator
	Config       config.ExperimentConfig
	Content      contracts.ContentReadService
	ContentClone contracts.ContentCloneService
	Sandbox      contracts.SandboxService
	SandboxClone contracts.SandboxCompositionCloneService
	Judge        contracts.JudgeService
	Sim          contracts.SimService
	Audit        audit.Writer
	EventBus     eventbus.Bus
	Storage      *storage.Storage
	Upload       config.UploadConfig
	FileService  storage.Service
	Auth         *auth.Manager
	Roles        contracts.IdentityService
}

// RegisterExperimentModule 构造实验 store/service,注册路由、事件和生命周期后台任务。
func RegisterExperimentModule(tasks *background.Runner, deps ExperimentModuleDeps) (*experiment.Service, error) {
	store := experiment.NewStore(deps.Database)
	svc, err := experiment.NewService(experiment.ServiceDeps{
		CleanupTimeout:   time.Duration(deps.Upload.CleanupTimeoutSeconds) * time.Second,
		Store:            store,
		IDs:              deps.IDs,
		Config:           deps.Config,
		Audit:            deps.Audit,
		Roles:            deps.Roles,
		Content:          deps.Content,
		ContentClone:     deps.ContentClone,
		Sandbox:          deps.Sandbox,
		SandboxClone:     deps.SandboxClone,
		Judge:            deps.Judge,
		Sim:              deps.Sim,
		Bus:              deps.EventBus,
		Storage:          deps.Storage,
		FileService:      deps.FileService,
		ReportMaxBytes:   deps.Upload.ExperimentReportMaxBytes,
		CloneMaxBytes:    deps.Upload.ContentAttachmentMaxBytes,
		ReportScanPolicy: upload.ScanPolicy{Required: deps.Upload.VirusScanRequired},
	})
	if err != nil {
		return nil, err
	}
	if err := experiment.RegisterRoutes(deps.Router, svc, deps.Auth, deps.Roles); err != nil {
		return nil, err
	}
	if _, err := experiment.SubscribeEvents(deps.EventBus, svc); err != nil {
		return nil, err
	}
	if err := tasks.Start(
		background.Task{
			Name:     "experiment.event_outbox",
			Interval: time.Duration(deps.Config.EventOutboxPollMs) * time.Millisecond,
			Run:      svc.RunExperimentEventOutboxOnce,
		},
		background.Task{
			Name:     "experiment.recycle",
			Interval: time.Duration(deps.Config.RecyclePollIntervalSeconds) * time.Second,
			Run:      svc.RunRecycleOnce,
		},
	); err != nil {
		return nil, err
	}
	return svc, nil
}
