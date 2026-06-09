// M9 服务层:聚合下层 contracts,并管理 M9 自有运维元数据。
package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
	"chaimir/pkg/crypto"
	"chaimir/pkg/snowflake"
)

// adminStore 是 M9 服务层依赖的数据访问接口,便于服务规则测试。
type adminStore interface {
	ListStatistics(context.Context, int16, int64, time.Time, time.Time) ([]StatisticDTO, error)
	ListConfigs(context.Context, int16, int64) ([]ConfigDTO, error)
	GetConfig(context.Context, int16, int64, string) (ConfigDTO, error)
	UpdateConfig(context.Context, int64, int64, int64, ConfigDTO, map[string]any) (ConfigDTO, error)
	GetConfigHistory(context.Context, int64, int64) (ConfigChangeLogDTO, error)
	ListConfigHistory(context.Context, int64, int, int) ([]ConfigChangeLogDTO, int64, error)
	ListAlertRules(context.Context, int16, int64, int, int) ([]AlertRuleDTO, int64, error)
	CreateAlertRule(context.Context, int64, int64, AlertRuleRequest) (AlertRuleDTO, error)
	UpdateAlertRule(context.Context, int64, int64, AlertRulePatchRequest) (AlertRuleDTO, error)
	ListAlertEvents(context.Context, int64, int16, int, int) ([]AlertEventDTO, int64, error)
	GetAlertEvent(context.Context, int64, int64) (AlertEventDTO, error)
	HandleAlertEvent(context.Context, int64, int64, int64, int16) (AlertEventDTO, error)
	RevertAlertEvent(context.Context, int64, int64) error
	ListBackups(context.Context, int, int) ([]BackupRecordDTO, int64, error)
	CreateBackupRecord(context.Context, int64, BackupTriggerRequest) (BackupRecordDTO, error)
}

// Service 是 M9 管理后台服务。
type Service struct {
	store      adminStore
	idgen      snowflake.Generator
	auditor    audit.Writer
	cipher     *crypto.Cipher
	deploy     config.DeployConfig
	minio      config.MinIOConfig
	monitoring config.MonitoringConfig
	identity   contracts.IdentityAdminService
	sandbox    contracts.SandboxService
	teaching   contracts.TeachingService
	experiment contracts.ExperimentService
	contest    contracts.ContestService
	notify     contracts.NotifyService
}

// NewService 构造 M9 服务并注入下层只读 contracts。
func NewService(database *db.DB, idgen *snowflake.Node, auditor audit.Writer, cipher *crypto.Cipher, deploy config.DeployConfig, minio config.MinIOConfig, monitoring config.MonitoringConfig, identity contracts.IdentityAdminService, sandbox contracts.SandboxService, teaching contracts.TeachingService, experiment contracts.ExperimentService, contest contracts.ContestService, notify contracts.NotifyService) *Service {
	return &Service{
		store: newRepo(database), idgen: idgen, auditor: auditor, cipher: cipher, deploy: deploy, minio: minio, monitoring: monitoring,
		identity: identity, sandbox: sandbox, teaching: teaching, experiment: experiment, contest: contest, notify: notify,
	}
}

// PlatformDashboard 聚合全平台看板;仅 SaaS 平台层可用。
func (s *Service) PlatformDashboard(ctx context.Context) (DashboardDTO, error) {
	id, err := requirePlatform(ctx, s.deploy)
	if err != nil {
		return DashboardDTO{}, err
	}
	if s.identity == nil {
		return DashboardDTO{}, apperr.ErrAdminIdentityUnavailable
	}
	out, err := s.platformDashboard(ctx)
	if err != nil {
		return DashboardDTO{}, err
	}
	out.Scope = ScopeGlobal
	if err := s.writeAudit(ctx, 0, id.AccountID, "admin.dashboard.view", "admin.dashboard", 0, map[string]any{"scope": "platform"}); err != nil {
		return DashboardDTO{}, err
	}
	return out, nil
}

