// judge service_worker 文件实现 M3 队列消费、沙箱判题执行和终态事件 outbox 发布。
package judge

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strconv"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/timex"
	"chaimir/internal/platform/upload"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
)

// JudgeExecutionResult 是一次 worker 执行后可落库的脱敏结果。
type JudgeExecutionResult struct {
	Passed          bool                `json:"passed"`
	Score           int32               `json:"score"`
	MaxScore        int32               `json:"max_score"`
	Details         []JudgeResultDetail `json:"details"`
	JudgeSandboxRef string              `json:"judge_sandbox_ref,omitempty"`
}

// RunWorkerOnce 领取一批排队任务并发布此前积压的终态事件。
func (s *Service) RunWorkerOnce(ctx context.Context) error {
	if err := s.publishPendingOutbox(ctx); err != nil {
		return err
	}
	limit := int32(s.cfg.WorkerBatchSize)
	if limit <= 0 {
		limit = 1
	}
	var tasks []JudgeTask
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		tasks, err = tx.DequeueJudgeTasks(ctx, limit)
		if err != nil {
			return apperr.ErrJudgeTaskPersistFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return err
	}
	for _, task := range tasks {
		if err := s.processTask(ctx, task); err != nil {
			logging.ErrorContext(ctx, "judge task processing failed", err.Error(), slog.Int64("tenant_id", task.TenantID), slog.Int64("task_id", task.ID))
		}
	}
	return s.publishPendingOutbox(ctx)
}

// processTask 执行单个判题任务,失败时按任务重试策略回队列或落失败终态。
func (s *Service) processTask(ctx context.Context, task JudgeTask) error {
	s.publishProgress(ctx, task.TenantID, task.ID, JudgeTaskStatusJudging, ProgressStageJudging, "判题任务正在执行")
	result, err := s.executeTask(ctx, task)
	if err != nil {
		return s.retryOrFail(ctx, task, err)
	}
	return s.completeTask(ctx, task, result)
}

// executeTask 根据判题器类型选择后端策略或沙箱命令执行路径。
func (s *Service) executeTask(ctx context.Context, task JudgeTask) (JudgeExecutionResult, error) {
	if task.InputSnapshot.JudgerType == JudgerTypeManual {
		return JudgeExecutionResult{}, apperr.ErrJudgeTaskStateInvalid
	}
	if !needsSandbox(task) {
		result, handled, err := s.executeJudgerStrategy(ctx, task, 0)
		if handled && err != nil {
			return JudgeExecutionResult{}, err
		}
		if handled {
			return normalizeExecutionResult(result, task.InputSnapshot.MaxScore, s.cfg.ResultDetailsMaxBytes)
		}
	}
	sandboxID, fresh, err := s.resolveSandbox(ctx, task)
	if err != nil {
		return JudgeExecutionResult{}, err
	}
	if fresh {
		defer s.destroyJudgeSandbox(ctx, task, sandboxID)
	}
	if result, handled, err := s.executeJudgerStrategy(ctx, task, sandboxID); handled {
		if err != nil {
			return JudgeExecutionResult{}, err
		}
		result.JudgeSandboxRef = judgeSandboxRef(sandboxID)
		return normalizeExecutionResult(result, task.InputSnapshot.MaxScore, s.cfg.ResultDetailsMaxBytes)
	}
	if task.InputSnapshot.JudgerType != JudgerTypeTestcase && task.InputSnapshot.JudgerType != JudgerTypeStaticScan {
		return JudgeExecutionResult{}, apperr.ErrJudgerConfigInvalid
	}
	return s.runJudgeCommand(ctx, task, sandboxID)
}

// needsSandbox 判断判题是否需要 M2 沙箱执行或链能力。
func needsSandbox(task JudgeTask) bool {
	switch task.InputSnapshot.JudgerType {
	case JudgerTypeFlag:
		return strings.TrimSpace(stringValue(task.InputSnapshot.Expectation["flag_chain_target"])) != ""
	case JudgerTypeSimCheckpoint:
		return false
	default:
		return true
	}
}

