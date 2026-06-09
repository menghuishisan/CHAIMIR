// M3 worker:消费判题队列,通过 contracts 调 M2/M5 完成判题执行、结果回写与终态 outbox 派发。
package judge

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/background"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/timex"
	"chaimir/internal/platform/upload"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
)

// JudgeExecutionResult 是判题器 stdout 必须输出的结构化结果。
type JudgeExecutionResult struct {
	Passed   bool  `json:"passed"`
	Score    int32 `json:"score"`
	MaxScore int32 `json:"max_score"`
	Details  any   `json:"details"`
}

// RunWorkerOnce 按配置批量消费并处理判题任务,供后台 worker 和测试入口复用。
func (s *Service) RunWorkerOnce(ctx context.Context) error {
	if err := s.requireEventBus(); err != nil {
		return err
	}
	if err := s.PublishPendingJudgeEvents(ctx); err != nil {
		return err
	}
	taskIDs, err := s.dequeueTasks(ctx)
	if err != nil {
		return err
	}
	if len(taskIDs) == 0 {
		return nil
	}
	var batchErr error
	for _, taskID := range taskIDs {
		task, err := s.markTaskJudgingAcrossTenant(ctx, taskID)
		if err != nil {
			batchErr = errors.Join(batchErr, err)
			continue
		}
		workerCtx := judgeTaskContext(ctx, task)
		if err := s.processTask(workerCtx, task); err != nil {
			batchErr = errors.Join(batchErr, s.retryOrFail(workerCtx, task, err))
			continue
		}
	}
	return batchErr
}

// StartWorker 按配置轮询队列,直到 context 取消。
func (s *Service) StartWorker(ctx context.Context) {
	background.Run(ctx, background.Task{
		Name:     "judge.queue_worker",
		Interval: time.Duration(s.cfg.QueuePollIntervalMs) * time.Millisecond,
		Run:      s.RunWorkerOnce,
	})
}

// dequeueTasks 从 Redis 优先级队列按配置取任务 ID;缺少队列能力时直接失败。
func (s *Service) dequeueTasks(ctx context.Context) ([]int64, error) {
	if s.redis != nil {
		items, err := s.redis.Raw().ZPopMin(ctx, "judge:queue", int64(s.normalizedWorkerBatchSize())).Result()
		if err != nil {
			return nil, apperr.ErrJudgeTaskQueuedFail.WithCause(err)
		}
		if len(items) == 0 {
			return nil, nil
		}
		taskIDs := make([]int64, 0, len(items))
		taskID, err := strconv.ParseInt(fmt.Sprint(items[0].Member), 10, 64)
		if err != nil {
			return nil, apperr.ErrJudgeTaskInvalid.WithCause(err)
		}
		taskIDs = append(taskIDs, taskID)
		for _, item := range items[1:] {
			taskID, err := strconv.ParseInt(fmt.Sprint(item.Member), 10, 64)
			if err != nil {
				return nil, apperr.ErrJudgeTaskInvalid.WithCause(err)
			}
			taskIDs = append(taskIDs, taskID)
		}
		return taskIDs, nil
	}
	return nil, apperr.ErrJudgeTaskQueuedFail
}

// markTaskJudgingAcrossTenant 领取任务并推进到 judging。
func (s *Service) markTaskJudgingAcrossTenant(ctx context.Context, taskID int64) (JudgeTaskSnapshot, error) {
	task, err := s.repo.claimQueuedTaskAcrossTenant(ctx, taskID)
	if err != nil {
		return JudgeTaskSnapshot{}, err
	}
	s.publishProgress(task.ID, JudgeTaskJudging, "正在执行判题")
	return task, nil
}

