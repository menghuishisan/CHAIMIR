// judge repo 文件定义 M3 持久化接口和数据库事务边界,是 service 访问数据库的唯一入口。
package judge

import (
	"context"
	"fmt"
	"strings"

	"chaimir/internal/modules/judge/internal/sqlcgen"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"

	"github.com/jackc/pgx/v5"
)

// Store 定义 service 所需的 judge 持久化能力,不暴露 sqlc 行类型。
type Store interface {
	PlatformTx(ctx context.Context, fn func(context.Context, TxStore) error) error
	TenantTx(ctx context.Context, tenantID int64, fn func(context.Context, TxStore) error) error
	PrivilegedTx(ctx context.Context, fn func(context.Context, TxStore) error) error
	IsNoRows(error) bool
}

// TxStore 定义单个事务内可调用的 judge 数据访问能力。
type TxStore interface {
	GetJudgerByCode(ctx context.Context, code string) (Judger, error)
	GetJudgerByID(ctx context.Context, id int64) (Judger, error)
	ListJudgers(ctx context.Context) ([]Judger, error)
	UpsertJudger(ctx context.Context, id int64, req JudgerRequest, spec JudgerResourceSpec, selftestStatus int16) (Judger, error)
	UpdateJudgerSelftest(ctx context.Context, id int64, selftestStatus, status int16) (Judger, error)
	CreateJudgeTask(ctx context.Context, task JudgeTask) (JudgeTask, error)
	GetJudgeTask(ctx context.Context, tenantID, taskID int64) (JudgeTask, error)
	GetJudgeTaskBySourceRef(ctx context.Context, tenantID int64, sourceRef, problemRef string) (JudgeTask, error)
	GetJudgeTaskInfo(ctx context.Context, tenantID, taskID int64) (JudgeTaskInfo, error)
	ListJudgeTasksBySourceRef(ctx context.Context, tenantID int64, sourceRef string) ([]JudgeTask, error)
	ListRecentJudgeTasksBySubmitterProblem(ctx context.Context, tenantID, submitterID int64, problemRef string, windowSeconds int32) ([]JudgeTask, error)
	ListJudgeTasks(ctx context.Context, tenantID int64, sourceRef string, pendingManual bool, sourceOwnerID int64, limit, offset int32) ([]JudgeTaskInfo, int64, error)
	CancelQueuedJudgeTask(ctx context.Context, tenantID, taskID int64) (JudgeTask, error)
	ResetJudgeTaskForRejudge(ctx context.Context, tenantID, taskID int64, snapshot JudgeInputSnapshot) (JudgeTask, error)
	DequeueJudgeTasks(ctx context.Context, limit int32) ([]JudgeTask, error)
	CompleteJudgeTask(ctx context.Context, tenantID, taskID int64) (JudgeTask, error)
	MarkJudgeTaskTimeout(ctx context.Context, tenantID, taskID int64, reason string) (JudgeTask, error)
	MarkJudgeTaskError(ctx context.Context, tenantID, taskID int64, reason string) (JudgeTask, error)
	RetryJudgeTask(ctx context.Context, tenantID, taskID int64, reason string) (JudgeTask, error)
	FailJudgeTask(ctx context.Context, tenantID, taskID int64, reason string) (JudgeTask, error)
	UpsertJudgeResult(ctx context.Context, result JudgeResult) (JudgeResult, error)
	CreateOutbox(ctx context.Context, id int64, tenantID, taskID int64, subject string, payload any) (JudgeEventOutbox, error)
	ListPendingOutbox(ctx context.Context, limit int32) ([]JudgeEventOutbox, error)
	MarkOutboxPublished(ctx context.Context, tenantID, id int64) (JudgeEventOutbox, error)
	MarkOutboxFailed(ctx context.Context, tenantID, id int64, reason string) (JudgeEventOutbox, error)
	CreateFingerprint(ctx context.Context, fp SubmissionFingerprint) (SubmissionFingerprint, error)
	FindExactFingerprints(ctx context.Context, tenantID int64, problemRef, codeHash string) ([]SubmissionFingerprint, error)
	ListFingerprintsForProblem(ctx context.Context, tenantID int64, problemRef, excludeSourceRef string) ([]SubmissionFingerprint, error)
}

