// M9 服务测试:覆盖管理后台聚合、配置乐观锁和告警状态机。
package admin

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
)

// TestSchoolDashboardAggregatesLowerModuleStats 确认学校看板只经 contracts 聚合下层统计。
func TestSchoolDashboardAggregatesLowerModuleStats(t *testing.T) {
	store := &fakeAdminStore{}
	svc := newTestService(store)
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 2001})

	out, err := svc.SchoolDashboard(ctx)
	if err != nil {
		t.Fatalf("SchoolDashboard returned error: %v", err)
	}
	if out.Identity.StudentCount != 42 || out.Teaching.CourseCount != 5 || out.Sandbox.ActiveSandboxCount != 3 ||
		out.Experiment.ActiveInstanceCount != 7 || out.Contest.ActiveContestCount != 2 {
		t.Fatalf("dashboard did not aggregate expected stats: %#v", out)
	}
	if store.writeCount != 0 {
		t.Fatalf("dashboard must not write admin tables, writes=%d", store.writeCount)
	}
}

// TestPlatformDashboardAggregatesLowerModuleStatsAcrossTenants 确认平台看板按租户列表跨校只读汇总资源、教学、实验和竞赛统计。
func TestPlatformDashboardAggregatesLowerModuleStatsAcrossTenants(t *testing.T) {
	store := &fakeAdminStore{}
	svc := newTestService(store)
	svc.deploy.PlatformEnabled = true
	svc.identity = &fakeIdentityAdmin{
		stats: contracts.IdentityStats{TenantCount: 2, AccountCount: 100, TeacherCount: 10, StudentCount: 90},
		tenants: []contracts.TenantSummary{
			{ID: 1001, Status: 1},
			{ID: 1002, Status: 1},
		},
	}
	ctx := tenant.WithContext(context.Background(), tenant.Identity{IsPlatform: true, AccountID: 9001})

	out, err := svc.PlatformDashboard(ctx)
	if err != nil {
		t.Fatalf("PlatformDashboard returned error: %v", err)
	}
	if out.Teaching.CourseCount != 10 || out.Sandbox.ActiveSandboxCount != 6 ||
		out.Experiment.ActiveInstanceCount != 14 || out.Contest.ActiveContestCount != 4 {
		t.Fatalf("platform dashboard did not aggregate lower module tenant stats: %#v", out)
	}
	if store.writeCount != 0 {
		t.Fatalf("dashboard aggregation must stay read-only for admin tables, writes=%d", store.writeCount)
	}
}

// TestDashboardRequiresAllStatsContracts 确认看板缺少任一下层统计契约时显式失败,避免返回不完整的运营数据。
func TestDashboardRequiresAllStatsContracts(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*Service)
	}{
		{name: "identity", mut: func(s *Service) { s.identity = nil }},
		{name: "sandbox", mut: func(s *Service) { s.sandbox = nil }},
		{name: "teaching", mut: func(s *Service) { s.teaching = nil }},
		{name: "experiment", mut: func(s *Service) { s.experiment = nil }},
		{name: "contest", mut: func(s *Service) { s.contest = nil }},
	}
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 2001})

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newTestService(&fakeAdminStore{})
			tc.mut(svc)

			_, err := svc.SchoolDashboard(ctx)
			if err == nil {
				t.Fatalf("expected missing %s stats contract to fail", tc.name)
			}
			if ae, ok := apperr.As(err); !ok || (ae.Code != apperr.ErrAdminDashboard.Code && ae.Code != apperr.ErrAdminIdentityUnavailable.Code) {
				t.Fatalf("expected dashboard dependency error, got %v", err)
			}
		})
	}
}

// TestUpdateConfigRejectsVersionConflict 确认配置更新必须遵守乐观锁版本。
func TestUpdateConfigRejectsVersionConflict(t *testing.T) {
	store := &fakeAdminStore{config: ConfigDTO{ID: "9001", Scope: ScopeTenant, TenantID: "1001", Key: "quota.warn", Value: map[string]any{"threshold": float64(80)}, Version: 3}}
	svc := newTestService(store)
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 2001})

	_, err := svc.UpdateConfig(ctx, "quota.warn", ConfigUpdateRequest{Scope: ScopeTenant, Value: map[string]any{"threshold": 90}, Version: 2})
	if err == nil {
		t.Fatalf("expected version conflict")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrAdminConfigConflict.Code {
		t.Fatalf("expected admin config conflict, got %v", err)
	}
	if store.writeCount != 0 {
		t.Fatalf("conflict must not write config, writes=%d", store.writeCount)
	}
}

