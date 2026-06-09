// M3 服务层:承载判题器管理、判题任务入队、结果回写与查重能力。
package judge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	platformredis "chaimir/internal/platform/redis"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/timex"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
	"chaimir/pkg/snowflake"

	"github.com/redis/go-redis/v9"
)

// Service 是 M3 评测引擎服务,负责控制面状态、队列与判题调度。
type Service struct {
	repo            *repo
	idgen           *snowflake.Node
	redis           *platformredis.Client
	bus             eventbus.Bus
	hub             *ws.Hub
	store           *storage.Storage
	cfg             config.JudgeConfig
	sandbox         contracts.SandboxService
	content         contracts.ContentJudgeService
	auditor         audit.Writer
	identity        contracts.IdentityService
	waitSandboxPoll func(context.Context, int) error
}

// judgeTaskResultView 是 M3 HTTP 输出使用的判题结果视图,不扩散到跨模块 contracts。
type judgeTaskResultView struct {
	TaskID          int64
	Passed          bool
	Score           int32
	MaxScore        int32
	Details         any
	JudgedAt        any
	IsRejudge       bool
	JudgeSandboxRef string
}

// judgeTaskView 是 M3 HTTP 查询结果视图,比跨模块摘要多包含判题结果详情。
type judgeTaskView struct {
	contracts.JudgeTaskInfo
	Result *judgeTaskResultView
}

// NewService 构造 M3 服务。
func NewService(
	database *db.DB,
	idgen *snowflake.Node,
	redisClient *platformredis.Client,
	bus eventbus.Bus,
	hub *ws.Hub,
	store *storage.Storage,
	cfg config.JudgeConfig,
	sandboxSvc contracts.SandboxService,
	contentSvc contracts.ContentJudgeService,
	auditor audit.Writer,
	identity contracts.IdentityService,
) *Service {
	// 服务层只持有平台基础设施与 contracts 接口,避免 M3 直接依赖其他业务模块实现。
	return &Service{
		repo:            newRepo(database),
		idgen:           idgen,
		redis:           redisClient,
		bus:             bus,
		hub:             hub,
		store:           store,
		cfg:             cfg,
		sandbox:         sandboxSvc,
		content:         contentSvc,
		auditor:         auditor,
		identity:        identity,
		waitSandboxPoll: waitJudgeSandboxPoll,
	}
}

// ListJudgers 查询判题器配置列表。
func (s *Service) ListJudgers(ctx context.Context) ([]map[string]any, error) {
	rows, err := s.repo.listJudgers(ctx, 100, 0)
	if err != nil {
		return nil, err
	}
	// 对外输出统一转换为 API DTO map,避免泄露 sqlc 内部类型。
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, judgerToMap(row))
	}
	return out, nil
}

// CreateJudger 注册判题器定义,默认接入中且待自检。
func (s *Service) CreateJudger(ctx context.Context, req CreateJudgerRequest) (map[string]any, error) {
	// 先校验判题器核心字段,避免无效平台配置进入数据库。
	if err := validateJudgerRequest(req.Code, req.Name, req.Type, req.ExecutorRef, req.DefaultTimeoutSec); err != nil {
		return nil, err
	}
	// 资源规格按 JSONB 保存,非人工/相似度类判题器需要满足运行规格约束。
	spec, err := jsonx.ObjectBytes(req.ResourceSpec, apperr.ErrJudgerInvalid)
	if err != nil {
		return nil, err
	}
	if _, err := parseJudgerResourceSpecForType(spec, req.Type); err != nil {
		return nil, err
	}
	status := req.Status
	if status == 0 {
		status = JudgerStatusOnboarding
	}
	row, err := s.repo.createJudger(ctx, req, s.idgen.Generate(), spec, status)
	if err != nil {
		return nil, err
	}
	if err := s.writeAudit(ctx, 0, auditActionJudgerCreate, auditTargetJudger, row.ID, map[string]any{"code": row.Code}); err != nil {
		return nil, err
	}
	return judgerToMap(row), nil
}

