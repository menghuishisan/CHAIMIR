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
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/crypto"
	"chaimir/pkg/snowflake"

	"github.com/gin-gonic/gin"
)

// ContestModuleDeps 是 M8 装配所需的依赖。
type ContestModuleDeps struct {
	CleanupTimeout time.Duration
	Router         gin.IRouter
	Database       *db.DB
	IDs            snowflake.Generator
	Config         config.ContestConfig
	AuthConfig     config.AuthConfig
	FileService    storage.Service
	Storage        *storage.Storage
	Content        contracts.ContentReadService
	ContentImport  contracts.ContentImportService
	Sandbox        contracts.SandboxService
	Judge          contracts.JudgeService
	Fingerprint    contracts.FingerprintService
	Audit          audit.Writer
	EventBus       eventbus.Bus
	WSHub          *ws.Hub
	Auth           *auth.Manager
	Roles          contracts.IdentityService
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
	if deps.Storage == nil {
		return nil, fmt.Errorf("contest module 缺少共享对象存储")
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
		CleanupTimeout: deps.CleanupTimeout,
		Store:          store,
		IDs:            deps.IDs,
		Config:         deps.Config,
		Audit:          deps.Audit,
		Roles:          deps.Roles,
		Content:        deps.Content,
		ContentImport:  deps.ContentImport,
		Sandbox:        deps.Sandbox,
		Judge:          deps.Judge,
		Fingerprint:    deps.Fingerprint,
		FileService:    deps.FileService,
		ReplayStore:    deps.Storage,
		ReplayBucket:   deps.Storage.BucketReport(),
		Bus:            deps.EventBus,
		WSHub:          deps.WSHub,
		Cipher:         cipher,
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
	// 赛事状态推进与自动归档共用轮询间隔。
	if err := background.Start(ctx,
		background.Task{
			Name:     "contest.leaderboard",
			Interval: time.Duration(deps.Config.MatchmakerPollIntervalSeconds) * time.Second,
			Run:      svc.RunLeaderboardPushOnce,
		},
		background.Task{
			Name:     "contest.matchmaker",
			Interval: time.Duration(deps.Config.MatchmakerPollIntervalSeconds) * time.Second,
			Run:      svc.RunMatchmakerOnce,
		},
		background.Task{
			Name:     "contest.auto_archive",
			Interval: time.Duration(deps.Config.AutoArchivePollIntervalSeconds) * time.Second,
			Run:      svc.RunAutoArchiveOnce,
		},
		background.Task{
			Name:     "contest.lifecycle",
			Interval: time.Duration(deps.Config.AutoArchivePollIntervalSeconds) * time.Second,
			Run: func(ctx context.Context) error {
				return svc.AdvanceContestStatusesOnce(ctx, time.Now())
			},
		},
	); err != nil {
		return nil, err
	}
	return svc, nil
}
