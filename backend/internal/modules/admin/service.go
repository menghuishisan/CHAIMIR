// admin service 文件实现 M9 管理后台聚合、配置、告警和备份业务编排。
package admin

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/secretmap"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/timex"
	"chaimir/internal/platform/transfer"
	"chaimir/pkg/apperr"
	pkgcrypto "chaimir/pkg/crypto"
	"chaimir/pkg/snowflake"
)

// Service 承载 M9 管理后台业务编排。
type Service struct {
	store        Store
	ids          snowflake.Generator
	audit        audit.Writer
	roles        roleReader
	identity     contracts.IdentityTenantReadService
	stats        contracts.IdentityStatsService
	auditRead    contracts.IdentityAuditReadService
	teaching     contracts.TeachingReadService
	sandbox      contracts.SandboxService
	experiment   contracts.ExperimentReadService
	contest      contracts.ContestReadService
	notify       contracts.NotifyService
	monitoring   config.MonitoringConfig
	secretCipher *pkgcrypto.Cipher
	transfers    transferService
	storage      objectStorage
	files        fileService
}

// objectStorage 描述 M9 导出产物写入统一对象存储所需能力。
type objectStorage interface {
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
	CompleteTask(context.Context, int64, int64, transfer.CompleteTaskRequest) (transfer.Task, error)
}

// ServiceDeps 是 M9 服务装配依赖。
type ServiceDeps struct {
	Store       Store
	IDs         snowflake.Generator
	Audit       audit.Writer
	Roles       roleReader
	Identity    contracts.IdentityTenantReadService
	Stats       contracts.IdentityStatsService
	AuditRead   contracts.IdentityAuditReadService
	Teaching    contracts.TeachingReadService
	Sandbox     contracts.SandboxService
	Experiment  contracts.ExperimentReadService
	Contest     contracts.ContestReadService
	Notify      contracts.NotifyService
	Monitoring  config.MonitoringConfig
	Cipher      *pkgcrypto.Cipher
	Transfers   transferService
	Storage     *storage.Storage
	Objects     objectStorage
	FileService fileService
}

// NewService 构造 M9 服务。
func NewService(deps ServiceDeps) (*Service, error) {
	if deps.Store == nil || deps.IDs == nil || deps.Audit == nil || deps.Roles == nil || deps.Identity == nil || deps.Stats == nil || deps.AuditRead == nil || deps.Teaching == nil || deps.Sandbox == nil || deps.Experiment == nil || deps.Contest == nil || deps.Notify == nil {
		return nil, fmt.Errorf("admin service 依赖不完整")
	}
	objects := deps.Objects
	if objects == nil {
		objects = deps.Storage
	}
	if deps.Transfers == nil || objects == nil || deps.FileService == nil {
		return nil, fmt.Errorf("admin service 缺少统一导入导出或文件服务依赖")
	}
	return &Service{store: deps.Store, ids: deps.IDs, audit: deps.Audit, roles: deps.Roles, identity: deps.Identity, stats: deps.Stats, auditRead: deps.AuditRead, teaching: deps.Teaching, sandbox: deps.Sandbox, experiment: deps.Experiment, contest: deps.Contest, notify: deps.Notify, monitoring: deps.Monitoring, secretCipher: deps.Cipher, transfers: deps.Transfers, storage: objects, files: deps.FileService}, nil
}

// PlatformDashboard 聚合平台级看板。
func (s *Service) PlatformDashboard(ctx context.Context) (DashboardDTO, error) {
	if _, err := requirePlatform(ctx); err != nil {
		return DashboardDTO{}, err
	}
	stats, err := s.stats.PlatformStats(ctx)
	if err != nil {
		return DashboardDTO{}, apperr.ErrAdminDashboardIdentityFailed.WithCause(err)
	}
	tenants, err := s.identity.ListTenants(ctx)
	if err != nil {
		return DashboardDTO{}, apperr.ErrAdminDashboardIdentityFailed.WithCause(err)
	}
	ops, err := s.aggregateTenantOperations(ctx, tenants)
	if err != nil {
		return DashboardDTO{}, err
	}
	return DashboardDTO{Scope: ScopeGlobal, TenantCount: stats.TenantCount, AccountCount: stats.AccountCount, TeacherCount: stats.TeacherCount, StudentCount: stats.StudentCount, ActiveAccountCount: stats.ActiveAccountCount, CourseCount: ops.CourseCount, ActiveCourseCount: ops.ActiveCourseCount, ExperimentCount: ops.ExperimentCount, ActiveInstanceCount: ops.ActiveInstanceCount, ContestCount: ops.ContestCount, ActiveContestCount: ops.ActiveContestCount, ActiveSandboxCount: ops.ActiveSandboxCount, PendingApplyCount: stats.PendingApplyCount, ResourceQuotaSnapshot: ops.ResourceQuotaSnapshot(), GeneratedAt: timex.Now()}, nil
}