// UpdateJudger 更新判题器定义。
func (s *Service) UpdateJudger(ctx context.Context, judgerID int64, req UpdateJudgerRequest) (map[string]any, error) {
	// 更新请求仍然要求完整字段,避免形成半配置判题器。
	if err := validateJudgerRequest("keep", req.Name, req.Type, req.ExecutorRef, req.DefaultTimeoutSec); err != nil {
		return nil, err
	}
	// 资源规格先序列化为 JSONB 字节,序列化失败按请求错误处理。
	spec, err := jsonx.ObjectBytes(req.ResourceSpec, apperr.ErrJudgerInvalid)
	if err != nil {
		return nil, err
	}
	if _, err := parseJudgerResourceSpecForType(spec, req.Type); err != nil {
		return nil, err
	}
	row, err := s.repo.updateJudger(ctx, judgerID, req, spec)
	if err != nil {
		return nil, err
	}
	if err := s.writeAudit(ctx, 0, auditActionJudgerUpdate, auditTargetJudger, row.ID, map[string]any{"id": ids.Format(row.ID)}); err != nil {
		return nil, err
	}
	return judgerToMap(row), nil
}

// RunJudgerSelftest 执行判题器自检并更新接入状态。
func (s *Service) RunJudgerSelftest(ctx context.Context, judgerID int64) (map[string]any, error) {
	// 第一步读取平台级判题器定义,不存在时返回 M3 业务错误码。
	row, err := s.repo.getJudgerByID(ctx, judgerID)
	if err != nil {
		return nil, err
	}
	// 第二步执行真实自检,人工/仿真类只校验配置,运行类会申请 M2 judge 沙箱并执行命令。
	detail, passed := s.executeJudgerSelftest(ctx, row)
	status := JudgerStatusAvailable
	selftestStatus := JudgerSelftestPassed
	if !passed {
		status = JudgerStatusOnboarding
		selftestStatus = JudgerSelftestFailed
	}
	// 第三步把自检结果写回平台配置,后续提交只接受 passed+available 的判题器。
	payload, err := jsonx.ObjectBytes(detail, apperr.ErrJudgerInvalid)
	if err != nil {
		return nil, err
	}
	row, err = s.repo.updateJudgerSelftest(ctx, judgerID, selftestStatus, payload, status)
	if err != nil {
		return nil, err
	}
	if err := s.writeAudit(ctx, 0, auditActionJudgerSelftest, auditTargetJudger, row.ID, map[string]any{"passed": passed}); err != nil {
		return nil, err
	}
	if !passed {
		return judgerToMap(row), apperr.ErrJudgerSelftestFailed
	}
	return judgerToMap(row), nil
}

// executeJudgerSelftest 执行判题器接入检查,返回可保存的结构化详情。
func (s *Service) executeJudgerSelftest(ctx context.Context, row JudgerSnapshot) (map[string]any, bool) {
	// 人工评分、Flag 和仿真检查点不在 M3 内启动判题沙箱,配置能解析即可通过接入检查。
	if row.Type == JudgerTypeManual || row.Type == JudgerTypeFlag || row.Type == JudgerTypeSimCheckpoint {
		return map[string]any{"mode": "config", "passed": true}, true
	}
	if s.sandbox == nil {
		return map[string]any{"mode": "sandbox", "passed": false, "reason": "sandbox_unavailable"}, false
	}
	id, ok := tenantFromContext(ctx)
	if !ok {
		return map[string]any{"mode": "sandbox", "passed": false, "reason": "unauthorized"}, false
	}
	spec, err := parseJudgerResourceSpecForType(row.ResourceSpec, row.Type)
	if err != nil {
		return map[string]any{"mode": "sandbox", "passed": false, "reason": "invalid_resource_spec"}, false
	}
	// 自检使用当前操作者租户创建短生命周期 judge 沙箱,source_ref 保持总规范四段格式。
	sourceRef := fmt.Sprintf("judge:%d:selftest:%d", timex.Now().Year(), row.ID)
	info, err := s.sandbox.CreateSandbox(ctx, contracts.SandboxCreateRequest{
		TenantID: id.TenantID, RuntimeCode: spec.RuntimeCode, RuntimeImageVersion: spec.RuntimeImageVersion,
		ToolCodes: spec.ToolCodes, InitScriptRef: spec.InitScriptRef,
		OwnerAccountID: id.AccountID, SourceRef: sourceRef,
	})
	if err != nil {
		return map[string]any{"mode": "sandbox", "passed": false, "reason": "create_failed"}, false
	}
	defer s.recycleSelftestSandbox(ctx, id.TenantID, sourceRef)
	if err := s.waitSandboxReady(ctx, info.SandboxID, row.DefaultTimeoutSec); err != nil {
		return map[string]any{"mode": "sandbox", "passed": false, "reason": "ready_failed"}, false
	}
	if _, err := s.sandbox.ExecSandboxCommand(ctx, contracts.SandboxExecRequest{
		SandboxID: info.SandboxID, Command: spec.Command, TimeoutSec: row.DefaultTimeoutSec,
	}); err != nil {
		return map[string]any{"mode": "sandbox", "passed": false, "reason": "command_failed"}, false
	}
	return map[string]any{"mode": "sandbox", "passed": true, "sandbox_id": ids.Format(info.SandboxID)}, true
}