// processTask 执行单个判题任务并写入正常结果。
func (s *Service) processTask(ctx context.Context, task JudgeTaskSnapshot) error {
	judger, spec, err := s.loadJudgerForTask(ctx, task.JudgerID)
	if err != nil {
		return err
	}
	if judger.Type == JudgerTypeManual {
		return nil
	}
	snapshot := jsonx.ObjectMap(task.InputSnapshot)
	if judger.Type == JudgerTypeFlag || judger.Type == JudgerTypeSimCheckpoint {
		result, err := s.executeJudgerStrategy(ctx, 0, snapshot, judger.Type, spec, judger.DefaultTimeoutSec)
		if err != nil {
			return err
		}
		return s.completeTask(ctx, task, result, 0)
	}
	sandboxID, recycleSourceRef, err := s.prepareJudgeSandbox(ctx, task, spec)
	if err != nil {
		if recycleSourceRef != "" {
			return errors.Join(err, s.recycleJudgeSandbox(ctx, task.TenantID, recycleSourceRef))
		}
		return err
	}
	if judger.Type == JudgerTypeTestcase || judger.Type == JudgerTypeStaticScan ||
		(judger.Type == JudgerTypeOnchainAssert && task.SandboxMode == SandboxModeFresh) {
		if err := s.injectJudgeInputs(ctx, sandboxID, task); err != nil {
			return s.recycleJudgeSandboxAfterError(ctx, task.TenantID, recycleSourceRef, err)
		}
	}
	result, err := s.executeJudgerStrategy(ctx, sandboxID, snapshot, judger.Type, spec, judger.DefaultTimeoutSec)
	if err != nil {
		return s.recycleJudgeSandboxAfterError(ctx, task.TenantID, recycleSourceRef, err)
	}
	if recycleSourceRef != "" {
		if err := s.recycleJudgeSandbox(ctx, task.TenantID, recycleSourceRef); err != nil {
			return err
		}
	}
	return s.completeTask(ctx, task, result, sandboxID)
}

// loadJudgerForTask 读取任务绑定的判题器并解析资源规格。
func (s *Service) loadJudgerForTask(ctx context.Context, judgerID int64) (JudgerSnapshot, JudgerResourceSpec, error) {
	judger, err := s.repo.getJudgerByID(ctx, judgerID)
	if err != nil {
		return JudgerSnapshot{}, JudgerResourceSpec{}, err
	}
	spec, err := parseJudgerResourceSpecForType(judger.ResourceSpec, judger.Type)
	if err != nil {
		return JudgerSnapshot{}, JudgerResourceSpec{}, err
	}
	return judger, spec, nil
}

// prepareJudgeSandbox 创建或解析本次判题使用的 M2 沙箱。
func (s *Service) prepareJudgeSandbox(ctx context.Context, task JudgeTaskSnapshot, spec JudgerResourceSpec) (int64, string, error) {
	if s.sandbox == nil {
		return 0, "", apperr.ErrJudgeConfigUnavailable
	}
	if task.SandboxMode == SandboxModeReuse {
		id, err := strconv.ParseInt(task.TargetSandboxRef, 10, 64)
		if err != nil || id <= 0 {
			return 0, "", apperr.ErrJudgeTaskInvalid
		}
		return id, "", nil
	}
	sourceRef := fmt.Sprintf("judge:%d:task:%d", timex.Now().Year(), task.ID)
	info, err := s.sandbox.CreateSandbox(ctx, contracts.SandboxCreateRequest{
		TenantID: task.TenantID, RuntimeCode: spec.RuntimeCode, RuntimeImageVersion: spec.RuntimeImageVersion,
		ToolCodes: spec.ToolCodes, InitScriptRef: spec.InitScriptRef,
		OwnerAccountID: task.SubmitterID, SourceRef: sourceRef,
	})
	if err != nil {
		return 0, "", err
	}
	if err := s.waitSandboxReady(ctx, info.SandboxID, spec.TimeoutSec); err != nil {
		return 0, sourceRef, err
	}
	return info.SandboxID, sourceRef, nil
}

// injectJudgeInputs 把提交代码和测试套件注入 judge 沙箱并解包。
func (s *Service) injectJudgeInputs(ctx context.Context, sandboxID int64, task JudgeTaskSnapshot) error {
	snapshot := jsonx.ObjectMap(task.InputSnapshot)
	suiteRef, _ := snapshot["suite_ref"].(string)
	if err := s.putObjectIntoSandbox(ctx, sandboxID, task.CodeStorageKey, "judge/submission.tgz"); err != nil {
		return err
	}
	if suiteRef != "" {
		if err := s.putObjectIntoSandbox(ctx, sandboxID, suiteRef, "judge/suite.tgz"); err != nil {
			return err
		}
	}
	command := []string{"sh", "-lc", "mkdir -p submission suite && tar -xzf judge/submission.tgz -C submission && if [ -f judge/suite.tgz ]; then tar -xzf judge/suite.tgz -C suite; fi"}
	_, err := s.sandbox.ExecSandboxCommand(ctx, contracts.SandboxExecRequest{
		SandboxID:  sandboxID,
		Command:    command,
		TimeoutSec: int32(s.cfg.InputInjectTimeoutSeconds),
	})
	return err
}