// SchoolDashboard 聚合学校级看板。
func (s *Service) SchoolDashboard(ctx context.Context) (DashboardDTO, error) {
	id, err := s.requireTenantAdmin(ctx)
	if err != nil {
		return DashboardDTO{}, err
	}
	stats, err := s.stats.TenantStats(ctx, id.TenantID)
	if err != nil {
		return DashboardDTO{}, apperr.ErrAdminDashboardIdentityFailed.WithCause(err)
	}
	out := DashboardDTO{Scope: ScopeTenant, TenantID: ids.ID(id.TenantID), AccountCount: stats.AccountCount, TeacherCount: stats.TeacherCount, StudentCount: stats.StudentCount, ActiveAccountCount: stats.ActiveAccountCount, GeneratedAt: timex.Now()}
	t, err := s.teaching.Stats(ctx, id.TenantID)
	if err != nil {
		return DashboardDTO{}, apperr.ErrAdminDashboardTeachingFailed.WithCause(err)
	}
	out.CourseCount = t.CourseCount
	out.ActiveCourseCount = t.ActiveCourseCount
	e, err := s.experiment.Stats(ctx, contracts.ExperimentStatsQuery{TenantID: id.TenantID})
	if err != nil {
		return DashboardDTO{}, apperr.ErrAdminDashboardExperimentFailed.WithCause(err)
	}
	out.ExperimentCount = e.ExperimentCount
	out.ActiveInstanceCount = e.ActiveInstanceCount
	c, err := s.contest.Stats(ctx, id.TenantID)
	if err != nil {
		return DashboardDTO{}, apperr.ErrAdminDashboardContestFailed.WithCause(err)
	}
	out.ContestCount = c.ContestCount
	out.ActiveContestCount = c.ActiveContestCount
	q, err := s.sandbox.Stats(ctx, id.TenantID)
	if err != nil {
		return DashboardDTO{}, apperr.ErrAdminDashboardSandboxFailed.WithCause(err)
	}
	out.ActiveSandboxCount = q.ActiveSandboxCount
	out.ResourceQuotaSnapshot = map[string]any{"max_concurrent_sandbox": q.MaxConcurrentSandbox, "max_cpu": q.MaxCPU, "max_memory_mb": q.MaxMemoryMB}
	return out, nil
}

// PlatformStatistics 读取平台级周期统计快照。
func (s *Service) PlatformStatistics(ctx context.Context, fromDate, toDate string) ([]StatisticsDTO, error) {
	if _, err := requirePlatform(ctx); err != nil {
		return nil, err
	}
	if err := validateDateRange(fromDate, toDate); err != nil {
		return nil, err
	}
	return runAdminRead(ctx, s.store, 0, func(ctx context.Context, tx TxStore) ([]StatisticsDTO, error) {
		rows, err := tx.ListPlatformStatistics(ctx, ScopeGlobal, 0, fromDate, toDate)
		if err != nil {
			return nil, apperr.ErrAdminStatisticsInvalid.WithCause(err)
		}
		return rows, nil
	})
}

// SchoolStatistics 读取当前学校的周期统计快照。
func (s *Service) SchoolStatistics(ctx context.Context, fromDate, toDate string) ([]StatisticsDTO, error) {
	id, err := s.requireTenantAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if err := validateDateRange(fromDate, toDate); err != nil {
		return nil, err
	}
	return runAdminRead(ctx, s.store, id.TenantID, func(ctx context.Context, tx TxStore) ([]StatisticsDTO, error) {
		rows, err := tx.ListPlatformStatistics(ctx, ScopeTenant, id.TenantID, fromDate, toDate)
		if err != nil {
			return nil, apperr.ErrAdminStatisticsInvalid.WithCause(err)
		}
		return rows, nil
	})
}

// RunStatisticsSnapshotOnce 生成当天平台与租户统计快照。
func (s *Service) RunStatisticsSnapshotOnce(ctx context.Context) error {
	statDate := timex.Now().Format("2006-01-02")
	tenants, err := s.identity.ListTenants(ctx)
	if err != nil {
		return apperr.ErrAdminStatisticsInvalid.WithCause(err)
	}
	if err := s.snapshotGlobalStats(ctx, statDate, tenants); err != nil {
		return err
	}
	for _, item := range tenants {
		if item.TenantID <= 0 {
			continue
		}
		if err := s.snapshotTenantStats(ctx, item.TenantID, statDate); err != nil {
			return err
		}
	}
	return nil
}