// platformDashboard 跨租户只读汇总下层模块统计,用于 SaaS 平台运营大盘。
func (s *Service) platformDashboard(ctx context.Context) (DashboardDTO, error) {
	if err := s.requireDashboardStatsContracts(); err != nil {
		return DashboardDTO{}, err
	}
	var out DashboardDTO
	identityStats, err := s.identity.Stats(ctx, 0)
	if err != nil {
		return DashboardDTO{}, apperr.ErrAdminDashboard.WithCause(err)
	}
	out.Identity = identityStats
	tenantIDs, err := s.platformTenantIDs(ctx)
	if err != nil {
		return DashboardDTO{}, err
	}
	for _, tenantID := range tenantIDs {
		if err := s.mergeTenantStats(ctx, &out, tenantID); err != nil {
			return DashboardDTO{}, err
		}
	}
	out.Scope = ScopeGlobal
	return out, nil
}

// SchoolDashboard 聚合当前学校看板。
func (s *Service) SchoolDashboard(ctx context.Context) (DashboardDTO, error) {
	id, err := s.requireSchoolAdmin(ctx)
	if err != nil {
		return DashboardDTO{}, err
	}
	out, err := s.dashboard(ctx, id.TenantID, ScopeTenant)
	if err != nil {
		return DashboardDTO{}, err
	}
	out.Scope = ScopeTenant
	out.TenantID = ids.Format(id.TenantID)
	return out, nil
}

// dashboard 并行契约由调用方可扩展;当前顺序聚合保证错误链清晰。
func (s *Service) dashboard(ctx context.Context, tenantID int64, scope int16) (DashboardDTO, error) {
	if err := s.requireDashboardStatsContracts(); err != nil {
		return DashboardDTO{}, err
	}
	var out DashboardDTO
	var err error
	out.Identity, err = s.identity.Stats(ctx, tenantID)
	if err != nil {
		return DashboardDTO{}, apperr.ErrAdminDashboard.WithCause(err)
	}
	if tenantID > 0 {
		out.Sandbox, err = s.sandbox.Stats(ctx, tenantID)
		if err != nil {
			return DashboardDTO{}, apperr.ErrAdminDashboard.WithCause(err)
		}
	}
	if tenantID > 0 {
		out.Teaching, err = s.teaching.Stats(ctx, tenantID)
		if err != nil {
			return DashboardDTO{}, apperr.ErrAdminDashboard.WithCause(err)
		}
	}
	if tenantID > 0 {
		out.Experiment, err = s.experiment.Stats(ctx, tenantID, 0)
		if err != nil {
			return DashboardDTO{}, apperr.ErrAdminDashboard.WithCause(err)
		}
	}
	if tenantID > 0 {
		out.Contest, err = s.contest.Stats(ctx, tenantID)
		if err != nil {
			return DashboardDTO{}, apperr.ErrAdminDashboard.WithCause(err)
		}
	}
	out.Scope = scope
	return out, nil
}

// platformTenantIDs 通过 M1 管理契约分页读取租户摘要,供平台看板跨校汇总。
func (s *Service) platformTenantIDs(ctx context.Context) ([]int64, error) {
	page, size := pagex.Normalize(1, 100)
	out := make([]int64, 0)
	for {
		rows, total, err := s.identity.AdminListTenants(ctx, 0, page, size)
		if err != nil {
			return nil, apperr.ErrAdminDashboard.WithCause(err)
		}
		for _, row := range rows {
			if row.ID > 0 {
				out = append(out, row.ID)
			}
		}
		if len(rows) < size || int64(len(out)) >= total {
			return out, nil
		}
		page++
	}
}

