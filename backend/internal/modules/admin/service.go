// admin service 文件实现 M9 管理后台聚合、配置、告警和备份业务编排。
package admin

import (
	"context"
	"encoding/csv"
	"fmt"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/snowflake"
)

// Service 承载 M9 管理后台业务编排。
type Service struct {
	store      Store
	ids        snowflake.Generator
	audit      audit.Writer
	roles      roleReader
	identity   contracts.IdentityTenantReadService
	stats      contracts.IdentityStatsService
	auditRead  contracts.IdentityAuditReadService
	teaching   contracts.TeachingReadService
	sandbox    contracts.SandboxService
	experiment contracts.ExperimentReadService
	contest    contracts.ContestReadService
	notify     contracts.NotifyService
	monitoring config.MonitoringConfig
}

// ServiceDeps 是 M9 服务装配依赖。
type ServiceDeps struct {
	Store      Store
	IDs        snowflake.Generator
	Audit      audit.Writer
	Roles      roleReader
	Identity   contracts.IdentityTenantReadService
	Stats      contracts.IdentityStatsService
	AuditRead  contracts.IdentityAuditReadService
	Teaching   contracts.TeachingReadService
	Sandbox    contracts.SandboxService
	Experiment contracts.ExperimentReadService
	Contest    contracts.ContestReadService
	Notify     contracts.NotifyService
	Monitoring config.MonitoringConfig
}

// NewService 构造 M9 服务。
func NewService(deps ServiceDeps) (*Service, error) {
	if deps.Store == nil || deps.IDs == nil || deps.Audit == nil || deps.Roles == nil || deps.Identity == nil || deps.Stats == nil || deps.AuditRead == nil {
		return nil, fmt.Errorf("admin service 依赖不完整")
	}
	return &Service{store: deps.Store, ids: deps.IDs, audit: deps.Audit, roles: deps.Roles, identity: deps.Identity, stats: deps.Stats, auditRead: deps.AuditRead, teaching: deps.Teaching, sandbox: deps.Sandbox, experiment: deps.Experiment, contest: deps.Contest, notify: deps.Notify, monitoring: deps.Monitoring}, nil
}

// PlatformDashboard 聚合平台级看板。
func (s *Service) PlatformDashboard(ctx context.Context) (DashboardDTO, error) {
	if _, err := requirePlatform(ctx); err != nil {
		return DashboardDTO{}, err
	}
	stats, err := s.stats.PlatformStats(ctx)
	if err != nil {
		return DashboardDTO{}, apperr.ErrAdminDashboardInvalid.WithCause(err)
	}
	return DashboardDTO{Scope: ScopeGlobal, TenantCount: stats.TenantCount, AccountCount: stats.AccountCount, TeacherCount: stats.TeacherCount, StudentCount: stats.StudentCount, ActiveAccountCount: stats.ActiveAccountCount, PendingApplyCount: stats.PendingApplyCount, GeneratedAt: timex.Now()}, nil
}

// SchoolDashboard 聚合学校级看板。
func (s *Service) SchoolDashboard(ctx context.Context) (DashboardDTO, error) {
	id, err := s.requireTenantAdmin(ctx)
	if err != nil {
		return DashboardDTO{}, err
	}
	stats, err := s.stats.TenantStats(ctx, id.TenantID)
	if err != nil {
		return DashboardDTO{}, apperr.ErrAdminDashboardInvalid.WithCause(err)
	}
	out := DashboardDTO{Scope: ScopeTenant, TenantID: id.TenantID, AccountCount: stats.AccountCount, TeacherCount: stats.TeacherCount, StudentCount: stats.StudentCount, ActiveAccountCount: stats.ActiveAccountCount, GeneratedAt: timex.Now()}
	if s.teaching != nil {
		t, err := s.teaching.Stats(ctx, id.TenantID)
		if err != nil {
			return DashboardDTO{}, apperr.ErrAdminDashboardInvalid.WithCause(err)
		}
		out.CourseCount = t.CourseCount
		out.ActiveCourseCount = t.ActiveCourseCount
	}
	if s.experiment != nil {
		e, err := s.experiment.Stats(ctx, contracts.ExperimentStatsQuery{TenantID: id.TenantID})
		if err != nil {
			return DashboardDTO{}, apperr.ErrAdminDashboardInvalid.WithCause(err)
		}
		out.ExperimentCount = e.ExperimentCount
		out.ActiveInstanceCount = e.ActiveInstanceCount
	}
	if s.contest != nil {
		c, err := s.contest.Stats(ctx, id.TenantID)
		if err != nil {
			return DashboardDTO{}, apperr.ErrAdminDashboardInvalid.WithCause(err)
		}
		out.ContestCount = c.ContestCount
		out.ActiveContestCount = c.ActiveContestCount
	}
	if s.sandbox != nil {
		q, err := s.sandbox.Stats(ctx, id.TenantID)
		if err != nil {
			return DashboardDTO{}, apperr.ErrAdminDashboardInvalid.WithCause(err)
		}
		out.ActiveSandboxCount = q.ActiveSandboxCount
		out.ResourceQuotaSnapshot = map[string]any{"max_concurrent_sandbox": q.MaxConcurrentSandbox, "max_cpu": q.MaxCPU, "max_memory_mb": q.MaxMemoryMB}
	}
	return out, nil
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
	}
	result, err := s.auditRead.QueryAuditLogs(ctx, query)
	if err != nil {
		return contracts.AuditQueryResult{}, apperr.ErrAdminAuditQueryInvalid.WithCause(err)
	}
	return result, nil
}