// recycleSelftestSandbox 回收自检沙箱;失败不覆盖自检主错误,但必须记录。
func (s *Service) recycleSelftestSandbox(ctx context.Context, tenantID int64, sourceRef string) {
	if err := s.sandbox.RecycleBySourceRef(ctx, tenantID, sourceRef, "judge-selftest"); err != nil {
		logging.ErrorContext(ctx, "judge selftest sandbox recycle failed", err.Error(), slog.String("source_ref", sourceRef))
	}
}

// waitSandboxReady 等待 M2 沙箱进入可执行状态。
func (s *Service) waitSandboxReady(ctx context.Context, sandboxID int64, timeoutSec int32) error {
	deadline := timex.Now().Add(time.Duration(timeoutSec) * time.Second)
	for timex.Now().Before(deadline) {
		info, err := s.sandbox.GetSandbox(ctx, sandboxID)
		if err != nil {
			return err
		}
		if info.Phase == contracts.SandboxPhaseReady && info.Status == contracts.SandboxStatusRunning {
			return nil
		}
		if info.Status == contracts.SandboxStatusError {
			return apperr.ErrSandboxCreateFail
		}
		if err := s.waitForSandboxPoll(ctx); err != nil {
			return err
		}
	}
	return apperr.ErrSandboxTimeout
}

// waitForSandboxPoll 在自检轮询之间等待,测试可注入等待函数避免真实 sleep。
func (s *Service) waitForSandboxPoll(ctx context.Context) error {
	if s.waitSandboxPoll != nil {
		return s.waitSandboxPoll(ctx, s.cfg.SandboxReadyPollIntervalMs)
	}
	return waitJudgeSandboxPoll(ctx, s.cfg.SandboxReadyPollIntervalMs)
}