// TestRollbackConfigUsesHistoryOldValue 确认配置回退使用选中历史的变更前值并写入新版本历史。
func TestRollbackConfigUsesHistoryOldValue(t *testing.T) {
	store := &fakeAdminStore{
		config:  ConfigDTO{ID: "9001", Scope: ScopeTenant, TenantID: "1001", Key: "quota.warn", Value: map[string]any{"threshold": float64(90)}, Version: 4},
		history: ConfigChangeLogDTO{ID: "8001", ConfigID: "9001", OldValue: map[string]any{"threshold": float64(80)}, NewValue: map[string]any{"threshold": float64(90)}},
	}
	svc := newTestService(store)
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 2001})

	out, err := svc.RollbackConfig(ctx, "quota.warn", ConfigRollbackRequest{Scope: ScopeTenant, HistoryID: "8001", Version: 4})
	if err != nil {
		t.Fatalf("RollbackConfig returned error: %v", err)
	}
	if out.Value["threshold"] != float64(80) {
		t.Fatalf("expected rollback value from history old_value, got %#v", out.Value)
	}
	if store.writeCount != 1 {
		t.Fatalf("expected one config write, got %d", store.writeCount)
	}
}

// TestHandleAlertEventAllowsOnlyPendingTransition 确认告警事件只能从待处理进入已处理或已忽略。
func TestHandleAlertEventAllowsOnlyPendingTransition(t *testing.T) {
	store := &fakeAdminStore{event: AlertEventDTO{ID: "7001", Status: AlertEventPending}}
	svc := newTestService(store)
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 2001})

	out, err := svc.HandleAlertEvent(ctx, 7001, AlertHandleRequest{Status: AlertEventHandled})
	if err != nil {
		t.Fatalf("HandleAlertEvent returned error: %v", err)
	}
	if out.Status != AlertEventHandled {
		t.Fatalf("expected handled event, got %#v", out)
	}

	_, err = svc.HandleAlertEvent(ctx, 7001, AlertHandleRequest{Status: AlertEventIgnored})
	if err == nil {
		t.Fatalf("expected state error for terminal alert event")
	}
}

// TestHandleAlertEventRequiresNotifyService 确认文档要求“告警处理经 M10 通知”时,缺少通知依赖会显式失败。
func TestHandleAlertEventRequiresNotifyService(t *testing.T) {
	store := &fakeAdminStore{event: AlertEventDTO{ID: "7001", Status: AlertEventPending}}
	svc := newTestService(store)
	svc.notify = nil
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 2001})

	_, err := svc.HandleAlertEvent(ctx, 7001, AlertHandleRequest{Status: AlertEventHandled})
	if err == nil {
		t.Fatalf("expected missing notify dependency to fail")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrAdminAlertNotifyFailed.Code {
		t.Fatalf("expected admin alert notify error, got %v", err)
	}
}

// TestHandleAlertEventDoesNotCommitWhenNotifyFails 确认通知失败时不会留下已处理但未通知的半成功告警状态。
func TestHandleAlertEventDoesNotCommitWhenNotifyFails(t *testing.T) {
	store := &fakeAdminStore{event: AlertEventDTO{ID: "7001", Status: AlertEventPending}}
	svc := newTestService(store)
	svc.notify = &fakeAdminNotify{sendErr: errors.New("notify down")}
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 2001})

	_, err := svc.HandleAlertEvent(ctx, 7001, AlertHandleRequest{Status: AlertEventHandled})
	if err == nil {
		t.Fatalf("expected notify failure")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrAdminAlertNotifyFailed.Code {
		t.Fatalf("expected admin alert notify error, got %v", err)
	}
	if store.writeCount != 0 || store.event.Status != AlertEventPending {
		t.Fatalf("alert event state must stay pending on notify failure, writes=%d event=%#v", store.writeCount, store.event)
	}
}