// mergeTenantStats 将单租户下层统计累加到平台看板,全程只经 contracts 读取。
func (s *Service) mergeTenantStats(ctx context.Context, out *DashboardDTO, tenantID int64) error {
	stats, err := s.sandbox.Stats(ctx, tenantID)
	if err != nil {
		return apperr.ErrAdminDashboard.WithCause(err)
	}
	out.Sandbox.ActiveSandboxCount += stats.ActiveSandboxCount
	out.Sandbox.MaxConcurrentSandbox += stats.MaxConcurrentSandbox
	out.Sandbox.MaxCPU += stats.MaxCPU
	out.Sandbox.MaxMemoryMB += stats.MaxMemoryMB
	out.Sandbox.IdleTimeoutMin = maxInt32(out.Sandbox.IdleTimeoutMin, stats.IdleTimeoutMin)
	out.Sandbox.MaxLifetimeMin = maxInt32(out.Sandbox.MaxLifetimeMin, stats.MaxLifetimeMin)
	out.Sandbox.MaxKeepaliveMin = maxInt32(out.Sandbox.MaxKeepaliveMin, stats.MaxKeepaliveMin)
	out.Sandbox.MaxSnapshotRetention = maxInt32(out.Sandbox.MaxSnapshotRetention, stats.MaxSnapshotRetention)

	teachingStats, err := s.teaching.Stats(ctx, tenantID)
	if err != nil {
		return apperr.ErrAdminDashboard.WithCause(err)
	}
	out.Teaching.CourseCount += teachingStats.CourseCount
	out.Teaching.ActiveCourseCount += teachingStats.ActiveCourseCount
	out.Teaching.LearningDurationSec += teachingStats.LearningDurationSec

	experimentStats, err := s.experiment.Stats(ctx, tenantID, 0)
	if err != nil {
		return apperr.ErrAdminDashboard.WithCause(err)
	}
	out.Experiment.ExperimentCount += experimentStats.ExperimentCount
	out.Experiment.ActiveInstanceCount += experimentStats.ActiveInstanceCount

	contestStats, err := s.contest.Stats(ctx, tenantID)
	if err != nil {
		return apperr.ErrAdminDashboard.WithCause(err)
	}
	out.Contest.ContestCount += contestStats.ContestCount
	out.Contest.ActiveContestCount += contestStats.ActiveContestCount
	out.Contest.TeamCount += contestStats.TeamCount
	return nil
}

// PlatformStatistics 查询平台周期统计快照。
func (s *Service) PlatformStatistics(ctx context.Context, from, to time.Time) ([]StatisticDTO, error) {
	if _, err := requirePlatform(ctx, s.deploy); err != nil {
		return nil, err
	}
	if err := validateStatisticsRange(from, to); err != nil {
		return nil, err
	}
	return s.store.ListStatistics(ctx, ScopeGlobal, 0, from, to)
}

// SchoolStatistics 查询当前学校周期统计快照。
func (s *Service) SchoolStatistics(ctx context.Context, from, to time.Time) ([]StatisticDTO, error) {
	id, err := s.requireSchoolAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if err := validateStatisticsRange(from, to); err != nil {
		return nil, err
	}
	return s.store.ListStatistics(ctx, ScopeTenant, id.TenantID, from, to)
}

// ListTenants 转发 M1 租户列表入口。
func (s *Service) ListTenants(ctx context.Context, status int16, page, size int) ([]contracts.TenantSummary, int64, error) {
	if _, err := requirePlatform(ctx, s.deploy); err != nil {
		return nil, 0, err
	}
	if s.identity == nil {
		return nil, 0, apperr.ErrAdminIdentityUnavailable
	}
	return s.identity.AdminListTenants(ctx, status, page, size)
}

// ListApplications 转发 M1 学校入驻申请列表入口。
func (s *Service) ListApplications(ctx context.Context, status int16, page, size int) ([]contracts.ApplicationSummary, int64, error) {
	if _, err := requirePlatform(ctx, s.deploy); err != nil {
		return nil, 0, err
	}
	if s.identity == nil {
		return nil, 0, apperr.ErrAdminIdentityUnavailable
	}
	return s.identity.AdminListApplications(ctx, status, page, size)
}

// ApproveApplication 转发 M1 审核通过业务并记录 M9 入口审计。
func (s *Service) ApproveApplication(ctx context.Context, applicationID int64, req ApplicationApproveRequest) (contracts.ApplicationApprovalResult, error) {
	id, err := requirePlatform(ctx, s.deploy)
	if err != nil {
		return contracts.ApplicationApprovalResult{}, err
	}
	if strings.TrimSpace(req.TenantCode) == "" || strings.TrimSpace(req.AdminPhone) == "" || strings.TrimSpace(req.AdminName) == "" {
		return contracts.ApplicationApprovalResult{}, apperr.ErrAdminApplicationInvalid
	}
	if s.identity == nil {
		return contracts.ApplicationApprovalResult{}, apperr.ErrAdminIdentityUnavailable
	}
	out, err := s.identity.AdminApproveApplication(ctx, contracts.ApplicationApproval{ApplicationID: applicationID, ReviewerID: id.AccountID, TenantCode: req.TenantCode, AdminPhone: req.AdminPhone, AdminName: req.AdminName})
	if err != nil {
		return contracts.ApplicationApprovalResult{}, err
	}
	return out, s.writeAudit(ctx, 0, id.AccountID, "admin.application.approve", "tenant_application", applicationID, map[string]any{"tenant_code": req.TenantCode})
}