// ListTenants 读取租户列表。
func (s *Service) ListTenants(ctx context.Context) ([]contracts.TenantSummary, error) {
	if _, err := requirePlatform(ctx); err != nil {
		return nil, err
	}
	return s.identity.ListTenants(ctx)
}

// ListApplications 读取入驻申请列表。
func (s *Service) ListApplications(ctx context.Context, status int16) ([]contracts.TenantApplicationSummary, error) {
	if _, err := requirePlatform(ctx); err != nil {
		return nil, err
	}
	return s.identity.ListTenantApplications(ctx, contracts.TenantApplicationQuery{Status: status})
}

// QueryAudit 查询统一审计日志。
func (s *Service) QueryAudit(ctx context.Context, query contracts.AuditQuery) (contracts.AuditQueryResult, error) {
	id, ok := tenant.FromContext(ctx)
	if !ok {
		return contracts.AuditQueryResult{}, apperr.ErrUnauthorized
	}
	if !id.IsPlatform {
		query.TenantID = id.TenantID
	} else {
		query.IncludePlatform = true
	}
	result, err := s.auditRead.QueryAuditLogs(ctx, query)
	if err != nil {
		return contracts.AuditQueryResult{}, apperr.ErrAdminAuditQueryInvalid.WithCause(err)
	}
	return result, nil
}

const auditExportSubject = "admin.audit_export"

// ExportAuditCSV 导出审计日志 CSV 并登记到统一导入导出中心。
func (s *Service) ExportAuditCSV(ctx context.Context, query contracts.AuditQuery) (transfer.TaskDTO, error) {
	id, err := s.currentAdminIdentity(ctx)
	if err != nil {
		return transfer.TaskDTO{}, err
	}
	result, err := s.collectAuditExportRows(ctx, query)
	if err != nil {
		return transfer.TaskDTO{}, err
	}
	fileName := "audit.csv"
	task, err := s.transfers.CreateTask(ctx, transfer.NewTaskRequest{
		TenantID:    id.TenantID,
		AccountID:   id.AccountID,
		Channel:     transfer.ChannelExport,
		Subject:     auditExportSubject,
		FileName:    fileName,
		ContentType: "text/csv; charset=utf-8",
	})
	if err != nil {
		return transfer.TaskDTO{}, apperr.ErrAdminAuditExportTaskCreateFailed.WithCause(err)
	}
	data, err := auditCSVBytes(result)
	if err != nil {
		return transfer.TaskDTO{}, apperr.ErrAdminAuditExportCSVFailed.WithCause(err)
	}
	plan, err := s.files.PlanUpload(ctx, storage.PlanUploadRequest{
		TenantID:           id.TenantID,
		AccountID:          id.AccountID,
		AllowPlatformScope: id.IsPlatform,
		Module:             "transfer",
		ResourceType:       string(transfer.ChannelExport),
		ResourceID:         fmt.Sprint(task.TaskID),
		FileName:           fileName,
		ContentType:        "text/csv; charset=utf-8",
		Size:               int64(len(data)),
		ExpectedBucket:     s.storage.BucketReport(),
		AllowedFileName:    true,
		Content:            data,
	})
	if err != nil {
		return transfer.TaskDTO{}, apperr.ErrAdminAuditExportUploadPlanFailed.WithCause(err)
	}
	if err := s.storage.Put(ctx, plan.Bucket, plan.Key, bytes.NewReader(data), int64(len(data)), "text/csv; charset=utf-8"); err != nil {
		return transfer.TaskDTO{}, apperr.ErrAdminAuditExportObjectWriteFailed.WithCause(err)
	}
	completed, err := s.transfers.CompleteTask(ctx, id.TenantID, task.TaskID, transfer.CompleteTaskRequest{ObjectRef: plan.ObjectRef, Size: int64(len(data))})
	if err != nil {
		return transfer.TaskDTO{}, apperr.ErrAdminAuditExportTaskCompleteFailed.WithCause(err)
	}
	if err := s.writeAudit(ctx, id, "admin.audit.export", "audit_log", 0, map[string]any{"size": result.Size, "total": result.Total, "transfer_task_id": task.TaskID}); err != nil {
		return transfer.TaskDTO{}, apperr.ErrAdminAuditWriteFailed.WithCause(err)
	}
	return exportTaskDTO(completed), nil
}

