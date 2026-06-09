// M3 评测服务规则测试:覆盖请求校验、判题器资源配置和相似度计算边界。
package judge

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"chaimir/internal/contracts"
	"chaimir/internal/modules/judge/internal/sqlcgen"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/upload"
	"chaimir/pkg/apperr"

	"github.com/jackc/pgx/v5/pgtype"
)

// TestValidateSubmitRequestRequiresDocumentedFields 确认判题提交必须带租户、题目、代码与来源。
func TestValidateSubmitRequestRequiresDocumentedFields(t *testing.T) {
	req := validSubmitRequest()
	if err := validateSubmitRequest(req); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}

	req.SourceRef = "bad"
	if err := validateSubmitRequest(req); err == nil {
		t.Fatalf("invalid source_ref must be rejected")
	}
}

// TestSandboxModeRejectsReuseWithoutTarget 确认 reuse 模式必须指向调用方现场沙箱。
func TestSandboxModeRejectsReuseWithoutTarget(t *testing.T) {
	req := validSubmitRequest()
	req.SandboxMode = SandboxModeReuseText
	req.TargetSandboxRef = ""
	if err := validateSubmitRequest(req); err == nil {
		t.Fatalf("reuse mode without target sandbox must be rejected")
	}
}

// TestParseJudgerResourceSpecRequiresRuntimeForFreshSandbox 确认 fresh 沙箱判题器必须声明运行时。
func TestParseJudgerResourceSpecRequiresRuntimeForFreshSandbox(t *testing.T) {
	spec, err := parseJudgerResourceSpec([]byte(`{"runtime_code":"evm-hardhat","runtime_image_version":"evm-hardhat@sha256:abc","genesis_ref":"minio://chaimir-code/genesis/evm.json","tool_codes":["terminal"],"command":["sh","-lc","npm test"],"result_file":"result.json"}`))
	if err != nil {
		t.Fatalf("valid resource spec rejected: %v", err)
	}
	if spec.RuntimeCode != "evm-hardhat" || len(spec.Command) != 3 {
		t.Fatalf("unexpected resource spec: %#v", spec)
	}

	if _, err := parseJudgerResourceSpec([]byte(`{"command":["echo","ok"]}`)); err == nil {
		t.Fatalf("resource spec without runtime_code must be rejected")
	}
}

// TestNonSandboxJudgersAllowEmptyResourceSpec 确认 Flag/仿真检查点等无沙箱判题器不强制 runtime/command。
func TestNonSandboxJudgersAllowEmptyResourceSpec(t *testing.T) {
	if _, err := parseJudgerResourceSpecForType([]byte(`{}`), JudgerTypeFlag); err != nil {
		t.Fatalf("flag judger without sandbox resource spec must be accepted: %v", err)
	}
	if _, err := parseJudgerResourceSpecForType([]byte(`{}`), JudgerTypeSimCheckpoint); err != nil {
		t.Fatalf("sim checkpoint judger without sandbox resource spec must be accepted: %v", err)
	}
	if _, err := parseJudgerResourceSpecForType([]byte(`{}`), JudgerTypeTestcase); err == nil {
		t.Fatalf("testcase judger must still require runtime and command")
	}
}

// TestCosineSimilarityHandlesSparseVectors 确认查重相似度对稀疏向量稳定且不会除零。
func TestCosineSimilarityHandlesSparseVectors(t *testing.T) {
	score := cosineSimilarity(map[string]float64{"a": 1, "b": 1}, map[string]float64{"a": 1})
	if score <= 0 || score >= 1 {
		t.Fatalf("expected partial similarity, got %f", score)
	}
	if cosineSimilarity(map[string]float64{}, map[string]float64{"a": 1}) != 0 {
		t.Fatalf("empty vector similarity must be zero")
	}
}

// TestFingerprintVectorFromTextNormalizesTokens 确认源码 token 向量归一化且大小写不影响查重。
func TestFingerprintVectorFromTextNormalizesTokens(t *testing.T) {
	vector := fingerprintVectorFromText("contract Counter { function increment() public {} function Increment() public {} }")
	if vector["function"] <= 0 || vector["increment"] <= 0 {
		t.Fatalf("expected function and increment tokens, got %#v", vector)
	}
	if vector["Increment"] != 0 {
		t.Fatalf("tokens must be normalized to lowercase, got %#v", vector)
	}
	if cosineSimilarity(vector, vector) < 0.99 {
		t.Fatalf("same vector similarity must be close to 1")
	}
}