// putObjectIntoSandbox 读取对象存储引用并写入沙箱文件。
func (s *Service) putObjectIntoSandbox(ctx context.Context, sandboxID int64, objectRef, targetPath string) (err error) {
	if s.store == nil {
		return apperr.ErrJudgeConfigUnavailable
	}
	ref, err := storage.ParseObjectRef(objectRef)
	if err != nil {
		return apperr.ErrJudgeTaskInvalid.WithCause(err)
	}
	reader, err := s.store.Get(ctx, ref.Bucket, ref.Key)
	if err != nil {
		return apperr.ErrJudgeTaskRunFail.WithCause(err)
	}
	defer func() {
		if closeErr := reader.Close(); closeErr != nil {
			err = errors.Join(err, apperr.ErrJudgeTaskRunFail.WithCause(closeErr))
		}
	}()
	raw, err := io.ReadAll(reader)
	if err != nil {
		return apperr.ErrJudgeTaskRunFail.WithCause(err)
	}
	safeArchive, err := safeJudgeInputArchive(raw, s.cfg)
	if err != nil {
		return err
	}
	return s.sandbox.PutSandboxFile(ctx, contracts.SandboxFileWrite{
		SandboxID: sandboxID, RelativePath: targetPath, ContentBase64: base64.StdEncoding.EncodeToString(safeArchive),
	})
}

// safeJudgeInputArchive 校验并重打判题输入归档,防止提交包覆盖套件或逃逸工作区。
func safeJudgeInputArchive(raw []byte, cfg config.JudgeConfig) (outBytes []byte, err error) {
	if cfg.InputArchiveMaxFiles <= 0 || cfg.InputArchiveMaxUnpackedBytes <= 0 {
		return nil, apperr.ErrJudgeInputArchiveInvalid
	}
	out, err := upload.RewriteTarGz(raw, judgeArchiveLimits(cfg))
	if err != nil {
		return nil, apperr.ErrJudgeInputArchiveInvalid.WithCause(err)
	}
	return out, nil
}

// judgeArchiveLimits 把 M3 配置转换为平台归档安全边界。
func judgeArchiveLimits(cfg config.JudgeConfig) upload.ArchiveLimits {
	return upload.ArchiveLimits{MaxFiles: cfg.InputArchiveMaxFiles, MaxUnpackedBytes: cfg.InputArchiveMaxUnpackedBytes}
}

// runJudgeCommand 执行判题器命令并解析结构化 stdout。
func (s *Service) runJudgeCommand(ctx context.Context, sandboxID int64, spec JudgerResourceSpec, defaultTimeout int32) (JudgeExecutionResult, error) {
	timeout := spec.TimeoutSec
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	res, err := s.sandbox.ExecSandboxCommand(ctx, contracts.SandboxExecRequest{
		SandboxID: sandboxID, Command: spec.Command, TimeoutSec: timeout,
	})
	if err != nil {
		return JudgeExecutionResult{}, err
	}
	return parseJudgeResult(res.Stdout, s.cfg.ResultDetailsMaxBytes)
}

// completeTask 写入判题结果、推进状态并创建完成事件 outbox。
func (s *Service) completeTask(ctx context.Context, task JudgeTaskSnapshot, result JudgeExecutionResult, sandboxID int64) error {
	details, err := jsonx.AnyBytes(result.Details, apperr.ErrJudgeTaskRunFail)
	if err != nil {
		return err
	}
	snapshot := jsonx.ObjectMap(task.InputSnapshot)
	isRejudge, _ := snapshot["rejudge"].(bool)
	sandboxRef := ids.Format(sandboxID)
	if sandboxID <= 0 {
		sandboxRef = ""
	}
	if err := s.writeAudit(ctx, task.TenantID, auditActionTaskComplete, auditTargetResult, task.ID, map[string]any{
		"source_ref": task.SourceRef,
		"score":      result.Score,
		"max_score":  result.MaxScore,
		"passed":     result.Passed,
		"is_rejudge": isRejudge,
	}); err != nil {
		return err
	}
	outbox, err := s.newJudgeEventOutbox(contracts.SubjectJudgeCompleted, contracts.JudgeCompletedEvent{
		TenantID: task.TenantID, TaskID: task.ID, SourceRef: task.SourceRef, Status: JudgeTaskDone, Score: int(result.Score),
	})
	if err != nil {
		return err
	}
	if err := s.repo.completeTaskResult(ctx, JudgeResultCreate{
		TaskID:          task.ID,
		TenantID:        task.TenantID,
		Passed:          result.Passed,
		Score:           result.Score,
		MaxScore:        result.MaxScore,
		Details:         details,
		JudgeSandboxRef: sandboxRef,
		IsRejudge:       isRejudge,
	}, outbox); err != nil {
		return apperr.ErrJudgeTaskPersistence.WithCause(err)
	}
	s.publishProgress(task.ID, JudgeTaskDone, "判题完成")
	if err := s.PublishPendingJudgeEvents(ctx); err != nil {
		logging.ErrorContext(ctx, "judge terminal event dispatch deferred", err.Error(), slog.Int64("task_id", task.ID))
	}
	return nil
}

