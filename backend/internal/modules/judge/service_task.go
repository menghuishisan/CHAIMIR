// judge service_task 文件实现 M3 任务状态机、人工评分和进度订阅,不直接访问 HTTP 或 sqlc。
package judge

import (
	"context"
	"fmt"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/response"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"
)

// ListTasks 按租户分页查询判题任务,供教师和学校管理员查看队列与人工评分项。
func (s *Service) ListTasks(ctx context.Context, tenantID, accountID int64, sourceRef string, pendingManual bool, page, size int) ([]map[string]any, int64, int, int, error) {
	if tenantID <= 0 || accountID <= 0 {
		return nil, 0, 0, 0, apperr.ErrJudgeSubmitInvalid
	}
	page, size = pagex.Normalize(page, size)
	offset := int32((page - 1) * size)
	limit := int32(size)
	var (
		items []JudgeTaskInfo
		total int64
	)
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		items, total, err = tx.ListJudgeTasks(ctx, tenantID, strings.TrimSpace(sourceRef), pendingManual, accountID, limit, offset)
		if err != nil {
			return apperr.ErrJudgeTaskNotFound.WithCause(err)
		}
		return nil
	}); err != nil {
		return nil, 0, 0, 0, err
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, taskInfoToMap(item))
	}
	return out, total, page, size, nil
}

// CancelTask 取消仍在排队中的任务,防止执行中任务被静默打断。
func (s *Service) CancelTask(ctx context.Context, tenantID, taskID int64) error {
	if tenantID <= 0 || taskID <= 0 {
		return apperr.ErrJudgeSubmitInvalid
	}
	var task JudgeTask
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		task, err = tx.CancelQueuedJudgeTask(ctx, tenantID, taskID)
		if err != nil {
			return apperr.ErrJudgeTaskStateInvalid.WithCause(err)
		}
		return nil
	}); err != nil {
		return err
	}
	s.publishProgress(ctx, tenantID, task.ID, task.Status, ProgressStageFailed, "判题任务已取消")
	return s.writeSystemAudit(ctx, tenantID, "judge.cancel", "judge_task", task.ID, map[string]any{"source_ref": task.SourceRef, "submitter_id": task.SubmitterID})
}

// RejudgeTask 按原输入快照重置任务,只允许已完成或失败终态进入重判。
func (s *Service) RejudgeTask(ctx context.Context, tenantID, taskID int64) (JudgeTaskInfo, error) {
	info, err := s.rejudgeTask(ctx, tenantID, taskID)
	if err != nil {
		return JudgeTaskInfo{}, err
	}
	if err := s.writeSystemAudit(ctx, tenantID, "judge.rejudge", "judge_task", info.Task.ID, map[string]any{"source_ref": info.Task.SourceRef, "submitter_id": info.Task.SubmitterID}); err != nil {
		return JudgeTaskInfo{}, err
	}
	return info, nil
}

// rejudgeTask 执行重判状态重置并返回最新任务快照,审计由调用场景决定。
func (s *Service) rejudgeTask(ctx context.Context, tenantID, taskID int64) (JudgeTaskInfo, error) {
	if tenantID <= 0 || taskID <= 0 {
		return JudgeTaskInfo{}, apperr.ErrJudgeSubmitInvalid
	}
	var task JudgeTask
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		existing, err := tx.GetJudgeTask(ctx, tenantID, taskID)
		if err != nil {
			return apperr.ErrJudgeTaskNotFound.WithCause(err)
		}
		snapshot := existing.InputSnapshot
		snapshot.Rejudge = true
		task, err = tx.ResetJudgeTaskForRejudge(ctx, tenantID, taskID, snapshot)
		if err != nil {
			return apperr.ErrJudgeTaskStateInvalid.WithCause(err)
		}
		return nil
	}); err != nil {
		return JudgeTaskInfo{}, err
	}
	s.publishProgress(ctx, tenantID, task.ID, task.Status, ProgressStageQueued, "判题任务已进入重判队列")
	return s.getTaskInfo(ctx, tenantID, task.ID)
}

// RejudgeTaskForUser 校验教师来源归属后触发单条重判。
func (s *Service) RejudgeTaskForUser(ctx context.Context, tenantID, accountID, taskID int64) (JudgeTaskInfo, error) {
	if err := s.ensureUserCanAccessTask(ctx, tenantID, accountID, taskID); err != nil {
		return JudgeTaskInfo{}, err
	}
	info, err := s.rejudgeTask(ctx, tenantID, taskID)
	if err != nil {
		return JudgeTaskInfo{}, err
	}
	if err := s.writeAuditFromContext(ctx, tenantID, "judge.rejudge.user", "judge_task", taskID, map[string]any{"source_ref": info.Task.SourceRef}); err != nil {
		return JudgeTaskInfo{}, err
	}
	return info, nil
}