// TestFingerprintVectorFromArchiveAppliesCommonArchiveLimits 确认查重特征提取不能无边界展开学生提交归档。
func TestFingerprintVectorFromArchiveAppliesCommonArchiveLimits(t *testing.T) {
	raw := testJudgeFingerprintArchive(t, map[string]string{
		"src/a.sol": "contract A {}",
		"src/b.sol": "contract B {}",
	})
	if _, err := fingerprintVectorFromArchive(raw, upload.ArchiveLimits{MaxFiles: 1, MaxUnpackedBytes: 1024}); err == nil {
		t.Fatalf("fingerprint archive with too many files must be rejected")
	}
	if _, err := fingerprintVectorFromArchive(raw, upload.ArchiveLimits{MaxFiles: 10, MaxUnpackedBytes: 4}); err == nil {
		t.Fatalf("fingerprint archive exceeding unpacked bytes must be rejected")
	}
	vector, err := fingerprintVectorFromArchive(raw, upload.ArchiveLimits{MaxFiles: 10, MaxUnpackedBytes: 1024})
	if err != nil {
		t.Fatalf("safe fingerprint archive rejected: %v", err)
	}
	if vector["contract"] <= 0 {
		t.Fatalf("expected tokens from safe archive, got %#v", vector)
	}
}

// TestWaitSandboxReadyStopsWhenContextIsCanceled 确认 M3 等待 M2 就绪时尊重调用方取消,不使用不可中断固定 sleep。
func TestWaitSandboxReadyStopsWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	svc := &Service{
		cfg:     config.JudgeConfig{SandboxReadyPollIntervalMs: 500},
		sandbox: &fakeWaitingSandbox{info: contracts.SandboxInfo{Phase: 1, Status: 1}},
		waitSandboxPoll: func(context.Context, int) error {
			cancel()
			return context.Canceled
		},
	}

	err := svc.waitSandboxReady(ctx, 9001, 30)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}

func testJudgeFingerprintArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var raw bytes.Buffer
	gz := gzip.NewWriter(&raw)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		data := []byte(body)
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(data))}); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatalf("write tar body: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return raw.Bytes()
}

// TestListManualPendingRequiresSourceRef 确认待人工评分列表必须限定上游来源,避免教师看到全租户任务。
func TestListManualPendingRequiresSourceRef(t *testing.T) {
	svc := &Service{}
	ctx := tenant.WithContext(context.Background(), tenant.Identity{TenantID: 1001, AccountID: 2001})

	_, err := svc.ListTasks(ctx, "", true)

	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrJudgeTaskInvalid.Code {
		t.Fatalf("manual pending list without source_ref must return %s, got %v", apperr.ErrJudgeTaskInvalid.Code, err)
	}
}

// TestListManualPendingSQLFiltersSourceRef 防止 J6 待人工评分查询退回全租户扫描。
func TestListManualPendingSQLFiltersSourceRef(t *testing.T) {
	data, err := os.ReadFile("../../../db/queries/judge.sql")
	if err != nil {
		t.Fatalf("read judge sql: %v", err)
	}
	sql := strings.ReplaceAll(string(data), "\r\n", "\n")
	start := strings.Index(sql, "-- name: ListManualPendingTasks")
	if start < 0 {
		t.Fatal("ListManualPendingTasks query missing")
	}
	next := strings.Index(sql[start+1:], "-- name:")
	block := sql[start:]
	if next >= 0 {
		block = sql[start : start+1+next]
	}
	if !strings.Contains(block, "jt.source_ref = $1") {
		t.Fatalf("manual pending query must filter by source_ref, got:\n%s", block)
	}
}