type store struct {
	database *db.DB
}

type txStore struct {
	q *sqlcgen.Queries
}

// NewStore 创建 judge 模块持久化入口,仅装配层应调用。
func NewStore(database *db.DB) Store {
	return &store{database: database}
}

// IsNoRows 判断底层查询是否未命中,由 repo 边界封装数据库错误细节。
func (s *store) IsNoRows(err error) bool {
	return db.IsNoRows(err)
}

// PlatformTx 在应用连接中访问 judger 等平台级表。
func (s *store) PlatformTx(ctx context.Context, fn func(context.Context, TxStore) error) error {
	if s == nil || s.database == nil {
		return fmt.Errorf("judge store 未初始化")
	}
	return s.database.WithAppTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return fn(ctx, &txStore{q: sqlcgen.New(tx)})
	})
}

// TenantTx 在注入 RLS 租户变量后访问租户内判题表。
func (s *store) TenantTx(ctx context.Context, tenantID int64, fn func(context.Context, TxStore) error) error {
	if s == nil || s.database == nil {
		return fmt.Errorf("judge store 未初始化")
	}
	return s.database.WithTenantTxID(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		return fn(ctx, &txStore{q: sqlcgen.New(tx)})
	})
}

// PrivilegedTx 用于 worker 跨租户扫描队列和 outbox,不得作为普通业务路径使用。
func (s *store) PrivilegedTx(ctx context.Context, fn func(context.Context, TxStore) error) error {
	if s == nil || s.database == nil {
		return fmt.Errorf("judge store 未初始化")
	}
	return s.database.WithPrivilegedModuleTx(ctx, "judge", func(ctx context.Context, tx pgx.Tx) error {
		return fn(ctx, &txStore{q: sqlcgen.New(tx)})
	})
}

// GetJudgerByCode 按 code 查询判题器。
func (s *txStore) GetJudgerByCode(ctx context.Context, code string) (Judger, error) {
	row, err := s.q.GetJudgerByCode(ctx, code)
	if err != nil {
		return Judger{}, err
	}
	return judgerFromRow(row)
}

// GetJudgerByID 按 ID 查询判题器。
func (s *txStore) GetJudgerByID(ctx context.Context, id int64) (Judger, error) {
	row, err := s.q.GetJudgerByID(ctx, id)
	if err != nil {
		return Judger{}, err
	}
	return judgerFromRow(row)
}

