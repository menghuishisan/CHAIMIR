// server teaching 文件负责装配 M6 教学模块、事件订阅和判题 outbox 后台任务。
package main

import (
	"context"
	"fmt"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/teaching"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/background"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/timex"
	"chaimir/internal/platform/transfer"
	"chaimir/internal/platform/upload"
	"chaimir/pkg/snowflake"

	"github.com/gin-gonic/gin"
)

// TeachingModuleDeps 是 M6 装配所需的依赖。
type TeachingModuleDeps struct {
	Router          gin.IRouter
	Database        *db.DB
	IDs             snowflake.Generator
	Config          config.TeachingConfig
	Upload          config.UploadConfig
	FileService     storage.Service
	Content         contracts.ContentReadService
	ContentClone    contracts.ContentCloneService
	ExperimentClone contracts.ExperimentCloneService
	Sim             contracts.SimService
	Judge           contracts.JudgeService
	Transfer        *transfer.Service
	Storage         *storage.Storage
	Audit           audit.Writer
	EventBus        eventbus.Bus
	Auth            *auth.Manager
	Roles           contracts.IdentityService
}

// RegisterTeachingModule 构造教学 store/service,注册路由、事件和 outbox worker。
func RegisterTeachingModule(ctx context.Context, deps TeachingModuleDeps) (*teaching.Service, error) {
	if ctx == nil {
		return nil, fmt.Errorf("teaching module 缺少后台任务 context")
	}
	if deps.Router == nil {
		return nil, fmt.Errorf("teaching module 缺少 HTTP router")
	}
	if deps.Database == nil {
		return nil, fmt.Errorf("teaching module 缺少 database")
	}
	if deps.Transfer == nil {
		return nil, fmt.Errorf("teaching module 缺少 transfer service")
	}
	if deps.Storage == nil {
		return nil, fmt.Errorf("teaching module 缺少统一对象存储")
	}
	store := teaching.NewStore(deps.Database)
	svc, err := teaching.NewService(teaching.ServiceDeps{
		CleanupTimeout:         time.Duration(deps.Upload.CleanupTimeoutSeconds) * time.Second,
		Store:                  store,
		IDs:                    deps.IDs,
		Audit:                  deps.Audit,
		Identity:               deps.Roles,
		Content:                deps.Content,
		ContentClone:           deps.ContentClone,
		ExperimentClone:        deps.ExperimentClone,
		Sim:                    deps.Sim,
		Judge:                  deps.Judge,
		Bus:                    deps.EventBus,
		Transfers:              deps.Transfer,
		Storage:                deps.Storage,
		FileService:            deps.FileService,
		Config:                 deps.Config,
		CourseMaterialMaxBytes: deps.Upload.CourseMaterialMaxBytes,
		CourseCoverMaxBytes:    deps.Upload.CourseCoverMaxBytes,
		MaterialScanPolicy:     upload.ScanPolicy{Required: deps.Upload.VirusScanRequired},
	})
	if err != nil {
		return nil, err
	}
	if err := teaching.RegisterRoutes(deps.Router, svc, deps.Auth, deps.Roles); err != nil {
		return nil, err
	}
	if _, err := teaching.SubscribeEvents(deps.EventBus, svc); err != nil {
		return nil, err
	}
	if err := background.Start(ctx,
		background.Task{
			Name:     "teaching.judge_outbox",
			Interval: time.Duration(deps.Config.JudgeOutboxPollIntervalMs) * time.Millisecond,
			Run: func(ctx context.Context) error {
				return svc.RunJudgeOutboxOnce(ctx, 0)
			},
		},
		background.Task{
			Name:     "teaching.grade_event_outbox",
			Interval: time.Duration(deps.Config.GradeEventOutboxPollMs) * time.Millisecond,
			Run:      svc.RunTeachingGradeEventOutboxOnce,
		},
		background.Task{
			Name:     "teaching.course_status",
			Interval: time.Duration(deps.Config.CourseStatusPollIntervalSeconds) * time.Second,
			Run: func(ctx context.Context) error {
				return svc.AdvanceCourseStatusesOnce(ctx, timex.Now())
			},
		},
		background.Task{
			Name:     "teaching.grade_export",
			Interval: time.Duration(deps.Config.GradeExportWorkerPollMs) * time.Millisecond,
			Run:      svc.RunGradeExportOnce,
		},
	); err != nil {
		return nil, err
	}
	return svc, nil
}
