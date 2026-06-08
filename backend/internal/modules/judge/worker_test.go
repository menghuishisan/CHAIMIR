// M3 worker 测试:覆盖判题输出解析、失败分类与 worker 可复现输入处理规则。
package judge

import (
	"context"
	"os"
	"strings"
	"testing"

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

// TestDequeueTaskRequiresRedisQueue 确认 worker 不走数据库扫描替代异步队列。
func TestDequeueTaskRequiresRedisQueue(t *testing.T) {
	svc := &Service{}

	_, err := svc.dequeueTask(context.Background())

	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrJudgeTaskQueuedFail.Code {
		t.Fatalf("missing redis queue must return %s, got %v", apperr.ErrJudgeTaskQueuedFail.Code, err)
	}
}

// TestDequeueTaskDoesNotScanDatabase 确认生产代码中没有 Redis 缺失时的 DB 队列路径。
func TestDequeueTaskDoesNotScanDatabase(t *testing.T) {
	data, err := os.ReadFile("worker.go")
	if err != nil {
		t.Fatalf("read worker.go: %v", err)
	}
	body := string(data)
	start := strings.Index(body, "func (s *Service) dequeueTask(")
	end := strings.Index(body, "// markTaskJudgingAcrossTenant")
	if start < 0 || end < start {
		t.Fatalf("dequeueTask function block not found")
	}
	block := body[start:end]
	if strings.Contains(block, "ListQueuedJudgeTasks") {
		t.Fatalf("dequeueTask must not scan DB as a queue replacement:\n%s", block)
	}
}

// TestProcessTaskRecyclesFreshSandboxBeforeCompleting 确认 fresh judge 沙箱回收成功后才写结果和发布完成事件。
func TestProcessTaskRecyclesFreshSandboxBeforeCompleting(t *testing.T) {
	data, err := os.ReadFile("worker.go")
	if err != nil {
		t.Fatalf("read worker.go: %v", err)
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
	data, err := os.ReadFile("worker.go")
	if err != nil {
		t.Fatalf("read worker.go: %v", err)
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

// TestJudgeWorkerWritesAuditForCompletionAndFailure 确认判题完成/失败关键操作进入统一 audit_log。
func TestJudgeWorkerWritesAuditForCompletionAndFailure(t *testing.T) {
	data, err := os.ReadFile("worker.go")
	if err != nil {
		t.Fatalf("read worker.go: %v", err)
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

// TestJudgeTaskContextBindsServiceSourceRef 确认 worker 调 M2 链能力时使用 source_ref 做内部归属校验。
func TestJudgeTaskContextBindsServiceSourceRef(t *testing.T) {
	data, err := os.ReadFile("worker.go")
	if err != nil {
		t.Fatalf("read worker.go: %v", err)
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
	data, err := os.ReadFile("worker.go")
	if err != nil {
		t.Fatalf("read worker.go: %v", err)
	}
	body := string(data)
	if strings.Contains(body, "maxJudgeResultDetailsBytes = 64 * 1024") {
		t.Fatalf("judge result detail size limit must come from config.JudgeConfig")
	}
	if strings.Contains(body, "TimeoutSec: 60") {
		t.Fatalf("judge input injection timeout must come from config.JudgeConfig")
	}
}