// collectAuditExportRows 按统一分页上限拉取当前权限范围内全部匹配审计记录。
func (s *Service) collectAuditExportRows(ctx context.Context, query contracts.AuditQuery) (contracts.AuditQueryResult, error) {
	var out contracts.AuditQueryResult
	for page := int32(1); ; page++ {
		query.Page = page
		query.Size = 0
		result, err := s.QueryAudit(ctx, query)
		if err != nil {
			return contracts.AuditQueryResult{}, err
		}
		if out.Page == 0 {
			out.Page = 1
			out.Size = result.Size
			out.Total = result.Total
		}
		out.List = append(out.List, result.List...)
		if len(result.List) == 0 || int64(len(out.List)) >= result.Total {
			break
		}
	}
	return out, nil
}

// auditCSVBytes 将审计查询结果编码为 CSV 文件内容。
func auditCSVBytes(result contracts.AuditQueryResult) ([]byte, error) {
	var b strings.Builder
	w := csv.NewWriter(&b)
	if err := w.Write([]string{"id", "tenant_id", "actor_id", "action", "target_type", "target_id", "trace_id", "created_at"}); err != nil {
		return nil, fmt.Errorf("写入 CSV 表头失败: %w", err)
	}
	for _, row := range result.List {
		if err := w.Write([]string{fmt.Sprint(row.ID), fmt.Sprint(row.TenantID), fmt.Sprint(row.ActorID), row.Action, row.TargetType, fmt.Sprint(row.TargetID), row.TraceID, row.CreatedAt.Format(time.RFC3339)}); err != nil {
			return nil, fmt.Errorf("写入 CSV 行失败: %w", err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("刷新 CSV 内容失败: %w", err)
	}
	return []byte(b.String()), nil
}

// ListConfigs 查询系统配置。
func (s *Service) ListConfigs(ctx context.Context, scope int16) ([]ConfigDTO, error) {
	id, err := s.currentAdminIdentity(ctx)
	if err != nil {
		return nil, err
	}
	tenantID := int64(0)
	if id.IsPlatform {
		if scope == 0 {
			scope = ScopeGlobal
		}
		if scope != ScopeGlobal {
			return nil, apperr.ErrAdminConfigInvalid
		}
	} else {
		scope = ScopeTenant
		tenantID = id.TenantID
	}
	return runAdminRead(ctx, s.store, tenantID, func(ctx context.Context, tx TxStore) ([]ConfigDTO, error) {
		rows, err := tx.ListSystemConfigs(ctx, scope, tenantID)
		if err != nil {
			return nil, err
		}
		return maskConfigs(rows), nil
	})
}

// UpdateConfig 更新或创建系统配置。
func (s *Service) UpdateConfig(ctx context.Context, key string, req ConfigUpdateRequest) (ConfigDTO, error) {
	id, err := s.currentAdminIdentity(ctx)
	if err != nil {
		return ConfigDTO{}, err
	}
	key = strings.TrimSpace(key)
	if key == "" || req.Value == nil {
		return ConfigDTO{}, apperr.ErrAdminConfigInvalid
	}
	if !id.IsPlatform {
		req.Scope = ScopeTenant
		req.TenantID = ids.ID(id.TenantID)
	}
	if id.IsPlatform && req.Scope == ScopeTenant {
		return ConfigDTO{}, apperr.ErrAdminConfigInvalid
	}
	protected, err := secretmap.Protect(s.secretCipher, req.Value, "系统配置")
	if err != nil {
		return ConfigDTO{}, apperr.ErrAdminConfigInvalid.WithCause(err)
	}
	req.Value = protected
	if err := validateScopeTenant(req.Scope, req.TenantID.Int64()); err != nil {
		return ConfigDTO{}, err
	}
	var out ConfigDTO
	err = s.runWrite(ctx, req.TenantID.Int64(), func(ctx context.Context, tx TxStore) error {
		old, err := tx.GetSystemConfig(ctx, req.Scope, req.TenantID.Int64(), key)
		if err != nil && !isNoRows(err) {
			return apperr.ErrAdminConfigInvalid.WithCause(err)
		}
		if isNoRows(err) {
			out, err = tx.CreateSystemConfig(ctx, s.ids.Generate(), req.Scope, req.TenantID.Int64(), key, req.Value, id.AccountID)
			if err != nil {
				return apperr.ErrAdminConfigInvalid.WithCause(err)
			}
			_, err = tx.CreateConfigChangeLog(ctx, s.ids.Generate(), out.ID.Int64(), req.TenantID.Int64(), map[string]any{}, out.Value, id.AccountID)
			return err
		}
		out, err = tx.UpdateSystemConfig(ctx, req.Scope, req.TenantID.Int64(), key, req.Value, id.AccountID, req.Version)
		if err != nil {
			return apperr.ErrAdminConfigConflict.WithCause(err)
		}
		_, err = tx.CreateConfigChangeLog(ctx, s.ids.Generate(), out.ID.Int64(), req.TenantID.Int64(), old.Value, out.Value, id.AccountID)
		return err
	})
	if err != nil {
		return ConfigDTO{}, err
	}
	if err := s.writeAudit(ctx, id, "admin.config.update", "system_config", out.ID.Int64(), map[string]any{"key": key}); err != nil {
		return ConfigDTO{}, apperr.ErrAdminAuditWriteFailed.WithCause(err)
	}
	out.Value = secretmap.Mask(out.Value)
	return out, nil
}

// ListConfigHistory 查询配置历史。
func (s *Service) ListConfigHistory(ctx context.Context, scope int16, tenantID int64, key string, page, size int) ([]ConfigChangeLogDTO, int64, int, int, error) {
	id, err := s.currentAdminIdentity(ctx)
	if err != nil {
		return nil, 0, page, size, err
	}
	page, size = pagex.Normalize(page, size)
	if !id.IsPlatform {
		scope = ScopeTenant
		tenantID = id.TenantID
	} else if scope != ScopeGlobal {
		return nil, 0, page, size, apperr.ErrAdminConfigInvalid
	}
	var total int64
	rows, err := runAdminRead(ctx, s.store, tenantID, func(ctx context.Context, tx TxStore) ([]ConfigChangeLogDTO, error) {
		cfg, err := tx.GetSystemConfig(ctx, scope, tenantID, key)
		if err != nil {
			return nil, apperr.ErrAdminConfigNotFound.WithCause(err)
		}
		rows, count, err := tx.ListConfigChangeLogs(ctx, cfg.ID.Int64(), page, size)
		if err != nil {
			return nil, err
		}
		total = count
		return maskConfigLogs(rows), nil
	})
	return rows, total, page, size, err
}

// RollbackConfig 把配置回退到指定历史记录的变更前值。
func (s *Service) RollbackConfig(ctx context.Context, key string, req ConfigRollbackRequest) (ConfigDTO, error) {
	id, err := s.currentAdminIdentity(ctx)
	if err != nil {
		return ConfigDTO{}, err
	}
	key = strings.TrimSpace(key)
	if key == "" || req.ChangeLogID <= 0 {
		return ConfigDTO{}, apperr.ErrAdminConfigInvalid
	}
	if !id.IsPlatform {
		req.Scope = ScopeTenant
		req.TenantID = ids.ID(id.TenantID)
	}
	if id.IsPlatform && req.Scope == ScopeTenant {
		return ConfigDTO{}, apperr.ErrAdminConfigInvalid
	}
	if err := validateScopeTenant(req.Scope, req.TenantID.Int64()); err != nil {
		return ConfigDTO{}, err
	}
	var out ConfigDTO
	err = s.runWrite(ctx, req.TenantID.Int64(), func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetSystemConfig(ctx, req.Scope, req.TenantID.Int64(), key)
		if err != nil {
			return apperr.ErrAdminConfigNotFound.WithCause(err)
		}
		history, err := tx.GetConfigChangeLog(ctx, req.ChangeLogID.Int64(), current.ID.Int64())
		if err != nil {
			return apperr.ErrAdminConfigNotFound.WithCause(err)
		}
		out, err = tx.UpdateSystemConfig(ctx, req.Scope, req.TenantID.Int64(), key, history.OldValue, id.AccountID, req.Version)
		if err != nil {
			return apperr.ErrAdminConfigConflict.WithCause(err)
		}
		_, err = tx.CreateConfigChangeLog(ctx, s.ids.Generate(), out.ID.Int64(), req.TenantID.Int64(), current.Value, out.Value, id.AccountID)
		return err
	})
	if err != nil {
		return ConfigDTO{}, err
	}
	if err := s.writeAudit(ctx, id, "admin.config.rollback", "system_config", out.ID.Int64(), map[string]any{"key": key, "change_log_id": req.ChangeLogID.String()}); err != nil {
		return ConfigDTO{}, apperr.ErrAdminAuditWriteFailed.WithCause(err)
	}
	out.Value = secretmap.Mask(out.Value)
	return out, nil
}

// ListAlertRules 查询告警规则。
func (s *Service) ListAlertRules(ctx context.Context, scope int16) ([]AlertRuleDTO, error) {
	id, err := s.currentAdminIdentity(ctx)
	if err != nil {
		return nil, err
	}
	tenantID := int64(0)
	if id.IsPlatform {
		if scope == 0 {
			scope = ScopeGlobal
		}
		if scope != ScopeGlobal {
			return nil, apperr.ErrAdminAlertInvalid
		}
	} else {
		scope = ScopeTenant
		tenantID = id.TenantID
	}
	return runAdminRead(ctx, s.store, tenantID, func(ctx context.Context, tx TxStore) ([]AlertRuleDTO, error) {
		return tx.ListAlertRules(ctx, scope, tenantID)
	})
}

// CreateAlertRule 创建告警规则。
func (s *Service) CreateAlertRule(ctx context.Context, req AlertRuleRequest) (AlertRuleDTO, error) {
	id, err := s.currentAdminIdentity(ctx)
	if err != nil {
		return AlertRuleDTO{}, err
	}
	if !id.IsPlatform {
		req.Scope = ScopeTenant
		req.TenantID = ids.ID(id.TenantID)
	}
	if id.IsPlatform && req.Scope == ScopeTenant {
		return AlertRuleDTO{}, apperr.ErrAdminAlertInvalid
	}
	if err := validateAlertRule(req); err != nil {
		return AlertRuleDTO{}, err
	}
	var out AlertRuleDTO
	err = s.runWrite(ctx, req.TenantID.Int64(), func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.CreateAlertRule(ctx, s.ids.Generate(), req)
		return err
	})
	if err != nil {
		return AlertRuleDTO{}, apperr.ErrAdminAlertInvalid.WithCause(err)
	}
	if err := s.writeAudit(ctx, id, "admin.alert_rule.create", "alert_rule", out.ID.Int64(), map[string]any{"scope": out.Scope, "tenant_id": out.TenantID.String(), "metric": out.Metric, "level": out.Level}); err != nil {
		return AlertRuleDTO{}, apperr.ErrAdminAuditWriteFailed.WithCause(err)
	}
	return out, nil
}

