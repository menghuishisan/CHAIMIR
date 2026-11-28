// 装配 M9 依赖，提供共同鉴权、事务和审计入口。
package admin

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
	"chaimir/internal/platform/transfer"
	"chaimir/pkg/apperr"
	"chaimir/pkg/snowflake"
)

// Service 承载 M9 管理后台业务编排。
type Service struct {
	cleanupTimeout time.Duration
	metrics        []contracts.AlertMetricReader
	recipients     contracts.AlertRecipientReader
	store          Store
	ids            snowflake.Generator
	audit          audit.Writer
	roles          roleReader
	identity       contracts.IdentityTenantReadService
	accounts       accountReader
	stats          contracts.IdentityStatsService
	auditRead      contracts.IdentityAuditReadService
	teaching       contracts.TeachingReadService
	sandbox        contracts.SandboxReadService
	experiment     contracts.ExperimentReadService
	contest        contracts.ContestReadService
	bus            eventbus.Bus
	monitoring     config.MonitoringConfig
	transfers      transferService
	storage        objectStorage
	files          fileService
	cfg            config.AdminConfig
}

// objectStorage 描述 M9 导出产物写入统一对象存储所需能力。
type objectStorage interface {
	Delete(ctx context.Context, bucket, key string) error
	Put(ctx context.Context, bucket, key string, r io.Reader, size int64, contentType string) error
	BucketReport() string
}

// fileService 描述 M9 复用统一文件服务规划对象路径所需能力。
type fileService interface {
	PlanUpload(ctx context.Context, req storage.PlanUploadRequest) (storage.UploadPlan, error)
}

// transferService 描述 M9 调用统一导入导出中心所需能力。
type transferService interface {
	CreateTask(context.Context, transfer.NewTaskRequest) (transfer.Task, error)
	GetTask(context.Context, int64, int64) (transfer.Task, error)
	ClaimTask(context.Context, int64, int64) (transfer.Task, error)
	RunWithLease(context.Context, transfer.Task, func(context.Context) error) error
	CompleteTask(context.Context, transfer.Task, transfer.CompleteTaskRequest) (transfer.Task, error)
	FailTask(context.Context, transfer.Task, error) (transfer.Task, error)
}

// ServiceDeps 是 M9 服务装配依赖。
type ServiceDeps struct {
	CleanupTimeout time.Duration
	Metrics        []contracts.AlertMetricReader
	Recipients     contracts.AlertRecipientReader
	Store          Store
	IDs            snowflake.Generator
	Audit          audit.Writer
	Roles          roleReader
	Identity       contracts.IdentityTenantReadService
	// Accounts 解析学校审计记录的操作人姓名。
	Accounts    accountReader
	Stats       contracts.IdentityStatsService
	AuditRead   contracts.IdentityAuditReadService
	Teaching    contracts.TeachingReadService
	Sandbox     contracts.SandboxReadService
	Experiment  contracts.ExperimentReadService
	Contest     contracts.ContestReadService
	Bus         eventbus.Bus
	Monitoring  config.MonitoringConfig
	Config      config.AdminConfig
	Transfers   transferService
	Storage     objectStorage
	FileService fileService
}

// NewService 构造 M9 服务。
func NewService(deps ServiceDeps) (*Service, error) {
	if deps.CleanupTimeout <= 0 {
		return nil, fmt.Errorf("上传清理超时必须大于 0")
	}
	if deps.Store == nil || deps.IDs == nil || deps.Audit == nil || deps.Roles == nil || deps.Identity == nil || deps.Accounts == nil || deps.Stats == nil || deps.AuditRead == nil || deps.Teaching == nil || deps.Sandbox == nil || deps.Experiment == nil || deps.Contest == nil || deps.Bus == nil {
		return nil, fmt.Errorf("admin service 依赖不完整")
	}
	if len(deps.Metrics) != 5 || deps.Recipients == nil {
		return nil, fmt.Errorf("admin alert dependencies incomplete")
	}
	for _, reader := range deps.Metrics {
		if reader == nil {
			return nil, fmt.Errorf("admin alert metric reader missing")
		}
	}
	if deps.Config.AlertPollSeconds <= 0 || deps.Config.AlertWindowSeconds <= 0 || deps.Config.AlertNotifyBatchSize <= 0 || deps.Config.AlertNotifyLeaseSeconds <= 0 || deps.Config.AlertNotifyMaxAttempts <= 0 {
		return nil, fmt.Errorf("admin alert configuration incomplete")
	}
	objects := deps.Storage
	if deps.Transfers == nil || objects == nil || deps.FileService == nil {
		return nil, fmt.Errorf("admin service 缺少统一导入导出或文件服务依赖")
	}
	if deps.Config.AuditExportWorkerBatchSize <= 0 || deps.Config.AuditExportWorkerPollMs <= 0 {
		return nil, fmt.Errorf("admin audit export worker 配置不完整")
	}
	return &Service{cleanupTimeout: deps.CleanupTimeout, metrics: deps.Metrics, recipients: deps.Recipients, store: deps.Store, ids: deps.IDs, audit: deps.Audit, roles: deps.Roles, identity: deps.Identity, accounts: deps.Accounts, stats: deps.Stats, auditRead: deps.AuditRead, teaching: deps.Teaching, sandbox: deps.Sandbox, experiment: deps.Experiment, contest: deps.Contest, bus: deps.Bus, monitoring: deps.Monitoring, transfers: deps.Transfers, storage: objects, files: deps.FileService, cfg: deps.Config}, nil
}