// RejectApplication 转发 M1 审核驳回业务并记录 M9 入口审计。
func (s *Service) RejectApplication(ctx context.Context, applicationID int64, req ApplicationRejectRequest) error {
	id, err := requirePlatform(ctx, s.deploy)
	if err != nil {
		return err
	}
	if strings.TrimSpace(req.Reason) == "" {
		return apperr.ErrAdminApplicationInvalid
	}
	if s.identity == nil {
		return apperr.ErrAdminIdentityUnavailable
	}
	if err := s.identity.AdminRejectApplication(ctx, applicationID, id.AccountID, req.Reason); err != nil {
		return err
	}
	return s.writeAudit(ctx, 0, id.AccountID, "admin.application.reject", "tenant_application", applicationID, map[string]any{"reason_recorded": true})
}

// ListAudit 查询统一审计中心。
func (s *Service) ListAudit(ctx context.Context, query contracts.AuditQuery, page, size int) ([]contracts.AuditRecord, int64, error) {
	if err := requireAdmin(ctx, s.deploy); err != nil {
		return nil, 0, err
	}
	if s.identity == nil {
		return nil, 0, apperr.ErrAdminIdentityUnavailable
	}
	return s.identity.ListAuditRecords(ctx, query, page, size)
}

// ExportAudit 查询审计导出数据并写导出审计。
func (s *Service) ExportAudit(ctx context.Context, query contracts.AuditQuery) ([]contracts.AuditRecord, error) {
	id, err := currentAdmin(ctx, s.deploy)
	if err != nil {
		return nil, err
	}
	if s.identity == nil {
		return nil, apperr.ErrAdminIdentityUnavailable
	}
	rows, _, err := s.identity.ListAuditRecords(ctx, query, 1, 10000)
	if err != nil {
		return nil, err
	}
	tenantID := id.TenantID
	if id.IsPlatform {
		tenantID = 0
	}
	return rows, s.writeAudit(ctx, tenantID, id.AccountID, "admin.audit.export", "audit_log", 0, map[string]any{"count": len(rows)})
}

// ListConfigs 查询系统配置,按管理员权限收敛 scope。
func (s *Service) ListConfigs(ctx context.Context, scope int16) ([]ConfigDTO, error) {
	id, resolved, err := s.resolveScope(ctx, scope)
	if err != nil {
		return nil, err
	}
	rows, err := s.store.ListConfigs(ctx, resolved, id.TenantID)
	if err != nil {
		return nil, err
	}
	return maskConfigs(rows), nil
}

// UpdateConfig 使用乐观锁更新配置并写变更历史。
func (s *Service) UpdateConfig(ctx context.Context, key string, req ConfigUpdateRequest) (ConfigDTO, error) {
	id, scope, err := s.resolveScope(ctx, req.Scope)
	if err != nil {
		return ConfigDTO{}, err
	}
	if strings.TrimSpace(key) == "" || req.Version <= 0 {
		return ConfigDTO{}, apperr.ErrAdminConfigInvalid
	}
	current, err := s.store.GetConfig(ctx, scope, id.TenantID, key)
	if err != nil {
		return ConfigDTO{}, err
	}
	if current.Version != req.Version {
		return ConfigDTO{}, apperr.ErrAdminConfigConflict
	}
	protected, err := protectConfigValue(s.cipher, req.Value)
	if err != nil {
		return ConfigDTO{}, apperr.ErrAdminConfigInvalid.WithCause(err)
	}
	updated, err := s.store.UpdateConfig(ctx, ids.ParseOrZero(current.ID), s.nextID(), id.AccountID, current, protected)
	if err != nil {
		return ConfigDTO{}, err
	}
	return maskConfig(updated), s.writeAudit(ctx, auditTenantID(id, scope), id.AccountID, "admin.config.update", "system_config", ids.ParseOrZero(updated.ID), map[string]any{"key": key, "scope": scope})
}