// TestSubmitJudgeTaskIsIdempotentBySourceRef 确认上游 outbox 重试不会为同一 source_ref 创建重复判题任务。
func TestSubmitJudgeTaskIsIdempotentBySourceRef(t *testing.T) {
	sqlData, err := os.ReadFile("../../../db/queries/judge.sql")
	if err != nil {
		t.Fatalf("read judge sql: %v", err)
	}
	if !strings.Contains(string(sqlData), "-- name: GetJudgeTaskBySourceRef :one") {
		t.Fatalf("judge SQL must support source_ref idempotency lookup")
	}
	migration, err := os.ReadFile("../../../db/migrations/0004_judge.up.sql")
	if err != nil {
		t.Fatalf("read judge migration: %v", err)
	}
	if !strings.Contains(string(migration), "uk_judge_task_source_ref") {
		t.Fatalf("judge_task must enforce tenant_id + source_ref uniqueness")
	}
	service, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	submit := functionSource(string(service), "SubmitJudgeTask")
	for _, required := range []string{"existingTaskBySourceRef", "ErrJudgeTaskNotFound"} {
		if !strings.Contains(submit, required) {
			t.Fatalf("SubmitJudgeTask must return existing task for duplicate source_ref, missing %s", required)
		}
	}
}

// TestJudgeProductionCodeUsesModuleSpecificErrors 确认 M3 不把真实业务错误折叠为通用内部错误。
func TestJudgeProductionCodeUsesModuleSpecificErrors(t *testing.T) {
	for _, file := range []string{"service.go", "service_worker.go", "audit.go"} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		if strings.Contains(string(data), "ErrInternal") {
			t.Fatalf("%s must use M3-specific error codes instead of ErrInternal", file)
		}
	}
}

// TestSimilarityRequestDoesNotAcceptClientVector 确认查重特征只由 M3 从提交对象生成,不接受调用方上传向量。
func TestSimilarityRequestDoesNotAcceptClientVector(t *testing.T) {
	dto, err := os.ReadFile("dto.go")
	if err != nil {
		t.Fatalf("read dto.go: %v", err)
	}
	if strings.Contains(string(dto), "Vector") || strings.Contains(string(dto), `json:"vector"`) {
		t.Fatalf("similarity request must not accept client supplied feature vector")
	}

	service, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	if strings.Contains(string(service), "req.Vector") {
		t.Fatalf("similarity service must derive vectors from stored code object, not use client vectors")
	}
}

// TestSimilarityDefaultThresholdComesFromConfig 确认查重默认相似度阈值来自统一配置。
func TestSimilarityDefaultThresholdComesFromConfig(t *testing.T) {
	data, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	block := functionSource(string(data), "Similarity")
	if !strings.Contains(block, "s.cfg.SimilarityDefaultThreshold") {
		t.Fatalf("Similarity must read default threshold from JudgeConfig")
	}
	if strings.Contains(block, "threshold = 0.8") {
		t.Fatalf("Similarity must not hard-code default threshold in service code")
	}
}

// TestSubmitRateLimitRequiresRedis 确认提交限频能力缺失时 fail-fast,不能静默跳过。
func TestSubmitRateLimitRequiresRedis(t *testing.T) {
	svc := &Service{cfg: config.JudgeConfig{SubmitRateLimitSec: 10}}
	err := svc.checkSubmitRate(context.Background(), validSubmitRequest())
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrJudgeTaskQueuedFail.Code {
		t.Fatalf("missing redis rate limiter must return %s, got %v", apperr.ErrJudgeTaskQueuedFail.Code, err)
	}
}

// TestManualScoreChecksJudgerType 确认人工评分只能写入 J6 人工评分任务。
func TestManualScoreChecksJudgerType(t *testing.T) {
	data, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	body := string(data)
	start := strings.Index(body, "func (s *Service) ManualScore(")
	end := strings.Index(body, "// ExactFingerprints")
	if start < 0 || end < start {
		t.Fatalf("ManualScore function block not found")
	}
	block := body[start:end]
	if !strings.Contains(block, "getTaskAndJudgerType") || !strings.Contains(block, "JudgerTypeManual") {
		t.Fatalf("ManualScore must verify the task uses manual judger type through repo before writing result")
	}
}