// resolveSandbox 返回本次判题使用的沙箱 ID,fresh 模式会创建并等待干净判题沙箱就绪。
func (s *Service) resolveSandbox(ctx context.Context, task JudgeTask) (int64, bool, error) {
	if task.SandboxMode == JudgeSandboxModeReuse {
		id, err := parseSandboxRef(task.TargetSandboxRef)
		if err != nil {
			return 0, false, err
		}
		info, err := s.sandbox.GetSandbox(ctx, task.TenantID, id)
		if err != nil {
			return 0, false, apperr.ErrJudgeWorkerFailed.WithCause(err)
		}
		if info.SourceRef != task.SourceRef || info.Status == contracts.SandboxStatusDestroyed || info.Status == contracts.SandboxStatusFailed {
			return 0, false, apperr.ErrJudgeTaskStateInvalid
		}
		return id, false, nil
	}
	info, err := s.sandbox.CreateSandbox(ctx, contracts.SandboxCreateRequest{
		TenantID:            task.TenantID,
		RuntimeCode:         task.InputSnapshot.RuntimeCode,
		RuntimeImageVersion: task.InputSnapshot.RuntimeImageVersion,
		ToolCodes:           task.InputSnapshot.ToolCodes,
		InitCodeRef:         task.CodeStorageKey,
		InitScriptRef:       task.InputSnapshot.InitScriptRef,
		OwnerAccountID:      task.SubmitterID,
		SourceRef:           task.SourceRef,
		KeepAlive:           false,
		SnapshotEnabled:     false,
	})
	if err != nil {
		return 0, false, apperr.ErrJudgeWorkerFailed.WithCause(err)
	}
	if err := s.waitSandboxReady(ctx, task, info.SandboxID); err != nil {
		return info.SandboxID, true, err
	}
	if err := s.injectPrivateSuite(ctx, task, info.SandboxID); err != nil {
		return info.SandboxID, true, err
	}
	return info.SandboxID, true, nil
}

// waitSandboxReady 轮询 M2 沙箱状态直到运行时可执行或超时。
func (s *Service) waitSandboxReady(ctx context.Context, task JudgeTask, sandboxID int64) error {
	timeout := time.Duration(task.InputSnapshot.TimeoutSec+sandboxReadyGraceSeconds) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	deadline := timex.Now().Add(timeout)
	interval := time.Duration(s.cfg.SandboxReadyPollIntervalMs) * time.Millisecond
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		info, err := s.sandbox.GetSandbox(ctx, task.TenantID, sandboxID)
		if err != nil {
			return apperr.ErrJudgeWorkerFailed.WithCause(err)
		}
		if info.Status == contracts.SandboxStatusFailed || info.Status == contracts.SandboxStatusDestroyed {
			return apperr.ErrJudgeWorkerFailed
		}
		if info.Status == contracts.SandboxStatusRunning && info.Phase >= contracts.SandboxPhaseReady {
			return nil
		}
		if timex.Now().After(deadline) {
			return apperr.ErrJudgeTimeout
		}
		select {
		case <-ctx.Done():
			return apperr.ErrJudgeWorkerFailed.WithCause(ctx.Err())
		case <-ticker.C:
		}
	}
}

// injectPrivateSuite 读取 M5 套件对象并通过 M2 私有卷域安全注入。
func (s *Service) injectPrivateSuite(ctx context.Context, task JudgeTask, sandboxID int64) error {
	if strings.TrimSpace(task.InputSnapshot.SuiteRef) == "" {
		return nil
	}
	injectCtx, cancel := context.WithTimeout(ctx, time.Duration(s.cfg.InputInjectTimeoutSeconds)*time.Second)
	defer cancel()
	name, data, err := s.readObjectRef(injectCtx, task.InputSnapshot.SuiteRef)
	if err != nil {
		return apperr.ErrJudgeSpecUnavailable.WithCause(err)
	}
	limits := upload.ArchiveLimits{MaxFiles: s.cfg.InputArchiveMaxFiles, MaxUnpackedBytes: s.cfg.InputArchiveMaxUnpackedBytes}
	tarball, err := upload.SafeArchiveTar(name, data, limits)
	if err != nil {
		return apperr.ErrJudgeInputArchiveInvalid.WithCause(err)
	}
	archiveName := strings.TrimSpace(task.InputSnapshot.SuiteArchiveName)
	if archiveName == "" {
		archiveName = name
	}
	return s.sandbox.PutSandboxPrivateArchive(injectCtx, contracts.SandboxPrivateArchiveInjectRequest{
		TenantID:      task.TenantID,
		SandboxID:     sandboxID,
		SourceRef:     task.SourceRef,
		Domain:        contracts.SandboxPrivateDomainJudge,
		ArchiveName:   archiveName,
		ContentBase64: base64.StdEncoding.EncodeToString(tarball),
	})
}