// ExportAuditCSV 导出审计日志 CSV。
func (s *Service) ExportAuditCSV(ctx context.Context, query contracts.AuditQuery) ([]byte, error) {
	query.Page = 1
	query.Size = 1000
	result, err := s.QueryAudit(ctx, query)
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	w := csv.NewWriter(&b)
	if err := w.Write([]string{"id", "tenant_id", "actor_id", "action", "target_type", "target_id", "trace_id", "created_at"}); err != nil {
		return nil, apperr.ErrAdminAuditExportFailed.WithCause(err)
	}
	for _, row := range result.List {
		if err := w.Write([]string{fmt.Sprint(row.ID), fmt.Sprint(row.TenantID), fmt.Sprint(row.ActorID), row.Action, row.TargetType, fmt.Sprint(row.TargetID), row.TraceID, row.CreatedAt.Format(time.RFC3339)}); err != nil {
			return nil, apperr.ErrAdminAuditExportFailed.WithCause(err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, apperr.ErrAdminAuditExportFailed.WithCause(err)
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
	if !id.IsPlatform {
		scope = ScopeTenant
		tenantID = id.TenantID
	}
	return runAdminRead(ctx, s.store, tenantID, func(ctx context.Context, tx TxStore) ([]ConfigDTO, error) {
		return tx.ListSystemConfigs(ctx, scope, tenantID)
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
		req.TenantID = id.TenantID
	}
	if err := validateScopeTenant(req.Scope, req.TenantID); err != nil {
		return ConfigDTO{}, err
	}
	var out ConfigDTO
	err = s.runWrite(ctx, req.TenantID, func(ctx context.Context, tx TxStore) error {
		old, err := tx.GetSystemConfig(ctx, req.Scope, req.TenantID, key)
		if err != nil && !isNoRows(err) {
			return apperr.ErrAdminConfigInvalid.WithCause(err)
		}
		if isNoRows(err) {
			out, err = tx.CreateSystemConfig(ctx, s.ids.Generate(), req.Scope, req.TenantID, key, req.Value, id.AccountID)
			if err != nil {
				return apperr.ErrAdminConfigInvalid.WithCause(err)
			}
			_, err = tx.CreateConfigChangeLog(ctx, s.ids.Generate(), out.ID, req.TenantID, map[string]any{}, out.Value, id.AccountID)
			return err
		}
		out, err = tx.UpdateSystemConfig(ctx, req.Scope, req.TenantID, key, req.Value, id.AccountID, req.Version)
		if err != nil {
			return apperr.ErrAdminConfigConflict.WithCause(err)
		}
		_, err = tx.CreateConfigChangeLog(ctx, s.ids.Generate(), out.ID, req.TenantID, old.Value, out.Value, id.AccountID)
		return err
	})
	if err != nil {
		return ConfigDTO{}, err
	}
	if err := s.writeAudit(ctx, id, "admin.config.update", "system_config", out.ID, map[string]any{"key": key}); err != nil {
		return ConfigDTO{}, apperr.ErrAdminAuditWriteFailed.WithCause(err)
	}
	return out, nil
}

// ListConfigHistory 查询配置历史。
func (s *Service) ListConfigHistory(ctx context.Context, scope int16, tenantID int64, key string, page, size int) ([]ConfigChangeLogDTO, error) {
	id, err := s.currentAdminIdentity(ctx)
	if err != nil {
		return nil, err
	}
	if !id.IsPlatform {
		scope = ScopeTenant
		tenantID = id.TenantID
	}
	return runAdminRead(ctx, s.store, tenantID, func(ctx context.Context, tx TxStore) ([]ConfigChangeLogDTO, error) {
		cfg, err := tx.GetSystemConfig(ctx, scope, tenantID, key)
		if err != nil {
			return nil, apperr.ErrAdminConfigNotFound.WithCause(err)
		}
		return tx.ListConfigChangeLogs(ctx, cfg.ID, page, size)
	})
}

// RollbackConfig 把配置回退到指定历史记录的变更前值。
func (s *Service) RollbackConfig(ctx context.Context, key string, req ConfigUpdateRequest) (ConfigDTO, error) {
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
		req.TenantID = id.TenantID
	}
	if err := validateScopeTenant(req.Scope, req.TenantID); err != nil {
		return ConfigDTO{}, err
	}
	var out ConfigDTO
	err = s.runWrite(ctx, req.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetSystemConfig(ctx, req.Scope, req.TenantID, key)
		if err != nil {
			return apperr.ErrAdminConfigNotFound.WithCause(err)
		}
		history, err := tx.GetConfigChangeLog(ctx, req.ChangeLogID, current.ID)
		if err != nil {
			return apperr.ErrAdminConfigNotFound.WithCause(err)
		}
		out, err = tx.UpdateSystemConfig(ctx, req.Scope, req.TenantID, key, history.OldValue, id.AccountID, req.Version)
		if err != nil {
			return apperr.ErrAdminConfigConflict.WithCause(err)
		}
		_, err = tx.CreateConfigChangeLog(ctx, s.ids.Generate(), out.ID, req.TenantID, current.Value, out.Value, id.AccountID)
		return err
	})
	if err != nil {
		return ConfigDTO{}, err
	}
	if err := s.writeAudit(ctx, id, "admin.config.rollback", "system_config", out.ID, map[string]any{"key": key, "change_log_id": req.ChangeLogID}); err != nil {
		return ConfigDTO{}, apperr.ErrAdminAuditWriteFailed.WithCause(err)
	}
	return out, nil
}

// ListAlertRules 查询告警规则。
func (s *Service) ListAlertRules(ctx context.Context, scope int16) ([]AlertRuleDTO, error) {
	id, err := s.currentAdminIdentity(ctx)
	if err != nil {
		return nil, err
	}
	tenantID := int64(0)
	if !id.IsPlatform {
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
		req.TenantID = id.TenantID
	}
	if err := validateAlertRule(req); err != nil {
		return AlertRuleDTO{}, err
	}
	var out AlertRuleDTO
	err = s.runWrite(ctx, req.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.CreateAlertRule(ctx, s.ids.Generate(), req)
		return err
	})
	if err != nil {
		return AlertRuleDTO{}, apperr.ErrAdminAlertInvalid.WithCause(err)
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
		req.TenantID = id.TenantID
	}
	if err := validateAlertRule(req); err != nil {
		return AlertRuleDTO{}, err
	}
	var out AlertRuleDTO
	err = s.runWrite(ctx, req.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.UpdateAlertRule(ctx, ruleID, req)
		return err
	})
	if err != nil {
		return AlertRuleDTO{}, apperr.ErrAdminAlertNotFound.WithCause(err)
	}
	return out, nil
}