// UpdateAlertRule 更新告警规则。
func (s *Service) UpdateAlertRule(ctx context.Context, ruleID int64, req AlertRuleRequest) (AlertRuleDTO, error) {
	id, err := s.currentAdminIdentity(ctx)
	if err != nil {
		return AlertRuleDTO{}, err
	}
	if !id.IsPlatform {
		req.Scope = ScopeTenant
		req.TenantID = ids.ID(id.TenantID)
	}
	if id.IsPlatform && req.Scope == ScopeTenant {
		return AlertRuleDTO{}, apperr.ErrAdminAlertInvalid
	}
	if err := validateAlertRule(req); err != nil {
		return AlertRuleDTO{}, err
	}
	var out AlertRuleDTO
	err = s.runWrite(ctx, req.TenantID.Int64(), func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.UpdateAlertRule(ctx, ruleID, req)
		return err
	})
	if err != nil {
		return AlertRuleDTO{}, apperr.ErrAdminAlertNotFound.WithCause(err)
	}
	if err := s.writeAudit(ctx, id, "admin.alert_rule.update", "alert_rule", out.ID.Int64(), map[string]any{"scope": out.Scope, "tenant_id": out.TenantID.String(), "metric": out.Metric, "level": out.Level}); err != nil {
		return AlertRuleDTO{}, apperr.ErrAdminAuditWriteFailed.WithCause(err)
	}
	return out, nil
}

