// server admin 文件负责装配 M9 管理后台模块。
package main

import (
	"fmt"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/admin"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/db"
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
	Auth       *auth.Manager
	Roles      auth.RoleChecker
}

// RegisterAdminModule 构造管理后台 store/service 并注册 HTTP 路由。
func RegisterAdminModule(deps AdminModuleDeps) (*admin.Service, error) {
	if deps.Router == nil {
		return nil, fmt.Errorf("admin module 缺少 HTTP router")
	}
	if deps.Database == nil {
		return nil, fmt.Errorf("admin module 缺少 database")
	}
	store := admin.NewStore(deps.Database)
	svc, err := admin.NewService(admin.ServiceDeps{Store: store, IDs: deps.IDs, Audit: deps.Audit, Roles: deps.Roles, Identity: deps.Identity, Stats: deps.Stats, AuditRead: deps.AuditRead, Teaching: deps.Teaching, Sandbox: deps.Sandbox, Experiment: deps.Experiment, Contest: deps.Contest, Notify: deps.Notify, Monitoring: deps.Monitoring})
	if err != nil {
		return nil, err
	}
	if err := admin.RegisterRoutes(deps.Router, svc, deps.Auth, deps.Roles); err != nil {
		return nil, err
	}
	return svc, nil
}