// retryOrFail 按重试策略把系统性失败任务重新入队或推进 failed 终态。
func (s *Service) retryOrFail(ctx context.Context, task JudgeTaskSnapshot, cause error) error {
	failureStatus, mappedCause := classifyJudgeTaskFailure(cause)
	if err := s.repo.markTaskStatus(ctx, task.TenantID, task.ID, failureStatus); err != nil {
		return apperr.ErrJudgeTaskPersistence.WithCause(err)
	}
	if retried, err := s.repo.retryTask(ctx, task.TenantID, task.ID); err == nil {
		s.publishProgress(task.ID, JudgeTaskQueued, "判题已重新排队")
		return s.enqueueTask(ctx, retried)
	}
	if err := s.writeAudit(ctx, task.TenantID, auditActionTaskFailed, auditTargetTask, task.ID, map[string]any{
		"source_ref": task.SourceRef,
		"reason":     safeJudgeFailureReason(mappedCause),
	}); err != nil {
		return err
	}
	outbox, err := s.newJudgeEventOutbox(contracts.SubjectJudgeFailed, contracts.JudgeFailedEvent{
		TenantID: task.TenantID, TaskID: task.ID, SourceRef: task.SourceRef, Reason: safeJudgeFailureReason(mappedCause),
	})
	if err != nil {
		return err
	}
	if err := s.repo.failTaskWithOutbox(ctx, task.TenantID, task.ID, outbox); err != nil {
		return apperr.ErrJudgeTaskRunFail.WithCause(err)
	}
	s.publishProgress(task.ID, JudgeTaskFailed, "判题失败,请稍后重试")
	if err := s.PublishPendingJudgeEvents(ctx); err != nil {
		logging.ErrorContext(ctx, "judge failed event dispatch deferred", err.Error(), slog.Int64("task_id", task.ID))
	}
	return mappedCause
}

// newJudgeEventOutbox 构造待持久化 outbox,具体写入由 repo 放进终态事务。
func (s *Service) newJudgeEventOutbox(subject string, payload any) (JudgeOutboxCreate, error) {
	tenantID := eventTenantID(payload)
	taskID := eventTaskID(payload)
	if tenantID <= 0 || taskID <= 0 {
		return JudgeOutboxCreate{}, apperr.ErrJudgeEventPublish
	}
	data, err := jsonx.AnyBytes(payload, apperr.ErrJudgeEventPublish)
	if err != nil {
		return JudgeOutboxCreate{}, err
	}
	return JudgeOutboxCreate{ID: s.idgen.Generate(), TenantID: tenantID, TaskID: taskID, Subject: subject, Payload: data}, nil
}

// PublishPendingJudgeEvents 派发 M3 自有 outbox 中的判题终态事件。
func (s *Service) PublishPendingJudgeEvents(ctx context.Context) error {
	if err := s.requireEventBus(); err != nil {
		return err
	}
	tenantIDs, err := s.repo.listPendingJudgeEventOutboxTenants(ctx, int32(s.normalizedWorkerBatchSize()))
	if err != nil {
		return err
	}
	for _, tenantID := range tenantIDs {
		if err := s.publishPendingJudgeEventsForTenant(ctx, tenantID); err != nil {
			return err
		}
	}
	return nil
}

// publishPendingJudgeEventsForTenant 发布单租户 pending/failed outbox 行。
func (s *Service) publishPendingJudgeEventsForTenant(ctx context.Context, tenantID int64) error {
	rows, err := s.repo.listPendingJudgeEventOutbox(ctx, tenantID, int32(s.normalizedWorkerBatchSize()))
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err := s.publishJudgeOutboxRow(ctx, row); err != nil {
			return err
		}
	}
	return nil
}