// RejudgeBatch 按来源标识批量重判已完成或失败任务。
func (s *Service) RejudgeBatch(ctx context.Context, tenantID int64, sourceRef string) error {
	sourceRef = strings.TrimSpace(sourceRef)
	if tenantID <= 0 || !auth.ValidSourceRef(sourceRef) {
		return apperr.ErrJudgeSubmitInvalid
	}
	var changed []JudgeTask
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		items, err := tx.ListJudgeTasksBySourceRef(ctx, tenantID, sourceRef)
		if err != nil {
			return apperr.ErrJudgeTaskNotFound.WithCause(err)
		}
		for _, item := range items {
			if item.Status != JudgeTaskStatusDone && item.Status != JudgeTaskStatusFailed {
				continue
			}
			snapshot := item.InputSnapshot
			snapshot.Rejudge = true
			updated, err := tx.ResetJudgeTaskForRejudge(ctx, tenantID, item.ID, snapshot)
			if err != nil {
				return apperr.ErrJudgeTaskStateInvalid.WithCause(err)
			}
			changed = append(changed, updated)
		}
		return nil
	}); err != nil {
		return err
	}
	for _, task := range changed {
		s.publishProgress(ctx, tenantID, task.ID, task.Status, ProgressStageQueued, "判题任务已进入重判队列")
	}
	if len(changed) == 0 {
		return apperr.ErrJudgeTaskStateInvalid
	}
	return s.writeSystemAudit(ctx, tenantID, "judge.rejudge_batch", "judge_source", 0, map[string]any{"source_ref": sourceRef, "count": len(changed)})
}

// ManualScore 保存人工评分结果并在同一事务写入终态 outbox。
func (s *Service) ManualScore(ctx context.Context, tenantID, taskID, scorerID int64, req ManualScoreRequest) (map[string]any, error) {
	if tenantID <= 0 || taskID <= 0 || scorerID <= 0 {
		return nil, apperr.ErrJudgeSubmitInvalid
	}
	if err := validateManualScore(req); err != nil {
		return nil, err
	}
	var info JudgeTaskInfo
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		task, err := tx.GetJudgeTask(ctx, tenantID, taskID)
		if err != nil {
			return apperr.ErrJudgeTaskNotFound.WithCause(err)
		}
		if err := ensureAccountCanAccessTask(task, scorerID); err != nil {
			return err
		}
		if task.Status != JudgeTaskStatusJudging || task.InputSnapshot.JudgerType != JudgerTypeManual {
			return apperr.ErrJudgeTaskStateInvalid
		}
		result := JudgeResult{
			ID:       s.ids.Generate(),
			TaskID:   task.ID,
			TenantID: task.TenantID,
			Passed:   req.Passed,
			Score:    req.Score,
			MaxScore: req.MaxScore,
			Details: []JudgeResultDetail{{
				Case:          "人工评分",
				Passed:        req.Passed,
				ExpectedLabel: "教师人工评分",
				Actual:        strings.TrimSpace(req.Comment),
			}},
			IsRejudge: task.InputSnapshot.Rejudge,
		}
		if err := validateResultDetails(result.Details); err != nil {
			return err
		}
		saved, err := tx.UpsertJudgeResult(ctx, result)
		if err != nil {
			return apperr.ErrJudgeTaskPersistFailed.WithCause(err)
		}
		task, err = tx.CompleteJudgeTask(ctx, tenantID, taskID)
		if err != nil {
			return apperr.ErrJudgeTaskPersistFailed.WithCause(err)
		}
		traceID := response.TraceFromContext(ctx)
		if strings.TrimSpace(traceID) == "" {
			return apperr.ErrJudgeTaskStateInvalid.WithCause(fmt.Errorf("人工评分缺少 trace_id"))
		}
		payload := contracts.JudgeCompletedEvent{TenantID: tenantID, TraceID: traceID, TaskID: task.ID, SourceRef: task.SourceRef, Status: contracts.JudgeTaskStatusDone, Score: saved.Score, Passed: saved.Passed, FinishedAt: saved.JudgedAt}
		if _, err := tx.CreateOutbox(ctx, s.ids.Generate(), tenantID, task.ID, contracts.SubjectJudgeCompleted, payload); err != nil {
			return apperr.ErrJudgeTaskPersistFailed.WithCause(err)
		}
		info = JudgeTaskInfo{Task: task, Result: &saved}
		return nil
	}); err != nil {
		return nil, err
	}
	s.publishProgress(ctx, tenantID, taskID, JudgeTaskStatusDone, ProgressStageDone, "判题任务已完成")
	if err := s.publishPendingOutbox(ctx); err != nil {
		return nil, err
	}
	if err := s.writeAuditFromContext(ctx, tenantID, "judge.manual_score", "judge_task", taskID, map[string]any{"score": req.Score, "max_score": req.MaxScore, "scorer_id": scorerID}); err != nil {
		return nil, err
	}
	return taskInfoToMap(info), nil
}

