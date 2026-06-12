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
	"chaimir/pkg/snowflake"

	"github.com/gin-gonic/gin"
)

// TeachingModuleDeps 汇总组合根装配 M6 需要的基础设施和跨模块契约。
type TeachingModuleDeps struct {
	Router   gin.IRouter
	Database *db.DB
	IDs      snowflake.Generator
	Config   config.TeachingConfig
	Content  contracts.ContentReadService
	Judge    contracts.JudgeService
	Notify   contracts.NotifyService
	Audit    audit.Writer
	EventBus eventbus.Bus
	Auth     *auth.Manager
	Roles    auth.RoleChecker
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
	store := teaching.NewStore(deps.Database)
	svc, err := teaching.NewService(teaching.ServiceDeps{
		Store:   store,
		IDs:     deps.IDs,
		Audit:   deps.Audit,
		Content: deps.Content,
		Judge:   deps.Judge,
		Notify:  deps.Notify,
		Bus:     deps.EventBus,
		Config:  deps.Config,
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
	task, err := teachingJudgeOutboxTask(deps.Config, svc)
	if err != nil {
		return nil, err
	}
	go background.Run(ctx, task)
	statusTask, err := teachingCourseStatusTask(deps.Config, svc)
	if err != nil {
		return nil, err
	}
	go background.Run(ctx, statusTask)
	return svc, nil
}

// teachingJudgeOutboxTask 把 M6 判题 outbox 派发接入统一后台任务运行器。
func teachingJudgeOutboxTask(cfg config.TeachingConfig, svc *teaching.Service) (background.Task, error) {
	if svc == nil {
		return background.Task{}, fmt.Errorf("teaching outbox worker 缺少 service")
	}
	if cfg.JudgeOutboxPollIntervalMs <= 0 {
		return background.Task{}, fmt.Errorf("TEACHING_JUDGE_OUTBOX_POLL_INTERVAL_MS 必须大于 0")
	}
	return background.Task{
		Name:     "teaching.judge_outbox",
		Interval: time.Duration(cfg.JudgeOutboxPollIntervalMs) * time.Millisecond,
		Run: func(ctx context.Context) error {
			return svc.RunJudgeOutboxOnce(ctx, 0)
		},
	}, nil
}

// teachingCourseStatusTask 把课程生命周期自动推进接入统一后台任务运行器。
func teachingCourseStatusTask(cfg config.TeachingConfig, svc *teaching.Service) (background.Task, error) {
	if svc == nil {
		return background.Task{}, fmt.Errorf("teaching course status worker 缺少 service")
	}
	if cfg.CourseStatusPollIntervalSeconds <= 0 {
		return background.Task{}, fmt.Errorf("TEACHING_COURSE_STATUS_POLL_INTERVAL_SECONDS 必须大于 0")
	}
	return background.Task{
		Name:     "teaching.course_status",
		Interval: time.Duration(cfg.CourseStatusPollIntervalSeconds) * time.Second,
		Run: func(ctx context.Context) error {
			return svc.AdvanceCourseStatusesOnce(ctx, time.Now().UTC())
		},
	}, nil
}
