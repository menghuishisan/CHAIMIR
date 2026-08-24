// judge service_worker 文件实现 M3 队列消费、沙箱判题执行和终态事件 outbox 发布。
package judge

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"maps"
	"reflect"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/intx"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/timex"
	"chaimir/internal/platform/upload"
	"chaimir/internal/platform/workload"
	"chaimir/pkg/apperr"
	pkgcrypto "chaimir/pkg/crypto"
	"chaimir/pkg/logging"
)

// JudgeExecutionResult 是一次 worker 执行后可落库的脱敏结果。
type JudgeExecutionResult struct {
	Passed          bool                       `json:"passed"`
	Score           int32                      `json:"score"`
	MaxScore        int32                      `json:"max_score"`
	Details         []JudgeResultDetail        `json:"details"`
	Replay          contracts.JudgeReplayTrace `json:"replay,omitempty"`
	JudgeSandboxRef string                     `json:"judge_sandbox_ref,omitempty"`
}

// RunWorkerOnce 领取一批排队任务并发布此前积压的终态事件。
func (s *Service) RunWorkerOnce(ctx context.Context) error {
	if err := s.publishPendingOutbox(ctx); err != nil {
		return err
	}
	limit, ok := intx.Int32(s.cfg.WorkerBatchSize)
	if !ok || limit <= 0 {
		limit = 1
	}
	var expired []JudgeTask
	var tasks []JudgeTask
	leaseToken, err := pkgcrypto.RandomToken(48)
	if err != nil {
		return apperr.ErrJudgeTaskPersistFailed.WithCause(err)
	}
	now := time.Now()
	leaseUntil := now.Add(time.Duration(s.cfg.TaskLeaseDurationMs) * time.Millisecond)
	staleBefore := now
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		expired, err = tx.FailExpiredJudgeTasks(ctx, limit, staleBefore)
		if err != nil {
			return apperr.ErrJudgeTaskPersistFailed.WithCause(err)
		}
		for _, task := range expired {
			payload := contracts.JudgeFailedEvent{TenantID: task.TenantID, TraceID: task.InputSnapshot.TraceID, TaskID: task.ID, SourceRef: task.SourceRef, Reason: task.LastError, FailedAt: now}
			if _, err := tx.CreateOutbox(ctx, s.ids.Generate(), task.TenantID, task.ID, contracts.SubjectJudgeFailed, payload); err != nil {
				return apperr.ErrJudgeTaskPersistFailed.WithCause(err)
			}
		}
		tasks, err = tx.DequeueJudgeTasks(ctx, limit, staleBefore, leaseUntil, leaseToken)
		if err != nil {
			return apperr.ErrJudgeTaskPersistFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return err
	}
	for _, task := range expired {
		s.publishProgress(ctx, task.TenantID, task.ID, task.Status, ProgressStageFailed, "判题任务执行失败")
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
	result, leaseLost, err := s.executeTaskWithLease(ctx, task)
	if leaseLost {
		slog.Default().LogAttrs(ctx, slog.LevelWarn, "judge task lease lost during execution", logging.AttrsFromContext(ctx, slog.Int64("tenant_id", task.TenantID), slog.Int64("task_id", task.ID))...)
		return nil
	}
	if err != nil {
		return s.retryOrFail(ctx, task, err)
	}
	return s.completeTask(ctx, task, result)
}

// executeTaskWithLease 在整个不可信代码执行期间续租,避免任务超过初始租约后被并发重复执行。
func (s *Service) executeTaskWithLease(ctx context.Context, task JudgeTask) (JudgeExecutionResult, bool, error) {
	leaseDuration := time.Duration(s.cfg.TaskLeaseDurationMs) * time.Millisecond
	if leaseDuration <= 0 {
		return JudgeExecutionResult{}, false, apperr.ErrJudgeTaskPersistFailed
	}
	executionCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan struct{})
	lost := make(chan struct{}, 1)
	go s.renewJudgeTaskLeaseUntilDone(executionCtx, task, leaseDuration, done, lost, cancel)
	result, err := s.executeTask(executionCtx, task)
	close(done)
	select {
	case <-lost:
		return JudgeExecutionResult{}, true, nil
	default:
		return result, false, err
	}
}

