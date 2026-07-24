// experiment service 文件定义 M7 服务依赖注入和通用业务编排,不接收数据库连接。
package experiment

import (
	"context"
	"fmt"
	"io"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/upload"
	"chaimir/pkg/apperr"
	"chaimir/pkg/snowflake"
)

// objectStorage 描述 M7 写入和清理实验报告所需的统一对象存储能力。
type objectStorage interface {
	Put(ctx context.Context, bucket, key string, r io.Reader, size int64, contentType string) error
	Delete(ctx context.Context, bucket, key string) error
	BucketReport() string
}

// fileService 描述 M7 复用统一上传规划和下载授权所需能力。
type fileService interface {
	PlanUpload(ctx context.Context, req storage.PlanUploadRequest) (storage.UploadPlan, error)
	IssueDownloadGrant(req storage.IssueDownloadGrantRequest) (string, storage.DownloadGrant, error)
}

// Service 承载 experiment 模块业务编排,依赖 repo 接口和跨模块 contracts。
type Service struct {
	store            Store
	ids              snowflake.Generator
	cfg              config.ExperimentConfig
	audit            audit.Writer
	roles            contracts.IdentityService
	content          contracts.ContentReadService
	sandbox          contracts.SandboxService
	judge            contracts.JudgeService
	sim              contracts.SimService
	bus              eventbus.Bus
	storage          objectStorage
	files            fileService
	auth             *auth.Manager
	reportMaxBytes   int64
	reportScanPolicy upload.ScanPolicy
}

// ServiceDeps 是 experiment service 的装配依赖集合。
type ServiceDeps struct {
	Store            Store
	IDs              snowflake.Generator
	Config           config.ExperimentConfig
	Audit            audit.Writer
	Roles            contracts.IdentityService
	Content          contracts.ContentReadService
	Sandbox          contracts.SandboxService
	Judge            contracts.JudgeService
	Sim              contracts.SimService
	Bus              eventbus.Bus
	Storage          *storage.Storage
	FileService      fileService
	Auth             *auth.Manager
	ReportMaxBytes   int64
	ReportScanPolicy upload.ScanPolicy
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
	if deps.Sandbox == nil {
		return nil, fmt.Errorf("experiment service 缺少 sandbox 契约")
	}
	if deps.Judge == nil {
		return nil, fmt.Errorf("experiment service 缺少 judge 契约")
	}
	if deps.Sim == nil {
		return nil, fmt.Errorf("experiment service 缺少 sim 契约")
	}
	if deps.Bus == nil {
		return nil, fmt.Errorf("experiment service 缺少事件总线")
	}
	if deps.Storage == nil || deps.FileService == nil || deps.ReportMaxBytes <= 0 {
		return nil, fmt.Errorf("experiment service 缺少统一文件服务或报告上传边界")
	}
	if deps.Auth == nil {
		return nil, fmt.Errorf("experiment service 缺少统一鉴权服务")
	}
	if deps.Config.RecyclePollIntervalSeconds <= 0 || deps.Config.RecycleBatchSize <= 0 || deps.Config.InstanceIdleTimeoutSeconds <= 0 || deps.Config.PausedTimeoutSeconds <= 0 || deps.Config.ScoreOutboxBatchSize <= 0 || deps.Config.ScoreOutboxStaleMs <= 0 {
		return nil, fmt.Errorf("experiment service 配置不完整")
	}
	return &Service{store: deps.Store, ids: deps.IDs, cfg: deps.Config, audit: deps.Audit, roles: deps.Roles, content: deps.Content, sandbox: deps.Sandbox, judge: deps.Judge, sim: deps.Sim, bus: deps.Bus, storage: deps.Storage, files: deps.FileService, auth: deps.Auth, reportMaxBytes: deps.ReportMaxBytes, reportScanPolicy: deps.ReportScanPolicy}, nil
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