// runJudgeCommand 执行平台配置的受控命令并解析结构化 stdout。
func (s *Service) runJudgeCommand(ctx context.Context, task JudgeTask, sandboxID int64) (JudgeExecutionResult, error) {
	if len(task.InputSnapshot.Command) == 0 {
		return JudgeExecutionResult{}, apperr.ErrJudgerConfigInvalid
	}
	stdin, err := encodeJSONBytes(map[string]any{
		"task_id":      task.ID,
		"source_ref":   task.SourceRef,
		"problem_ref":  task.ProblemRef,
		"suite_ref":    task.InputSnapshot.SuiteRef,
		"expectation":  task.InputSnapshot.Expectation,
		"extra_input":  task.InputSnapshot.ExtraInput,
		"max_score":    task.InputSnapshot.MaxScore,
		"version_hash": task.InputSnapshot.VersionHash,
		"rejudge":      task.InputSnapshot.Rejudge,
	})
	if err != nil {
		return JudgeExecutionResult{}, apperr.ErrJudgeWorkerFailed.WithCause(err)
	}
	execResult, err := s.sandbox.ExecSandboxCommand(ctx, contracts.SandboxExecRequest{
		TenantID:   task.TenantID,
		SandboxID:  sandboxID,
		SourceRef:  task.SourceRef,
		Command:    task.InputSnapshot.Command,
		Stdin:      stdin,
		TimeoutSec: task.InputSnapshot.TimeoutSec,
	})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return JudgeExecutionResult{}, apperr.ErrJudgeTimeout.WithCause(err)
		}
		return JudgeExecutionResult{}, apperr.ErrJudgeWorkerFailed.WithCause(err)
	}
	result, err := decodeCommandResult(execResult.Stdout, s.cfg.ResultDetailsMaxBytes)
	if err != nil {
		return JudgeExecutionResult{}, err
	}
	result.JudgeSandboxRef = judgeSandboxRef(sandboxID)
	return normalizeExecutionResult(result, task.InputSnapshot.MaxScore, s.cfg.ResultDetailsMaxBytes)
}