// ConfigHistory 查询配置变更历史。
func (s *Service) ConfigHistory(ctx context.Context, key string, scope int16, page, size int) ([]ConfigChangeLogDTO, int64, error) {
	id, resolved, err := s.resolveScope(ctx, scope)
	if err != nil {
		return nil, 0, err
	}
	cfg, err := s.store.GetConfig(ctx, resolved, id.TenantID, key)
	if err != nil {
		return nil, 0, err
	}
	rows, total, err := s.store.ListConfigHistory(ctx, ids.ParseOrZero(cfg.ID), page, size)
	if err != nil {
		return nil, 0, err
	}
	return maskConfigHistory(rows), total, nil
}

// RollbackConfig 使用历史记录回退配置值,并复用配置更新的乐观锁与变更留痕。
func (s *Service) RollbackConfig(ctx context.Context, key string, req ConfigRollbackRequest) (ConfigDTO, error) {
	id, scope, err := s.resolveScope(ctx, req.Scope)
	if err != nil {
		return ConfigDTO{}, err
	}
	historyID, ok := ids.Parse(req.HistoryID)
	if strings.TrimSpace(key) == "" || !ok || req.Version <= 0 {
		return ConfigDTO{}, apperr.ErrAdminConfigInvalid
	}
	current, err := s.store.GetConfig(ctx, scope, id.TenantID, key)
	if err != nil {
		return ConfigDTO{}, err
	}
	if current.Version != req.Version {
		return ConfigDTO{}, apperr.ErrAdminConfigConflict
	}
	history, err := s.store.GetConfigHistory(ctx, ids.ParseOrZero(current.ID), historyID)
	if err != nil {
		return ConfigDTO{}, err
	}
	updated, err := s.store.UpdateConfig(ctx, ids.ParseOrZero(current.ID), s.nextID(), id.AccountID, current, history.OldValue)
	if err != nil {
		return ConfigDTO{}, err
	}
	return maskConfig(updated), s.writeAudit(ctx, auditTenantID(id, scope), id.AccountID, "admin.config.rollback", "system_config", ids.ParseOrZero(updated.ID), map[string]any{"key": key, "history_id": req.HistoryID})
}

// ListAlertRules 查询告警规则。
func (s *Service) ListAlertRules(ctx context.Context, scope int16, page, size int) ([]AlertRuleDTO, int64, error) {
	id, resolved, err := s.resolveScope(ctx, scope)
	if err != nil {
		return nil, 0, err
	}
	return s.store.ListAlertRules(ctx, resolved, id.TenantID, page, size)
}

// CreateAlertRule 创建业务级告警规则。
func (s *Service) CreateAlertRule(ctx context.Context, req AlertRuleRequest) (AlertRuleDTO, error) {
	id, scope, err := s.resolveScope(ctx, req.Scope)
	if err != nil {
		return AlertRuleDTO{}, err
	}
	if err := validateAlertRule(req.Name, req.Metric, req.Level); err != nil {
		return AlertRuleDTO{}, err
	}
	req.Scope = scope
	out, err := s.store.CreateAlertRule(ctx, s.nextID(), id.TenantID, req)
	if err != nil {
		return AlertRuleDTO{}, err
	}
	return out, s.writeAudit(ctx, auditTenantID(id, scope), id.AccountID, "admin.alert_rule.create", "alert_rule", ids.ParseOrZero(out.ID), map[string]any{"metric": req.Metric})
}

// UpdateAlertRule 更新业务级告警规则。
func (s *Service) UpdateAlertRule(ctx context.Context, ruleID int64, req AlertRulePatchRequest) (AlertRuleDTO, error) {
	id, err := currentAdmin(ctx, s.deploy)
	if err != nil {
		return AlertRuleDTO{}, err
	}
	if err := validateAlertRule(req.Name, req.Metric, req.Level); err != nil {
		return AlertRuleDTO{}, err
	}
	scopeTenantID := tenantScopeID(id)
	out, err := s.store.UpdateAlertRule(ctx, scopeTenantID, ruleID, req)
	if err != nil {
		return AlertRuleDTO{}, err
	}
	return out, s.writeAudit(ctx, auditTenantIDForIdentity(id), id.AccountID, "admin.alert_rule.update", "alert_rule", ruleID, map[string]any{"enabled": req.Enabled})
}

