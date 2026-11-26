package experiment

import (
	"context"
	"fmt"
	"io"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/upload"
	"chaimir/pkg/apperr"
	"chaimir/pkg/snowflake"
)

const (
	experimentModuleName     = "experiment"
	experimentReportResource = "report"
)

type objectStorage interface {
	CopyObject(ctx context.Context, sourceBucket, sourceKey, targetBucket, targetKey string) (int64, error)
	Delete(ctx context.Context, bucket, key string) error
	Put(ctx context.Context, bucket, key string, r io.Reader, size int64, contentType string) error
	BucketReport() string
	BucketCode() string
	BucketAttach() string
}

// fileService 描述 M7 复用统一文件上传规划所需的最小能力。
type fileService interface {
	PlanUpload(ctx context.Context, req storage.PlanUploadRequest) (storage.UploadPlan, error)
	IssueDownloadGrant(req storage.IssueDownloadGrantRequest) (string, storage.DownloadGrant, error)
}

// Service 承载 experiment 模块业务编排,依赖 repo 接口和跨模块 contracts。
type Service struct {
	cleanupTimeout   time.Duration
	store            Store
	ids              snowflake.Generator
	cfg              config.ExperimentConfig
	audit            audit.Writer
	roles            contracts.IdentityService
	content          contracts.ContentReadService
	contentClone     contracts.ContentCloneService
	sandbox          contracts.SandboxService
	sandboxClone     contracts.SandboxCompositionCloneService
	judge            contracts.JudgeService
	sim              contracts.SimService
	bus              eventbus.Bus
	storage          objectStorage
	files            fileService
	reportMaxBytes   int64
	cloneMaxBytes    int64
	reportScanPolicy upload.ScanPolicy
}

// ServiceDeps 是 experiment service 的装配依赖集合。
type ServiceDeps struct {
	CleanupTimeout   time.Duration
	Store            Store
	IDs              snowflake.Generator
	Config           config.ExperimentConfig
	Audit            audit.Writer
	Roles            contracts.IdentityService
	Content          contracts.ContentReadService
	ContentClone     contracts.ContentCloneService
	Sandbox          contracts.SandboxService
	SandboxClone     contracts.SandboxCompositionCloneService
	Judge            contracts.JudgeService
	Sim              contracts.SimService
	Bus              eventbus.Bus
	Storage          *storage.Storage
	FileService      fileService
	ReportMaxBytes   int64
	CloneMaxBytes    int64
	ReportScanPolicy upload.ScanPolicy
}

// isValidJudgeSandboxMode 在 service 边界校验 M3 沙箱模式契约,避免 rules 层依赖 contracts。
func isValidJudgeSandboxMode(mode string) bool {
	return mode == contracts.JudgeSandboxModeFresh || mode == contracts.JudgeSandboxModeReuse
}

// NewService 构造 experiment 服务,不接收数据库连接,由装配层传入 Store。
func NewService(deps ServiceDeps) (*Service, error) {
	if deps.CleanupTimeout <= 0 {
		return nil, fmt.Errorf("上传清理超时必须大于 0")
	}
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
	if deps.Storage == nil {
		return nil, fmt.Errorf("experiment service 缺少统一对象存储")
	}
	if deps.FileService == nil {
		return nil, fmt.Errorf("experiment service 缺少统一文件服务")
	}
	if deps.ReportMaxBytes <= 0 || deps.CloneMaxBytes <= 0 {
		return nil, fmt.Errorf("experiment service 缺少报告或附件大小配置")
	}
	if deps.Config.CleanupTimeoutSeconds <= 0 || deps.Config.RecyclePollIntervalSeconds <= 0 || deps.Config.RecycleBatchSize <= 0 || deps.Config.InstanceIdleTimeoutSeconds <= 0 || deps.Config.PausedTimeoutSeconds <= 0 || deps.Config.EventOutboxBatchSize <= 0 || deps.Config.EventOutboxStaleMs <= 0 {
		return nil, fmt.Errorf("experiment service 配置不完整")
	}
	if deps.ContentClone == nil || deps.SandboxClone == nil {
		return nil, fmt.Errorf("experiment service 缺少课程依赖复制契约")
	}
	return &Service{cleanupTimeout: deps.CleanupTimeout, store: deps.Store, ids: deps.IDs, cfg: deps.Config, audit: deps.Audit, roles: deps.Roles, content: deps.Content, contentClone: deps.ContentClone, sandbox: deps.Sandbox, sandboxClone: deps.SandboxClone, judge: deps.Judge, sim: deps.Sim, bus: deps.Bus, storage: deps.Storage, files: deps.FileService, reportMaxBytes: deps.ReportMaxBytes, cloneMaxBytes: deps.CloneMaxBytes, reportScanPolicy: deps.ReportScanPolicy}, nil
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

// isSchoolAdmin 判断当前账号是否具备学校管理员权限,并显式保留角色服务错误链。
func (s *Service) isSchoolAdmin(ctx context.Context, accountID int64) (bool, error) {
	ok, err := s.roles.HasRole(ctx, accountID, contracts.RoleSchoolAdmin)
	if err != nil {
		return false, apperr.ErrForbidden.WithCause(err)
	}
	return ok, nil
}

// ensureTeacherCanManage 校验教师作者或学校管理员权限,统一处理角色查询失败。
func (s *Service) ensureTeacherCanManage(ctx context.Context, accountID int64, item Experiment) error {
	isSchoolAdmin, err := s.isSchoolAdmin(ctx, accountID)
	if err != nil {
		return err
	}
	return ensureTeacherCanManage(accountID, isSchoolAdmin, item)
}
