// M3 worker:消费判题队列,通过 contracts 调 M2/M5 完成判题执行、结果回写与事件发布。
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
	"chaimir/internal/modules/judge/internal/sqlcgen"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/background"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"

	"github.com/jackc/pgx/v5/pgtype"
)

// JudgeExecutionResult 是判题器 stdout 必须输出的结构化结果。
type JudgeExecutionResult struct {
	Passed   bool  `json:"passed"`
	Score    int32 `json:"score"`
	MaxScore int32 `json:"max_score"`
	Details  any   `json:"details"`
}

// RunWorkerOnce 消费并处理一个判题任务,供后台 worker 和测试入口复用。
func (s *Service) RunWorkerOnce(ctx context.Context) error {
	if err := s.requireEventBus(); err != nil {
		return err
	}
	taskID, err := s.dequeueTask(ctx)
	if err != nil {
		return err
	}
	if taskID == 0 {
		return nil
	}
	var task sqlcgen.JudgeTask
	task, err = s.markTaskJudgingAcrossTenant(ctx, taskID)
	if err != nil {
		return err
	}
	workerCtx := judgeTaskContext(ctx, task)
	if err := s.processTask(workerCtx, task); err != nil {
		return s.retryOrFail(workerCtx, task, err)
	}
	return nil
}

// StartWorker 按配置轮询队列,直到 context 取消。
func (s *Service) StartWorker(ctx context.Context) {
	background.Run(ctx, background.Task{
		Name:     "judge.queue_worker",
		Interval: time.Duration(s.cfg.QueuePollIntervalMs) * time.Millisecond,
		Run:      s.RunWorkerOnce,
	})
}

// dequeueTask 从 Redis 优先级队列取一个任务 ID;缺少队列能力时直接失败。
func (s *Service) dequeueTask(ctx context.Context) (int64, error) {
	if s.redis != nil {
		items, err := s.redis.Raw().ZPopMin(ctx, "judge:queue", 1).Result()
		if err != nil {
			return 0, apperr.ErrJudgeTaskQueuedFail.WithCause(err)
		}
		if len(items) == 0 {
			return 0, nil
		}
		taskID, err := strconv.ParseInt(fmt.Sprint(items[0].Member), 10, 64)
		if err != nil {
			return 0, apperr.ErrJudgeTaskInvalid.WithCause(err)
		}
		return taskID, nil
	}
	return 0, apperr.ErrJudgeTaskQueuedFail
}

// markTaskJudgingAcrossTenant 领取任务并推进到 judging。
func (s *Service) markTaskJudgingAcrossTenant(ctx context.Context, taskID int64) (sqlcgen.JudgeTask, error) {
	var current sqlcgen.JudgeTask
	// 第一步通过 app 事务读取任务的 tenant_id,随后所有写操作回到租户事务。
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		var err error
		current, err = q.GetJudgeTaskByID(ctx, taskID)
		if db.IsNoRows(err) {
			return apperr.ErrJudgeTaskNotFound
		}
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return sqlcgen.JudgeTask{}, ae
		}
		return sqlcgen.JudgeTask{}, apperr.ErrJudgeTaskPersistence.WithCause(err)
	}
	var task sqlcgen.JudgeTask
	// 第二步在任务所属租户下原子领取 queued 任务,避免重复 worker 并发执行。
	if err := s.repo.inTenantID(ctx, current.TenantID, func(q *sqlcgen.Queries) error {
		var err error
		task, err = q.MarkJudgeTaskJudging(ctx, taskID)
		if db.IsNoRows(err) {
			return apperr.ErrJudgeTaskInvalidState
		}
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return sqlcgen.JudgeTask{}, ae
		}
		return sqlcgen.JudgeTask{}, apperr.ErrJudgeTaskPersistence.WithCause(err)
	}
	s.publishProgress(task.ID, JudgeTaskJudging, "正在执行判题")
	return task, nil
}