// MonitoringPanels 返回外部监控面板入口。
func (s *Service) MonitoringPanels(ctx context.Context) ([]MonitoringPanel, error) {
	if _, err := requirePlatform(ctx); err != nil {
		return nil, err
	}
	return ParseMonitoringPanels(s.monitoring.PanelsJSON, s.monitoring.AllowedPanelOrigins)
}

// runWrite 按是否存在 tenant_id 选择平台事务或租户 RLS 事务。
func (s *Service) runWrite(ctx context.Context, tenantID int64, fn func(context.Context, TxStore) error) error {
	if tenantID > 0 {
		return s.store.TenantTx(ctx, tenantID, fn)
	}
	return s.store.PlatformTx(ctx, fn)
}

// runAdminRead 在平台或租户事务中执行只读聚合查询。
func runAdminRead[T any](ctx context.Context, store Store, tenantID int64, fn func(context.Context, TxStore) (T, error)) (T, error) {
	var out T
	var err error
	run := func(ctx context.Context, tx TxStore) error {
		out, err = fn(ctx, tx)
		return err
	}
	if tenantID > 0 {
		err = store.TenantTx(ctx, tenantID, run)
	} else {
		err = store.PlatformTx(ctx, run)
	}
	return out, err
}

// currentAdminIdentity 读取并校验管理后台允许使用的身份。
func (s *Service) currentAdminIdentity(ctx context.Context) (tenant.Identity, error) {
	id, ok := tenant.FromContext(ctx)
	if !ok || id.AccountID <= 0 {
		return tenant.Identity{}, apperr.ErrUnauthorized
	}
	if id.IsPlatform {
		return id, nil
	}
	return s.requireTenantAdmin(ctx)
}

// requirePlatform 校验当前请求来自平台管理员身份。
func requirePlatform(ctx context.Context) (tenant.Identity, error) {
	id, ok := tenant.FromContext(ctx)
	if !ok || id.AccountID <= 0 {
		return tenant.Identity{}, apperr.ErrUnauthorized
	}
	if !id.IsPlatform {
		return tenant.Identity{}, apperr.ErrForbidden
	}
	return id, nil
}

// requireTenantAdmin 校验当前请求来自学校管理员身份。
func (s *Service) requireTenantAdmin(ctx context.Context) (tenant.Identity, error) {
	id, ok := tenant.FromContext(ctx)
	if !ok || id.TenantID <= 0 || id.AccountID <= 0 || id.IsPlatform {
		return tenant.Identity{}, apperr.ErrForbidden
	}
	has, err := s.roles.HasRole(ctx, id.AccountID, contracts.RoleSchoolAdmin)
	if err != nil {
		return tenant.Identity{}, apperr.ErrForbidden.WithCause(err)
	}
	if !has {
		return tenant.Identity{}, apperr.ErrForbidden
	}
	return id, nil
}

// writeAudit 将 M9 管理操作写入 identity 共享审计表。
func (s *Service) writeAudit(ctx context.Context, id tenant.Identity, action, targetType string, targetID int64, detail map[string]any) error {
	role := contracts.RoleNumSchoolAdmin
	tenantID := id.TenantID
	if id.IsPlatform {
		role = contracts.RoleNumPlatformAdmin
		tenantID = 0
	}
	entry, err := audit.BuildEntry(ctx, tenantID, id.AccountID, role, action, targetType, targetID, detail)
	if err != nil {
		return apperr.ErrAdminAuditWriteFailed.WithCause(err)
	}
	if err := s.audit.Write(ctx, entry); err != nil {
		return apperr.ErrAdminAuditWriteFailed.WithCause(err)
	}
	return nil
}