// ListJudgers 查询判题器列表。
func (s *txStore) ListJudgers(ctx context.Context) ([]Judger, error) {
	rows, err := s.q.ListJudgers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Judger, 0, len(rows))
	for _, row := range rows {
		item, err := judgerFromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// UpsertJudger 新建或更新判题器定义。
func (s *txStore) UpsertJudger(ctx context.Context, id int64, req JudgerRequest, spec JudgerResourceSpec, selftestStatus int16) (Judger, error) {
	raw, err := jsonx.AnyBytes(spec, apperr.ErrJudgerConfigInvalid)
	if err != nil {
		return Judger{}, err
	}
	row, err := s.q.UpsertJudger(ctx, sqlcgen.UpsertJudgerParams{
		ID:                id,
		Code:              req.Code,
		Name:              req.Name,
		Type:              req.Type,
		ExecutorRef:       req.ExecutorRef,
		RuntimeRequired:   req.RuntimeRequired,
		DefaultTimeoutSec: req.DefaultTimeoutSec,
		ResourceSpec:      raw,
		SelftestStatus:    selftestStatus,
		Status:            req.Status,
	})
	if err != nil {
		return Judger{}, err
	}
	return judgerFromRow(row)
}

// UpdateJudgerSelftest 更新判题器自检状态。
func (s *txStore) UpdateJudgerSelftest(ctx context.Context, id int64, selftestStatus, status int16) (Judger, error) {
	row, err := s.q.UpdateJudgerSelftest(ctx, sqlcgen.UpdateJudgerSelftestParams{ID: id, SelftestStatus: selftestStatus, Status: status})
	if err != nil {
		return Judger{}, err
	}
	return judgerFromRow(row)
}

// CreateJudgeTask 创建判题任务。
func (s *txStore) CreateJudgeTask(ctx context.Context, task JudgeTask) (JudgeTask, error) {
	raw, err := jsonx.AnyBytes(task.InputSnapshot, apperr.ErrJudgeSubmitInvalid)
	if err != nil {
		return JudgeTask{}, err
	}
	row, err := s.q.CreateJudgeTask(ctx, sqlcgen.CreateJudgeTaskParams{
		ID:               task.ID,
		TenantID:         task.TenantID,
		JudgerID:         task.JudgerID,
		SourceRef:        task.SourceRef,
		SourceOwnerID:    task.SourceOwnerID,
		SourceCourseID:   task.SourceCourseID,
		SourceScope:      task.SourceScope,
		SubmitterID:      task.SubmitterID,
		ProblemRef:       task.ProblemRef,
		CodeStorageKey:   task.CodeStorageKey,
		CodeHash:         task.CodeHash,
		InputSnapshot:    raw,
		SandboxMode:      task.SandboxMode,
		TargetSandboxRef: pgtypex.Text(task.TargetSandboxRef),
		Priority:         task.Priority,
		Status:           task.Status,
		MaxRetries:       task.MaxRetries,
	})
	if err != nil {
		return JudgeTask{}, err
	}
	return taskFromCreateRow(row)
}

// GetJudgeTask 查询任务。
func (s *txStore) GetJudgeTask(ctx context.Context, tenantID, taskID int64) (JudgeTask, error) {
	row, err := s.q.GetJudgeTask(ctx, sqlcgen.GetJudgeTaskParams{TenantID: tenantID, ID: taskID})
	if err != nil {
		return JudgeTask{}, err
	}
	return taskFromRow(row)
}

// GetJudgeTaskBySourceRef 按来源和题目读取已存在任务,允许实验实例下多个检查点各自幂等。
func (s *txStore) GetJudgeTaskBySourceRef(ctx context.Context, tenantID int64, sourceRef, problemRef string) (JudgeTask, error) {
	row, err := s.q.GetJudgeTaskBySourceRef(ctx, sqlcgen.GetJudgeTaskBySourceRefParams{TenantID: tenantID, SourceRef: sourceRef, ProblemRef: problemRef})
	if err != nil {
		return JudgeTask{}, err
	}
	return taskFromRow(row)
}

// GetJudgeTaskInfo 查询任务及其结果。
func (s *txStore) GetJudgeTaskInfo(ctx context.Context, tenantID, taskID int64) (JudgeTaskInfo, error) {
	row, err := s.q.GetJudgeTaskWithResult(ctx, sqlcgen.GetJudgeTaskWithResultParams{TenantID: tenantID, ID: taskID})
	if err != nil {
		return JudgeTaskInfo{}, err
	}
	return taskInfoFromJoined(row)
}

// ListJudgeTasksBySourceRef 查询来源下所有任务。
func (s *txStore) ListJudgeTasksBySourceRef(ctx context.Context, tenantID int64, sourceRef string) ([]JudgeTask, error) {
	rows, err := s.q.ListJudgeTasksBySourceRef(ctx, sqlcgen.ListJudgeTasksBySourceRefParams{TenantID: tenantID, SourceRef: sourceRef})
	if err != nil {
		return nil, err
	}
	return tasksFromRows(rows)
}

// ListRecentJudgeTasksBySubmitterProblem 查询同账号同题限频窗口内的提交任务。
func (s *txStore) ListRecentJudgeTasksBySubmitterProblem(ctx context.Context, tenantID, submitterID int64, problemRef string, windowSeconds int32) ([]JudgeTask, error) {
	rows, err := s.q.ListRecentJudgeTasksBySubmitterProblem(ctx, sqlcgen.ListRecentJudgeTasksBySubmitterProblemParams{TenantID: tenantID, SubmitterID: submitterID, ProblemRef: problemRef, Column4: windowSeconds})
	if err != nil {
		return nil, err
	}
	return tasksFromRows(rows)
}

// ListJudgeTasks 查询任务分页列表。
func (s *txStore) ListJudgeTasks(ctx context.Context, tenantID int64, sourceRef string, pendingManual bool, sourceOwnerID int64, limit, offset int32) ([]JudgeTaskInfo, int64, error) {
	rows, err := s.q.ListJudgeTasks(ctx, sqlcgen.ListJudgeTasksParams{TenantID: tenantID, Column2: sourceRef, Column3: pendingManual, Column4: sourceOwnerID, Limit: limit, Offset: offset})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountJudgeTasks(ctx, sqlcgen.CountJudgeTasksParams{TenantID: tenantID, Column2: sourceRef, Column3: pendingManual, Column4: sourceOwnerID})
	if err != nil {
		return nil, 0, err
	}
	items, err := taskInfosFromRows(rows)
	return items, total, err
}

// CancelQueuedJudgeTask 取消排队任务。
func (s *txStore) CancelQueuedJudgeTask(ctx context.Context, tenantID, taskID int64) (JudgeTask, error) {
	row, err := s.q.CancelQueuedJudgeTask(ctx, sqlcgen.CancelQueuedJudgeTaskParams{TenantID: tenantID, ID: taskID})
	if err != nil {
		return JudgeTask{}, err
	}
	return taskFromRow(row)
}

// ResetJudgeTaskForRejudge 重置任务进入重判队列。
func (s *txStore) ResetJudgeTaskForRejudge(ctx context.Context, tenantID, taskID int64, snapshot JudgeInputSnapshot) (JudgeTask, error) {
	raw, err := jsonx.AnyBytes(snapshot, apperr.ErrJudgeSubmitInvalid)
	if err != nil {
		return JudgeTask{}, err
	}
	row, err := s.q.ResetJudgeTaskForRejudge(ctx, sqlcgen.ResetJudgeTaskForRejudgeParams{TenantID: tenantID, ID: taskID, InputSnapshot: raw})
	if err != nil {
		return JudgeTask{}, err
	}
	return taskFromRow(row)
}

// DequeueJudgeTasks 跨租户领取队列任务。
func (s *txStore) DequeueJudgeTasks(ctx context.Context, limit int32) ([]JudgeTask, error) {
	rows, err := s.q.DequeueJudgeTasks(ctx, limit)
	if err != nil {
		return nil, err
	}
	return tasksFromRows(rows)
}

// CompleteJudgeTask 标记任务完成。
func (s *txStore) CompleteJudgeTask(ctx context.Context, tenantID, taskID int64) (JudgeTask, error) {
	row, err := s.q.CompleteJudgeTask(ctx, sqlcgen.CompleteJudgeTaskParams{TenantID: tenantID, ID: taskID})
	if err != nil {
		return JudgeTask{}, err
	}
	return taskFromRow(row)
}

// MarkJudgeTaskTimeout 标记任务进入超时中间态。
func (s *txStore) MarkJudgeTaskTimeout(ctx context.Context, tenantID, taskID int64, reason string) (JudgeTask, error) {
	row, err := s.q.MarkJudgeTaskTimeout(ctx, sqlcgen.MarkJudgeTaskTimeoutParams{TenantID: tenantID, ID: taskID, LastError: pgtypex.Text(reason)})
	if err != nil {
		return JudgeTask{}, err
	}
	return taskFromRow(row)
}

// MarkJudgeTaskError 标记任务进入系统错误中间态。
func (s *txStore) MarkJudgeTaskError(ctx context.Context, tenantID, taskID int64, reason string) (JudgeTask, error) {
	row, err := s.q.MarkJudgeTaskError(ctx, sqlcgen.MarkJudgeTaskErrorParams{TenantID: tenantID, ID: taskID, LastError: pgtypex.Text(reason)})
	if err != nil {
		return JudgeTask{}, err
	}
	return taskFromRow(row)
}

// RetryJudgeTask 标记任务重试入队。
func (s *txStore) RetryJudgeTask(ctx context.Context, tenantID, taskID int64, reason string) (JudgeTask, error) {
	row, err := s.q.RetryJudgeTask(ctx, sqlcgen.RetryJudgeTaskParams{TenantID: tenantID, ID: taskID, LastError: pgtypex.Text(reason)})
	if err != nil {
		return JudgeTask{}, err
	}
	return taskFromRow(row)
}

// FailJudgeTask 标记任务失败终态。
func (s *txStore) FailJudgeTask(ctx context.Context, tenantID, taskID int64, reason string) (JudgeTask, error) {
	row, err := s.q.FailJudgeTask(ctx, sqlcgen.FailJudgeTaskParams{TenantID: tenantID, ID: taskID, LastError: pgtypex.Text(reason)})
	if err != nil {
		return JudgeTask{}, err
	}
	return taskFromRow(row)
}

// UpsertJudgeResult 保存判题结果。
func (s *txStore) UpsertJudgeResult(ctx context.Context, result JudgeResult) (JudgeResult, error) {
	raw, err := jsonx.AnyBytes(result.Details, apperr.ErrJudgeTaskPersistFailed)
	if err != nil {
		return JudgeResult{}, err
	}
	row, err := s.q.UpsertJudgeResult(ctx, sqlcgen.UpsertJudgeResultParams{
		ID:              result.ID,
		TaskID:          result.TaskID,
		TenantID:        result.TenantID,
		Passed:          result.Passed,
		Score:           result.Score,
		MaxScore:        result.MaxScore,
		Details:         raw,
		JudgeSandboxRef: result.JudgeSandboxRef,
		IsRejudge:       result.IsRejudge,
	})
	if err != nil {
		return JudgeResult{}, err
	}
	details, err := decodeDetails(row.Details)
	if err != nil {
		return JudgeResult{}, err
	}
	return JudgeResult{ID: row.ID, TaskID: row.TaskID, TenantID: row.TenantID, Version: row.Version, Passed: row.Passed, Score: row.Score, MaxScore: row.MaxScore, Details: details, JudgeSandboxRef: row.JudgeSandboxRef, JudgedAt: timex.FromTimestamptz(row.JudgedAt), IsRejudge: row.IsRejudge}, nil
}

// CreateOutbox 写入终态事件 outbox。
func (s *txStore) CreateOutbox(ctx context.Context, id int64, tenantID, taskID int64, subject string, payload any) (JudgeEventOutbox, error) {
	raw, err := jsonx.AnyBytes(payload, apperr.ErrJudgeEventPublishFailed)
	if err != nil {
		return JudgeEventOutbox{}, err
	}
	var meta struct {
		TenantID int64  `json:"tenant_id"`
		TraceID  string `json:"trace_id"`
	}
	if err := jsonx.DecodeStrict(raw, &meta); err != nil {
		return JudgeEventOutbox{}, apperr.ErrJudgeEventPublishFailed.WithCause(err)
	}
	if meta.TenantID != tenantID || strings.TrimSpace(meta.TraceID) == "" {
		return JudgeEventOutbox{}, apperr.ErrJudgeEventPublishFailed.WithCause(fmt.Errorf("判题终态事件缺少真实 tenant_id 或 trace_id"))
	}
	row, err := s.q.CreateJudgeOutbox(ctx, sqlcgen.CreateJudgeOutboxParams{ID: id, TenantID: tenantID, TaskID: taskID, Subject: subject, Payload: raw})
	if err != nil {
		return JudgeEventOutbox{}, err
	}
	return outboxFromRow(row), nil
}

// ListPendingOutbox 查询待发布事件。
func (s *txStore) ListPendingOutbox(ctx context.Context, limit int32) ([]JudgeEventOutbox, error) {
	rows, err := s.q.ListPendingJudgeOutbox(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]JudgeEventOutbox, 0, len(rows))
	for _, row := range rows {
		out = append(out, outboxFromRow(row))
	}
	return out, nil
}