// waitJudgeSandboxPoll 使用 context-aware timer 控制沙箱就绪轮询间隔。
func waitJudgeSandboxPoll(ctx context.Context, intervalMs int) error {
	if intervalMs <= 0 {
		return nil
	}
	timer := time.NewTimer(time.Duration(intervalMs) * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// SubmitJudgeTask 创建判题任务、记录指纹并入队。
func (s *Service) SubmitJudgeTask(ctx context.Context, req contracts.JudgeSubmitRequest) (contracts.JudgeTaskInfo, error) {
	// 第一步校验提交参数,保证题目版本、代码引用和沙箱模式都可追溯。
	if err := validateSubmitRequest(req); err != nil {
		return contracts.JudgeTaskInfo{}, err
	}
	if existing, err := s.existingTaskBySourceRef(ctx, req.TenantID, req.SourceRef); err == nil {
		return taskInfoFromTask(existing), nil
	} else if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrJudgeTaskNotFound.Code {
		return contracts.JudgeTaskInfo{}, err
	}
	// 第二步执行提交限频,防止同账号同题反复占用评测资源。
	if err := s.checkSubmitRate(ctx, req); err != nil {
		return contracts.JudgeTaskInfo{}, err
	}
	// 第三步加载可用判题器,只允许已自检通过的配置承接任务。
	judger, err := s.loadAvailableJudger(ctx, req.JudgerCode)
	if err != nil {
		return contracts.JudgeTaskInfo{}, err
	}
	if judger.Type != JudgerTypeManual && s.content == nil {
		return contracts.JudgeTaskInfo{}, apperr.ErrJudgeConfigUnavailable
	}
	// 第四步读取判题器级重试策略,任务创建后按该策略推进状态机。
	maxRetries, err := maxRetriesForJudger(judger.ResourceSpec, judger.Type, s.cfg.DefaultMaxRetries)
	if err != nil {
		return contracts.JudgeTaskInfo{}, err
	}
	// 第五步构建输入快照,把 M5 判题规格固化为可复现数据。
	snapshot, problemRef, err := s.buildInputSnapshot(ctx, req, judger)
	if err != nil {
		return contracts.JudgeTaskInfo{}, err
	}
	// 第六步从提交对象提取相似度向量,禁止写入空的假指纹。
	simVector, err := s.buildSubmissionVector(ctx, req.CodeStorageKey)
	if err != nil {
		return contracts.JudgeTaskInfo{}, err
	}
	taskID := s.idgen.Generate()
	taskStatus := JudgeTaskQueued
	if judger.Type == JudgerTypeManual {
		taskStatus = JudgeTaskJudging
	}
	// 第七步在租户事务内创建任务和查重指纹,保证 RLS 与数据归属一致。
	row, err := s.repo.createTaskWithFingerprint(ctx, JudgeTaskCreate{
		TaskID:           taskID,
		FingerprintID:    s.idgen.Generate(),
		TenantID:         req.TenantID,
		JudgerID:         judger.ID,
		SourceRef:        req.SourceRef,
		SubmitterID:      req.SubmitterID,
		ProblemRef:       problemRef,
		CodeStorageKey:   req.CodeStorageKey,
		CodeHash:         req.CodeHash,
		InputSnapshot:    snapshot,
		SandboxMode:      normalizedSandboxMode(req.SandboxMode),
		TargetSandboxRef: req.TargetSandboxRef,
		Priority:         normalizePriority(req.Priority),
		Status:           taskStatus,
		MaxRetries:       maxRetries,
		SimVector:        simVector,
	})
	if err != nil {
		return contracts.JudgeTaskInfo{}, err
	}
	if err := s.writeAudit(ctx, req.TenantID, auditActionTaskSubmit, auditTargetTask, row.ID, map[string]any{
		"source_ref":  row.SourceRef,
		"problem_ref": row.ProblemRef,
		"judger_code": judger.Code,
	}); err != nil {
		if markErr := s.markTaskErrorAfterSubmitAuditFailure(ctx, row); markErr != nil {
			return contracts.JudgeTaskInfo{}, errors.Join(err, markErr)
		}
		return contracts.JudgeTaskInfo{}, err
	}
	if judger.Type == JudgerTypeManual {
		s.publishProgress(row.ID, JudgeTaskJudging, "等待人工评分")
		return taskInfoFromTask(row), nil
	}
	// 第八步提交 Redis 队列;入队失败返回明确错误码,不静默吞掉中间态。
	if err := s.enqueueTask(ctx, row); err != nil {
		return contracts.JudgeTaskInfo{}, apperr.ErrJudgeTaskQueuedFail.WithCause(err)
	}
	return taskInfoFromTask(row), nil
}

// existingTaskBySourceRef 按上游资源引用查询已创建任务,支撑调用方 outbox 幂等重试。
func (s *Service) existingTaskBySourceRef(ctx context.Context, tenantID int64, sourceRef string) (JudgeTaskSnapshot, error) {
	return s.repo.getTaskBySourceRef(ctx, tenantID, sourceRef)
}

// markTaskErrorAfterSubmitAuditFailure 防止审计失败的任务继续进入判题队列。
func (s *Service) markTaskErrorAfterSubmitAuditFailure(ctx context.Context, row JudgeTaskSnapshot) error {
	if err := s.repo.markTaskStatus(ctx, row.TenantID, row.ID, JudgeTaskError); err != nil {
		return apperr.ErrJudgeTaskPersistence.WithCause(err)
	}
	return nil
}

// GetJudgeTask 查询判题任务摘要。
func (s *Service) GetJudgeTask(ctx context.Context, taskID int64) (contracts.JudgeTaskInfo, error) {
	view, err := s.getJudgeTaskView(ctx, taskID)
	if err != nil {
		return contracts.JudgeTaskInfo{}, err
	}
	return view.JudgeTaskInfo, nil
}

// getJudgeTaskView 查询 M3 HTTP 需要的任务摘要与结果详情。
func (s *Service) getJudgeTaskView(ctx context.Context, taskID int64) (judgeTaskView, error) {
	// 查询必须依赖鉴权中间件注入的租户身份,禁止从请求参数决定租户。
	id, ok := tenantFromContext(ctx)
	if !ok {
		return judgeTaskView{}, apperr.ErrUnauthorized
	}
	// 任务与结果在同一租户事务内读取,避免跨租户数据泄露。
	return s.repo.getTaskView(ctx, id.TenantID, taskID)
}

// Rejudge 按原输入快照重新入队。
func (s *Service) Rejudge(ctx context.Context, taskID int64) (contracts.JudgeTaskInfo, error) {
	// 重判沿用当前用户的租户上下文,不接受调用方传入租户 ID。
	id, ok := tenantFromContext(ctx)
	if !ok {
		return contracts.JudgeTaskInfo{}, apperr.ErrUnauthorized
	}
	// 先把任务恢复为 queued,未命中时返回 M3 任务不存在错误。
	row, err := s.repo.markTaskRejudge(ctx, id.TenantID, taskID)
	if err != nil {
		return contracts.JudgeTaskInfo{}, err
	}
	if err := s.writeAudit(ctx, id.TenantID, auditActionTaskRejudge, auditTargetTask, row.ID, map[string]any{"source_ref": row.SourceRef}); err != nil {
		return contracts.JudgeTaskInfo{}, err
	}
	// 审计成功后再重新写入队列,避免审计失败时后台 worker 已开始执行重判。
	if err := s.enqueueTask(ctx, row); err != nil {
		return contracts.JudgeTaskInfo{}, apperr.ErrJudgeTaskQueuedFail.WithCause(err)
	}
	return taskInfoFromTask(row), nil
}

// CancelTask 取消仍处于 queued 的判题任务。
func (s *Service) CancelTask(ctx context.Context, taskID int64) (contracts.JudgeTaskInfo, error) {
	// 第一步从上下文获取租户身份,取消操作只能作用于本租户任务。
	id, ok := tenantFromContext(ctx)
	if !ok {
		return contracts.JudgeTaskInfo{}, apperr.ErrUnauthorized
	}
	// 第二步只取消 queued 状态任务,已经执行的任务必须由 worker 进入终态。
	row, err := s.repo.cancelQueuedTask(ctx, id.TenantID, taskID)
	if err != nil {
		return contracts.JudgeTaskInfo{}, err
	}
	if err := s.writeAudit(ctx, id.TenantID, auditActionTaskCancel, auditTargetTask, row.ID, map[string]any{"source_ref": row.SourceRef}); err != nil {
		return contracts.JudgeTaskInfo{}, err
	}
	return taskInfoFromTask(row), nil
}

// RejudgeBatch 按来源标识批量重判。
func (s *Service) RejudgeBatch(ctx context.Context, sourceRef string) ([]map[string]any, error) {
	// 第一步校验来源标识,批量操作只能绑定一个明确上游资源。
	if !auth.ValidSourceRef(sourceRef) {
		return nil, apperr.ErrJudgeTaskInvalid
	}
	id, ok := tenantFromContext(ctx)
	if !ok {
		return nil, apperr.ErrUnauthorized
	}
	// 第二步读取同来源历史任务,避免跨来源误触发重判。
	rows, err := s.repo.listTasksBySourceRef(ctx, id.TenantID, sourceRef, 500, 0)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	// 第三步逐条恢复 queued 并入队,任何一条失败都显式返回,避免部分失败被吞掉。
	for _, row := range rows {
		info, err := s.Rejudge(ctx, row.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, taskInfoToMap(info))
	}
	return out, nil
}

// ListTasks 查询任务列表,支持待人工评分和来源过滤。
func (s *Service) ListTasks(ctx context.Context, sourceRef string, pendingManual bool) ([]map[string]any, error) {
	id, ok := tenantFromContext(ctx)
	if !ok {
		return nil, apperr.ErrUnauthorized
	}
	// 待人工评分列表只返回 J6 且处于 judging 的任务,供教师录入结果。
	if pendingManual {
		if !auth.ValidSourceRef(sourceRef) {
			return nil, apperr.ErrJudgeTaskInvalid
		}
		rows, err := s.repo.listManualPendingTasks(ctx, id.TenantID, sourceRef, 100, 0)
		if err != nil {
			return nil, err
		}
		return tasksToMaps(rows), nil
	}
	// 普通列表必须带来源标识,避免开放无边界全租户扫描。
	if !auth.ValidSourceRef(sourceRef) {
		return nil, apperr.ErrJudgeTaskInvalid
	}
	rows, err := s.repo.listTasksBySourceRef(ctx, id.TenantID, sourceRef, 100, 0)
	if err != nil {
		return nil, err
	}
	return tasksToMaps(rows), nil
}

// ManualScore 写入人工评分结果。
func (s *Service) ManualScore(ctx context.Context, taskID int64, req ManualScoreRequest) (contracts.JudgeTaskInfo, error) {
	// 第一步校验分值边界,避免写入负分或超过满分的结果。
	if req.MaxScore <= 0 || req.Score < 0 || req.Score > req.MaxScore {
		return contracts.JudgeTaskInfo{}, apperr.ErrJudgeManualScoreInvalid
	}
	// 第二步确认终态事件通道可用,避免人工评分写入 outbox 后无法派发给上层聚合。
	if err := s.requireEventBus(); err != nil {
		return contracts.JudgeTaskInfo{}, err
	}
	// 第三步从鉴权上下文取得租户身份,不信任请求参数提供租户。
	id, ok := tenantFromContext(ctx)
	if !ok {
		return contracts.JudgeTaskInfo{}, apperr.ErrUnauthorized
	}
	// 第四步把人工评分依据结构化保存,便于申诉与审计复核。
	detail, err := jsonx.ObjectBytes(map[string]any{
		"comment":   req.Comment,
		"score":     req.Score,
		"max_score": req.MaxScore,
		"passed":    req.Score >= req.MaxScore,
	}, apperr.ErrJudgeManualScoreInvalid)
	if err != nil {
		return contracts.JudgeTaskInfo{}, err
	}
	// 第五步先读取任务和判题器类型,确认 J6 后再写审计与终态结果。
	task, judgerType, err := s.repo.getTaskAndJudgerType(ctx, id.TenantID, taskID)
	if err != nil {
		return contracts.JudgeTaskInfo{}, err
	}
	if judgerType != JudgerTypeManual {
		return contracts.JudgeTaskInfo{}, apperr.ErrJudgeTaskInvalidState
	}
	if err := s.writeAudit(ctx, id.TenantID, auditActionManualScore, auditTargetResult, taskID, map[string]any{"score": req.Score, "max_score": req.MaxScore}); err != nil {
		return contracts.JudgeTaskInfo{}, err
	}
	// 第六步在同一租户事务内写入人工结果、推进状态并创建终态事件 outbox。
	outbox, err := s.newJudgeEventOutbox(contracts.SubjectJudgeCompleted, contracts.JudgeCompletedEvent{
		TenantID: id.TenantID, TaskID: task.ID, SourceRef: task.SourceRef, Status: JudgeTaskDone, Score: int(req.Score),
	})
	if err != nil {
		return contracts.JudgeTaskInfo{}, err
	}
	if err := s.repo.completeTaskResult(ctx, JudgeResultCreate{
		TaskID:    taskID,
		TenantID:  id.TenantID,
		Passed:    req.Score >= req.MaxScore,
		Score:     req.Score,
		MaxScore:  req.MaxScore,
		Details:   detail,
		IsRejudge: false,
	}, outbox); err != nil {
		return contracts.JudgeTaskInfo{}, apperr.ErrJudgeTaskPersistence.WithCause(err)
	}
	if err := s.PublishPendingJudgeEvents(ctx); err != nil {
		logging.ErrorContext(ctx, "judge manual score event dispatch deferred", err.Error(), slog.Int64("task_id", task.ID))
	}
	// 第七步以任务摘要为基础补齐人工评分字段后返回。
	info := taskInfoFromTask(task)
	info.Status = JudgeTaskDone
	info.Score = req.Score
	info.Passed = req.Score >= req.MaxScore
	return info, nil
}

// ExactFingerprints 查询完全相同代码哈希的提交指纹。
func (s *Service) ExactFingerprints(ctx context.Context, problemRef, codeHash string) ([]map[string]any, error) {
	// 查重条件必须同时包含题目引用和代码哈希,避免跨题误判。
	if strings.TrimSpace(problemRef) == "" || strings.TrimSpace(codeHash) == "" {
		return nil, apperr.ErrFingerprintInvalid
	}
	// 指纹是租户数据,查询通过租户事务受 RLS 约束。
	rows, err := s.repo.listExactFingerprints(ctx, problemRef, codeHash, 100, 0)
	if err != nil {
		return nil, err
	}
	// 输出前转换 ID 与时间字段,保持 API 表达稳定。
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, fingerprintToMap(row))
	}
	return out, nil
}

