// M3 worker 测试:覆盖判题输出解析、失败分类与 worker 可复现输入处理规则。
package judge

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"strings"
	"testing"

	"chaimir/internal/platform/config"
	"chaimir/pkg/apperr"
)

// TestParseJudgeResultRequiresStructuredJSON 确认判题器输出必须是结构化结果。
func TestParseJudgeResultRequiresStructuredJSON(t *testing.T) {
	result, err := parseJudgeResult([]byte(`{"passed":true,"score":100,"max_score":100,"details":[{"case":"basic","passed":true}]}`), 65536)
	if err != nil {
		t.Fatalf("valid judge result rejected: %v", err)
	}
	if !result.Passed || result.Score != 100 || result.MaxScore != 100 {
		t.Fatalf("unexpected judge result: %#v", result)
	}

	if _, err := parseJudgeResult([]byte(`not-json`), 65536); err == nil {
		t.Fatalf("invalid judge output must be rejected")
	}
}

// TestParseJudgeResultRejectsSensitiveDetails 确认判题器输出不能把答案、flag 或套件源码写入结果详情。
func TestParseJudgeResultRejectsSensitiveDetails(t *testing.T) {
	raw := []byte(`{"passed":false,"score":0,"max_score":100,"details":[{"case":"secret","expected":"revert","actual":"ok","answer":"transfer owner flag"}]}`)

	_, err := parseJudgeResult(raw, 65536)

	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrJudgeTaskRunFail.Code {
		t.Fatalf("sensitive judge details must be rejected with %s, got %v", apperr.ErrJudgeTaskRunFail.Code, err)
	}
}

// TestParseJudgeResultRequiresExplainableDetails 确认 J1/J4 输出必须包含可解释详情数组。
func TestParseJudgeResultRequiresExplainableDetails(t *testing.T) {
	for _, raw := range [][]byte{
		[]byte(`{"passed":true,"score":100,"max_score":100}`),
		[]byte(`{"passed":true,"score":100,"max_score":100,"details":{"case":"basic","passed":true}}`),
		[]byte(`{"passed":true,"score":100,"max_score":100,"details":[{"case":"basic"}]}`),
	} {
		if _, err := parseJudgeResult(raw, 65536); err == nil {
			t.Fatalf("judge result without explainable details must be rejected: %s", raw)
		}
	}
}

// TestParseJudgeResultUsesConfiguredDetailsLimit 确认结果详情大小上限由调用方配置控制。
func TestParseJudgeResultUsesConfiguredDetailsLimit(t *testing.T) {
	raw := []byte(`{"passed":true,"score":100,"max_score":100,"details":[{"case":"basic","passed":true}]}`)

	if _, err := parseJudgeResult(raw, 1); err == nil {
		t.Fatalf("judge result exceeding configured details limit must be rejected")
	}
	if _, err := parseJudgeResult(raw, 65536); err != nil {
		t.Fatalf("judge result within configured details limit must be accepted: %v", err)
	}
}

// TestJudgeTimeoutCauseMapsToTimeoutStatus 确认 M2 沙箱超时和 context 超时会进入 M3 timeout 分类。
func TestJudgeTimeoutCauseMapsToTimeoutStatus(t *testing.T) {
	for _, cause := range []error{apperr.ErrSandboxTimeout, context.DeadlineExceeded} {
		status, mapped := classifyJudgeTaskFailure(cause)
		if status != JudgeTaskTimeout {
			t.Fatalf("timeout cause %v must map to status %d, got %d", cause, JudgeTaskTimeout, status)
		}
		if ae, ok := apperr.As(mapped); !ok || ae.Code != apperr.ErrJudgeTaskTimeout.Code {
			t.Fatalf("timeout cause %v must map to %s, got %v", cause, apperr.ErrJudgeTaskTimeout.Code, mapped)
		}
	}
}

// TestDequeueTasksRequiresRedisQueue 确认 worker 不走数据库扫描替代异步队列。
func TestDequeueTasksRequiresRedisQueue(t *testing.T) {
	svc := &Service{}

	_, err := svc.dequeueTasks(context.Background())

	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrJudgeTaskQueuedFail.Code {
		t.Fatalf("missing redis queue must return %s, got %v", apperr.ErrJudgeTaskQueuedFail.Code, err)
	}
}

