// M7 服务规则测试:覆盖实验组件校验、实例状态机、编排补偿、事件回写和得分汇总边界。
package experiment

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
)

// TestValidateExperimentComponentsRequiresAtLeastOneRunnableComponent 确认实验至少包含沙箱或仿真组件。
func TestValidateExperimentComponentsRequiresAtLeastOneRunnableComponent(t *testing.T) {
	err := validateExperimentComponents(ExperimentComponents{
		Checkpoints: []CheckpointComponent{{ID: "cp1", JudgerCode: "testcase", ItemCode: "p1", ItemVersion: "1.0.0", Score: 100}},
	})
	if err == nil {
		t.Fatalf("experiment without env or sim must be rejected")
	}
	if err := validateExperimentComponents(ExperimentComponents{
		Envs:        []EnvComponent{{RuntimeCode: "evm-hardhat", ToolCodes: []string{"terminal"}}},
		Checkpoints: []CheckpointComponent{{ID: "cp1", JudgerCode: "testcase", ItemCode: "p1", ItemVersion: "1.0.0", Score: 100}},
	}); err != nil {
		t.Fatalf("valid component config rejected: %v", err)
	}
}

// TestValidateExperimentComponentsRequiresCheckpointScoreWithinLimit 确认检查点分值合计不能超过 100。
func TestValidateExperimentComponentsRequiresCheckpointScoreWithinLimit(t *testing.T) {
	err := validateExperimentComponents(ExperimentComponents{
		Sims: []SimComponent{{PackageCode: "builtin__pow-mining", Version: "1.0.0"}},
		Checkpoints: []CheckpointComponent{
			{ID: "cp1", JudgerCode: "testcase", ItemCode: "p1", ItemVersion: "1.0.0", Score: 70},
			{ID: "cp2", JudgerCode: "testcase", ItemCode: "p2", ItemVersion: "1.0.0", Score: 40},
		},
	})
	if err == nil {
		t.Fatalf("checkpoint score above 100 must be rejected")
	}
}

// TestInstanceStatusTransitionsFollowDocumentedLifecycle 确认实例状态流转符合文档状态机。
func TestInstanceStatusTransitionsFollowDocumentedLifecycle(t *testing.T) {
	if err := validateInstanceTransition(InstanceStatusCreating, InstanceStatusRunning); err != nil {
		t.Fatalf("creating should move to running: %v", err)
	}
	if err := validateInstanceTransition(InstanceStatusRunning, InstanceStatusPaused); err != nil {
		t.Fatalf("running should pause: %v", err)
	}
	if err := validateInstanceTransition(InstanceStatusPaused, InstanceStatusReleased); err != nil {
		t.Fatalf("paused should become released when sandbox recycled: %v", err)
	}
	if err := validateInstanceTransition(InstanceStatusRecycled, InstanceStatusRunning); err == nil {
		t.Fatalf("recycled instance must not resume")
	}
}

// TestComputeExperimentScoreSumsCheckpointAndReportScore 确认实验总分由检查点与报告分汇总且封顶 100。
func TestComputeExperimentScoreSumsCheckpointAndReportScore(t *testing.T) {
	score, err := computeExperimentScore([]ScorePart{{Score: 40}, {Score: 35}}, ptrFloat(20))
	if err != nil {
		t.Fatalf("valid score parts rejected: %v", err)
	}
	if score != 95 {
		t.Fatalf("unexpected score: %v", score)
	}
	if _, err := computeExperimentScore([]ScorePart{{Score: 90}}, ptrFloat(20)); err == nil {
		t.Fatalf("score above 100 must be rejected")
	}
}

// TestCreateExperimentAllowsIncompleteWizardDraft 确认向导草稿可保存不完整组件配置。
func TestCreateExperimentAllowsIncompleteWizardDraft(t *testing.T) {
	store := &fakeExperimentStore{}
	svc := newExperimentTestService(store)

	out, err := svc.CreateExperiment(testTenantContext(), ExperimentRequest{Name: "draft", WizardStep: 1})
	if err != nil {
		t.Fatalf("incomplete wizard draft must be persisted: %v", err)
	}
	if out.Status != ExperimentStatusDraft {
		t.Fatalf("draft should remain draft, got %d", out.Status)
	}
}

// TestStartInstanceRecyclesStartedSandboxWhenSimFails 确认仿真拉起失败时补偿回收已创建沙箱。
func TestStartInstanceRecyclesStartedSandboxWhenSimFails(t *testing.T) {
	store := &fakeExperimentStore{
		experiment: ExperimentDTO{
			ID: "7001", Status: ExperimentStatusPublished, CollabMode: CollabModeSingle, Components: ExperimentComponents{
				Envs: []EnvComponent{{RuntimeCode: "evm-hardhat", ToolCodes: []string{"terminal"}}},
				Sims: []SimComponent{{PackageCode: "builtin__pow-mining", Version: "1.0.0"}},
			},
		},
	}
	sandbox := &fakeSandboxService{}
	sim := &fakeSimService{createErr: errors.New("sim create failed")}
	svc := newExperimentTestService(store)
	svc.idgen = fixedIDGen(8001)
	svc.sandbox = sandbox
	svc.sim = sim

	_, err := svc.StartInstance(testTenantContext(), 7001, StartInstanceRequest{})
	if err == nil {
		t.Fatalf("sim failure must be returned")
	}
	if len(sandbox.recycled) != 1 {
		t.Fatalf("started sandbox must be recycled once, got %d", len(sandbox.recycled))
	}
	if store.instanceStatus != InstanceStatusError {
		t.Fatalf("instance status should be error after compensation, got %d", store.instanceStatus)
	}
}