// ListAlertEvents 查询告警事件。
func (s *Service) ListAlertEvents(ctx context.Context, status int16, page, size int) ([]AlertEventDTO, error) {
	id, err := s.currentAdminIdentity(ctx)
	if err != nil {
		return nil, err
	}
	tenantID := int64(0)
	if !id.IsPlatform {
		tenantID = id.TenantID
	}
	return runAdminRead(ctx, s.store, tenantID, func(ctx context.Context, tx TxStore) ([]AlertEventDTO, error) {
		return tx.ListAlertEvents(ctx, status, tenantID, page, size)
	})
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
		out, err = tx.HandleAlertEvent(ctx, eventID, req.Status, id.AccountID)
		return err
	})
	if err != nil {
		return AlertEventDTO{}, apperr.ErrAdminAlertNotFound.WithCause(err)
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
func (s *Service) ListBackups(ctx context.Context, page, size int) ([]BackupRecordDTO, error) {
	if _, err := requirePlatform(ctx); err != nil {
		return nil, err
	}
	return runAdminRead(ctx, s.store, 0, func(ctx context.Context, tx TxStore) ([]BackupRecordDTO, error) {
		return tx.ListBackupRecords(ctx, page, size)
	})
}

// TriggerBackup 创建手工备份记录。
func (s *Service) TriggerBackup(ctx context.Context, req BackupTriggerRequest) (BackupRecordDTO, error) {
	id, err := requirePlatform(ctx)
	if err != nil {
		return BackupRecordDTO{}, err
	}
	if req.Type != BackupTypeFull && req.Type != BackupTypeIncremental {
		return BackupRecordDTO{}, apperr.ErrAdminBackupInvalid
	}
	var out BackupRecordDTO
	err = s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		ref := fmt.Sprintf("backup://manual/%d", s.ids.Generate())
		out, err = tx.CreateBackupRecord(ctx, s.ids.Generate(), req.Type, ref, 0, 2)
		return err
	})
	if err != nil {
		return BackupRecordDTO{}, apperr.ErrAdminBackupInvalid.WithCause(err)
	}
	if err := s.writeAudit(ctx, id, "admin.backup.trigger", "backup_record", out.ID, map[string]any{"type": req.Type}); err != nil {
		return BackupRecordDTO{}, apperr.ErrAdminAuditWriteFailed.WithCause(err)
	}
	return out, nil
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
	if err := validateScopeTenant(req.Scope, req.TenantID); err != nil {
		return apperr.ErrAdminAlertInvalid
	}
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Metric) == "" || req.Level < 1 || req.Level > 4 {
		return apperr.ErrAdminAlertInvalid
	}
	return nil
}

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