// TestCreateAlertRuleUsesResolvedTenantScope 确认学校管理员创建告警规则时写入租户级 scope。
func TestCreateAlertRuleUsesResolvedTenantScope(t *testing.T) {
	store := &fakeAdminStore{}
	svc := newTestService(store)
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 2001})

	_, err := svc.CreateAlertRule(ctx, AlertRuleRequest{Name: "quota", Metric: "quota_usage", Condition: map[string]any{"gt": 80}, Level: AlertLevelWarning, Enabled: true})
	if err != nil {
		t.Fatalf("CreateAlertRule returned error: %v", err)
	}
	if store.createdRule.Scope != ScopeTenant || store.createdRule.TenantID != "1001" {
		t.Fatalf("expected tenant scoped rule, got %#v", store.createdRule)
	}
}

// TestMonitoringPanelsRequiresPlatformInService 确认监控入口在服务层也要求平台管理员,不能只依赖 HTTP 路由中间件。
func TestMonitoringPanelsRequiresPlatformInService(t *testing.T) {
	svc := newTestService(&fakeAdminStore{})
	svc.deploy.PlatformEnabled = true
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 2001})

	_, err := svc.MonitoringPanels(ctx)
	if err == nil {
		t.Fatalf("expected school admin context to be rejected by service")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrForbidden.Code {
		t.Fatalf("expected forbidden error, got %v", err)
	}
}

// TestIdentityAdminEntrypointsRequireConfiguredContract 确认 M9 转发 M1 的入口缺少身份契约时显式失败,避免生产请求 panic 或静默返回空数据。
func TestIdentityAdminEntrypointsRequireConfiguredContract(t *testing.T) {
	svc := newTestService(&fakeAdminStore{})
	svc.identity = nil
	svc.deploy.PlatformEnabled = true
	platformCtx := tenant.WithContext(context.Background(), tenant.Identity{IsPlatform: true, AccountID: 9001})

	cases := []struct {
		name string
		run  func() error
	}{
		{name: "platform dashboard", run: func() error {
			_, err := svc.PlatformDashboard(platformCtx)
			return err
		}},
		{name: "list tenants", run: func() error {
			_, _, err := svc.ListTenants(platformCtx, 0, 1, 20)
			return err
		}},
		{name: "list applications", run: func() error {
			_, _, err := svc.ListApplications(platformCtx, 0, 1, 20)
			return err
		}},
		{name: "approve application", run: func() error {
			_, err := svc.ApproveApplication(platformCtx, 7001, ApplicationApproveRequest{TenantCode: "demo", AdminPhone: "13800000000", AdminName: "校管"})
			return err
		}},
		{name: "reject application", run: func() error {
			return svc.RejectApplication(platformCtx, 7001, ApplicationRejectRequest{Reason: "资料不完整"})
		}},
		{name: "list audit", run: func() error {
			_, _, err := svc.ListAudit(platformCtx, contracts.AuditQuery{}, 1, 20)
			return err
		}},
		{name: "export audit", run: func() error {
			_, err := svc.ExportAudit(platformCtx, contracts.AuditQuery{})
			return err
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.run()
			if err == nil {
				t.Fatalf("expected missing identity admin dependency to fail")
			}
			if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrAdminIdentityUnavailable.Code {
				t.Fatalf("expected admin identity unavailable error, got %v", err)
			}
		})
	}
}

// TestAdminRepoUsesPlatformNoRows 守护 M9 数据访问未命中判断统一走 platform/db.IsNoRows。
func TestAdminRepoUsesPlatformNoRows(t *testing.T) {
	data, err := os.ReadFile("repo.go")
	if err != nil {
		t.Fatalf("read repo: %v", err)
	}
	text := string(data)
	if strings.Contains(text, "pgx.ErrNoRows") || strings.Contains(text, "errors.Is(") {
		t.Fatalf("admin repo must use platform db.IsNoRows instead of direct pgx no rows checks")
	}
}

// newTestService 构造带固定下层统计的 M9 服务。
func newTestService(store adminStore) *Service {
	return &Service{
		store:   store,
		idgen:   fixedIDGen(9901),
		auditor: &noopAdminAuditWriter{},
		notify:  &fakeAdminNotify{},
		identity: &fakeIdentityAdmin{
			stats: contracts.IdentityStats{TenantCount: 1, AccountCount: 50, TeacherCount: 8, StudentCount: 42, PendingApplicationCount: 4},
		},
		sandbox:    &fakeSandbox{stats: contracts.SandboxStats{TenantID: 1001, ActiveSandboxCount: 3, MaxConcurrentSandbox: 20}},
		teaching:   &fakeTeaching{stats: contracts.TeachingStats{TenantID: 1001, CourseCount: 5, ActiveCourseCount: 4, LearningDurationSec: 3600}},
		experiment: &fakeExperiment{stats: contracts.ExperimentStats{TenantID: 1001, ExperimentCount: 9, ActiveInstanceCount: 7}},
		contest:    &fakeContest{stats: contracts.ContestStats{TenantID: 1001, ContestCount: 6, ActiveContestCount: 2, TeamCount: 12}},
	}
}