// TestFinishInstancePublishesScoredEvent 确认完成实例会汇总检查点与报告分并发布得分事件。
func TestFinishInstancePublishesScoredEvent(t *testing.T) {
	store := &fakeExperimentStore{
		instance:          ExperimentInstanceDTO{ID: "8001", TenantID: "100", ExperimentID: "7001", OwnerAccountID: "200", Status: InstanceStatusRunning},
		checkpointResults: []ScorePart{{Score: 40}, {Score: 30}},
		reportScore:       ptrFloat(20),
	}
	bus := &fakeEventBus{}
	svc := newExperimentTestService(store)
	svc.bus = bus

	out, err := svc.FinishInstance(testTenantContext(), 8001)
	if err != nil {
		t.Fatalf("finish instance rejected: %v", err)
	}
	if out.Score == nil || *out.Score != 90 {
		t.Fatalf("unexpected score: %v", out.Score)
	}
	if len(bus.published) != 1 || bus.published[0].subject != contracts.SubjectExperimentScored {
		t.Fatalf("experiment.scored event must be published, got %#v", bus.published)
	}
}

// TestFinishInstanceRequiresConfiguredEventBus 确认实验得分事件缺少总线时显式失败,避免 M11 聚合链路静默丢失。
func TestFinishInstanceRequiresConfiguredEventBus(t *testing.T) {
	store := &fakeExperimentStore{
		instance:          ExperimentInstanceDTO{ID: "8001", TenantID: "100", ExperimentID: "7001", OwnerAccountID: "200", Status: InstanceStatusRunning},
		checkpointResults: []ScorePart{{Score: 40}, {Score: 30}},
		reportScore:       ptrFloat(20),
	}
	svc := newExperimentTestService(store)
	svc.bus = nil

	_, err := svc.FinishInstance(testTenantContext(), 8001)
	if err == nil {
		t.Fatalf("expected missing event bus to fail")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrExperimentEventFailed.Code {
		t.Fatalf("expected experiment event failed error, got %v", err)
	}
}

// TestHandleJudgeCompletedUpsertsCheckpointResult 确认 M3 判题完成事件按 judge_task_ref 回写检查点结果。
func TestHandleJudgeCompletedUpsertsCheckpointResult(t *testing.T) {
	store := &fakeExperimentStore{pendingJudge: PendingCheckpoint{TenantID: 100, InstanceID: 8001, CheckpointID: "cp1"}}
	svc := &Service{store: store, idgen: fixedIDGen(9001)}

	err := svc.HandleJudgeCompleted(context.Background(), contracts.JudgeCompletedEvent{TenantID: 100, TaskID: 3001, Score: 40, Status: 3})
	if err != nil {
		t.Fatalf("judge completed event rejected: %v", err)
	}
	if store.lastCheckpoint.CheckpointID != "cp1" || store.lastCheckpoint.Score != 40 {
		t.Fatalf("checkpoint result not upserted from event: %#v", store.lastCheckpoint)
	}
}

// TestHandleSandboxRecycledMarksInstanceReleased 确认沙箱独立回收事件会把运行实例转为环境已释放。
func TestHandleSandboxRecycledMarksInstanceReleased(t *testing.T) {
	store := &fakeExperimentStore{}
	svc := &Service{store: store}

	err := svc.HandleSandboxRecycled(context.Background(), contracts.SandboxRecycledEvent{TenantID: 100, SandboxID: 9001, SourceRef: "exp:2026:instance:8001"})
	if err != nil {
		t.Fatalf("sandbox recycled event rejected: %v", err)
	}
	if store.releasedSandboxID != 9001 {
		t.Fatalf("released sandbox id not recorded: %d", store.releasedSandboxID)
	}
}

// TestInstanceLifecycleUsesPersistedSourceRef 确认跨年恢复/回收不会重新计算 source_ref 导致引擎归属漂移。
func TestInstanceLifecycleUsesPersistedSourceRef(t *testing.T) {
	serviceSrc, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service: %v", err)
	}
	for _, name := range []string{"RecycleInstance", "JudgeCheckpoint", "rebuildReleasedInstance"} {
		block := functionSource(string(serviceSrc), name)
		if !strings.Contains(block, "instance.SourceRef") {
			t.Fatalf("%s must use persisted instance.SourceRef", name)
		}
		if strings.Contains(block, "sourceRefForInstance(") {
			t.Fatalf("%s must not recompute source_ref from current clock", name)
		}
	}
	sql, err := os.ReadFile("../../../db/queries/experiment.sql")
	if err != nil {
		t.Fatalf("read experiment sql: %v", err)
	}
	if !strings.Contains(string(sql), "source_ref") {
		t.Fatalf("experiment_instance must persist source_ref")
	}
}