// Similarity 计算提交向量与同题历史指纹的相似度。
func (s *Service) Similarity(ctx context.Context, req FingerprintSimilarityRequest) ([]map[string]any, error) {
	// 第一步校验题目引用和提交对象,两者缺一无法限定比较范围。
	if strings.TrimSpace(req.ProblemRef) == "" || strings.TrimSpace(req.CodeStorageKey) == "" {
		return nil, apperr.ErrFingerprintInvalid
	}
	// 第二步从对象存储读取待比较提交并生成特征向量,禁止调用方伪造向量。
	rawVector, err := s.buildSubmissionVector(ctx, req.CodeStorageKey)
	if err != nil {
		return nil, apperr.ErrSimilarityFailed.WithCause(err)
	}
	queryVector := decodeVector(rawVector)
	if len(queryVector) == 0 {
		return nil, apperr.ErrFingerprintInvalid
	}
	// 第三步设置阈值,未显式指定时使用文档默认相似度阈值。
	threshold := req.Threshold
	if threshold <= 0 {
		threshold = s.cfg.SimilarityDefaultThreshold
	}
	if threshold <= 0 || threshold >= 1 {
		return nil, apperr.ErrFingerprintInvalid
	}
	// 第四步只读取同题历史指纹,避免跨题向量参与比较。
	rows, err := s.repo.listFingerprintsByProblem(ctx, req.ProblemRef, 500, 0)
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	// 第五步逐条解码并计算余弦相似度,只返回达到阈值的命中项。
	for _, row := range rows {
		vector := decodeVector(row.SimVector)
		score := cosineSimilarity(queryVector, vector)
		if score >= threshold {
			out = append(out, map[string]any{
				"submitter_id": ids.Format(row.SubmitterID),
				"source_ref":   row.SourceRef,
				"similarity":   score,
			})
		}
	}
	return out, nil
}