type fixedIDGen int64

// Generate 返回固定 ID,让 M9 服务测试不依赖生产雪花节点。
func (g fixedIDGen) Generate() int64 { return int64(g) }

type noopAdminAuditWriter struct{}

func (w *noopAdminAuditWriter) Write(context.Context, audit.Entry) error { return nil }

type fakeAdminStore struct {
	config      ConfigDTO
	history     ConfigChangeLogDTO
	event       AlertEventDTO
	createdRule AlertRuleDTO
	writeCount  int
}

func (f *fakeAdminStore) ListStatistics(context.Context, int16, int64, time.Time, time.Time) ([]StatisticDTO, error) {
	return nil, nil
}
func (f *fakeAdminStore) ListConfigs(context.Context, int16, int64) ([]ConfigDTO, error) {
	return nil, nil
}
func (f *fakeAdminStore) GetConfig(context.Context, int16, int64, string) (ConfigDTO, error) {
	return f.config, nil
}
func (f *fakeAdminStore) UpdateConfig(_ context.Context, _ int64, _ int64, _ int64, _ ConfigDTO, value map[string]any) (ConfigDTO, error) {
	f.writeCount++
	f.config.Version++
	f.config.Value = value
	return f.config, nil
}
func (f *fakeAdminStore) GetConfigHistory(context.Context, int64, int64) (ConfigChangeLogDTO, error) {
	return f.history, nil
}
func (f *fakeAdminStore) ListConfigHistory(context.Context, int64, int, int) ([]ConfigChangeLogDTO, int64, error) {
	return nil, 0, nil
}
func (f *fakeAdminStore) ListAlertRules(context.Context, int16, int64, int, int) ([]AlertRuleDTO, int64, error) {
	return nil, 0, nil
}
func (f *fakeAdminStore) CreateAlertRule(_ context.Context, _ int64, tenantID int64, req AlertRuleRequest) (AlertRuleDTO, error) {
	f.writeCount++
	f.createdRule = AlertRuleDTO{ID: "1", Scope: req.Scope, TenantID: ids.Format(tenantID)}
	return f.createdRule, nil
}
func (f *fakeAdminStore) UpdateAlertRule(context.Context, int64, int64, AlertRulePatchRequest) (AlertRuleDTO, error) {
	f.writeCount++
	return AlertRuleDTO{}, nil
}
func (f *fakeAdminStore) ListAlertEvents(context.Context, int64, int16, int, int) ([]AlertEventDTO, int64, error) {
	return []AlertEventDTO{f.event}, 1, nil
}
func (f *fakeAdminStore) GetAlertEvent(context.Context, int64, int64) (AlertEventDTO, error) {
	return f.event, nil
}
func (f *fakeAdminStore) HandleAlertEvent(context.Context, int64, int64, int64, int16) (AlertEventDTO, error) {
	f.writeCount++
	f.event.Status = AlertEventHandled
	return f.event, nil
}
func (f *fakeAdminStore) RevertAlertEvent(context.Context, int64, int64) error {
	if f.writeCount > 0 {
		f.writeCount--
	}
	f.event.Status = AlertEventPending
	return nil
}
func (f *fakeAdminStore) ListBackups(context.Context, int, int) ([]BackupRecordDTO, int64, error) {
	return nil, 0, nil
}
func (f *fakeAdminStore) CreateBackupRecord(context.Context, int64, BackupTriggerRequest) (BackupRecordDTO, error) {
	f.writeCount++
	return BackupRecordDTO{}, nil
}

type fakeIdentityAdmin struct {
	stats   contracts.IdentityStats
	tenants []contracts.TenantSummary
}