// ListAlertEvents 查询告警事件。
func (s *Service) ListAlertEvents(ctx context.Context, status int16, page, size int) ([]AlertEventDTO, int64, int, int, error) {
	id, err := s.currentAdminIdentity(ctx)
	if err != nil {
		return nil, 0, page, size, err
	}
	page, size = pagex.Normalize(page, size)
	tenantID := int64(0)
	if !id.IsPlatform {
		tenantID = id.TenantID
	}
	var total int64
	rows, err := runAdminRead(ctx, s.store, tenantID, func(ctx context.Context, tx TxStore) ([]AlertEventDTO, error) {
		out, count, err := tx.ListAlertEvents(ctx, status, tenantID, page, size)
		total = count
		return out, err
	})
	return rows, total, page, size, err
}

// HandleAlertEvent 处理告警事件。
func (s *Service) HandleAlertEvent(ctx context.Context, eventID int64, req AlertEventRequest) (AlertEventDTO, error) {
	id, err := s.currentAdminIdentity(ctx)
	if err != nil {
		return AlertEventDTO{}, err
	}
	if req.Status != AlertStatusHandled && req.Status != AlertStatusIgnored {
		return AlertEventDTO{}, apperr.ErrAdminAlertStateInvalid
	}
	var out AlertEventDTO
	err = s.runWrite(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.HandleAlertEvent(ctx, eventID, id.TenantID, req.Status, id.AccountID)
		return err
	})
	if err != nil {
		return AlertEventDTO{}, apperr.ErrAdminAlertNotFound.WithCause(err)
	}
	if out.TenantID > 0 {
		if err := s.notify.Push(ctx, contracts.NotifyPushRequest{
			TenantID: out.TenantID.Int64(),
			Topic:    fmt.Sprintf("tenant:%d:alert", out.TenantID),
			Payload: map[string]any{
				"event_id":   out.ID,
				"rule_id":    out.RuleID,
				"level":      out.Level,
				"status":     out.Status,
				"handler_id": out.HandlerID,
			},
		}); err != nil {
			return AlertEventDTO{}, apperr.ErrAdminAlertInvalid.WithCause(err)
		}
	}
	if err := s.writeAudit(ctx, id, "admin.alert.handle", "alert_event", out.ID.Int64(), map[string]any{"status": out.Status}); err != nil {
		return AlertEventDTO{}, apperr.ErrAdminAuditWriteFailed.WithCause(err)
	}
	return out, nil
}