// ProgressSubscription 校验任务存在后返回进度订阅 topic 和当前快照。
func (s *Service) ProgressSubscription(ctx context.Context, tenantID, accountID, taskID int64) (string, ProgressMessage, error) {
	if tenantID <= 0 || accountID <= 0 || taskID <= 0 {
		return "", ProgressMessage{}, apperr.ErrJudgeSubmitInvalid
	}
	info, err := s.getTaskInfo(ctx, tenantID, taskID)
	if err != nil {
		return "", ProgressMessage{}, err
	}
	if err := ensureAccountCanAccessTask(info.Task, accountID); err != nil {
		return "", ProgressMessage{}, err
	}
	return judgeProgressTopic(tenantID, taskID), ProgressMessage{TaskID: taskID, Status: statusText(info.Task.Status), Stage: progressStage(info.Task.Status), Message: progressMessage(info.Task.Status)}, nil
}

// GetTaskInfoForUser 校验用户态来源归属后返回任务与结果。
func (s *Service) GetTaskInfoForUser(ctx context.Context, tenantID, accountID, taskID int64) (JudgeTaskInfo, error) {
	info, err := s.getTaskInfo(ctx, tenantID, taskID)
	if err != nil {
		return JudgeTaskInfo{}, err
	}
	if err := ensureAccountCanAccessTask(info.Task, accountID); err != nil {
		return JudgeTaskInfo{}, err
	}
	return info, nil
}

// ensureUserCanAccessTask 读取任务并执行用户态来源归属校验。
func (s *Service) ensureUserCanAccessTask(ctx context.Context, tenantID, accountID, taskID int64) error {
	info, err := s.getTaskInfo(ctx, tenantID, taskID)
	if err != nil {
		return err
	}
	return ensureAccountCanAccessTask(info.Task, accountID)
}

// ensureAccountCanAccessTask 防止普通教师按租户横向访问非本人来源任务。
func ensureAccountCanAccessTask(task JudgeTask, accountID int64) error {
	if accountID <= 0 {
		return apperr.ErrJudgeTaskForbidden
	}
	if task.SourceOwnerID > 0 {
		if task.SourceOwnerID == accountID || task.SubmitterID == accountID {
			return nil
		}
		return apperr.ErrJudgeTaskForbidden
	}
	if task.SubmitterID == accountID && task.SourceScope != "" {
		return nil
	}
	return apperr.ErrJudgeTaskForbidden
}

// progressStage 将任务状态映射为用户向进度阶段。
func progressStage(status int16) string {
	switch status {
	case JudgeTaskStatusDone:
		return ProgressStageDone
	case JudgeTaskStatusFailed, JudgeTaskStatusCancelled, JudgeTaskStatusError, JudgeTaskStatusTimeout:
		return ProgressStageFailed
	case JudgeTaskStatusJudging:
		return ProgressStageJudging
	default:
		return ProgressStageQueued
	}
}

// progressMessage 返回当前状态的用户向说明。
func progressMessage(status int16) string {
	switch status {
	case JudgeTaskStatusDone:
		return "判题任务已完成"
	case JudgeTaskStatusFailed, JudgeTaskStatusError:
		return "判题任务执行失败"
	case JudgeTaskStatusTimeout:
		return "判题任务执行超时"
	case JudgeTaskStatusCancelled:
		return "判题任务已取消"
	case JudgeTaskStatusJudging:
		return "判题任务正在执行"
	default:
		return "判题任务等待执行"
	}
}

// bindProgressConn 把 WebSocket 连接绑定到账号并订阅任务进度。
func (s *Service) bindProgressConn(ctx context.Context, conn *ws.Conn, tenantID, accountID, taskID int64) error {
	topic, initial, err := s.ProgressSubscription(ctx, tenantID, accountID, taskID)
	if err != nil {
		return err
	}
	if err := conn.BindSession(ws.SessionKey{TenantID: tenantID, AccountID: accountID}); err != nil {
		return apperr.ErrJudgeTaskStateInvalid.WithCause(err)
	}
	if err := s.wsHub.Subscribe(conn, topic); err != nil {
		return apperr.ErrJudgeTaskStateInvalid.WithCause(err)
	}
	return conn.SendJSON(initial)
}