// TestDequeueTasksDoesNotScanDatabase 确认生产代码中没有 Redis 缺失时的 DB 队列路径。
func TestDequeueTasksDoesNotScanDatabase(t *testing.T) {
	data, err := os.ReadFile("service_worker.go")
	if err != nil {
		t.Fatalf("read service_worker.go: %v", err)
	}
	body := string(data)
	start := strings.Index(body, "func (s *Service) dequeueTasks(")
	end := strings.Index(body, "// markTaskJudgingAcrossTenant")
	if start < 0 || end < start {
		t.Fatalf("dequeueTasks function block not found")
	}
	block := body[start:end]
	if strings.Contains(block, "ListQueuedJudgeTasks") {
		t.Fatalf("dequeueTasks must not scan DB as a queue replacement:\n%s", block)
	}
}

// TestJudgeWorkerBatchSizeControlsQueuePop 确认 JUDGE_WORKER_BATCH_SIZE 真实控制每轮队列消费量。
func TestJudgeWorkerBatchSizeControlsQueuePop(t *testing.T) {
	data, err := os.ReadFile("service_worker.go")
	if err != nil {
		t.Fatalf("read service_worker.go: %v", err)
	}
	body := string(data)
	dequeueBlock := functionSource(body, "dequeueTasks")
	if !strings.Contains(dequeueBlock, "s.normalizedWorkerBatchSize()") {
		t.Fatalf("dequeueTasks must use normalizedWorkerBatchSize for Redis ZPopMin count")
	}
	if strings.Contains(dequeueBlock, `ZPopMin(ctx, "judge:queue", 1)`) {
		t.Fatalf("dequeueTasks must not hard-code a single task per worker tick")
	}
	runBlock := functionSource(body, "RunWorkerOnce")
	if !strings.Contains(runBlock, "for _, taskID := range taskIDs") {
		t.Fatalf("RunWorkerOnce must process every dequeued task in the configured batch")
	}
	if !strings.Contains(runBlock, "errors.Join") {
		t.Fatalf("RunWorkerOnce must collect per-task errors instead of dropping later popped tasks")
	}
	if strings.Contains(runBlock, "return s.retryOrFail(workerCtx, task, err)") {
		t.Fatalf("RunWorkerOnce must not stop the batch immediately after one task failure")
	}
}

// TestProcessTaskRecyclesFreshSandboxBeforeCompleting 确认 fresh judge 沙箱回收成功后才写结果和终态 outbox。
func TestProcessTaskRecyclesFreshSandboxBeforeCompleting(t *testing.T) {
	data, err := os.ReadFile("service_worker.go")
	if err != nil {
		t.Fatalf("read service_worker.go: %v", err)
	}
	body := string(data)
	start := strings.Index(body, "func (s *Service) processTask(")
	end := strings.Index(body, "// loadJudgerForTask")
	if start < 0 || end < start {
		t.Fatalf("processTask function block not found")
	}
	block := body[start:end]
	if strings.Contains(block, "defer s.recycleJudgeSandbox") {
		t.Fatalf("fresh judge sandbox recycle must be an explicit step before completion, not a deferred log-only cleanup")
	}
	recycleIdx := strings.Index(block, "s.recycleJudgeSandbox(ctx, task.TenantID, recycleSourceRef)")
	completeIdx := strings.Index(block, "s.completeTask(ctx, task, result, sandboxID)")
	if recycleIdx < 0 || completeIdx < 0 || recycleIdx > completeIdx {
		t.Fatalf("processTask must recycle fresh sandbox before completeTask")
	}
}

// TestOnchainFreshJudgeInjectsSubmission 确认 J2 fresh 链上断言会先注入学生提交代码。
func TestOnchainFreshJudgeInjectsSubmission(t *testing.T) {
	data, err := os.ReadFile("service_worker.go")
	if err != nil {
		t.Fatalf("read service_worker.go: %v", err)
	}
	body := string(data)
	start := strings.Index(body, "func (s *Service) processTask(")
	end := strings.Index(body, "// loadJudgerForTask")
	if start < 0 || end < start {
		t.Fatalf("processTask function block not found")
	}
	block := body[start:end]
	if !strings.Contains(block, "judger.Type == JudgerTypeOnchainAssert") {
		t.Fatalf("J2 onchain assert in fresh mode must inject submission before chain steps")
	}
}