// completeTask 在同一租户事务中保存结果、完成任务并写 outbox。
func (s *Service) completeTask(ctx context.Context, task JudgeTask, result JudgeExecutionResult) error {
	var completed JudgeTask
	if err := s.store.TenantTx(ctx, task.TenantID, func(ctx context.Context, tx TxStore) error {
		saved, err := tx.UpsertJudgeResult(ctx, JudgeResult{
			TaskID:          task.ID,
			TenantID:        task.TenantID,
			Passed:          result.Passed,
			Score:           result.Score,
			MaxScore:        result.MaxScore,
			Details:         result.Details,
			JudgeSandboxRef: result.JudgeSandboxRef,
			IsRejudge:       task.InputSnapshot.Rejudge,
		})
		if err != nil {
			return apperr.ErrJudgeTaskPersistFailed.WithCause(err)
		}
		completed, err = tx.CompleteJudgeTask(ctx, task.TenantID, task.ID)
		if err != nil {
			return apperr.ErrJudgeTaskPersistFailed.WithCause(err)
		}
		payload := contracts.JudgeCompletedEvent{TenantID: task.TenantID, TaskID: task.ID, SourceRef: task.SourceRef, Status: contracts.JudgeTaskStatusDone, Score: saved.Score, Passed: saved.Passed, FinishedAt: saved.JudgedAt}
		if _, err := tx.CreateOutbox(ctx, s.ids.Generate(), task.TenantID, task.ID, contracts.SubjectJudgeCompleted, payload); err != nil {
			return apperr.ErrJudgeTaskPersistFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return err
	}
	s.publishProgress(ctx, task.TenantID, task.ID, completed.Status, ProgressStageDone, "判题任务已完成")
	return nil
}

// retryOrFail 按任务快照中的重试上限回队列或写失败终态事件。
func (s *Service) retryOrFail(ctx context.Context, task JudgeTask, cause error) error {
	reason := safeFailureReason(cause)
	intermediateStatus := JudgeTaskStatusError
	if isJudgeTimeout(cause) {
		intermediateStatus = JudgeTaskStatusTimeout
	}
	if err := s.markFailureIntermediate(ctx, task, intermediateStatus, reason); err != nil {
		return err
	}
	if task.RetryCount < task.MaxRetries {
		var retry JudgeTask
		if err := s.store.TenantTx(ctx, task.TenantID, func(ctx context.Context, tx TxStore) error {
			var err error
			retry, err = tx.RetryJudgeTask(ctx, task.TenantID, task.ID, reason)
			if err != nil {
				return apperr.ErrJudgeTaskPersistFailed.WithCause(err)
			}
			return nil
		}); err != nil {
			return err
		}
		s.publishProgress(ctx, task.TenantID, task.ID, retry.Status, ProgressStageQueued, "判题任务将自动重试")
		return nil
	}
	var failed JudgeTask
	if err := s.store.TenantTx(ctx, task.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		failed, err = tx.FailJudgeTask(ctx, task.TenantID, task.ID, reason)
		if err != nil {
			return apperr.ErrJudgeTaskPersistFailed.WithCause(err)
		}
		payload := contracts.JudgeFailedEvent{TenantID: task.TenantID, TaskID: task.ID, SourceRef: task.SourceRef, Reason: reason, FailedAt: timex.Now()}
		if _, err := tx.CreateOutbox(ctx, s.ids.Generate(), task.TenantID, task.ID, contracts.SubjectJudgeFailed, payload); err != nil {
			return apperr.ErrJudgeTaskPersistFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return err
	}
	s.publishProgress(ctx, task.TenantID, task.ID, failed.Status, ProgressStageFailed, "判题任务执行失败")
	return nil
}

// markFailureIntermediate 按状态机先持久化 timeout/error 中间态,再进入重试或失败终态。
func (s *Service) markFailureIntermediate(ctx context.Context, task JudgeTask, status int16, reason string) error {
	return s.store.TenantTx(ctx, task.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		if status == JudgeTaskStatusTimeout {
			_, err = tx.MarkJudgeTaskTimeout(ctx, task.TenantID, task.ID, reason)
		} else {
			_, err = tx.MarkJudgeTaskError(ctx, task.TenantID, task.ID, reason)
		}
		if err != nil {
			return apperr.ErrJudgeTaskPersistFailed.WithCause(err)
		}
		return nil
	})
}

// publishPendingOutbox 发布已落库的终态事件并回写发布状态。
func (s *Service) publishPendingOutbox(ctx context.Context) error {
	var items []JudgeEventOutbox
	limit := int32(s.cfg.WorkerBatchSize)
	if limit <= 0 {
		limit = 10
	}
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		items, err = tx.ListPendingOutbox(ctx, limit)
		if err != nil {
			return apperr.ErrJudgeEventPublishFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return err
	}
	for _, item := range items {
		var payload any
		if err := jsonx.DecodeStrict(item.Payload, &payload); err != nil {
			s.recordOutboxPublishFailure(ctx, item, err)
			return apperr.ErrJudgeEventPublishFailed.WithCause(err)
		}
		if err := s.bus.Publish(ctx, item.Subject, payload); err != nil {
			s.recordOutboxPublishFailure(ctx, item, err)
			return apperr.ErrJudgeEventPublishFailed.WithCause(err)
		}
		if err := s.markOutboxPublished(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// recordOutboxPublishFailure 记录发布失败状态,失败标记本身也必须进入结构化日志。
func (s *Service) recordOutboxPublishFailure(ctx context.Context, item JudgeEventOutbox, cause error) {
	if err := s.markOutboxFailed(ctx, item, cause); err != nil {
		logging.ErrorContext(ctx, "judge outbox failure mark failed", err.Error(), slog.Int64("tenant_id", item.TenantID), slog.Int64("task_id", item.TaskID), slog.Int64("outbox_id", item.ID), slog.String("subject", item.Subject))
	}
}

// isJudgeTimeout 判断错误链是否表示判题超时。
func isJudgeTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if app, ok := apperr.As(err); ok && app.Code == apperr.CodeJudgeTimeout {
		return true
	}
	return false
}

// resultDetailsJSONBytes 统计结果详情 JSON 大小,复用平台 JSON 边界而不是手写估算。
func resultDetailsJSONBytes(details []JudgeResultDetail) (int, error) {
	raw, err := jsonx.AnyBytes(details, apperr.ErrInternal)
	if err != nil {
		return 0, err
	}
	return len(raw), nil
}

// hasDeterministicExpectation 判断快照期望是否包含非确定性显式标记。
func hasDeterministicExpectation(expectation map[string]any) bool {
	if expectation == nil {
		return true
	}
	for _, key := range []string{"random", "wall_clock", "external_network"} {
		if value, ok := expectation[key]; ok && !reflect.ValueOf(value).IsZero() {
			return false
		}
	}
	return true
}

// markOutboxPublished 用特权事务回写 outbox 发布成功状态。
func (s *Service) markOutboxPublished(ctx context.Context, item JudgeEventOutbox) error {
	return s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		_, err := tx.MarkOutboxPublished(ctx, item.TenantID, item.ID)
		if err != nil {
			return apperr.ErrJudgeEventPublishFailed.WithCause(err)
		}
		return nil
	})
}

// markOutboxFailed 用特权事务记录 outbox 发布失败原因。
func (s *Service) markOutboxFailed(ctx context.Context, item JudgeEventOutbox, cause error) error {
	return s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		_, err := tx.MarkOutboxFailed(ctx, item.TenantID, item.ID, safeFailureReason(cause))
		if err != nil {
			return apperr.ErrJudgeEventPublishFailed.WithCause(err)
		}
		return nil
	})
}

