// server contest 文件负责装配 M8 竞赛模块、事件订阅和对抗赛撮合后台任务。
package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/contest"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/background"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/eventbus"
	"chaimir/pkg/crypto"
	"chaimir/pkg/snowflake"

	"github.com/gin-gonic/gin"
)

// ContestModuleDeps 汇总组合根装配 M8 需要的基础设施和跨模块契约。
type ContestModuleDeps struct {
	Router        gin.IRouter
	Database      *db.DB
	IDs           snowflake.Generator
	Config        config.ContestConfig
	AuthConfig    config.AuthConfig
	Content       contracts.ContentReadService
	ContentImport contracts.ContentImportService
	Sandbox       contracts.SandboxService
	Judge         contracts.JudgeService
	Fingerprint   contracts.FingerprintService
	Notify        contracts.NotifyService
	Audit         audit.Writer
	EventBus      eventbus.Bus
	Auth          *auth.Manager
	Roles         contracts.IdentityService
}

// RegisterContestModule 构造竞赛 store/service,注册路由、事件和撮合后台任务。
func RegisterContestModule(ctx context.Context, deps ContestModuleDeps) (*contest.Service, error) {
	if ctx == nil {
		return nil, fmt.Errorf("contest module 缺少后台任务 context")
	}
	if deps.Router == nil {
		return nil, fmt.Errorf("contest module 缺少 HTTP router")
	}
	if deps.Database == nil {
		return nil, fmt.Errorf("contest module 缺少 database")
	}
	store := contest.NewStore(deps.Database)
	key, err := base64.StdEncoding.DecodeString(deps.AuthConfig.EncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("contest module 解析 APP_ENCRYPTION_KEY 失败: %w", err)
	}
	cipher, err := crypto.NewCipher(key)
	if err != nil {
		return nil, err
	}
	svc, err := contest.NewService(contest.ServiceDeps{
		Store:         store,
		IDs:           deps.IDs,
		Config:        deps.Config,
		Audit:         deps.Audit,
		Roles:         deps.Roles,
		Content:       deps.Content,
		ContentImport: deps.ContentImport,
		Sandbox:       deps.Sandbox,
		Judge:         deps.Judge,
		Fingerprint:   deps.Fingerprint,
		Notify:        deps.Notify,
		Bus:           deps.EventBus,
		Cipher:        cipher,
	})
	if err != nil {
		return nil, err
	}
	if err := contest.RegisterRoutes(deps.Router, svc, deps.Auth, deps.Roles); err != nil {
		return nil, err
	}
	if _, err := contest.SubscribeEvents(deps.EventBus, svc); err != nil {
		return nil, err
	}
	task, err := contestMatchmakerTask(deps.Config, svc)
	if err != nil {
		return nil, err
	}
	go background.Run(ctx, task)
	archiveTask, err := contestAutoArchiveTask(deps.Config, svc)
	if err != nil {
		return nil, err
	}
	go background.Run(ctx, archiveTask)
	return svc, nil
}

// contestMatchmakerTask 把 M8 对抗赛撮合接入统一后台任务运行器。
func contestMatchmakerTask(cfg config.ContestConfig, svc *contest.Service) (background.Task, error) {
	if svc == nil {
		return background.Task{}, fmt.Errorf("contest matchmaker task 缺少 service")
	}
	if cfg.MatchmakerPollIntervalSeconds <= 0 {
		return background.Task{}, fmt.Errorf("CONTEST_MATCHMAKER_POLL_INTERVAL_SECONDS 必须大于 0")
	}
	return background.Task{Name: "contest.matchmaker", Interval: time.Duration(cfg.MatchmakerPollIntervalSeconds) * time.Second, Run: svc.RunMatchmakerOnce}, nil
}

// contestAutoArchiveTask 把 M8 到期竞赛自动收尾接入统一后台任务运行器。
func contestAutoArchiveTask(cfg config.ContestConfig, svc *contest.Service) (background.Task, error) {
	if svc == nil {
		return background.Task{}, fmt.Errorf("contest auto archive task 缺少 service")
	}
	if cfg.AutoArchivePollIntervalSeconds <= 0 {
		return background.Task{}, fmt.Errorf("CONTEST_AUTO_ARCHIVE_POLL_INTERVAL_SECONDS 必须大于 0")
	}
	return background.Task{Name: "contest.auto_archive", Interval: time.Duration(cfg.AutoArchivePollIntervalSeconds) * time.Second, Run: svc.RunAutoArchiveOnce}, nil
}
