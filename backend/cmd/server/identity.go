// server identity 文件负责装配 M1 身份与租户模块及全平台审计写入器。
package main

import (
	"fmt"

	"chaimir/internal/modules/identity"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/redis"
	"chaimir/pkg/snowflake"

	"github.com/gin-gonic/gin"
)

// IdentityModuleDeps 汇总组合根装配 M1 需要的基础设施。
type IdentityModuleDeps struct {
	Router   gin.IRouter
	Database *db.DB
	Auth     *auth.Manager
	Redis    *redis.Client
	IDs      snowflake.Generator
	Config   config.Config
}

// RegisterIdentityModule 构造身份模块 store/service 并注册 HTTP 路由。
func RegisterIdentityModule(deps IdentityModuleDeps) (*identity.Service, error) {
	if deps.Router == nil {
		return nil, fmt.Errorf("identity module 缺少 HTTP router")
	}
	if deps.Database == nil {
		return nil, fmt.Errorf("identity module 缺少 database")
	}
	if deps.Auth == nil {
		return nil, fmt.Errorf("identity module 缺少 auth manager")
	}
	store := identity.NewStore(deps.Database)
	svc, err := identity.NewService(identity.ServiceDeps{
		Store:          store,
		Auth:           deps.Auth,
		Redis:          deps.Redis,
		IDs:            deps.IDs,
		AuthConfig:     deps.Config.Auth,
		IdentityConfig: deps.Config.Identity,
		UploadConfig:   deps.Config.Upload,
		DeployConfig:   deps.Config.Deploy,
		SMSSender:      identity.NewSMSSender(deps.Config.SMS),
	})
	if err != nil {
		return nil, err
	}
	if err := identity.RegisterRoutes(deps.Router, svc, deps.Auth); err != nil {
		return nil, err
	}
	return svc, nil
}