// MonitoringPanels 返回外部监控面板入口。
func (s *Service) MonitoringPanels(ctx context.Context) ([]MonitoringPanel, error) {
	if _, err := requirePlatform(ctx); err != nil {
		return nil, err
	}
	return ParseMonitoringPanels(s.monitoring.PanelsJSON)
}

// ListBackups 查询备份记录。
func (s *Service) ListBackups(ctx context.Context, page, size int) ([]BackupRecordDTO, int64, int, int, error) {
	if _, err := requirePlatform(ctx); err != nil {
		return nil, 0, page, size, err
	}
	page, size = pagex.Normalize(page, size)
	var total int64
	rows, err := runAdminRead(ctx, s.store, 0, func(ctx context.Context, tx TxStore) ([]BackupRecordDTO, error) {
		out, count, err := tx.ListBackupRecords(ctx, page, size)
		total = count
		return out, err
	})
	return rows, total, page, size, err
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

// validateAlertRule 校验告警规则范围、指标和等级。
func validateAlertRule(req AlertRuleRequest) error {
	if err := validateScopeTenant(req.Scope, req.TenantID.Int64()); err != nil {
		return apperr.ErrAdminAlertInvalid
	}
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Metric) == "" || req.Level < 1 || req.Level > 4 {
		return apperr.ErrAdminAlertInvalid
	}
	if len(req.Condition) != 3 {
		return apperr.ErrAdminAlertInvalid
	}
	operator, ok := req.Condition["operator"].(string)
	if !ok || (operator != "gt" && operator != "gte" && operator != "lt" && operator != "lte" && operator != "eq") {
		return apperr.ErrAdminAlertInvalid
	}
	if _, ok := jsonx.Float64FromNumberOK(req.Condition["threshold"]); !ok {
		return apperr.ErrAdminAlertInvalid
	}
	duration, ok := jsonx.Float64FromNumberOK(req.Condition["duration_minutes"])
	if !ok || duration < 0 {
		return apperr.ErrAdminAlertInvalid
	}
	return nil
}

// writeAudit 将 M9 管理操作写入 identity 共享审计表。
func (s *Service) writeAudit(ctx context.Context, id tenant.Identity, action, targetType string, targetID int64, detail map[string]any) error {
	role := int16(2)
	tenantID := id.TenantID
	if id.IsPlatform {
		role = 1
		tenantID = 0
	}
	entry, err := audit.BuildEntry(ctx, tenantID, id.AccountID, role, action, targetType, targetID, detail)
	if err != nil {
		return err
	}
	return s.audit.Write(ctx, entry)
}