// executeJudgerSelftest 用判题器声明的自检样例验证配置可执行性。
func (s *Service) executeJudgerSelftest(ctx context.Context, j Judger) error {
	if len(j.ResourceSpec.Selftest) == 0 {
		return apperr.ErrJudgerConfigInvalid
	}
	expectation, _ := j.ResourceSpec.Selftest["expectation"].(map[string]any)
	extra, _ := j.ResourceSpec.Selftest["extra_input"].(map[string]any)
	snapshot, err := s.snapshotExpectationForJudger(j.Type, expectation, extra)
	if err != nil {
		return err
	}
	task := JudgeTask{
		ID:          s.ids.Generate(),
		TenantID:    int64FromAny(j.ResourceSpec.Selftest["tenant_id"]),
		SubmitterID: int64FromAny(j.ResourceSpec.Selftest["submitter_id"]),
		SourceRef:   stringValue(j.ResourceSpec.Selftest["source_ref"]),
		InputSnapshot: JudgeInputSnapshot{
			JudgerType:          j.Type,
			RuntimeCode:         j.ResourceSpec.RuntimeCode,
			RuntimeImageVersion: j.ResourceSpec.RuntimeImageVersion,
			GenesisRef:          j.ResourceSpec.GenesisRef,
			ToolCodes:           append([]string(nil), j.ResourceSpec.ToolCodes...),
			InitScriptRef:       j.ResourceSpec.InitScriptRef,
			Command:             append([]string(nil), j.ResourceSpec.Command...),
			TimeoutSec:          timeoutForSnapshot(j),
			MaxScore:            int32FromAny(j.ResourceSpec.Selftest["max_score"]),
			Expectation:         snapshot,
			ExtraInput:          extra,
		},
		SandboxMode: JudgeSandboxModeFresh,
		MaxRetries:  0,
	}
	if task.TenantID <= 0 || task.SubmitterID <= 0 || strings.TrimSpace(task.SourceRef) == "" {
		return apperr.ErrJudgerConfigInvalid
	}
	if task.InputSnapshot.MaxScore <= 0 {
		task.InputSnapshot.MaxScore = 100
	}
	result, err := s.executeTask(ctx, task)
	if err != nil {
		return err
	}
	if !result.Passed {
		return apperr.ErrJudgerSelftestFailed
	}
	return nil
}