// MarkOutboxPublished 标记 outbox 发布成功。
func (s *txStore) MarkOutboxPublished(ctx context.Context, tenantID, id int64) (JudgeEventOutbox, error) {
	row, err := s.q.MarkJudgeOutboxPublished(ctx, sqlcgen.MarkJudgeOutboxPublishedParams{TenantID: tenantID, ID: id})
	if err != nil {
		return JudgeEventOutbox{}, err
	}
	return outboxFromRow(row), nil
}

// MarkOutboxFailed 标记 outbox 发布失败。
func (s *txStore) MarkOutboxFailed(ctx context.Context, tenantID, id int64, reason string) (JudgeEventOutbox, error) {
	row, err := s.q.MarkJudgeOutboxFailed(ctx, sqlcgen.MarkJudgeOutboxFailedParams{TenantID: tenantID, ID: id, LastError: pgtypex.Text(reason)})
	if err != nil {
		return JudgeEventOutbox{}, err
	}
	return outboxFromRow(row), nil
}

// CreateFingerprint 保存提交特征。
func (s *txStore) CreateFingerprint(ctx context.Context, fp SubmissionFingerprint) (SubmissionFingerprint, error) {
	raw, err := jsonx.AnyBytes(fp.SimVector, apperr.ErrFingerprintRequestInvalid)
	if err != nil {
		return SubmissionFingerprint{}, err
	}
	row, err := s.q.CreateSubmissionFingerprint(ctx, sqlcgen.CreateSubmissionFingerprintParams{ID: fp.ID, TenantID: fp.TenantID, SourceRef: fp.SourceRef, ProblemRef: fp.ProblemRef, SubmitterID: fp.SubmitterID, CodeHash: fp.CodeHash, SimVector: raw})
	if err != nil {
		return SubmissionFingerprint{}, err
	}
	return fingerprintFromRow(row)
}

