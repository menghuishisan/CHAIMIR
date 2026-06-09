// M3 数据访问层:只读写 judge 模块自有表,全部经 sqlc 生成查询。
package judge

import (
	"context"

	"chaimir/internal/modules/judge/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// repo 封装 judge 模块数据库事务入口。
type repo struct {
	db *db.DB
}

// newRepo 构造 M3 repo。
func newRepo(database *db.DB) *repo {
	return &repo{db: database}
}

// queryFunc 是 M3 数据访问闭包,统一接收 sqlc 查询对象。
type queryFunc func(q *sqlcgen.Queries) error

// inTenant 从 ctx 取租户并注入 RLS 后执行查询。
func (r *repo) inTenant(ctx context.Context, fn queryFunc) error {
	return r.db.WithTenantTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// inTenantID 用显式租户 ID 注入 RLS,供内部 contracts 调用与 worker 使用。
func (r *repo) inTenantID(ctx context.Context, tenantID int64, fn queryFunc) error {
	return r.db.WithTenantTxID(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// inApp 访问 judger 平台级配置表。
func (r *repo) inApp(ctx context.Context, fn queryFunc) error {
	return r.db.WithAppTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return fn(sqlcgen.New(tx))
	})
}

// listJudgers 查询平台级判题器配置列表。
func (r *repo) listJudgers(ctx context.Context, limit, offset int32) ([]JudgerSnapshot, error) {
	var rows []sqlcgen.Judger
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		found, err := q.ListJudgers(ctx, sqlcgen.ListJudgersParams{Limit: limit, Offset: offset})
		rows = found
		return err
	}); err != nil {
		return nil, apperr.ErrJudgerPersistence.WithCause(err)
	}
	return judgerSnapshotsFromRows(rows), nil
}

// createJudger 写入平台级判题器定义。
func (r *repo) createJudger(ctx context.Context, req CreateJudgerRequest, id int64, spec []byte, status int16) (JudgerSnapshot, error) {
	var row sqlcgen.Judger
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.CreateJudger(ctx, sqlcgen.CreateJudgerParams{
			ID:                id,
			Code:              req.Code,
			Name:              req.Name,
			Type:              req.Type,
			ExecutorRef:       req.ExecutorRef,
			RuntimeRequired:   req.RuntimeRequired,
			DefaultTimeoutSec: req.DefaultTimeoutSec,
			ResourceSpec:      spec,
			SelftestStatus:    JudgerSelftestPending,
			SelftestDetail:    []byte("{}"),
			Status:            status,
		})
		return err
	}); err != nil {
		return JudgerSnapshot{}, apperr.ErrJudgerPersistence.WithCause(err)
	}
	return judgerSnapshotFromRow(row), nil
}

// updateJudger 更新平台级判题器定义。
func (r *repo) updateJudger(ctx context.Context, judgerID int64, req UpdateJudgerRequest, spec []byte) (JudgerSnapshot, error) {
	var row sqlcgen.Judger
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.UpdateJudger(ctx, sqlcgen.UpdateJudgerParams{
			ID:                judgerID,
			Name:              req.Name,
			Type:              req.Type,
			ExecutorRef:       req.ExecutorRef,
			RuntimeRequired:   req.RuntimeRequired,
			DefaultTimeoutSec: req.DefaultTimeoutSec,
			ResourceSpec:      spec,
			Status:            req.Status,
		})
		if db.IsNoRows(err) {
			return apperr.ErrJudgerNotFound
		}
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return JudgerSnapshot{}, ae
		}
		return JudgerSnapshot{}, apperr.ErrJudgerPersistence.WithCause(err)
	}
	return judgerSnapshotFromRow(row), nil
}

// getJudgerByID 读取平台级判题器定义。
func (r *repo) getJudgerByID(ctx context.Context, judgerID int64) (JudgerSnapshot, error) {
	var row sqlcgen.Judger
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.GetJudgerByID(ctx, judgerID)
		if db.IsNoRows(err) {
			return apperr.ErrJudgerNotFound
		}
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return JudgerSnapshot{}, ae
		}
		return JudgerSnapshot{}, apperr.ErrJudgerPersistence.WithCause(err)
	}
	return judgerSnapshotFromRow(row), nil
}