// TestRejudgeSQLOnlyAllowsTerminalTasks 确认重判只允许已完成或失败终态任务,不会重排正在执行的任务。
func TestRejudgeSQLOnlyAllowsTerminalTasks(t *testing.T) {
	data, err := os.ReadFile("../../../db/queries/judge.sql")
	if err != nil {
		t.Fatalf("read judge sql: %v", err)
	}
	sql := strings.ReplaceAll(string(data), "\r\n", "\n")
	start := strings.Index(sql, "-- name: MarkJudgeTaskRejudge")
	if start < 0 {
		t.Fatal("MarkJudgeTaskRejudge query missing")
	}
	next := strings.Index(sql[start+1:], "-- name:")
	block := sql[start:]
	if next >= 0 {
		block = sql[start : start+1+next]
	}
	if !strings.Contains(block, "status IN (3, 7)") {
		t.Fatalf("rejudge query must only allow done/failed tasks, got:\n%s", block)
	}
}

// TestRejudgeWritesAuditBeforeQueueing 确认重判审计失败时不会先把任务放回 worker 队列。
func TestRejudgeWritesAuditBeforeQueueing(t *testing.T) {
	data, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	body := string(data)
	start := strings.Index(body, "func (s *Service) Rejudge(")
	end := strings.Index(body, "// CancelTask")
	if start < 0 || end < start {
		t.Fatalf("Rejudge function block not found")
	}
	block := body[start:end]
	auditIdx := strings.Index(block, "s.writeAudit(")
	enqueueIdx := strings.Index(block, "s.enqueueTask(ctx, row)")
	if auditIdx < 0 || enqueueIdx < 0 {
		t.Fatalf("Rejudge must write audit and enqueue task")
	}
	if auditIdx > enqueueIdx {
		t.Fatalf("Rejudge must write audit before enqueueing to avoid half-success on audit failure")
	}
}

// TestJudgerSelftestSQLDoesNotRestrictImpossibleStatus 确认判题器自检更新不被任务状态枚举污染。
func TestJudgerSelftestSQLDoesNotRestrictImpossibleStatus(t *testing.T) {
	data, err := os.ReadFile("../../../db/queries/judge.sql")
	if err != nil {
		t.Fatalf("read judge sql: %v", err)
	}
	sql := strings.ReplaceAll(string(data), "\r\n", "\n")
	start := strings.Index(sql, "-- name: UpdateJudgerSelftest")
	if start < 0 {
		t.Fatal("UpdateJudgerSelftest query missing")
	}
	next := strings.Index(sql[start+1:], "-- name:")
	block := sql[start:]
	if next >= 0 {
		block = sql[start : start+1+next]
	}
	if strings.Contains(block, "status IN (3, 7)") {
		t.Fatalf("judger selftest update must not use judge task status condition, got:\n%s", block)
	}
}

// TestJudgeMigrationKeepsOwnTableForeignKeys 确认 M3 自有表之间用 FK 约束保持一致性。
func TestJudgeMigrationKeepsOwnTableForeignKeys(t *testing.T) {
	data, err := os.ReadFile("../../../db/migrations/0004_judge.up.sql")
	if err != nil {
		t.Fatalf("read judge migration: %v", err)
	}
	body := strings.ToLower(string(data))
	for _, want := range []string{
		"judger_id          bigint       not null references judger(id)",
		"task_id           bigint      primary key references judge_task(id)",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("judge migration missing FK fragment %q", want)
		}
	}
}

// TestSubmitJudgeTaskUsesJudgerMaxRetries 确认任务重试策略优先来自判题器资源配置。
func TestSubmitJudgeTaskUsesJudgerMaxRetries(t *testing.T) {
	data, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	body := string(data)
	start := strings.Index(body, "func (s *Service) SubmitJudgeTask(")
	end := strings.Index(body, "// GetJudgeTask")
	if start < 0 || end < start {
		t.Fatalf("SubmitJudgeTask function block not found")
	}
	block := body[start:end]
	if !strings.Contains(block, "maxRetriesForJudger") || strings.Contains(block, "MaxRetries: int32(s.cfg.DefaultMaxRetries)") {
		t.Fatalf("SubmitJudgeTask must derive max_retries from judger resource_spec before falling back to config")
	}
}

// TestBuildInputSnapshotIncludesSandboxImageVersion 确认可复现快照固化 judge 沙箱镜像版本。
func TestBuildInputSnapshotIncludesSandboxImageVersion(t *testing.T) {
	data, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	body := string(data)
	start := strings.Index(body, "func (s *Service) buildInputSnapshot(")
	end := strings.Index(body, "// buildSubmissionVector")
	if start < 0 || end < start {
		t.Fatalf("buildInputSnapshot function block not found")
	}
	block := body[start:end]
	if !strings.Contains(block, "sandbox_image_version") || !strings.Contains(block, "genesis_ref") {
		t.Fatalf("input snapshot must include sandbox_image_version and genesis_ref for deterministic rejudge")
	}
}

