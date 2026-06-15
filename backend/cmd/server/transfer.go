// server transfer 文件负责装配统一导入导出中心基础层路由和服务。
package main

import (
	"fmt"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/transfer"
	"chaimir/pkg/snowflake"

	"github.com/gin-gonic/gin"
)

// TransferDeps 汇总组合根装配统一导入导出中心需要的基础设施。
type TransferDeps struct {
	Router     gin.IRouter
	Database   *db.DB
	IDs        snowflake.Generator
	Config     config.TransferConfig
	AuthConfig config.AuthConfig
	Auth       *auth.Manager
	Roles      contracts.IdentityService
}

// RegisterTransfer 构造统一导入导出中心 store/service 并注册 HTTP 路由。
func RegisterTransfer(deps TransferDeps) (*transfer.Service, error) {
	if deps.Router == nil {
		return nil, fmt.Errorf("transfer 缺少 HTTP router")
	}
	if deps.Database == nil {
		return nil, fmt.Errorf("transfer 缺少 database")
	}
	store := transfer.NewStore(deps.Database)
	svc, err := transfer.NewService(transfer.ServiceDeps{
		Store: store,
		IDs:   deps.IDs,
		Manager: transfer.Manager{
			Config: transfer.Config{
				MaxAttempts:      deps.Config.TaskMaxAttempts,
				RetryDelay:       time.Duration(deps.Config.TaskRetryDelayMs) * time.Millisecond,
				DownloadGrantTTL: time.Duration(deps.Config.TaskDownloadTTLSeconds) * time.Second,
			},
			StorageSigningKey: deps.AuthConfig.HMACKey,
		},
	})
	if err != nil {
		return nil, err
	}
	if err := transfer.RegisterRoutes(deps.Router, svc, deps.Auth, deps.Roles); err != nil {
		return nil, err
	}
	return svc, nil
}