// loadAvailableJudger 读取并校验判题器可用状态。
func (s *Service) loadAvailableJudger(ctx context.Context, code string) (JudgerSnapshot, error) {
	// 判题器按平台配置读取,编码不存在时返回 M3 专用错误码。
	row, err := s.repo.getJudgerByCode(ctx, code)
	if err != nil {
		return JudgerSnapshot{}, err
	}
	// 仅自检通过且处于 available 的判题器可以承接任务。
	if row.Status != JudgerStatusAvailable || row.SelftestStatus != JudgerSelftestPassed {
		return JudgerSnapshot{}, apperr.ErrJudgerUnavailable
	}
	return row, nil
}

// buildInputSnapshot 组装可复现输入快照,题目判题配置只经 M5 contracts 获取。
func (s *Service) buildInputSnapshot(ctx context.Context, req contracts.JudgeSubmitRequest, judger JudgerSnapshot) ([]byte, string, error) {
	// 第一步固定提交侧元数据,后续 worker 只依赖快照复现判题输入。
	problemRef := req.ItemCode + ":" + req.ItemVersion
	snapshot := map[string]any{
		"judger_code":      judger.Code,
		"judger_version":   judger.UpdatedAtText,
		"executor_ref":     judger.ExecutorRef,
		"code_hash":        req.CodeHash,
		"code_storage_key": req.CodeStorageKey,
		"problem_ref":      problemRef,
		"extra_input":      req.ExtraInput,
	}
	if judger.Type != JudgerTypeManual && judger.Type != JudgerTypeFlag && judger.Type != JudgerTypeSimCheckpoint {
		spec, err := parseJudgerResourceSpecForType(judger.ResourceSpec, judger.Type)
		if err != nil {
			return nil, "", err
		}
		snapshot["sandbox_image_version"] = spec.RuntimeImageVersion
		snapshot["genesis_ref"] = spec.GenesisRef
	}
	// 第二步非人工判题必须经 M5 contracts 获取测试套件与期望配置。
	if judger.Type != JudgerTypeManual {
		spec, err := s.content.GetJudgeSpec(ctx, req.ItemCode, req.ItemVersion)
		if err != nil {
			return nil, "", apperr.ErrJudgeConfigUnavailable.WithCause(err)
		}
		expectation, err := snapshotExpectationForJudger(req, problemRef, judger.Type, spec.Expectation)
		if err != nil {
			return nil, "", err
		}
		snapshot["suite_ref"] = spec.SuiteRef
		snapshot["judge_spec_hash"] = spec.VersionHash
		snapshot["expectation"] = expectation
		snapshot["max_score"] = spec.MaxScore
	}
	// 第三步把快照序列化为 JSONB,作为重判和审计复现依据。
	data, err := jsonx.ObjectBytes(snapshot, apperr.ErrJudgeTaskInvalid)
	if err != nil {
		return nil, "", err
	}
	return data, problemRef, nil
}