// TestPrepareJudgeSandboxPinsRuntimeImageVersion 确认 M3 创建 judge 沙箱时把固定镜像版本传给 M2。
func TestPrepareJudgeSandboxPinsRuntimeImageVersion(t *testing.T) {
	data, err := os.ReadFile("service_worker.go")
	if err != nil {
		t.Fatalf("read service_worker.go: %v", err)
	}
	body := string(data)
	start := strings.Index(body, "func (s *Service) prepareJudgeSandbox(")
	end := strings.Index(body, "// injectJudgeInputs")
	if start < 0 || end < start {
		t.Fatalf("prepareJudgeSandbox function block not found")
	}
	block := body[start:end]
	if !strings.Contains(block, "RuntimeImageVersion: spec.RuntimeImageVersion") {
		t.Fatalf("prepareJudgeSandbox must pass pinned runtime_image_version to M2 CreateSandbox")
	}
}

// TestManualScorePublishesJudgeCompleted 确认人工评分完成后通过可恢复 outbox 记录 judge.completed。
func TestManualScorePublishesJudgeCompleted(t *testing.T) {
	data, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	body := string(data)
	start := strings.Index(body, "func (s *Service) ManualScore(")
	end := strings.Index(body, "// ExactFingerprints")
	if start < 0 || end < start {
		t.Fatalf("ManualScore function block not found")
	}
	block := body[start:end]
	if !strings.Contains(block, "newJudgeEventOutbox") || !strings.Contains(block, "repo.completeTaskResult") {
		t.Fatalf("ManualScore must persist judge.completed through the recoverable outbox")
	}
	if strings.Contains(block, "publishJudgeCompleted") {
		t.Fatalf("ManualScore must not directly publish judge.completed after result persistence")
	}
	auditIdx := strings.Index(block, "s.writeAudit(")
	resultIdx := strings.Index(block, "repo.completeTaskResult")
	outboxIdx := strings.Index(block, "newJudgeEventOutbox")
	if auditIdx < 0 || resultIdx < 0 || outboxIdx < 0 || auditIdx > resultIdx || auditIdx > outboxIdx {
		t.Fatalf("ManualScore must write audit before result/outbox persistence")
	}
}

// TestJudgeEventsRequireConfiguredBus 确认 M3 终态事件缺少总线时 fail-fast,不能用 nil 判断静默跳过。
func TestJudgeEventsRequireConfiguredBus(t *testing.T) {
	for _, file := range []string{"service.go", "service_worker.go"} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		if strings.Contains(string(data), "if s.bus != nil") {
			t.Fatalf("%s must not skip judge events when event bus is missing", file)
		}
	}
}

// TestManualScoreDetailsIncludeStructuredResult 确认 J6 人工评分详情包含结构化分值和通过状态。
func TestManualScoreDetailsIncludeStructuredResult(t *testing.T) {
	data, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	body := string(data)
	start := strings.Index(body, "func (s *Service) ManualScore(")
	end := strings.Index(body, "// ExactFingerprints")
	if start < 0 || end < start {
		t.Fatalf("ManualScore function block not found")
	}
	block := body[start:end]
	for _, want := range []string{`"comment":`, "req.Comment", `"score":`, "req.Score", `"max_score":`, "req.MaxScore", `"passed":`, "req.Score >= req.MaxScore"} {
		if !strings.Contains(block, want) {
			t.Fatalf("ManualScore details must include %s", want)
		}
	}
}

// TestSubmitJudgeTaskWritesAudit 确认内部判题提交调度写入统一 audit_log。
func TestSubmitJudgeTaskWritesAudit(t *testing.T) {
	data, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	body := string(data)
	start := strings.Index(body, "func (s *Service) SubmitJudgeTask(")
	end := strings.Index(body, "// GetJudgeTask")
	if start < 0 || end < start {
		t.Fatalf("SubmitJudgeTask function block not found")
	}
	block := body[start:end]
	if !strings.Contains(block, "auditActionTaskSubmit") || !strings.Contains(block, "s.writeAudit(") {
		t.Fatalf("SubmitJudgeTask must write audit_log for judge submission")
	}
}