// decodeCommandResult 解析判题器 stdout JSON 并限制可见详情大小。
func decodeCommandResult(stdout []byte, maxBytes int) (JudgeExecutionResult, error) {
	if maxBytes <= 0 {
		maxBytes = 64 << 10
	}
	if len(stdout) == 0 || len(stdout) > maxBytes {
		return JudgeExecutionResult{}, apperr.ErrJudgeWorkerFailed
	}
	var out JudgeExecutionResult
	if err := jsonx.DecodeStrict(stdout, &out); err != nil {
		return JudgeExecutionResult{}, apperr.ErrJudgeWorkerFailed.WithCause(err)
	}
	return out, nil
}

// normalizeExecutionResult 校验分数边界和脱敏详情。
func normalizeExecutionResult(result JudgeExecutionResult, snapshotMax int32, maxDetailsBytes int) (JudgeExecutionResult, error) {
	if result.MaxScore <= 0 {
		result.MaxScore = snapshotMax
	}
	if result.MaxScore <= 0 || result.Score < 0 || result.Score > result.MaxScore {
		return JudgeExecutionResult{}, apperr.ErrJudgeWorkerFailed
	}
	if err := validateResultDetails(result.Details); err != nil {
		return JudgeExecutionResult{}, err
	}
	size, err := resultDetailsJSONBytes(result.Details)
	if err != nil {
		return JudgeExecutionResult{}, apperr.ErrJudgeWorkerFailed.WithCause(err)
	}
	if size > maxDetailsBytes {
		return JudgeExecutionResult{}, apperr.ErrJudgeWorkerFailed
	}
	return result, nil
}

// destroyJudgeSandbox 尽力销毁 fresh 判题沙箱,失败只进日志不覆盖判题结果。
func (s *Service) destroyJudgeSandbox(ctx context.Context, task JudgeTask, sandboxID int64) {
	if sandboxID <= 0 {
		return
	}
	if err := s.sandbox.DestroySandbox(ctx, contracts.SandboxControlRequest{TenantID: task.TenantID, SandboxID: sandboxID, SourceRef: task.SourceRef}); err != nil {
		logging.ErrorContext(ctx, "judge sandbox destroy failed", err.Error(), slog.Int64("tenant_id", task.TenantID), slog.Int64("task_id", task.ID), slog.Int64("sandbox_id", sandboxID))
	}
}

// parseSandboxRef 解析复用沙箱引用,支持纯 ID 与 sandbox:<id> 形式。
func parseSandboxRef(raw string) (int64, error) {
	value := strings.TrimSpace(raw)
	value = strings.TrimPrefix(value, "sandbox:")
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, apperr.ErrJudgeSubmitInvalid
	}
	return id, nil
}

// judgeSandboxRef 生成判题结果中的沙箱快照引用。
func judgeSandboxRef(id int64) string {
	if id <= 0 {
		return ""
	}
	return fmt.Sprintf("sandbox:%d", id)
}

// int64FromAny 从自检 JSON 配置读取 int64。
func int64FromAny(v any) int64 {
	return jsonx.Int64FromAny(v, 0)
}

// int32FromAny 从自检 JSON 配置读取 int32。
func int32FromAny(v any) int32 {
	n := int64FromAny(v)
	if n <= 0 || n > 1<<31-1 {
		return 0
	}
	return int32(n)
}

const sandboxReadyGraceSeconds int32 = 30