// publishJudgeOutboxRow 发布单条 outbox 并写回 published/failed 状态。
func (s *Service) publishJudgeOutboxRow(ctx context.Context, row JudgeOutboxSnapshot) error {
	payload, err := decodeJudgeOutboxPayload(row)
	if err == nil {
		err = s.bus.Publish(ctx, row.Subject, payload)
	}
	if err != nil {
		markErr := s.repo.markJudgeEventOutboxFailed(ctx, row.TenantID, row.ID, safeJudgeOutboxError(err))
		if markErr != nil {
			return errors.Join(apperr.ErrJudgeEventPublish.WithCause(err), apperr.ErrJudgeTaskPersistence.WithCause(markErr))
		}
		return apperr.ErrJudgeEventPublish.WithCause(err)
	}
	if err := s.repo.markJudgeEventOutboxPublished(ctx, row.TenantID, row.ID); err != nil {
		return apperr.ErrJudgeTaskPersistence.WithCause(err)
	}
	return nil
}

// decodeJudgeOutboxPayload 按 subject 解码终态事件载荷。
func decodeJudgeOutboxPayload(row JudgeOutboxSnapshot) (any, error) {
	switch row.Subject {
	case contracts.SubjectJudgeCompleted:
		var event contracts.JudgeCompletedEvent
		if err := json.Unmarshal(row.Payload, &event); err != nil {
			return nil, apperr.ErrJudgeEventPublish.WithCause(err)
		}
		return event, nil
	case contracts.SubjectJudgeFailed:
		var event contracts.JudgeFailedEvent
		if err := json.Unmarshal(row.Payload, &event); err != nil {
			return nil, apperr.ErrJudgeEventPublish.WithCause(err)
		}
		return event, nil
	default:
		return nil, apperr.ErrJudgeEventPublish
	}
}

// eventTenantID 读取 M3 终态事件的租户 ID。
func eventTenantID(payload any) int64 {
	switch event := payload.(type) {
	case contracts.JudgeCompletedEvent:
		return event.TenantID
	case contracts.JudgeFailedEvent:
		return event.TenantID
	default:
		return 0
	}
}

// eventTaskID 读取 M3 终态事件的任务 ID。
func eventTaskID(payload any) int64 {
	switch event := payload.(type) {
	case contracts.JudgeCompletedEvent:
		return event.TaskID
	case contracts.JudgeFailedEvent:
		return event.TaskID
	default:
		return 0
	}
}

// normalizedWorkerBatchSize 返回 worker 每轮处理数量,避免配置错误导致全表扫描。
func (s *Service) normalizedWorkerBatchSize() int {
	if s.cfg.WorkerBatchSize <= 0 {
		return 1
	}
	return s.cfg.WorkerBatchSize
}

// requireEventBus 确认 M3 终态事件派发通道已注入,避免 outbox 永久无法通知上层聚合模块。
func (s *Service) requireEventBus() error {
	if s.bus == nil {
		return apperr.ErrJudgeEventPublish
	}
	return nil
}

// classifyJudgeTaskFailure 把外部执行错误映射为 M3 状态机中的 timeout/error。
func classifyJudgeTaskFailure(cause error) (int16, error) {
	if errors.Is(cause, context.DeadlineExceeded) {
		return JudgeTaskTimeout, apperr.ErrJudgeTaskTimeout.WithCause(cause)
	}
	if ae, ok := apperr.As(cause); ok {
		if ae.Code == apperr.ErrSandboxTimeout.Code || ae.Code == apperr.ErrJudgeTaskTimeout.Code {
			return JudgeTaskTimeout, apperr.ErrJudgeTaskTimeout.WithCause(cause)
		}
	}
	return JudgeTaskError, apperr.ErrJudgeTaskRunFail.WithCause(cause)
}

// recycleJudgeSandboxAfterError 在判题异常后仍尝试回收 fresh 沙箱,并保留原始失败原因。
func (s *Service) recycleJudgeSandboxAfterError(ctx context.Context, tenantID int64, sourceRef string, cause error) error {
	if sourceRef == "" {
		return cause
	}
	if err := s.recycleJudgeSandbox(ctx, tenantID, sourceRef); err != nil {
		return errors.Join(cause, err)
	}
	return cause
}