// buildSubmissionVector 读取提交代码对象并生成查重相似度向量。
func (s *Service) buildSubmissionVector(ctx context.Context, codeStorageKey string) (payload []byte, err error) {
	// 第一步确认对象存储能力已注入,避免写入空指纹破坏查重结果。
	if s.store == nil {
		return nil, apperr.ErrJudgeConfigUnavailable
	}
	ref, err := storage.ParseObjectRef(codeStorageKey)
	if err != nil {
		return nil, apperr.ErrJudgeTaskInvalid.WithCause(err)
	}
	// 第二步读取提交对象,读取失败说明提交不可复现,判题任务不能落库。
	reader, err := s.store.Get(ctx, ref.Bucket, ref.Key)
	if err != nil {
		return nil, apperr.ErrJudgeTaskQueuedFail.WithCause(err)
	}
	defer func() {
		if closeErr := reader.Close(); closeErr != nil {
			err = errors.Join(err, apperr.ErrJudgeTaskQueuedFail.WithCause(closeErr))
		}
	}()
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, apperr.ErrJudgeTaskQueuedFail.WithCause(err)
	}
	// 第三步提取 token 向量并保存为 JSONB,供后续相似度查询复用。
	vector, err := fingerprintVectorFromArchive(data, judgeArchiveLimits(s.cfg))
	if err != nil {
		return nil, err
	}
	payload, err = jsonx.AnyBytes(vector, apperr.ErrFingerprintInvalid)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