func (f *fakeIdentityAdmin) Stats(context.Context, int64) (contracts.IdentityStats, error) {
	return f.stats, nil
}
func (f *fakeIdentityAdmin) AdminListTenants(context.Context, int16, int, int) ([]contracts.TenantSummary, int64, error) {
	return f.tenants, int64(len(f.tenants)), nil
}
func (f *fakeIdentityAdmin) AdminListApplications(context.Context, int16, int, int) ([]contracts.ApplicationSummary, int64, error) {
	return nil, 0, nil
}
func (f *fakeIdentityAdmin) AdminApproveApplication(context.Context, contracts.ApplicationApproval) (contracts.ApplicationApprovalResult, error) {
	return contracts.ApplicationApprovalResult{}, nil
}
func (f *fakeIdentityAdmin) AdminRejectApplication(context.Context, int64, int64, string) error {
	return nil
}
func (f *fakeIdentityAdmin) ListAuditRecords(context.Context, contracts.AuditQuery, int, int) ([]contracts.AuditRecord, int64, error) {
	return nil, 0, nil
}

type fakeIdentityReader struct{ roles []string }

func (f *fakeIdentityReader) GetAccount(context.Context, int64) (contracts.AccountInfo, error) {
	return contracts.AccountInfo{Roles: f.roles}, nil
}
func (f *fakeIdentityReader) BatchGetAccounts(context.Context, []int64) ([]contracts.AccountInfo, error) {
	return nil, nil
}
func (f *fakeIdentityReader) HasRole(_ context.Context, _ int64, role string) (bool, error) {
	for _, actual := range f.roles {
		if actual == role {
			return true, nil
		}
	}
	return false, nil
}

type fakeAdminNotify struct {
	sendErr error
}

func (f *fakeAdminNotify) Send(context.Context, contracts.NotifySendRequest) error {
	return f.sendErr
}

func (f *fakeAdminNotify) Push(context.Context, contracts.NotifyPushRequest) error { return nil }

type fakeSandbox struct{ stats contracts.SandboxStats }

func (f *fakeSandbox) CreateSandbox(context.Context, contracts.SandboxCreateRequest) (contracts.SandboxInfo, error) {
	return contracts.SandboxInfo{}, nil
}
func (f *fakeSandbox) GetSandbox(context.Context, int64) (contracts.SandboxInfo, error) {
	return contracts.SandboxInfo{}, nil
}
func (f *fakeSandbox) RecycleBySourceRef(context.Context, int64, string, string) error  { return nil }
func (f *fakeSandbox) PutSandboxFile(context.Context, contracts.SandboxFileWrite) error { return nil }
func (f *fakeSandbox) SaveSandboxFiles(context.Context, int64) (string, error)          { return "", nil }
func (f *fakeSandbox) ExecSandboxCommand(context.Context, contracts.SandboxExecRequest) (contracts.SandboxExecResult, error) {
	return contracts.SandboxExecResult{}, nil
}
func (f *fakeSandbox) ChainDeploy(context.Context, int64, map[string]any) (map[string]any, error) {
	return nil, nil
}
func (f *fakeSandbox) ChainSendTx(context.Context, int64, map[string]any) (map[string]any, error) {
	return nil, nil
}
func (f *fakeSandbox) ChainQuery(context.Context, int64, string) (map[string]any, error) {
	return nil, nil
}
func (f *fakeSandbox) ChainReset(context.Context, int64) error { return nil }
func (f *fakeSandbox) Stats(context.Context, int64) (contracts.SandboxStats, error) {
	return f.stats, nil
}

type fakeTeaching struct{ stats contracts.TeachingStats }

func (f *fakeTeaching) ListCourseGrades(context.Context, int64, int64) ([]contracts.TeachingCourseGrade, error) {
	return nil, nil
}
func (f *fakeTeaching) ListStudentGrades(context.Context, int64, int64) ([]contracts.TeachingCourseGrade, error) {
	return nil, nil
}
func (f *fakeTeaching) Stats(context.Context, int64) (contracts.TeachingStats, error) {
	return f.stats, nil
}

type fakeExperiment struct{ stats contracts.ExperimentStats }

func (f *fakeExperiment) Stats(context.Context, int64, int64) (contracts.ExperimentStats, error) {
	return f.stats, nil
}

type fakeContest struct{ stats contracts.ContestStats }

func (f *fakeContest) Stats(context.Context, int64) (contracts.ContestStats, error) {
	return f.stats, nil
}
func (f *fakeContest) ListStudentAchievements(context.Context, int64, int64) ([]contracts.ContestAchievement, error) {
	return nil, nil
}