// ListAlertEvents 查询业务级告警事件。
func (s *Service) ListAlertEvents(ctx context.Context, status int16, page, size int) ([]AlertEventDTO, int64, error) {
	id, err := currentAdmin(ctx, s.deploy)
	if err != nil {
		return nil, 0, err
	}
	return s.store.ListAlertEvents(ctx, tenantScopeID(id), status, page, size)
}

// HandleAlertEvent 处理或忽略待处理告警事件,并经 M10 通知相关管理员。
func (s *Service) HandleAlertEvent(ctx context.Context, eventID int64, req AlertHandleRequest) (AlertEventDTO, error) {
	id, err := currentAdmin(ctx, s.deploy)
	if err != nil {
		return AlertEventDTO{}, err
	}
	if req.Status != AlertEventHandled && req.Status != AlertEventIgnored {
		return AlertEventDTO{}, apperr.ErrAdminAlertState
	}
	scopeTenantID := tenantScopeID(id)
	current, err := s.store.GetAlertEvent(ctx, scopeTenantID, eventID)
	if err != nil {
		return AlertEventDTO{}, err
	}
	if current.Status != AlertEventPending {
		return AlertEventDTO{}, apperr.ErrAdminAlertState
	}
	out, err := s.store.HandleAlertEvent(ctx, scopeTenantID, eventID, id.AccountID, req.Status)
	if err != nil {
		return AlertEventDTO{}, err
	}
	if s.notify == nil {
		if rollbackErr := s.store.RevertAlertEvent(ctx, scopeTenantID, eventID); rollbackErr != nil {
			return AlertEventDTO{}, apperr.ErrAdminAlertNotifyFailed.WithCause(rollbackErr)
		}
		return AlertEventDTO{}, apperr.ErrAdminAlertNotifyFailed
	}
	if err := s.notify.Send(ctx, contracts.NotifySendRequest{
		TenantID:  id.TenantID,
		Type:      "admin.alert.handled",
		Receivers: []int64{id.AccountID},
		Params:    map[string]string{"message": out.Message},
	}); err != nil {
		if rollbackErr := s.store.RevertAlertEvent(ctx, scopeTenantID, eventID); rollbackErr != nil {
			return AlertEventDTO{}, apperr.ErrAdminAlertNotifyFailed.WithCause(rollbackErr)
		}
		return AlertEventDTO{}, apperr.ErrAdminAlertNotifyFailed.WithCause(err)
	}
	return out, s.writeAudit(ctx, auditTenantIDForIdentity(id), id.AccountID, "admin.alert_event.handle", "alert_event", eventID, map[string]any{"status": req.Status})
}

// MonitoringPanels 返回外接监控面板嵌入入口。
func (s *Service) MonitoringPanels(ctx context.Context) ([]MonitoringPanelDTO, error) {
	if _, err := requirePlatform(ctx, s.deploy); err != nil {
		return nil, err
	}
	return parseMonitoringPanels(s.monitoring.PanelsJSON)
}

// ListBackups 查询备份记录。
func (s *Service) ListBackups(ctx context.Context, page, size int) ([]BackupRecordDTO, int64, error) {
	if _, err := requirePlatform(ctx, s.deploy); err != nil {
		return nil, 0, err
	}
	return s.store.ListBackups(ctx, page, size)
}

// TriggerBackup 记录一次备份触发请求;实际执行由 CronJob 或运维系统完成。
func (s *Service) TriggerBackup(ctx context.Context, req BackupTriggerRequest) (BackupRecordDTO, error) {
	id, err := requirePlatform(ctx, s.deploy)
	if err != nil {
		return BackupRecordDTO{}, err
	}
	if req.Type != BackupTypeFull && req.Type != BackupTypeIncremental {
		return BackupRecordDTO{}, apperr.ErrAdminBackupInvalid
	}
	recordID := s.nextID()
	backupRef, err := s.backupObjectRef(recordID)
	if err != nil {
		return BackupRecordDTO{}, err
	}
	req.StorageRef = backupRef
	out, err := s.store.CreateBackupRecord(ctx, recordID, req)
	if err != nil {
		return BackupRecordDTO{}, err
	}
	return out, s.writeAudit(ctx, 0, id.AccountID, "admin.backup.trigger", "backup_record", ids.ParseOrZero(out.ID), map[string]any{"type": req.Type})
}