// TestJudgeInputArchivesAreSanitizedBeforeInjection 确认提交和套件归档先后端校验重打包,不能直接在容器内解原始对象。
func TestJudgeInputArchivesAreSanitizedBeforeInjection(t *testing.T) {
	data, err := os.ReadFile("service_worker.go")
	if err != nil {
		t.Fatalf("read service_worker.go: %v", err)
	}
	body := string(data)
	start := strings.Index(body, "func (s *Service) putObjectIntoSandbox(")
	end := strings.Index(body, "// runJudgeCommand")
	if start < 0 || end < start {
		t.Fatalf("putObjectIntoSandbox function block not found")
	}
	block := body[start:end]
	if !strings.Contains(block, "safeJudgeInputArchive") {
		t.Fatalf("putObjectIntoSandbox must sanitize and repack judge input archives before writing to sandbox")
	}
	if strings.Contains(block, "ContentBase64: base64.StdEncoding.EncodeToString(data)") {
		t.Fatalf("putObjectIntoSandbox must not inject raw object bytes into container tar extraction path")
	}
}

// TestSafeJudgeInputArchiveRejectsTraversal 确认恶意提交归档不会进入 judge 沙箱解包阶段。
func TestSafeJudgeInputArchiveRejectsTraversal(t *testing.T) {
	cfg := config.JudgeConfig{InputArchiveMaxFiles: 10, InputArchiveMaxUnpackedBytes: 1024}
	if _, err := safeJudgeInputArchive(testJudgeArchive(t, map[string]string{"src/main.sol": "contract C {}"}), cfg); err != nil {
		t.Fatalf("safe judge archive rejected: %v", err)
	}
	if _, err := safeJudgeInputArchive(testJudgeArchive(t, map[string]string{"../suite/evil.js": "pwn"}), cfg); err == nil {
		t.Fatalf("judge input archive traversal must be rejected")
	}
}

// TestSafeJudgeInputArchiveAppliesConfiguredLimits 确认判题输入归档规模来自 JudgeConfig。
func TestSafeJudgeInputArchiveAppliesConfiguredLimits(t *testing.T) {
	if _, err := safeJudgeInputArchive(testJudgeArchive(t, map[string]string{
		"a.txt": "a",
		"b.txt": "b",
	}), config.JudgeConfig{InputArchiveMaxFiles: 1, InputArchiveMaxUnpackedBytes: 1024}); err == nil {
		t.Fatalf("judge input archive with too many files must be rejected")
	}
	if _, err := safeJudgeInputArchive(testJudgeArchive(t, map[string]string{
		"a.txt": "abcd",
	}), config.JudgeConfig{InputArchiveMaxFiles: 10, InputArchiveMaxUnpackedBytes: 3}); err == nil {
		t.Fatalf("judge input archive exceeding unpacked bytes must be rejected")
	}
}

// TestJudgeWorkerWritesAuditForCompletionAndFailure 确认判题完成/失败关键操作进入统一 audit_log。
func TestJudgeWorkerWritesAuditForCompletionAndFailure(t *testing.T) {
	data, err := os.ReadFile("service_worker.go")
	if err != nil {
		t.Fatalf("read service_worker.go: %v", err)
	}
	body := string(data)
	completeStart := strings.Index(body, "func (s *Service) completeTask(")
	retryStart := strings.Index(body, "func (s *Service) retryOrFail(")
	if completeStart < 0 || retryStart < completeStart {
		t.Fatalf("completeTask/retryOrFail function blocks not found")
	}
	completeBlock := body[completeStart:retryStart]
	if !strings.Contains(completeBlock, "auditActionTaskComplete") || !strings.Contains(completeBlock, "s.writeAudit(") {
		t.Fatalf("completeTask must write audit_log for judge result completion")
	}
	retryEnd := strings.Index(body[retryStart:], "// recycleJudgeSandbox")
	if retryEnd < 0 {
		t.Fatalf("retryOrFail block end not found")
	}
	retryBlock := body[retryStart : retryStart+retryEnd]
	if !strings.Contains(retryBlock, "auditActionTaskFailed") || !strings.Contains(retryBlock, "s.writeAudit(") {
		t.Fatalf("retryOrFail must write audit_log for failed terminal judge task")
	}
}