// FindExactFingerprints 查询完全相同提交。
func (s *txStore) FindExactFingerprints(ctx context.Context, tenantID int64, problemRef, codeHash string) ([]SubmissionFingerprint, error) {
	rows, err := s.q.FindExactFingerprints(ctx, sqlcgen.FindExactFingerprintsParams{TenantID: tenantID, ProblemRef: problemRef, CodeHash: codeHash})
	if err != nil {
		return nil, err
	}
	return fingerprintsFromRows(rows)
}

// ListFingerprintsForProblem 查询同题指纹用于相似度计算。
func (s *txStore) ListFingerprintsForProblem(ctx context.Context, tenantID int64, problemRef, excludeSourceRef string) ([]SubmissionFingerprint, error) {
	rows, err := s.q.ListFingerprintsForProblem(ctx, sqlcgen.ListFingerprintsForProblemParams{TenantID: tenantID, ProblemRef: problemRef, Column3: excludeSourceRef})
	if err != nil {
		return nil, err
	}
	return fingerprintsFromRows(rows)
}

// tasksFromRows 批量转换任务行。
func tasksFromRows(rows []sqlcgen.JudgeTask) ([]JudgeTask, error) {
	out := make([]JudgeTask, 0, len(rows))
	for _, row := range rows {
		item, err := taskFromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// fingerprintsFromRows 批量转换指纹行。
func fingerprintsFromRows(rows []sqlcgen.SubmissionFingerprint) ([]SubmissionFingerprint, error) {
	out := make([]SubmissionFingerprint, 0, len(rows))
	for _, row := range rows {
		item, err := fingerprintFromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}