// renewJudgeTaskLeaseUntilDone 以小于租约的周期续租;任一 CAS 失败都取消执行上下文。
func (s *Service) renewJudgeTaskLeaseUntilDone(ctx context.Context, task JudgeTask, leaseDuration time.Duration, done <-chan struct{}, lost chan<- struct{}, cancel context.CancelFunc) {
	interval := leaseDuration / 3
	if interval > 30*time.Second {
		interval = 30 * time.Second
	}
	if interval < time.Second {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			leaseUntil := timex.Now().Add(leaseDuration)
			var renewed bool
			err := s.store.TenantTx(ctx, task.TenantID, func(ctx context.Context, tx TxStore) error {
				var err error
				renewed, err = tx.RenewJudgeTaskLease(ctx, task.TenantID, task.ID, task.LeaseToken, leaseUntil)
				return err
			})
			if err == nil && renewed {
				continue
			}
			if err != nil {
				logging.ErrorContext(ctx, "judge task lease renewal failed", err.Error(), slog.Int64("tenant_id", task.TenantID), slog.Int64("task_id", task.ID))
			}
			select {
			case lost <- struct{}{}:
			default:
			}
			cancel()
			return
		}
	}
}

// executeTask 根据判题器类型选择后端策略或沙箱命令执行路径。
func (s *Service) executeTask(ctx context.Context, task JudgeTask) (JudgeExecutionResult, error) {
	if task.InputSnapshot.JudgerType == JudgerTypeManual {
		return JudgeExecutionResult{}, apperr.ErrJudgeTaskStateInvalid
	}
	executionTask, err := s.taskWithExecutionExpectation(ctx, task)
	if err != nil {
		return JudgeExecutionResult{}, err
	}
	if !needsSandbox(task) {
		result, handled, err := s.executeJudgerStrategy(ctx, executionTask, 0)
		if handled && err != nil {
			return JudgeExecutionResult{}, err
		}
		if handled {
			return normalizeExecutionResult(result, executionTask.InputSnapshot.MaxScore, s.cfg.ResultDetailsMaxBytes)
		}
	}
	sandboxID, fresh, err := s.resolveSandbox(ctx, executionTask)
	if fresh {
		defer s.destroyJudgeSandbox(ctx, executionTask, sandboxID)
	}
	if err != nil {
		return JudgeExecutionResult{}, err
	}
	if result, handled, err := s.executeJudgerStrategy(ctx, executionTask, sandboxID); handled {
		if err != nil {
			return JudgeExecutionResult{}, err
		}
		result.JudgeSandboxRef = judgeSandboxRef(sandboxID)
		return normalizeExecutionResult(result, executionTask.InputSnapshot.MaxScore, s.cfg.ResultDetailsMaxBytes)
	}
	if executionTask.InputSnapshot.JudgerType != JudgerTypeTestcase && executionTask.InputSnapshot.JudgerType != JudgerTypeStaticScan {
		return JudgeExecutionResult{}, apperr.ErrJudgerConfigInvalid
	}
	return s.runJudgeCommand(ctx, executionTask, sandboxID)
}

// taskWithExecutionExpectation 为 J2/J5 执行期装载 M5 全量配置,但不改变已持久化的 input_snapshot。
func (s *Service) taskWithExecutionExpectation(ctx context.Context, task JudgeTask) (JudgeTask, error) {
	if task.InputSnapshot.JudgerType != JudgerTypeOnchainAssert && task.InputSnapshot.JudgerType != JudgerTypeSimCheckpoint {
		return task, nil
	}
	if strings.TrimSpace(task.InputSnapshot.ItemCode) == "" || strings.TrimSpace(task.InputSnapshot.ItemVersion) == "" {
		return task, nil
	}
	if s.content == nil {
		return JudgeTask{}, apperr.ErrJudgeSpecUnavailable
	}
	spec, err := s.content.GetJudgeSpec(ctx, task.TenantID, task.InputSnapshot.ItemCode, task.InputSnapshot.ItemVersion)
	if err != nil {
		return JudgeTask{}, apperr.ErrJudgeSpecUnavailable.WithCause(err)
	}
	if strings.TrimSpace(spec.VersionHash) != "" && strings.TrimSpace(spec.VersionHash) != strings.TrimSpace(task.InputSnapshot.VersionHash) {
		return JudgeTask{}, apperr.ErrJudgeSpecUnavailable
	}
	if spec.JudgerCode != "" && spec.JudgerCode != task.InputSnapshot.JudgerCode {
		return JudgeTask{}, apperr.ErrJudgeSpecUnavailable
	}
	expectation, err := s.executionExpectationForJudger(task.InputSnapshot.JudgerType, spec, task.InputSnapshot.Expectation)
	if err != nil {
		return JudgeTask{}, err
	}
	task.InputSnapshot.Expectation = expectation
	return task, nil
}