// resolveScope 根据管理员身份解析配置/告警 scope,禁止学校管理员访问全局 scope。
func (s *Service) resolveScope(ctx context.Context, requested int16) (tenant.Identity, int16, error) {
	id, err := currentAdmin(ctx, s.deploy)
	if err != nil {
		return tenant.Identity{}, 0, err
	}
	if !id.IsPlatform {
		verified, err := s.requireSchoolAdmin(ctx)
		if err != nil {
			return tenant.Identity{}, 0, err
		}
		id = verified
	}
	if requested == 0 {
		if id.IsPlatform {
			return id, ScopeGlobal, nil
		}
		return id, ScopeTenant, nil
	}
	if requested == ScopeGlobal && !id.IsPlatform {
		return tenant.Identity{}, 0, apperr.ErrForbidden
	}
	if requested != ScopeGlobal && requested != ScopeTenant {
		return tenant.Identity{}, 0, apperr.ErrAdminConfigInvalid
	}
	return id, requested, nil
}

// nextID 生成 M9 自有表主键。
func (s *Service) nextID() int64 {
	return s.idgen.Generate()
}

// writeAudit 通过平台审计 Writer 追加 M9 高权限操作审计。
func (s *Service) writeAudit(ctx context.Context, tenantID, actorID int64, action, targetType string, targetID int64, detail map[string]any) error {
	if s.auditor == nil {
		return apperr.ErrAdminAuditWriteFailed
	}
	actorRole := audit.ActorRoleSchoolAdmin
	if tenantID == 0 {
		actorRole = audit.ActorRolePlatformAdmin
	}
	entry, err := audit.BuildEntry(ctx, tenantID, actorID, actorRole, action, targetType, targetID, detail)
	if err != nil {
		return apperr.ErrAdminAuditWriteFailed.WithCause(err)
	}
	if err := s.auditor.Write(ctx, entry); err != nil {
		return apperr.ErrAdminAuditWriteFailed.WithCause(err)
	}
	return nil
}

// requireDashboardStatsContracts 确认 M9 看板聚合所需下层只读契约完整注入。
func (s *Service) requireDashboardStatsContracts() error {
	if s.identity == nil {
		return apperr.ErrAdminIdentityUnavailable
	}
	if s.sandbox == nil {
		return dashboardDependencyError("sandbox")
	}
	if s.teaching == nil {
		return dashboardDependencyError("teaching")
	}
	if s.experiment == nil {
		return dashboardDependencyError("experiment")
	}
	if s.contest == nil {
		return dashboardDependencyError("contest")
	}
	return nil
}

// dashboardDependencyError 保留缺失依赖名到错误链,响应仍只暴露用户向文案。
func dashboardDependencyError(name string) error {
	return apperr.ErrAdminDashboard.WithCause(fmt.Errorf("admin dashboard missing %s stats contract", name))
}

// currentAdmin 读取当前管理员身份并校验平台层开关。
func currentAdmin(ctx context.Context, deploy config.DeployConfig) (tenant.Identity, error) {
	id, ok := tenant.FromContext(ctx)
	if !ok {
		return tenant.Identity{}, apperr.ErrUnauthorized
	}
	if id.IsPlatform && !deploy.PlatformEnabled {
		return tenant.Identity{}, apperr.ErrForbidden
	}
	return id, nil
}

// requirePlatform 要求 SaaS 平台管理员身份。
func requirePlatform(ctx context.Context, deploy config.DeployConfig) (tenant.Identity, error) {
	id, err := currentAdmin(ctx, deploy)
	if err != nil {
		return tenant.Identity{}, err
	}
	if !id.IsPlatform {
		return tenant.Identity{}, apperr.ErrForbidden
	}
	return id, nil
}