// TestJudgeTerminalEventsUseRecoverableOutbox 确认判题终态事件先写 M3 自有 outbox,避免结果落库后发布失败造成状态裂缝。
func TestJudgeTerminalEventsUseRecoverableOutbox(t *testing.T) {
	migration, err := os.ReadFile("../../../db/migrations/0004_judge.up.sql")
	if err != nil {
		t.Fatalf("read judge migration: %v", err)
	}
	if !strings.Contains(string(migration), "CREATE TABLE judge_event_outbox") {
		t.Fatalf("M3 must persist terminal events in judge_event_outbox")
	}
	sql, err := os.ReadFile("../../../db/queries/judge.sql")
	if err != nil {
		t.Fatalf("read judge sql: %v", err)
	}
	for _, required := range []string{
		"CreateJudgeEventOutbox",
		"ListPendingJudgeEventOutbox",
		"MarkJudgeEventOutboxPublished",
		"FailJudgeEventOutbox",
	} {
		if !strings.Contains(string(sql), required) {
			t.Fatalf("judge SQL must include recoverable outbox query %s", required)
		}
	}
	worker, err := os.ReadFile("service_worker.go")
	if err != nil {
		t.Fatalf("read service_worker.go: %v", err)
	}
	body := string(worker)
	completeBlock := functionSource(body, "completeTask")
	if !strings.Contains(completeBlock, "newJudgeEventOutbox") || !strings.Contains(completeBlock, "repo.completeTaskResult") {
		t.Fatalf("completeTask must write judge.completed outbox in the result transaction")
	}
	if strings.Contains(completeBlock, "publishJudgeCompleted") {
		t.Fatalf("completeTask must not directly publish judge.completed after result persistence")
	}
	if !strings.Contains(body, "PublishPendingJudgeEvents") {
		t.Fatalf("M3 must expose a recoverable outbox dispatcher")
	}
	if strings.Contains(completeBlock, "return s.PublishPendingJudgeEvents(ctx)") {
		t.Fatalf("completeTask must not return outbox dispatch failure after terminal state is persisted")
	}
}

// TestJudgeTerminalAuditPrecedesResultPersistence 确认审计失败不会留下已完成结果或终态事件。
func TestJudgeTerminalAuditPrecedesResultPersistence(t *testing.T) {
	worker, err := os.ReadFile("service_worker.go")
	if err != nil {
		t.Fatalf("read service_worker.go: %v", err)
	}
	completeBlock := functionSource(string(worker), "completeTask")
	auditIdx := strings.Index(completeBlock, "s.writeAudit(")
	resultIdx := strings.Index(completeBlock, "repo.completeTaskResult")
	outboxIdx := strings.Index(completeBlock, "newJudgeEventOutbox")
	if auditIdx < 0 || resultIdx < 0 || outboxIdx < 0 || auditIdx > resultIdx || auditIdx > outboxIdx {
		t.Fatalf("completeTask must write audit before result/outbox persistence")
	}
	retryBlock := functionSource(string(worker), "retryOrFail")
	failAuditIdx := strings.Index(retryBlock, "s.writeAudit(")
	failIdx := strings.Index(retryBlock, "repo.failTaskWithOutbox")
	failOutboxIdx := strings.Index(retryBlock, "newJudgeEventOutbox")
	if failAuditIdx < 0 || failIdx < 0 || failOutboxIdx < 0 || failAuditIdx > failIdx || failAuditIdx > failOutboxIdx {
		t.Fatalf("retryOrFail must write failed audit before failed status/outbox persistence")
	}
}

func testJudgeArchive(t *testing.T, files map[string]string) []byte {
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

// TestJudgeTaskContextBindsServiceSourceRef 确认 worker 调 M2 链能力时使用 source_ref 做内部归属校验。
func TestJudgeTaskContextBindsServiceSourceRef(t *testing.T) {
	data, err := os.ReadFile("service_worker.go")
	if err != nil {
		t.Fatalf("read service_worker.go: %v", err)
	}
	body := string(data)
	start := strings.Index(body, "func judgeTaskContext(")
	if start < 0 {
		t.Fatalf("judgeTaskContext function block not found")
	}
	block := body[start:]
	if !strings.Contains(block, "auth.WithServiceSourceRef(ctx, task.SourceRef)") {
		t.Fatalf("judgeTaskContext must bind task.SourceRef so M2 validates reuse sandbox access by source_ref")
	}
}

// TestJudgeWorkerOperationalLimitsComeFromConfig 确认 worker 运行阈值来自统一配置,不在模块内硬编码。
func TestJudgeWorkerOperationalLimitsComeFromConfig(t *testing.T) {
	data, err := os.ReadFile("service_worker.go")
	if err != nil {
		t.Fatalf("read service_worker.go: %v", err)
	}
	body := string(data)
	if strings.Contains(body, "maxJudgeResultDetailsBytes = 64 * 1024") {
		t.Fatalf("judge result detail size limit must come from config.JudgeConfig")
	}
	if strings.Contains(body, "TimeoutSec: 60") {
		t.Fatalf("judge input injection timeout must come from config.JudgeConfig")
	}
}