// getJudgerByCode 按 code 读取平台级判题器定义。
func (r *repo) getJudgerByCode(ctx context.Context, code string) (JudgerSnapshot, error) {
	var row sqlcgen.Judger
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.GetJudgerByCode(ctx, code)
		if db.IsNoRows(err) {
			return apperr.ErrJudgerNotFound
		}
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return JudgerSnapshot{}, ae
		}
		return JudgerSnapshot{}, apperr.ErrJudgerPersistence.WithCause(err)
	}
	return judgerSnapshotFromRow(row), nil
}

// updateJudgerSelftest 写回判题器自检结果。
func (r *repo) updateJudgerSelftest(ctx context.Context, judgerID int64, selftestStatus int16, detail []byte, status int16) (JudgerSnapshot, error) {
	var row sqlcgen.Judger
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.UpdateJudgerSelftest(ctx, sqlcgen.UpdateJudgerSelftestParams{
			ID:             judgerID,
			SelftestStatus: selftestStatus,
			SelftestDetail: detail,
			Status:         status,
		})
		return err
	}); err != nil {
		return JudgerSnapshot{}, apperr.ErrJudgerPersistence.WithCause(err)
	}
	return judgerSnapshotFromRow(row), nil
}

// createTaskWithFingerprint 在同一租户事务内创建判题任务和查重指纹。
func (r *repo) createTaskWithFingerprint(ctx context.Context, input JudgeTaskCreate) (JudgeTaskSnapshot, error) {
	var row sqlcgen.JudgeTask
	if err := r.inTenantID(ctx, input.TenantID, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.CreateJudgeTask(ctx, sqlcgen.CreateJudgeTaskParams{
			ID:               input.TaskID,
			TenantID:         input.TenantID,
			JudgerID:         input.JudgerID,
			SourceRef:        input.SourceRef,
			SubmitterID:      input.SubmitterID,
			ProblemRef:       input.ProblemRef,
			CodeStorageKey:   input.CodeStorageKey,
			CodeHash:         input.CodeHash,
			InputSnapshot:    input.InputSnapshot,
			SandboxMode:      input.SandboxMode,
			TargetSandboxRef: pgtypex.Text(input.TargetSandboxRef),
			Priority:         input.Priority,
			Status:           input.Status,
			MaxRetries:       input.MaxRetries,
		})
		if err != nil {
			return err
		}
		_, err = q.CreateSubmissionFingerprint(ctx, sqlcgen.CreateSubmissionFingerprintParams{
			ID:          input.FingerprintID,
			TenantID:    input.TenantID,
			SourceRef:   input.SourceRef,
			ProblemRef:  input.ProblemRef,
			SubmitterID: input.SubmitterID,
			CodeHash:    input.CodeHash,
			SimVector:   input.SimVector,
		})
		return err
	}); err != nil {
		return JudgeTaskSnapshot{}, apperr.ErrJudgeTaskQueuedFail.WithCause(err)
	}
	return judgeTaskSnapshotFromRow(row), nil
}

// getTaskBySourceRef 按上游来源引用查询已存在判题任务。
func (r *repo) getTaskBySourceRef(ctx context.Context, tenantID int64, sourceRef string) (JudgeTaskSnapshot, error) {
	var row sqlcgen.JudgeTask
	if err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var e error
		row, e = q.GetJudgeTaskBySourceRef(ctx, sqlcgen.GetJudgeTaskBySourceRefParams{TenantID: tenantID, SourceRef: sourceRef})
		if db.IsNoRows(e) {
			return apperr.ErrJudgeTaskNotFound
		}
		return e
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return JudgeTaskSnapshot{}, ae
		}
		return JudgeTaskSnapshot{}, apperr.ErrJudgeTaskPersistence.WithCause(err)
	}
	return judgeTaskSnapshotFromRow(row), nil
}

// markTaskStatus 更新判题任务状态。
func (r *repo) markTaskStatus(ctx context.Context, tenantID, taskID int64, status int16) error {
	return r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		_, err := q.UpdateJudgeTaskStatus(ctx, sqlcgen.UpdateJudgeTaskStatusParams{ID: taskID, Status: status})
		return err
	})
}

// getTaskView 查询任务摘要及其结果详情。
func (r *repo) getTaskView(ctx context.Context, tenantID, taskID int64) (judgeTaskView, error) {
	var row sqlcgen.GetJudgeTaskWithResultRow
	if err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.GetJudgeTaskWithResult(ctx, taskID)
		if db.IsNoRows(err) {
			return apperr.ErrJudgeTaskNotFound
		}
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return judgeTaskView{}, ae
		}
		return judgeTaskView{}, apperr.ErrJudgeTaskPersistence.WithCause(err)
	}
	return taskViewFromJoined(row), nil
}