// recycleJudgeSandbox 回收 fresh 判题沙箱;失败必须返回给状态机,不得在结果/outbox 后才做日志式清理。
func (s *Service) recycleJudgeSandbox(ctx context.Context, tenantID int64, sourceRef string) error {
	if err := s.sandbox.RecycleBySourceRef(ctx, tenantID, sourceRef, "judge-completed"); err != nil {
		logging.ErrorContext(ctx, "judge sandbox recycle failed", err.Error(), slog.String("source_ref", sourceRef))
		return apperr.ErrJudgeTaskRunFail.WithCause(err)
	}
	return nil
}

// parseJudgeResult 解析判题器 stdout 中的结构化结果。
func parseJudgeResult(raw []byte, maxDetailsBytes int) (JudgeExecutionResult, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return JudgeExecutionResult{}, apperr.ErrJudgeTaskRunFail
	}
	var result JudgeExecutionResult
	if err := json.Unmarshal(trimmed, &result); err != nil {
		return JudgeExecutionResult{}, apperr.ErrJudgeTaskRunFail.WithCause(err)
	}
	if result.MaxScore <= 0 || result.Score < 0 || result.Score > result.MaxScore {
		return JudgeExecutionResult{}, apperr.ErrJudgeTaskRunFail
	}
	details, err := json.Marshal(result.Details)
	if err != nil {
		return JudgeExecutionResult{}, apperr.ErrJudgeTaskRunFail.WithCause(err)
	}
	if maxDetailsBytes <= 0 || len(details) > maxDetailsBytes || containsSensitiveJudgeDetail(result.Details) {
		return JudgeExecutionResult{}, apperr.ErrJudgeTaskRunFail
	}
	if !hasExplainableJudgeDetails(result.Details) {
		return JudgeExecutionResult{}, apperr.ErrJudgeTaskRunFail
	}
	return result, nil
}

// hasExplainableJudgeDetails 校验判题命令输出包含可定位的逐项解释。
func hasExplainableJudgeDetails(v any) bool {
	items, ok := v.([]any)
	if !ok || len(items) == 0 {
		return false
	}
	for _, item := range items {
		detail, ok := item.(map[string]any)
		if !ok {
			return false
		}
		if _, ok := detail["passed"].(bool); !ok {
			return false
		}
		if assertionString(detail["case"]) == "" && assertionString(detail["source"]) == "" && assertionString(detail["target"]) == "" {
			return false
		}
	}
	return true
}

// containsSensitiveJudgeDetail 递归检查判题详情键名,防止答案、flag、套件源码进入学生可查结果。
func containsSensitiveJudgeDetail(v any) bool {
	switch item := v.(type) {
	case map[string]any:
		for key, value := range item {
			normalized := strings.ToLower(strings.ReplaceAll(key, "_", ""))
			for _, marker := range []string{"answer", "flag", "secret", "suitesource", "testsource", "privatekey", "mnemonic"} {
				if strings.Contains(normalized, marker) {
					return true
				}
			}
			if containsSensitiveJudgeDetail(value) {
				return true
			}
		}
	case []any:
		for _, value := range item {
			if containsSensitiveJudgeDetail(value) {
				return true
			}
		}
	}
	return false
}

// judgeTaskContext 为后台 worker 构造带租户身份与审计上下文的 context。
func judgeTaskContext(parent context.Context, task JudgeTaskSnapshot) context.Context {
	ctx := audit.WithRequestContext(context.Background(), audit.RequestContextFrom(parent))
	ctx = tenant.WithContext(ctx, tenant.Identity{TenantID: task.TenantID, AccountID: task.SubmitterID})
	ctx = auth.WithServiceSourceRef(ctx, task.SourceRef)
	return logging.WithAttrs(ctx, slog.Int64("tenant_id", task.TenantID), slog.Int64("account_id", task.SubmitterID))
}

// safeJudgeFailureReason 把内部失败折叠为事件可消费的稳定原因。
func safeJudgeFailureReason(err error) string {
	if ae, ok := apperr.As(err); ok {
		return ae.Code
	}
	return apperr.ErrJudgeTaskRunFail.Code
}

// safeJudgeOutboxError 生成可持久化的脱敏失败摘要。
func safeJudgeOutboxError(err error) string {
	msg := logging.SanitizeError(err.Error())
	if len(msg) > 255 {
		return msg[:255]
	}
	return msg
}
