// experiment service 文件定义 M7 服务依赖注入和通用业务编排,不接收数据库连接。
package experiment

import (
	"context"
	"fmt"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
	"chaimir/pkg/snowflake"
)

// objectStorage 描述 M7 报告引用校验所需的对象存储桶信息。
type objectStorage interface {
	BucketReport() string
}

// Service 承载 experiment 模块业务编排,依赖 repo 接口和跨模块 contracts。
type Service struct {
	store   Store
	ids     snowflake.Generator
	cfg     config.ExperimentConfig
	audit   audit.Writer
	roles   auth.RoleChecker
	content contracts.ContentReadService
	sandbox contracts.SandboxService
	judge   contracts.JudgeService
	sim     contracts.SimService
	bus     eventbus.Bus
	storage objectStorage
}

// ServiceDeps 是 experiment service 的装配依赖集合。
type ServiceDeps struct {
	Store   Store
	IDs     snowflake.Generator
	Config  config.ExperimentConfig
	Audit   audit.Writer
	Roles   auth.RoleChecker
	Content contracts.ContentReadService
	Sandbox contracts.SandboxService
	Judge   contracts.JudgeService
	Sim     contracts.SimService
	Bus     eventbus.Bus
	Storage *storage.Storage
}

// NewService 构造 experiment 服务,不接收数据库连接,由装配层传入 Store。
func NewService(deps ServiceDeps) (*Service, error) {
	if deps.Store == nil {
		return nil, fmt.Errorf("experiment service 缺少 store")
	}
	if deps.IDs == nil {
		return nil, fmt.Errorf("experiment service 缺少 ID 生成器")
	}
	if deps.Audit == nil {
		return nil, fmt.Errorf("experiment service 缺少审计写入器")
	}
	if deps.Roles == nil {
		return nil, fmt.Errorf("experiment service 缺少角色只读契约")
	}
	if deps.Content == nil {
		return nil, fmt.Errorf("experiment service 缺少 content 契约")
	}
	if deps.Bus == nil {
		return nil, fmt.Errorf("experiment service 缺少事件总线")
	}
	if deps.Storage == nil {
		return nil, fmt.Errorf("experiment service 缺少统一对象存储")
	}
	if deps.Config.RecyclePollIntervalSeconds <= 0 || deps.Config.RecycleBatchSize <= 0 || deps.Config.InstanceIdleTimeoutSeconds <= 0 || deps.Config.PausedTimeoutSeconds <= 0 {
		return nil, fmt.Errorf("experiment service 配置不完整")
	}
	return &Service{store: deps.Store, ids: deps.IDs, cfg: deps.Config, audit: deps.Audit, roles: deps.Roles, content: deps.Content, sandbox: deps.Sandbox, judge: deps.Judge, sim: deps.Sim, bus: deps.Bus, storage: deps.Storage}, nil
}

// currentIdentity 读取租户账号身份。
func currentIdentity(ctx context.Context) (tenant.Identity, error) {
	id, ok := tenant.FromContext(ctx)
	if !ok || id.TenantID <= 0 || id.AccountID <= 0 {
		return tenant.Identity{}, apperr.ErrUnauthorized
	}
	return id, nil
}

// currentServiceTenant 读取内部服务租户边界。
func currentServiceTenant(ctx context.Context) (int64, error) {
	id, ok := tenant.FromContext(ctx)
	if !ok || id.TenantID <= 0 || !id.IsSystem {
		return 0, apperr.ErrServiceUnauthorized
	}
	return id.TenantID, nil
}

// isSchoolAdmin 判断当前账号是否具备学校管理员权限。
func (s *Service) isSchoolAdmin(ctx context.Context, accountID int64) bool {
	ok, err := s.roles.HasRole(ctx, accountID, contracts.RoleSchoolAdmin)
	return err == nil && ok
}

// mapExperimentLoadError 将数据库未命中归一为实验不存在。
func mapExperimentLoadError(err error) error {
	if err == nil {
		return nil
	}
	if isNoRows(err) {
		return apperr.ErrExperimentNotFound
	}
	return err
}