// markTaskRejudge 把终态任务恢复为 queued 以便重新执行。
func (r *repo) markTaskRejudge(ctx context.Context, tenantID, taskID int64) (JudgeTaskSnapshot, error) {
	var row sqlcgen.JudgeTask
	if err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.MarkJudgeTaskRejudge(ctx, taskID)
		if db.IsNoRows(err) {
			return apperr.ErrJudgeTaskNotFound
		}
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return JudgeTaskSnapshot{}, ae
		}
		return JudgeTaskSnapshot{}, apperr.ErrJudgeTaskPersistence.WithCause(err)
	}
	return judgeTaskSnapshotFromRow(row), nil
}

// cancelQueuedTask 取消仍处于 queued 的判题任务。
func (r *repo) cancelQueuedTask(ctx context.Context, tenantID, taskID int64) (JudgeTaskSnapshot, error) {
	var row sqlcgen.JudgeTask
	if err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var err error
		row, err = q.CancelQueuedJudgeTask(ctx, taskID)
		if db.IsNoRows(err) {
			return apperr.ErrJudgeTaskInvalidState
		}
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return JudgeTaskSnapshot{}, ae
		}
		return JudgeTaskSnapshot{}, apperr.ErrJudgeTaskPersistence.WithCause(err)
	}
	return judgeTaskSnapshotFromRow(row), nil
}

// listTasksBySourceRef 查询指定来源下的判题任务。
func (r *repo) listTasksBySourceRef(ctx context.Context, tenantID int64, sourceRef string, limit, offset int32) ([]JudgeTaskSnapshot, error) {
	var rows []sqlcgen.JudgeTask
	if err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		found, err := q.ListTasksBySourceRef(ctx, sqlcgen.ListTasksBySourceRefParams{SourceRef: sourceRef, Limit: limit, Offset: offset})
		rows = found
		return err
	}); err != nil {
		return nil, apperr.ErrJudgeTaskPersistence.WithCause(err)
	}
	return judgeTaskSnapshotsFromRows(rows), nil
}

// listManualPendingTasks 查询待人工评分任务。
func (r *repo) listManualPendingTasks(ctx context.Context, tenantID int64, sourceRef string, limit, offset int32) ([]JudgeTaskSnapshot, error) {
	var rows []sqlcgen.JudgeTask
	if err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		found, err := q.ListManualPendingTasks(ctx, sqlcgen.ListManualPendingTasksParams{SourceRef: sourceRef, Limit: limit, Offset: offset})
		rows = found
		return err
	}); err != nil {
		return nil, apperr.ErrJudgeTaskPersistence.WithCause(err)
	}
	return judgeTaskSnapshotsFromRows(rows), nil
}

// getTaskAndJudgerType 读取任务及绑定判题器类型,供人工评分前置校验。
func (r *repo) getTaskAndJudgerType(ctx context.Context, tenantID, taskID int64) (JudgeTaskSnapshot, int16, error) {
	var task sqlcgen.JudgeTask
	var judger sqlcgen.Judger
	if err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var err error
		task, err = q.GetJudgeTaskByID(ctx, taskID)
		if db.IsNoRows(err) {
			return apperr.ErrJudgeTaskNotFound
		}
		if err != nil {
			return err
		}
		judger, err = q.GetJudgerByID(ctx, task.JudgerID)
		if db.IsNoRows(err) {
			return apperr.ErrJudgerNotFound
		}
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return JudgeTaskSnapshot{}, 0, ae
		}
		return JudgeTaskSnapshot{}, 0, apperr.ErrJudgeTaskPersistence.WithCause(err)
	}
	return judgeTaskSnapshotFromRow(task), judger.Type, nil
}