// needsSandbox 判断判题是否需要 M2 沙箱执行或链能力。
func needsSandbox(task JudgeTask) bool {
	switch task.InputSnapshot.JudgerType {
	case JudgerTypeFlag:
		return strings.TrimSpace(jsonx.StringFromAny(task.InputSnapshot.Expectation["flag_chain_target"])) != ""
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
		if info.TenantID != task.TenantID || info.Status == contracts.SandboxStatusDestroyed || info.Status == contracts.SandboxStatusFailed {
			return 0, false, apperr.ErrJudgeTaskStateInvalid
		}
		if info.SourceRef != strings.TrimSpace(task.InputSnapshot.SandboxSourceRef) {
			return 0, false, apperr.ErrJudgeTaskStateInvalid
		}
		return id, false, nil
	}
	info, err := s.sandbox.CreateSandbox(ctx, contracts.SandboxCreateRequest{
		TenantID:            task.TenantID,
		CompositionSnapshot: task.InputSnapshot.CompositionSnapshot,
		InitScriptRef:       task.InputSnapshot.InitScriptRef,
		OwnerAccountID:      task.SubmitterID,
		SourceRef:           task.SourceRef,
		ScopeRef:            task.SourceRef,
		KeepAlive:           false,
		SnapshotEnabled:     false,
		PrivateSidecars:     privateSidecarsForSandbox(task.InputSnapshot.ExecutionSidecars),
	})
	if err != nil {
		return 0, false, apperr.ErrJudgeWorkerFailed.WithCause(err)
	}
	if err := s.waitSandboxReady(ctx, task, info.SandboxID); err != nil {
		return info.SandboxID, true, err
	}
	if judgerRequiresCode(task.InputSnapshot.JudgerType, task.SandboxMode) {
		if err := s.injectStudentCode(ctx, task, info.SandboxID); err != nil {
			return info.SandboxID, true, err
		}
	}
	if err := s.injectPrivateSuite(ctx, task, info.SandboxID); err != nil {
		return info.SandboxID, true, err
	}
	return info.SandboxID, true, nil
}

// sandboxSourceRef 返回 M2 资源所有权来源; reuse 判题不能使用本次任务事件来源。
func sandboxSourceRef(task JudgeTask) string {
	if task.SandboxMode == JudgeSandboxModeReuse && strings.TrimSpace(task.InputSnapshot.SandboxSourceRef) != "" {
		return task.InputSnapshot.SandboxSourceRef
	}
	return task.SourceRef
}

// privateSidecarsForSandbox 把 M3 判题器内部 WorkloadSpec 转为 M2 跨模块 DTO。
func privateSidecarsForSandbox(items []workload.ComponentSpec) []contracts.SandboxPrivateSidecarSpec {
	out := make([]contracts.SandboxPrivateSidecarSpec, 0, len(items))
	for _, item := range items {
		env := make([]contracts.SandboxEnvVarSpec, 0, len(item.Env))
		for _, v := range item.Env {
			env = append(env, contracts.SandboxEnvVarSpec{Name: v.Name, Value: v.Value})
		}
		mounts := make([]contracts.SandboxEphemeralMountSpec, 0, len(item.EphemeralMounts))
		for _, mount := range item.EphemeralMounts {
			mounts = append(mounts, contracts.SandboxEphemeralMountSpec{Name: mount.Name, MountPath: mount.MountPath})
		}
		out = append(out, contracts.SandboxPrivateSidecarSpec{
			Name:                   item.Name,
			ImageURL:               item.ImageURL,
			Command:                append([]string(nil), item.Command...),
			Args:                   append([]string(nil), item.Args...),
			Env:                    env,
			Resources:              contracts.SandboxResourceSpec{Requests: maps.Clone(item.Resources.Requests), Limits: maps.Clone(item.Resources.Limits)},
			Workdir:                item.Workdir,
			ReadOnlyRootFilesystem: item.ReadOnlyRootFilesystem,
			Labels:                 maps.Clone(item.Labels),
			MountWorkspace:         item.MountWorkspace,
			EphemeralMounts:        mounts,
		})
	}
	return out
}