// requireSchoolAdmin 要求租户内学校管理员上下文,并通过 M1 角色契约做 service 层授权确认。
func (s *Service) requireSchoolAdmin(ctx context.Context) (tenant.Identity, error) {
	id, ok := tenant.FromContext(ctx)
	if !ok {
		return tenant.Identity{}, apperr.ErrUnauthorized
	}
	if id.IsPlatform || id.TenantID <= 0 {
		return tenant.Identity{}, apperr.ErrForbidden
	}
	if s.identity == nil {
		return tenant.Identity{}, apperr.ErrAdminIdentityUnavailable
	}
	has, err := s.identity.HasRole(ctx, id.AccountID, contracts.RoleSchoolAdmin)
	if err != nil {
		return tenant.Identity{}, apperr.ErrForbidden.WithCause(err)
	}
	if !has {
		return tenant.Identity{}, apperr.ErrForbidden
	}
	return id, nil
}

// requireAdmin 要求平台管理员或学校管理员身份。
func requireAdmin(ctx context.Context, deploy config.DeployConfig) error {
	_, err := currentAdmin(ctx, deploy)
	return err
}

// validateAlertRule 校验告警规则基本字段和等级。
func validateAlertRule(name, metric string, level int16) error {
	if strings.TrimSpace(name) == "" || strings.TrimSpace(metric) == "" {
		return apperr.ErrAdminAlertInvalid
	}
	if level < AlertLevelInfo || level > AlertLevelUrgent {
		return apperr.ErrAdminAlertInvalid
	}
	return nil
}

// validateStatisticsRange 要求调用方显式给出统计日期范围,避免查询边界隐藏成当前日期。
func validateStatisticsRange(from, to time.Time) error {
	if from.IsZero() || to.IsZero() || to.Before(from) {
		return apperr.ErrAdminStatisticsQueryInvalid
	}
	return nil
}

// backupObjectRef 生成后端受控的备份对象引用,不接受客户端传入存储位置。
func (s *Service) backupObjectRef(recordID int64) (string, error) {
	if strings.TrimSpace(s.minio.BucketBackup) == "" {
		return "", apperr.ErrAdminBackupInvalid
	}
	key, err := storage.ObjectKey(0, "admin", "backup", ids.Format(recordID)+".dump")
	if err != nil {
		return "", apperr.ErrAdminBackupInvalid.WithCause(err)
	}
	return "minio://" + s.minio.BucketBackup + "/" + key, nil
}

// auditTenantID 根据 scope 返回审计租户范围。
func auditTenantID(id tenant.Identity, scope int16) int64 {
	if scope == ScopeGlobal {
		return 0
	}
	return id.TenantID
}

// auditTenantIDForIdentity 根据身份返回审计租户范围。
func auditTenantIDForIdentity(id tenant.Identity) int64 {
	if id.IsPlatform {
		return 0
	}
	return id.TenantID
}

// tenantScopeID 返回 repo 过滤使用的租户范围;平台管理员为 0 表示平台级全局范围。
func tenantScopeID(id tenant.Identity) int64 {
	if id.IsPlatform {
		return 0
	}
	return id.TenantID
}

// maxInt32 返回两个 int32 中较大的值,用于平台看板展示跨租户资源上限。
func maxInt32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

// parseMonitoringPanels 从环境配置 JSON 解析外接监控面板。
func parseMonitoringPanels(raw string) ([]MonitoringPanelDTO, error) {
	if strings.TrimSpace(raw) == "" {
		return []MonitoringPanelDTO{}, nil
	}
	var out []MonitoringPanelDTO
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, apperr.ErrAdminMonitoringInvalid.WithCause(err)
	}
	for _, panel := range out {
		if strings.TrimSpace(panel.Key) == "" || strings.TrimSpace(panel.Name) == "" || !validMonitoringPanelURL(panel.URL) {
			return nil, apperr.ErrAdminMonitoringInvalid
		}
	}
	return out, nil
}

// validMonitoringPanelURL 校验外接监控嵌入地址,避免把凭据、查询串或非 HTTPS 地址下发给前端。
func validMonitoringPanelURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	return u.Scheme == "https" && u.Host != "" && u.User == nil && u.RawQuery == "" && u.Fragment == ""
}