// completeTaskResult 写入判题结果、推进任务终态并创建 outbox。
func (r *repo) completeTaskResult(ctx context.Context, result JudgeResultCreate, outbox JudgeOutboxCreate) error {
	return r.inTenantID(ctx, result.TenantID, func(q *sqlcgen.Queries) error {
		if _, err := q.CreateJudgeResult(ctx, sqlcgen.CreateJudgeResultParams{
			TaskID:          result.TaskID,
			TenantID:        result.TenantID,
			Passed:          result.Passed,
			Score:           result.Score,
			MaxScore:        result.MaxScore,
			Details:         result.Details,
			JudgeSandboxRef: optionalText(result.JudgeSandboxRef),
			IsRejudge:       result.IsRejudge,
		}); err != nil {
			return err
		}
		if _, err := q.UpdateJudgeTaskStatus(ctx, sqlcgen.UpdateJudgeTaskStatusParams{ID: result.TaskID, Status: JudgeTaskDone}); err != nil {
			return err
		}
		return createJudgeEventOutboxWithQuery(ctx, q, outbox)
	})
}

// failTaskWithOutbox 推进失败终态并创建 judge.failed outbox。
func (r *repo) failTaskWithOutbox(ctx context.Context, tenantID, taskID int64, outbox JudgeOutboxCreate) error {
	return r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		if _, err := q.FailJudgeTask(ctx, taskID); err != nil {
			return err
		}
		return createJudgeEventOutboxWithQuery(ctx, q, outbox)
	})
}

// claimQueuedTaskAcrossTenant 跨租户领取队列任务,读取 tenant_id 后回到租户事务更新。
func (r *repo) claimQueuedTaskAcrossTenant(ctx context.Context, taskID int64) (JudgeTaskSnapshot, error) {
	var current sqlcgen.JudgeTask
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		var err error
		current, err = q.GetJudgeTaskByID(ctx, taskID)
		if db.IsNoRows(err) {
			return apperr.ErrJudgeTaskNotFound
		}
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return JudgeTaskSnapshot{}, ae
		}
		return JudgeTaskSnapshot{}, apperr.ErrJudgeTaskPersistence.WithCause(err)
	}
	var task sqlcgen.JudgeTask
	if err := r.inTenantID(ctx, current.TenantID, func(q *sqlcgen.Queries) error {
		var err error
		task, err = q.MarkJudgeTaskJudging(ctx, taskID)
		if db.IsNoRows(err) {
			return apperr.ErrJudgeTaskInvalidState
		}
		return err
	}); err != nil {
		if ae, ok := apperr.As(err); ok {
			return JudgeTaskSnapshot{}, ae
		}
		return JudgeTaskSnapshot{}, apperr.ErrJudgeTaskPersistence.WithCause(err)
	}
	return judgeTaskSnapshotFromRow(task), nil
}

// retryTask 尝试按重试策略把任务重新放回 queued。
func (r *repo) retryTask(ctx context.Context, tenantID, taskID int64) (JudgeTaskSnapshot, error) {
	var row sqlcgen.JudgeTask
	err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var e error
		row, e = q.RetryJudgeTask(ctx, taskID)
		return e
	})
	return judgeTaskSnapshotFromRow(row), err
}

// listPendingJudgeEventOutboxTenants 查询仍有待发布事件的租户列表。
func (r *repo) listPendingJudgeEventOutboxTenants(ctx context.Context, limit int32) ([]int64, error) {
	var tenantIDs []int64
	if err := r.inApp(ctx, func(q *sqlcgen.Queries) error {
		var err error
		tenantIDs, err = q.ListPendingJudgeEventOutboxTenants(ctx, limit)
		return err
	}); err != nil {
		return nil, apperr.ErrJudgeEventPublish.WithCause(err)
	}
	return tenantIDs, nil
}

// listPendingJudgeEventOutbox 查询单租户待发布 outbox。
func (r *repo) listPendingJudgeEventOutbox(ctx context.Context, tenantID int64, limit int32) ([]JudgeOutboxSnapshot, error) {
	var rows []sqlcgen.JudgeEventOutbox
	if err := r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		var err error
		rows, err = q.ListPendingJudgeEventOutbox(ctx, limit)
		return err
	}); err != nil {
		return nil, apperr.ErrJudgeEventPublish.WithCause(err)
	}
	return outboxSnapshotsFromRows(rows), nil
}

// markJudgeEventOutboxFailed 记录 outbox 发布失败原因。
func (r *repo) markJudgeEventOutboxFailed(ctx context.Context, tenantID, outboxID int64, lastError string) error {
	return r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		_, err := q.FailJudgeEventOutbox(ctx, sqlcgen.FailJudgeEventOutboxParams{ID: outboxID, LastError: pgtypex.Text(lastError)})
		return err
	})
}