// waitSandboxReady 轮询 M2 沙箱状态直到运行时可执行或超时。
func (s *Service) waitSandboxReady(ctx context.Context, task JudgeTask, sandboxID int64) error {
	graceSeconds, ok := intx.Int32(s.cfg.SandboxReadyGraceSeconds)
	if !ok || graceSeconds < 0 {
		return apperr.ErrJudgeTimeout
	}
	timeoutSeconds := int64(task.InputSnapshot.TimeoutSec) + int64(graceSeconds)
	if timeoutSeconds <= 0 {
		return apperr.ErrJudgeTimeout
	}
	timeout := time.Duration(timeoutSeconds) * time.Second
	if timeout <= 0 {
		return apperr.ErrJudgeTimeout
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
		if (info.Status == contracts.SandboxStatusReady || info.Status == contracts.SandboxStatusRunning || info.Status == contracts.SandboxStatusIdle) && info.Phase >= contracts.SandboxPhaseReady {
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

// injectStudentCode 注入 M3 已校验并重打包的学生提交,避免信任原始对象引用。
func (s *Service) injectStudentCode(ctx context.Context, task JudgeTask, sandboxID int64) error {
	if strings.TrimSpace(task.InputSnapshot.SanitizedCodeArchiveRef) == "" {
		return apperr.ErrJudgeInputArchiveInvalid
	}
	archiveName := strings.TrimSpace(task.InputSnapshot.SanitizedCodeArchiveName)
	if archiveName == "" {
		archiveName = "submission.tar"
	}
	injectCtx, cancel := context.WithTimeout(ctx, time.Duration(s.cfg.InputInjectTimeoutSeconds)*time.Second)
	defer cancel()
	_, data, err := s.readObjectRef(injectCtx, task.InputSnapshot.SanitizedCodeArchiveRef)
	if err != nil {
		return apperr.ErrJudgeInputArchiveInvalid.WithCause(err)
	}
	return s.sandbox.PutSandboxPrivateArchive(injectCtx, contracts.SandboxPrivateArchiveInjectRequest{
		TenantID:      task.TenantID,
		SandboxID:     sandboxID,
		SourceRef:     task.SourceRef,
		Domain:        contracts.SandboxPrivateDomainJudge,
		ArchiveName:   archiveName,
		ContentBase64: base64.StdEncoding.EncodeToString(data),
	})
}

// runJudgeCommand 执行平台配置的受控命令并解析结构化 stdout。
func (s *Service) runJudgeCommand(ctx context.Context, task JudgeTask, sandboxID int64) (JudgeExecutionResult, error) {
	if len(task.InputSnapshot.Command) == 0 {
		return JudgeExecutionResult{}, apperr.ErrJudgerConfigInvalid
	}
	stdin, err := jsonx.EncodeLineBytes(map[string]any{
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
		Container:  task.InputSnapshot.ExecTarget,
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
			ID:              s.ids.Generate(),
			TaskID:          task.ID,
			TenantID:        task.TenantID,
			Passed:          result.Passed,
			Score:           result.Score,
			MaxScore:        result.MaxScore,
			Details:         result.Details,
			Replay:          result.Replay,
			JudgeSandboxRef: result.JudgeSandboxRef,
			IsRejudge:       task.InputSnapshot.Rejudge,
		})
		if err != nil {
			return apperr.ErrJudgeTaskPersistFailed.WithCause(err)
		}
		completed, err = tx.CompleteJudgeTask(ctx, task.TenantID, task.ID, task.LeaseToken)
		if err != nil {
			return apperr.ErrJudgeTaskPersistFailed.WithCause(err)
		}
		payload := contracts.JudgeCompletedEvent{TenantID: task.TenantID, TraceID: task.InputSnapshot.TraceID, TaskID: task.ID, SourceRef: task.SourceRef, Status: contracts.JudgeTaskStatusDone, Score: saved.Score, Passed: saved.Passed, FinishedAt: saved.JudgedAt}
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
	if task.RetryCount < task.MaxRetries {
		var retry JudgeTask
		if err := s.store.TenantTx(ctx, task.TenantID, func(ctx context.Context, tx TxStore) error {
			var err error
			retry, err = tx.RetryJudgeTask(ctx, task.TenantID, task.ID, reason, task.LeaseToken)
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
		failed, err = tx.FailJudgeTask(ctx, task.TenantID, task.ID, reason, task.LeaseToken)
		if err != nil {
			return apperr.ErrJudgeTaskPersistFailed.WithCause(err)
		}
		payload := contracts.JudgeFailedEvent{TenantID: task.TenantID, TraceID: task.InputSnapshot.TraceID, TaskID: task.ID, SourceRef: task.SourceRef, Reason: reason, FailedAt: timex.Now()}
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

// publishPendingOutbox 发布已落库的终态事件并回写发布状态。
func (s *Service) publishPendingOutbox(ctx context.Context) error {
	var items []JudgeEventOutbox
	limit, ok := intx.Int32(s.cfg.WorkerBatchSize)
	if !ok || limit <= 0 {
		limit = 10
	}
	leaseToken, err := pkgcrypto.RandomToken(48)
	if err != nil {
		return apperr.ErrJudgeEventPublishFailed.WithCause(err)
	}
	leaseUntil := time.Now().Add(time.Duration(s.cfg.OutboxLeaseDurationMs) * time.Millisecond)
	maxAttempts, ok := intx.Int32(s.cfg.OutboxMaxAttempts)
	if !ok || maxAttempts <= 0 {
		return apperr.ErrJudgeEventPublishFailed
	}
	staleBefore := time.Now()
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		if _, err = tx.ExhaustExpiredEventOutbox(ctx, maxAttempts, staleBefore); err != nil {
			return apperr.ErrJudgeEventPublishFailed.WithCause(err)
		}
		items, err = tx.ClaimPendingOutbox(ctx, limit, maxAttempts, staleBefore, leaseUntil, leaseToken)
		if err != nil {
			return apperr.ErrJudgeEventPublishFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return err
	}
	for _, item := range items {
		if err := s.bus.Publish(ctx, item.Subject, json.RawMessage(item.Payload)); err != nil {
			s.recordOutboxPublishFailure(ctx, item, err)
			continue
		}
		if err := s.markOutboxPublished(ctx, item); err != nil {
			s.recordOutboxPublishFailure(ctx, item, err)
			continue
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
		_, err := tx.MarkOutboxPublished(ctx, item.TenantID, item.ID, item.LeaseToken)
		if err != nil {
			return apperr.ErrJudgeEventPublishFailed.WithCause(err)
		}
		return nil
	})
}

// markOutboxFailed 用特权事务记录 outbox 发布失败原因。
func (s *Service) markOutboxFailed(ctx context.Context, item JudgeEventOutbox, cause error) error {
	return s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		_, err := tx.MarkOutboxFailed(ctx, item.TenantID, item.ID, safeFailureReason(cause), item.LeaseToken)
		if err != nil {
			return apperr.ErrJudgeEventPublishFailed.WithCause(err)
		}
		return nil
	})
}

// executeJudgerSelftest 用判题器声明的自检样例验证配置可执行性。
func (s *Service) executeJudgerSelftest(ctx context.Context, j Judger) error {
	if err := validateJudgerResourceSpecForUse(j); err != nil {
		return err
	}
	if len(j.ResourceSpec.Selftest) == 0 {
		return apperr.ErrJudgerConfigInvalid
	}
	selftestArchive, err := selftestSubmissionArchive()
	if err != nil {
		return apperr.ErrJudgerConfigInvalid.WithCause(err)
	}
	expectation, _ := j.ResourceSpec.Selftest["expectation"].(map[string]any)
	extra, _ := j.ResourceSpec.Selftest["extra_input"].(map[string]any)
	snapshot, err := s.snapshotExpectationForJudger(j.Type, expectation, extra)
	if err != nil {
		return err
	}
	executionExpectation := snapshot
	if j.Type == JudgerTypeOnchainAssert || j.Type == JudgerTypeSimCheckpoint {
		executionExpectation = cloneExpectationMap(expectation)
	}
	task := JudgeTask{
		ID:          s.ids.Generate(),
		TenantID:    int64FromAny(j.ResourceSpec.Selftest["tenant_id"]),
		SubmitterID: int64FromAny(j.ResourceSpec.Selftest["submitter_id"]),
		SourceRef:   strings.TrimSpace(jsonx.StringFromAny(j.ResourceSpec.Selftest["source_ref"])),
		InputSnapshot: JudgeInputSnapshot{
			JudgerType:               j.Type,
			CompositionSnapshot:      j.ResourceSpec.CompositionSnapshot,
			GenesisRef:               j.ResourceSpec.GenesisRef,
			InitScriptRef:            j.ResourceSpec.InitScriptRef,
			Command:                  append([]string(nil), j.ResourceSpec.Command...),
			ExecTarget:               strings.TrimSpace(j.ResourceSpec.ExecTarget),
			ExecutionSidecars:        append([]workload.ComponentSpec(nil), j.ResourceSpec.ExecutionSidecars...),
			TimeoutSec:               timeoutForSnapshot(j),
			MaxScore:                 int32FromAny(j.ResourceSpec.Selftest["max_score"]),
			Expectation:              executionExpectation,
			ExtraInput:               extra,
			SanitizedCodeArchiveName: "selftest-submission.tar",
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
	sanitized, err := s.storeSanitizedCodeArchive(ctx, task.TenantID, task.ID, selftestArchive)
	if err != nil {
		return err
	}
	task.InputSnapshot.SanitizedCodeArchiveRef = sanitized
	result, err := s.executeTask(ctx, task)
	if err != nil {
		return err
	}
	if !result.Passed {
		return apperr.ErrJudgerSelftestFailed
	}
	return nil
}

// selftestSubmissionArchive 生成最小安全提交包,供判题器自检走完整 fresh 注入链路。
func selftestSubmissionArchive() ([]byte, error) {
	const name = "main.txt"
	const body = "judge selftest submission\n"
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
		return nil, err
	}
	if _, err := tw.Write([]byte(body)); err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// decodeCommandResult 解析判题器 stdout JSON 并限制可见详情大小。
func decodeCommandResult(stdout []byte, maxBytes int) (JudgeExecutionResult, error) {
	if maxBytes <= 0 {
		return JudgeExecutionResult{}, apperr.ErrJudgeWorkerFailed
	}
	if len(stdout) == 0 || len(stdout) > maxBytes {
		return JudgeExecutionResult{}, apperr.ErrJudgeWorkerFailed
	}
	var out JudgeExecutionResult
	if err := jsonx.DecodeStrictKnownFields(stdout, &out); err != nil {
		return JudgeExecutionResult{}, apperr.ErrJudgeWorkerFailed.WithCause(err)
	}
	return out, nil
}

// normalizeExecutionResult 校验分数边界,并限制脱敏详情与回放轨迹的持久化大小。
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
	replaySize, err := jsonx.AnyBytes(result.Replay, apperr.ErrJudgeWorkerFailed)
	if err != nil {
		return JudgeExecutionResult{}, err
	}
	if len(replaySize) > maxDetailsBytes {
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

// parseSandboxRef 解析复用沙箱引用,统一只接受十进制沙箱 ID。
func parseSandboxRef(raw string) (int64, error) {
	value := strings.TrimSpace(raw)
	id, ok := ids.Parse(value)
	if !ok {
		return 0, apperr.ErrJudgeSubmitInvalid
	}
	return id, nil
}

// judgeSandboxRef 生成判题结果中的沙箱快照引用。
func judgeSandboxRef(id int64) string {
	if id <= 0 {
		return ""
	}
	return "sandbox:" + ids.Format(id)
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