// enqueueTask 把任务 ID 写入 Redis 有序集合;score 越小越先消费。
func (s *Service) enqueueTask(ctx context.Context, row JudgeTaskSnapshot) error {
	if s.redis == nil {
		return apperr.ErrJudgeTaskQueuedFail
	}
	// 分数由优先级和创建时间组成,保证高优先级任务先出队且同级按时间排序。
	score := float64(100-int(row.Priority))*1_000_000_000 + float64(row.CreatedAtUnixMs)
	return s.redis.Raw().ZAdd(ctx, "judge:queue", redisZ(row.ID, score)).Err()
}

// checkSubmitRate 使用 Redis 限频,防止同账号同题刷判题资源。
func (s *Service) checkSubmitRate(ctx context.Context, req contracts.JudgeSubmitRequest) error {
	// 未启用限频窗口时不占用 Redis;窗口启用但 Redis 缺失必须失败,不能绕过防滥用边界。
	if s.cfg.SubmitRateLimitSec <= 0 {
		return nil
	}
	if s.redis == nil {
		return apperr.ErrJudgeTaskQueuedFail
	}
	key := fmt.Sprintf("judge:rate:%d:%d:%s:%s", req.TenantID, req.SubmitterID, req.ItemCode, req.ItemVersion)
	// SetNX 成功表示窗口内首次提交,失败则返回明确限频错误码。
	ok, err := s.redis.SetNX(ctx, key, time.Duration(s.cfg.SubmitRateLimitSec)*time.Second)
	if err != nil {
		return apperr.ErrJudgeTaskQueuedFail.WithCause(err)
	}
	if !ok {
		return apperr.ErrJudgeTaskRateLimited
	}
	return nil
}

// validateJudgerRequest 校验判题器管理请求。
func validateJudgerRequest(code, name string, typ int16, executorRef string, timeoutSec int32) error {
	// 判题器核心字段必须完整,类型必须落在文档定义的枚举范围内。
	if strings.TrimSpace(code) == "" || strings.TrimSpace(name) == "" ||
		strings.TrimSpace(executorRef) == "" || timeoutSec <= 0 ||
		typ < JudgerTypeTestcase || typ > JudgerTypeManual {
		return apperr.ErrJudgerInvalid
	}
	return nil
}

// decodeVector 把数据库中的 JSON 向量解码为浮点特征表;异常数据按空向量处理。
func decodeVector(raw []byte) map[string]float64 {
	return jsonx.Decode(raw, map[string]float64{})
}

// normalizePriority 规范化任务优先级,未传值时使用普通优先级。
func normalizePriority(priority int16) int16 {
	if priority <= 0 {
		return 2
	}
	return priority
}

// redisZ 构造 Redis 有序集合成员。
func redisZ(taskID int64, score float64) redis.Z {
	return redis.Z{Score: score, Member: ids.Format(taskID)}
}