// markJudgeEventOutboxPublished 标记 outbox 已发布。
func (r *repo) markJudgeEventOutboxPublished(ctx context.Context, tenantID, outboxID int64) error {
	return r.inTenantID(ctx, tenantID, func(q *sqlcgen.Queries) error {
		_, err := q.MarkJudgeEventOutboxPublished(ctx, outboxID)
		return err
	})
}

// listExactFingerprints 查询完全相同代码哈希的提交指纹。
func (r *repo) listExactFingerprints(ctx context.Context, problemRef, codeHash string, limit, offset int32) ([]SubmissionFingerprintSnapshot, error) {
	var rows []sqlcgen.SubmissionFingerprint
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, err := q.ListExactFingerprints(ctx, sqlcgen.ListExactFingerprintsParams{
			ProblemRef: problemRef,
			CodeHash:   codeHash,
			Limit:      limit,
			Offset:     offset,
		})
		rows = found
		return err
	}); err != nil {
		return nil, apperr.ErrJudgeTaskPersistence.WithCause(err)
	}
	return fingerprintSnapshotsFromRows(rows), nil
}

// listFingerprintsByProblem 查询同题历史指纹。
func (r *repo) listFingerprintsByProblem(ctx context.Context, problemRef string, limit, offset int32) ([]SubmissionFingerprintSnapshot, error) {
	var rows []sqlcgen.SubmissionFingerprint
	if err := r.inTenant(ctx, func(q *sqlcgen.Queries) error {
		found, err := q.ListFingerprintsByProblem(ctx, sqlcgen.ListFingerprintsByProblemParams{
			ProblemRef: problemRef,
			Limit:      limit,
			Offset:     offset,
		})
		rows = found
		return err
	}); err != nil {
		return nil, apperr.ErrSimilarityFailed.WithCause(err)
	}
	return fingerprintSnapshotsFromRows(rows), nil
}

// createJudgeEventOutboxWithQuery 在调用方事务内写入终态事件 outbox。
func createJudgeEventOutboxWithQuery(ctx context.Context, q *sqlcgen.Queries, outbox JudgeOutboxCreate) error {
	_, err := q.CreateJudgeEventOutbox(ctx, sqlcgen.CreateJudgeEventOutboxParams{
		ID:       outbox.ID,
		TenantID: outbox.TenantID,
		TaskID:   outbox.TaskID,
		Subject:  outbox.Subject,
		Payload:  outbox.Payload,
		Status:   JudgeEventOutboxPending,
	})
	return err
}

// optionalText 把可选字符串转换为数据库 nullable text。
func optionalText(value string) pgtype.Text {
	if value == "" {
		return pgtype.Text{}
	}
	return pgtypex.Text(value)
}

// judgerSnapshotsFromRows 批量转换判题器表行。
func judgerSnapshotsFromRows(rows []sqlcgen.Judger) []JudgerSnapshot {
	out := make([]JudgerSnapshot, 0, len(rows))
	for _, row := range rows {
		out = append(out, judgerSnapshotFromRow(row))
	}
	return out
}

// judgeTaskSnapshotsFromRows 批量转换判题任务表行。
func judgeTaskSnapshotsFromRows(rows []sqlcgen.JudgeTask) []JudgeTaskSnapshot {
	out := make([]JudgeTaskSnapshot, 0, len(rows))
	for _, row := range rows {
		out = append(out, judgeTaskSnapshotFromRow(row))
	}
	return out
}

// fingerprintSnapshotsFromRows 批量转换提交指纹表行。
func fingerprintSnapshotsFromRows(rows []sqlcgen.SubmissionFingerprint) []SubmissionFingerprintSnapshot {
	out := make([]SubmissionFingerprintSnapshot, 0, len(rows))
	for _, row := range rows {
		out = append(out, fingerprintSnapshotFromRow(row))
	}
	return out
}

// outboxSnapshotsFromRows 批量转换 outbox 表行。
func outboxSnapshotsFromRows(rows []sqlcgen.JudgeEventOutbox) []JudgeOutboxSnapshot {
	out := make([]JudgeOutboxSnapshot, 0, len(rows))
	for _, row := range rows {
		out = append(out, outboxSnapshotFromRow(row))
	}
	return out
}

// tenantFromContext 读取当前租户身份。
func tenantFromContext(ctx context.Context) (tenant.Identity, bool) {
	return tenant.FromContext(ctx)
}