// TestServiceFileDoesNotOwnWebSocketHandlers 确认 service.go 不承载 WS 接入/广播职责。
func TestServiceFileDoesNotOwnWebSocketHandlers(t *testing.T) {
	data, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	body := string(data)
	for _, forbidden := range []string{
		`"net/http"`,
		"func (s *Service) ServeProgressWS(",
		"func (s *Service) publishProgress(",
		"func judgeProgressTopic(",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("service.go must not own websocket handler/broadcast responsibility, found %s", forbidden)
		}
	}
	websocket, err := os.ReadFile("api_websocket.go")
	if err != nil {
		t.Fatalf("judge websocket API responsibility must live in api_websocket.go: %v", err)
	}
	for _, required := range []string{"serveProgressWS", "judgeProgressTopic"} {
		if !strings.Contains(string(websocket), required) {
			t.Fatalf("api_websocket.go must contain %s", required)
		}
	}
	serviceProgress, err := os.ReadFile("service_websocket.go")
	if err != nil {
		t.Fatalf("read service_websocket.go: %v", err)
	}
	if !strings.Contains(string(serviceProgress), "publishProgress") {
		t.Fatalf("service_websocket.go must keep progress publication service logic")
	}
}

// TestTaskInfoMapIncludesDocumentedResult 确认 GET /tasks/{id} 输出文档要求的结果详情。
func TestTaskInfoMapIncludesDocumentedResult(t *testing.T) {
	view := taskViewFromJoined(sqlcgen.GetJudgeTaskWithResultRow{
		ID:                    8801,
		TenantID:              1001,
		SourceRef:             "experiment:2026:instance:55",
		SubmitterID:           2001,
		Status:                JudgeTaskDone,
		ResultPassed:          pgtype.Bool{Bool: false, Valid: true},
		ResultScore:           pgtype.Int4{Int32: 60, Valid: true},
		ResultMaxScore:        pgtype.Int4{Int32: 100, Valid: true},
		ResultDetails:         []byte(`[{"case":"basic","passed":false,"actual":"ok"}]`),
		ResultJudgedAt:        pgtype.Timestamptz{Valid: true},
		ResultIsRejudge:       pgtype.Bool{Bool: true, Valid: true},
		ResultJudgeSandboxRef: pgtype.Text{String: "9001", Valid: true},
	})

	out := taskViewToMap(view)
	result, ok := out["result"].(map[string]any)
	if !ok {
		t.Fatalf("task output must include nested result, got %#v", out)
	}
	if result["score"] != int32(60) || result["max_score"] != int32(100) || result["passed"] != false {
		t.Fatalf("result score fields mismatch: %#v", result)
	}
	details, ok := result["details"].([]any)
	if !ok || len(details) != 1 {
		t.Fatalf("result details must be decoded JSON array, got %#v", result["details"])
	}
	if result["is_rejudge"] != true || result["judge_sandbox_ref"] != "9001" {
		t.Fatalf("result audit fields mismatch: %#v", result)
	}
}

// validSubmitRequest 构造一份满足文档字段要求的判题提交请求。
func validSubmitRequest() contracts.JudgeSubmitRequest {
	return contracts.JudgeSubmitRequest{
		TenantID:       1001,
		JudgerCode:     "testcase",
		ItemCode:       "prob-1",
		ItemVersion:    "1.0.0",
		CodeStorageKey: "minio://chaimir-code/submissions/1.tgz",
		CodeHash:       "5e884898da28047151d0e56f8dc6292773603d0d6aabbdd62a11ef721d1542d8",
		SubmitterID:    2001,
		SourceRef:      "experiment:2026:instance:55",
		SandboxMode:    SandboxModeFreshText,
		Priority:       2,
	}
}

// functionSource 返回指定 Service 方法的大致源码片段,用于守护高风险流程不退回旧路径。
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

type fakeWaitingSandbox struct {
	contracts.SandboxService
	info contracts.SandboxInfo
}

func (f *fakeWaitingSandbox) GetSandbox(context.Context, int64) (contracts.SandboxInfo, error) {
	return f.info, nil
}