// TestJudgeEventsBindSourceRef 确认 M3 判题事件必须与实例 source_ref 绑定,不能只凭 task_id 回写。
func TestJudgeEventsBindSourceRef(t *testing.T) {
	serviceSrc, err := os.ReadFile("events.go")
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	for _, name := range []string{"HandleJudgeCompleted", "HandleJudgeFailed"} {
		block := functionSource(string(serviceSrc), name)
		if !strings.Contains(block, "pending.SourceRef") || !strings.Contains(block, "event.SourceRef") {
			t.Fatalf("%s must compare judge event source_ref with pending checkpoint source_ref", name)
		}
	}
}

// TestExperimentManagementWritesUseAtomicOwnershipChecks 防止报告批改和小组成员管理绕过实验归属授权。
func TestExperimentManagementWritesUseAtomicOwnershipChecks(t *testing.T) {
	serviceSrc, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service: %v", err)
	}
	serviceText := string(serviceSrc)
	for _, required := range []string{"ensureGroupManager", "GradeReportAuthorized", "AddGroupMemberAuthorized"} {
		if !strings.Contains(serviceText, required) {
			t.Fatalf("management writes must use unified authorization path, missing %s", required)
		}
	}
	if strings.Contains(functionSource(serviceText, "GradeReport"), "s.store.GradeReport(ctx, reportID") {
		t.Fatalf("report grading must not directly update by report id without experiment ownership")
	}
	if strings.Contains(functionSource(serviceText, "AddGroupMember"), "s.store.AddGroupMember(ctx, id, s.nextID(), groupID") {
		t.Fatalf("group membership changes must not directly write by group id without experiment ownership")
	}

	sql, err := os.ReadFile("../../../db/queries/experiment.sql")
	if err != nil {
		t.Fatalf("read experiment sql: %v", err)
	}
	queryText := string(sql)
	for _, required := range []string{"GradeExperimentReportAuthorized", "AddGroupMemberAuthorized", "GetExperimentGroupByIDAndExperiment", "e.author_id = @actor_id", "@is_platform::boolean"} {
		if !strings.Contains(queryText, required) {
			t.Fatalf("experiment SQL must enforce manager ownership atomically, missing %s", required)
		}
	}
}

// TestExperimentAPIErrorsUseModuleCodes 防止 M7 API 边界退回通用 ErrBadRequest。
func TestExperimentAPIErrorsUseModuleCodes(t *testing.T) {
	apiSrc, err := os.ReadFile("api.go")
	if err != nil {
		t.Fatalf("read api: %v", err)
	}
	if strings.Contains(string(apiSrc), "ErrBadRequest") {
		t.Fatalf("M7 API must return experiment module error codes instead of ErrBadRequest")
	}
	codes, err := os.ReadFile("../../../pkg/apperr/experiment_codes.go")
	if err != nil {
		t.Fatalf("read experiment codes: %v", err)
	}
	if !strings.Contains(string(codes), "ErrExperimentStatsQueryInvalid") {
		t.Fatalf("invalid stats query must have a dedicated M7 error code")
	}
}

// TestExperimentProductionCodeAvoidsGlobalInternalErrors 防止 M7 生产路径把模块错误退回全局内部错误码。
func TestExperimentProductionCodeAvoidsGlobalInternalErrors(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("list experiment files: %v", err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		if strings.Contains(string(src), "ErrInternal") {
			t.Fatalf("%s must use M7 dedicated error codes instead of global ErrInternal", file)
		}
	}
}

// functionSource 返回指定服务方法源码片段,用于守护高风险授权入口不退回旧路径。
func functionSource(src, name string) string {
	start := strings.Index(src, "func (s *Service) "+name+"(")
	if start < 0 {
		return ""
	}
	next := strings.Index(src[start+1:], "\nfunc (s *Service) ")
	if next < 0 {
		return src[start:]
	}
	return src[start : start+1+next]
}

// ptrFloat 返回 float64 指针,用于测试可选报告分。
func ptrFloat(v float64) *float64 { return &v }

// testTenantContext 构造带租户身份的服务测试上下文。
func testTenantContext() context.Context {
	return tenant.WithContext(context.Background(), tenant.Identity{TenantID: 100, AccountID: 200})
}

func newExperimentTestService(store experimentStore) *Service {
	return &Service{
		store:    store,
		idgen:    fixedIDGen(7001),
		auditor:  &noopExperimentAuditWriter{},
		identity: &experimentAuditIdentity{account: contracts.AccountInfo{AccountID: 200, TenantID: 100, BaseIdentity: 2, Roles: []string{"teacher"}}},
	}
}

type noopExperimentAuditWriter struct{}

func (w *noopExperimentAuditWriter) Write(context.Context, audit.Entry) error { return nil }