// snapshotGlobalStats 聚合平台级只读指标并写入 M9 自有统计快照。
func (s *Service) snapshotGlobalStats(ctx context.Context, statDate string, tenants []contracts.TenantSummary) error {
	stats, err := s.stats.PlatformStats(ctx)
	if err != nil {
		return apperr.ErrAdminStatisticsInvalid.WithCause(err)
	}
	ops, err := s.aggregateTenantOperations(ctx, tenants)
	if err != nil {
		return apperr.ErrAdminStatisticsInvalid.WithCause(err)
	}
	metrics := map[string]any{
		"tenant_count":           stats.TenantCount,
		"account_count":          stats.AccountCount,
		"teacher_count":          stats.TeacherCount,
		"student_count":          stats.StudentCount,
		"active_account_count":   stats.ActiveAccountCount,
		"pending_apply_count":    stats.PendingApplyCount,
		"course_count":           ops.CourseCount,
		"active_course_count":    ops.ActiveCourseCount,
		"learning_duration_sec":  ops.LearningDurationSec,
		"experiment_count":       ops.ExperimentCount,
		"active_instance_count":  ops.ActiveInstanceCount,
		"contest_count":          ops.ContestCount,
		"active_contest_count":   ops.ActiveContestCount,
		"participant_count":      ops.ParticipantCount,
		"active_sandbox_count":   ops.ActiveSandboxCount,
		"max_concurrent_sandbox": ops.MaxConcurrentSandbox,
		"max_cpu":                ops.MaxCPU,
		"max_memory_mb":          ops.MaxMemoryMB,
	}
	return s.upsertStatistics(ctx, 0, ScopeGlobal, 0, statDate, metrics)
}

// snapshotTenantStats 聚合单租户只读指标并写入 M9 自有统计快照。
func (s *Service) snapshotTenantStats(ctx context.Context, tenantID int64, statDate string) error {
	stats, err := s.stats.TenantStats(ctx, tenantID)
	if err != nil {
		return apperr.ErrAdminStatisticsInvalid.WithCause(err)
	}
	metrics := map[string]any{
		"account_count":        stats.AccountCount,
		"teacher_count":        stats.TeacherCount,
		"student_count":        stats.StudentCount,
		"active_account_count": stats.ActiveAccountCount,
	}
	t, err := s.teaching.Stats(ctx, tenantID)
	if err != nil {
		return apperr.ErrAdminStatisticsInvalid.WithCause(err)
	}
	metrics["course_count"] = t.CourseCount
	metrics["active_course_count"] = t.ActiveCourseCount
	metrics["learning_duration_sec"] = t.LearningDurationSec
	e, err := s.experiment.Stats(ctx, contracts.ExperimentStatsQuery{TenantID: tenantID})
	if err != nil {
		return apperr.ErrAdminStatisticsInvalid.WithCause(err)
	}
	metrics["experiment_count"] = e.ExperimentCount
	metrics["active_instance_count"] = e.ActiveInstanceCount
	c, err := s.contest.Stats(ctx, tenantID)
	if err != nil {
		return apperr.ErrAdminStatisticsInvalid.WithCause(err)
	}
	metrics["contest_count"] = c.ContestCount
	metrics["active_contest_count"] = c.ActiveContestCount
	metrics["participant_count"] = c.ParticipantCount
	q, err := s.sandbox.Stats(ctx, tenantID)
	if err != nil {
		return apperr.ErrAdminStatisticsInvalid.WithCause(err)
	}
	metrics["active_sandbox_count"] = q.ActiveSandboxCount
	metrics["max_concurrent_sandbox"] = q.MaxConcurrentSandbox
	metrics["max_cpu"] = q.MaxCPU
	metrics["max_memory_mb"] = q.MaxMemoryMB
	return s.upsertStatistics(ctx, tenantID, ScopeTenant, tenantID, statDate, metrics)
}

// upsertStatistics 在正确的 RLS 边界内写入 M9 自有统计快照。
func (s *Service) upsertStatistics(ctx context.Context, txTenantID int64, scope int16, tenantID int64, statDate string, metrics map[string]any) error {
	run := func(ctx context.Context, tx TxStore) error {
		_, err := tx.UpsertPlatformStatistics(ctx, s.ids.Generate(), scope, tenantID, statDate, metrics)
		return err
	}
	var err error
	if txTenantID > 0 {
		err = s.store.TenantTx(ctx, txTenantID, run)
	} else {
		err = s.store.PlatformTx(ctx, run)
	}
	if err != nil {
		return apperr.ErrAdminStatisticsInvalid.WithCause(err)
	}
	return nil
}

// maskConfigs 返回配置列表前隐藏敏感字段。
func maskConfigs(rows []ConfigDTO) []ConfigDTO {
	out := make([]ConfigDTO, 0, len(rows))
	for _, row := range rows {
		row.Value = secretmap.Mask(row.Value)
		out = append(out, row)
	}
	return out
}

// maskConfigLogs 返回配置历史前隐藏敏感字段。
func maskConfigLogs(rows []ConfigChangeLogDTO) []ConfigChangeLogDTO {
	out := make([]ConfigChangeLogDTO, 0, len(rows))
	for _, row := range rows {
		row.OldValue = secretmap.Mask(row.OldValue)
		row.NewValue = secretmap.Mask(row.NewValue)
		out = append(out, row)
	}
	return out
}