// processTask 执行单个判题任务并写入正常结果。
func (s *Service) processTask(ctx context.Context, task sqlcgen.JudgeTask) error {
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
func (s *Service) loadJudgerForTask(ctx context.Context, judgerID int64) (sqlcgen.Judger, JudgerResourceSpec, error) {
	var judger sqlcgen.Judger
	if err := s.repo.inApp(ctx, func(q *sqlcgen.Queries) error {
		var err error
		judger, err = q.GetJudgerByID(ctx, judgerID)
		if db.IsNoRows(err) {
			return apperr.ErrJudgerNotFound
		}
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return sqlcgen.Judger{}, JudgerResourceSpec{}, ae
		}
		return sqlcgen.Judger{}, JudgerResourceSpec{}, apperr.ErrJudgerPersistence.WithCause(err)
	}
	spec, err := parseJudgerResourceSpecForType(judger.ResourceSpec, judger.Type)
	if err != nil {
		return sqlcgen.Judger{}, JudgerResourceSpec{}, err
	}
	return judger, spec, nil
}

// prepareJudgeSandbox 创建或解析本次判题使用的 M2 沙箱。
func (s *Service) prepareJudgeSandbox(ctx context.Context, task sqlcgen.JudgeTask, spec JudgerResourceSpec) (int64, string, error) {
	if s.sandbox == nil {
		return 0, "", apperr.ErrJudgeConfigUnavailable
	}
	if task.SandboxMode == SandboxModeReuse {
		id, err := strconv.ParseInt(task.TargetSandboxRef.String, 10, 64)
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
func (s *Service) injectJudgeInputs(ctx context.Context, sandboxID int64, task sqlcgen.JudgeTask) error {
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
	data, err := io.ReadAll(reader)
	if err != nil {
		return apperr.ErrJudgeTaskRunFail.WithCause(err)
	}
	return s.sandbox.PutSandboxFile(ctx, contracts.SandboxFileWrite{
		SandboxID: sandboxID, RelativePath: targetPath, ContentBase64: base64.StdEncoding.EncodeToString(data),
	})
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

// completeTask 写入判题结果、推进状态并发布完成事件。
func (s *Service) completeTask(ctx context.Context, task sqlcgen.JudgeTask, result JudgeExecutionResult, sandboxID int64) error {
	details, err := jsonx.AnyBytes(result.Details, apperr.ErrJudgeTaskRunFail)
	if err != nil {
		return err
	}
	snapshot := jsonx.ObjectMap(task.InputSnapshot)
	isRejudge, _ := snapshot["rejudge"].(bool)
	sandboxRef := pgText(ids.Format(sandboxID))
	if sandboxID <= 0 {
		sandboxRef = pgtype.Text{}
	}
	if err := s.repo.inTenantID(ctx, task.TenantID, func(q *sqlcgen.Queries) error {
		if _, err := q.CreateJudgeResult(ctx, sqlcgen.CreateJudgeResultParams{
			TaskID: task.ID, TenantID: task.TenantID, Passed: result.Passed, Score: result.Score,
			MaxScore: result.MaxScore, Details: details, JudgeSandboxRef: sandboxRef,
			IsRejudge: isRejudge,
		}); err != nil {
			return err
		}
		_, err := q.UpdateJudgeTaskStatus(ctx, sqlcgen.UpdateJudgeTaskStatusParams{ID: task.ID, Status: JudgeTaskDone})
		return err
	}); err != nil {
		return apperr.ErrJudgeTaskPersistence.WithCause(err)
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
	s.publishProgress(task.ID, JudgeTaskDone, "判题完成")
	if err := s.publishJudgeCompleted(ctx, contracts.JudgeCompletedEvent{
		TenantID: task.TenantID, TaskID: task.ID, SourceRef: task.SourceRef, Status: JudgeTaskDone, Score: int(result.Score),
	}); err != nil {
		return err
	}
	return nil
}

// retryOrFail 按重试策略把系统性失败任务重新入队或推进 failed 终态。
func (s *Service) retryOrFail(ctx context.Context, task sqlcgen.JudgeTask, cause error) error {
	failureStatus, mappedCause := classifyJudgeTaskFailure(cause)
	if err := s.repo.inTenantID(ctx, task.TenantID, func(q *sqlcgen.Queries) error {
		_, err := q.UpdateJudgeTaskStatus(ctx, sqlcgen.UpdateJudgeTaskStatusParams{ID: task.ID, Status: failureStatus})
		return err
	}); err != nil {
		return apperr.ErrJudgeTaskPersistence.WithCause(err)
	}
	var retried sqlcgen.JudgeTask
	if err := s.repo.inTenantID(ctx, task.TenantID, func(q *sqlcgen.Queries) error {
		var err error
		retried, err = q.RetryJudgeTask(ctx, task.ID)
		return err
	}); err == nil {
		s.publishProgress(task.ID, JudgeTaskQueued, "判题已重新排队")
		return s.enqueueTask(ctx, retried)
	}
	if err := s.repo.inTenantID(ctx, task.TenantID, func(q *sqlcgen.Queries) error {
		_, err := q.FailJudgeTask(ctx, task.ID)
		return err
	}); err != nil {
		return apperr.ErrJudgeTaskRunFail.WithCause(err)
	}
	if err := s.writeAudit(ctx, task.TenantID, auditActionTaskFailed, auditTargetTask, task.ID, map[string]any{
		"source_ref": task.SourceRef,
		"reason":     safeJudgeFailureReason(mappedCause),
	}); err != nil {
		return err
	}
	s.publishProgress(task.ID, JudgeTaskFailed, "判题失败,请稍后重试")
	if err := s.publishJudgeFailed(ctx, contracts.JudgeFailedEvent{
		TenantID: task.TenantID, TaskID: task.ID, SourceRef: task.SourceRef, Reason: safeJudgeFailureReason(mappedCause),
	}); err != nil {
		return err
	}
	return mappedCause
}

// requireEventBus 确认 M3 终态事件通道已注入,避免判题结果无法通知上层聚合模块。
func (s *Service) requireEventBus() error {
	if s.bus == nil {
		return apperr.ErrJudgeEventPublish
	}
	return nil
}

// publishJudgeCompleted 发布判题完成事件,供 M6/M7/M8 订阅更新上层状态。
func (s *Service) publishJudgeCompleted(ctx context.Context, event contracts.JudgeCompletedEvent) error {
	if err := s.requireEventBus(); err != nil {
		return err
	}
	if err := s.bus.Publish(ctx, contracts.SubjectJudgeCompleted, event); err != nil {
		return apperr.ErrJudgeEventPublish.WithCause(err)
	}
	return nil
}

// publishJudgeFailed 发布判题失败事件,供上层模块把提交/实验/竞赛状态推进到失败终态。
func (s *Service) publishJudgeFailed(ctx context.Context, event contracts.JudgeFailedEvent) error {
	if err := s.requireEventBus(); err != nil {
		return err
	}
	if err := s.bus.Publish(ctx, contracts.SubjectJudgeFailed, event); err != nil {
		return apperr.ErrJudgeEventPublish.WithCause(err)
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

// recycleJudgeSandbox 回收 fresh 判题沙箱;失败必须返回给状态机,不能在完成事件后才记录日志。
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
		if stringValue(detail["case"]) == "" && stringValue(detail["source"]) == "" && stringValue(detail["target"]) == "" {
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
func judgeTaskContext(parent context.Context, task sqlcgen.JudgeTask) context.Context {
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
