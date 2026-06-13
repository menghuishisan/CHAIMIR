// judge service 文件定义服务依赖注入和通用业务编排,不接收数据库连接。
package judge

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/upload"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
	"chaimir/pkg/snowflake"
)

// objectStorage 描述 M3 读取提交代码和判题套件所需的对象存储能力。
type objectStorage interface {
	Get(ctx context.Context, bucket, key string) (io.ReadCloser, error)
	BucketCode() string
}

// Service 承载 judge 模块业务编排,依赖 repo 接口和平台横切能力。
type Service struct {
	store   Store
	ids     snowflake.Generator
	cfg     config.JudgeConfig
	hmacKey []byte
	minio   objectStorage
	sandbox contracts.SandboxService
	content contracts.ContentJudgeReadService
	audit   audit.Writer
	bus     eventbus.Bus
	wsHub   *ws.Hub
}

// ServiceDeps 是 judge service 的装配依赖集合。
type ServiceDeps struct {
	Store    Store
	IDs      snowflake.Generator
	Config   config.JudgeConfig
	Auth     config.AuthConfig
	Storage  *storage.Storage
	Sandbox  contracts.SandboxService
	Content  contracts.ContentJudgeReadService
	Audit    audit.Writer
	EventBus eventbus.Bus
	WSHub    *ws.Hub
}

// NewService 构造 judge 服务,不接收数据库连接,由装配层传入 Store。
func NewService(deps ServiceDeps) (*Service, error) {
	if deps.Store == nil {
		return nil, fmt.Errorf("judge service 缺少 store")
	}
	if deps.IDs == nil {
		return nil, fmt.Errorf("judge service 缺少 ID 生成器")
	}
	if deps.Storage == nil {
		return nil, fmt.Errorf("judge service 缺少统一对象存储")
	}
	if strings.TrimSpace(deps.Auth.HMACKey) == "" {
		return nil, fmt.Errorf("judge service 缺少 HMAC 密钥")
	}
	if deps.Sandbox == nil {
		return nil, fmt.Errorf("judge service 缺少 sandbox 契约")
	}
	if deps.Content == nil {
		return nil, fmt.Errorf("judge service 缺少 content 判题配置契约")
	}
	if deps.Audit == nil {
		return nil, fmt.Errorf("judge service 缺少审计写入器")
	}
	if deps.EventBus == nil {
		return nil, fmt.Errorf("judge service 缺少事件总线")
	}
	return &Service{
		store:   deps.Store,
		ids:     deps.IDs,
		cfg:     deps.Config,
		hmacKey: []byte(deps.Auth.HMACKey),
		minio:   deps.Storage,
		sandbox: deps.Sandbox,
		content: deps.Content,
		audit:   deps.Audit,
		bus:     deps.EventBus,
		wsHub:   deps.WSHub,
	}, nil
}

// ListJudgers 返回平台级判题器列表。
func (s *Service) ListJudgers(ctx context.Context) ([]map[string]any, error) {
	var items []Judger
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		items, err = tx.ListJudgers(ctx)
		return err
	}); err != nil {
		return nil, apperr.ErrJudgerNotFound.WithCause(err)
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		mapped, err := judgerToMap(item)
		if err != nil {
			return nil, err
		}
		out = append(out, mapped)
	}
	return out, nil
}

// CreateJudger 注册或更新判题器定义。
func (s *Service) CreateJudger(ctx context.Context, req CreateJudgerRequest) (map[string]any, error) {
	spec, err := validateJudgerRequest(req)
	if err != nil {
		return nil, err
	}
	var out Judger
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.UpsertJudger(ctx, s.ids.Generate(), normalizeJudgerRequest(req), spec, JudgerSelftestPending)
		if err != nil {
			return apperr.ErrJudgerPersistFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return judgerToMap(out)
}

// UpdateJudger 更新判题器配置。
func (s *Service) UpdateJudger(ctx context.Context, id int64, req UpdateJudgerRequest) (map[string]any, error) {
	if id <= 0 {
		return nil, apperr.ErrPathIDInvalid
	}
	spec, err := validateJudgerRequest(req)
	if err != nil {
		return nil, err
	}
	var out Judger
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		existing, err := tx.GetJudgerByID(ctx, id)
		if err != nil {
			return apperr.ErrJudgerNotFound.WithCause(err)
		}
		normalized := normalizeJudgerRequest(req)
		normalized.Code = existing.Code
		out, err = tx.UpsertJudger(ctx, existing.ID, normalized, spec, JudgerSelftestPending)
		if err != nil {
			return apperr.ErrJudgerPersistFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return judgerToMap(out)
}

// RunJudgerSelftest 执行判题器样例自检,自检必须真实经过对应执行路径。
func (s *Service) RunJudgerSelftest(ctx context.Context, id int64) (map[string]any, error) {
	var j Judger
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		j, err = tx.GetJudgerByID(ctx, id)
		if err != nil {
			return apperr.ErrJudgerNotFound.WithCause(err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	status := JudgerSelftestPassed
	judgerStatus := JudgerStatusAvailable
	if err := s.executeJudgerSelftest(ctx, j); err != nil {
		status = JudgerSelftestFailed
		judgerStatus = JudgerStatusDisabled
	}
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		j, err = tx.UpdateJudgerSelftest(ctx, id, status, judgerStatus)
		if err != nil {
			return apperr.ErrJudgerPersistFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if status == JudgerSelftestFailed {
		out, err := judgerToMap(j)
		if err != nil {
			return nil, err
		}
		return out, apperr.ErrJudgerSelftestFailed
	}
	return judgerToMap(j)
}

// SubmitJudgeTask 创建判题任务、输入快照和提交指纹。
func (s *Service) SubmitJudgeTask(ctx context.Context, req contracts.JudgeSubmitRequest) (contracts.JudgeTaskInfo, error) {
	if err := validateSubmitRequest(req); err != nil {
		return contracts.JudgeTaskInfo{}, err
	}
	j, err := s.loadAvailableJudger(ctx, req.JudgerCode)
	if err != nil {
		return contracts.JudgeTaskInfo{}, err
	}
	mode, _ := normalizedSandboxMode(req.SandboxMode)
	if j.Type == JudgerTypeManual {
		mode = JudgeSandboxModeFresh
	}
	spec, err := s.content.GetJudgeSpec(ctx, req.TenantID, req.ItemCode, req.ItemVersion)
	if err != nil {
		return contracts.JudgeTaskInfo{}, apperr.ErrJudgeSpecUnavailable.WithCause(err)
	}
	snapshot, err := s.buildInputSnapshot(j, spec, req.ExtraInput)
	if err != nil {
		return contracts.JudgeTaskInfo{}, err
	}
	task := JudgeTask{
		ID:               s.ids.Generate(),
		TenantID:         req.TenantID,
		JudgerID:         j.ID,
		SourceRef:        req.SourceRef,
		SubmitterID:      req.SubmitterID,
		ProblemRef:       req.ItemCode + ":" + req.ItemVersion,
		CodeStorageKey:   req.CodeStorageKey,
		CodeHash:         strings.TrimSpace(req.CodeHash),
		InputSnapshot:    snapshot,
		SandboxMode:      mode,
		TargetSandboxRef: strings.TrimSpace(req.TargetSandboxRef),
		Priority:         normalizePriority(req.Priority),
		Status:           JudgeTaskStatusQueued,
		MaxRetries:       maxRetriesForJudger(j, s.cfg.DefaultMaxRetries),
	}
	if j.Type == JudgerTypeManual {
		task.Status = JudgeTaskStatusJudging
	}
	if err := s.checkSubmitRate(ctx, task); err != nil {
		return contracts.JudgeTaskInfo{}, err
	}
	vector, err := s.buildSubmissionVector(ctx, task.CodeStorageKey)
	if err != nil {
		return contracts.JudgeTaskInfo{}, err
	}
	if err := s.store.TenantTx(ctx, task.TenantID, func(ctx context.Context, tx TxStore) error {
		created, err := tx.CreateJudgeTask(ctx, task)
		if err != nil {
			return apperr.ErrJudgeTaskEnqueueFailed.WithCause(err)
		}
		task = created
		if _, err := tx.CreateFingerprint(ctx, SubmissionFingerprint{
			ID:          s.ids.Generate(),
			TenantID:    task.TenantID,
			SourceRef:   task.SourceRef,
			ProblemRef:  task.ProblemRef,
			SubmitterID: task.SubmitterID,
			CodeHash:    task.CodeHash,
			SimVector:   vector,
		}); err != nil {
			return apperr.ErrFingerprintSimilarityFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return contracts.JudgeTaskInfo{}, err
	}
	s.publishProgress(task.TenantID, task.ID, task.Status, ProgressStageQueued, "判题任务已提交")
	if err := s.writeAudit(ctx, task.TenantID, task.SubmitterID, 5, "judge.submit", "judge_task", task.ID, map[string]any{"source_ref": task.SourceRef, "problem_ref": task.ProblemRef}); err != nil {
		return contracts.JudgeTaskInfo{}, err
	}
	return contractTaskInfoFromModel(JudgeTaskInfo{Task: task}), nil
}

// GetJudgeTask 读取任务状态与结果摘要。
func (s *Service) GetJudgeTask(ctx context.Context, tenantID, taskID int64) (contracts.JudgeTaskInfo, error) {
	info, err := s.getTaskInfo(ctx, tenantID, taskID)
	if err != nil {
		return contracts.JudgeTaskInfo{}, err
	}
	return contractTaskInfoFromModel(info), nil
}

// CancelJudgeTask 取消排队中的判题任务。
func (s *Service) CancelJudgeTask(ctx context.Context, tenantID, taskID int64) error {
	return s.CancelTask(ctx, tenantID, taskID)
}

// Rejudge 按原输入快照重新判题。
func (s *Service) Rejudge(ctx context.Context, tenantID, taskID int64) (contracts.JudgeTaskInfo, error) {
	info, err := s.RejudgeTask(ctx, tenantID, taskID)
	if err != nil {
		return contracts.JudgeTaskInfo{}, err
	}
	return contractTaskInfoFromModel(info), nil
}

// RejudgeBySourceRef 按来源标识批量重判任务。
func (s *Service) RejudgeBySourceRef(ctx context.Context, tenantID int64, sourceRef string) error {
	return s.RejudgeBatch(ctx, tenantID, sourceRef)
}

// ExactFingerprints 查询完全相同提交。
func (s *Service) ExactFingerprints(ctx context.Context, tenantID int64, problemRef, codeHash string) ([]contracts.FingerprintMatch, error) {
	if tenantID <= 0 || strings.TrimSpace(problemRef) == "" || !isSHA256Hex(codeHash) {
		return nil, apperr.ErrFingerprintRequestInvalid
	}
	var items []SubmissionFingerprint
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		items, err = tx.FindExactFingerprints(ctx, tenantID, strings.TrimSpace(problemRef), strings.TrimSpace(codeHash))
		if err != nil {
			return apperr.ErrFingerprintNotFound.WithCause(err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	out := make([]contracts.FingerprintMatch, 0, len(items))
	for _, item := range items {
		out = append(out, fingerprintToMatch(item, 1))
	}
	return out, nil
}

// FindExactMatch 实现跨模块查重契约。
func (s *Service) FindExactMatch(ctx context.Context, tenantID int64, problemRef, codeHash string) ([]contracts.FingerprintMatch, error) {
	return s.ExactFingerprints(ctx, tenantID, problemRef, codeHash)
}

// FindSimilarity 实现跨模块相似度查重契约。
func (s *Service) FindSimilarity(ctx context.Context, req contracts.FingerprintSimilarityRequest) ([]contracts.FingerprintMatch, error) {
	return s.Similarity(ctx, req.TenantID, FingerprintSimilarityRequest{
		ProblemRef:       req.ProblemRef,
		CodeStorageKey:   req.CodeStorageKey,
		CodeHash:         req.CodeHash,
		ExcludeSourceRef: req.ExcludeSourceRef,
		Threshold:        req.Threshold,
	})
}

// Similarity 读取对象生成特征向量并返回相似命中。
func (s *Service) Similarity(ctx context.Context, tenantID int64, req FingerprintSimilarityRequest) ([]contracts.FingerprintMatch, error) {
	if tenantID <= 0 || strings.TrimSpace(req.ProblemRef) == "" || strings.TrimSpace(req.CodeStorageKey) == "" {
		return nil, apperr.ErrFingerprintRequestInvalid
	}
	threshold := req.Threshold
	if threshold <= 0 {
		threshold = s.cfg.SimilarityDefaultThreshold
	}
	if threshold <= 0 || threshold >= 1 {
		return nil, apperr.ErrFingerprintRequestInvalid
	}
	vector, err := s.buildSubmissionVector(ctx, req.CodeStorageKey)
	if err != nil {
		return nil, err
	}
	var items []SubmissionFingerprint
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		items, err = tx.ListFingerprintsForProblem(ctx, tenantID, strings.TrimSpace(req.ProblemRef), strings.TrimSpace(req.ExcludeSourceRef))
		if err != nil {
			return apperr.ErrFingerprintSimilarityFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	out := []contracts.FingerprintMatch{}
	for _, item := range items {
		score := cosineSimilarity(vector, item.SimVector)
		if score >= threshold {
			out = append(out, fingerprintToMatch(item, score))
		}
	}
	return out, nil
}

// loadAvailableJudger 读取可用判题器并校验自检状态。
func (s *Service) loadAvailableJudger(ctx context.Context, code string) (Judger, error) {
	var j Judger
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		j, err = tx.GetJudgerByCode(ctx, strings.TrimSpace(code))
		if err != nil {
			return apperr.ErrJudgerNotFound.WithCause(err)
		}
		return nil
	}); err != nil {
		return Judger{}, err
	}
	if j.Status != JudgerStatusAvailable || j.SelftestStatus != JudgerSelftestPassed {
		return Judger{}, apperr.ErrJudgerUnavailable
	}
	return j, nil
}

// buildInputSnapshot 从 M5 判题配置和判题器定义构造脱敏快照。
func (s *Service) buildInputSnapshot(j Judger, spec contracts.ContentJudgeSpec, extra map[string]any) (JudgeInputSnapshot, error) {
	if spec.JudgerCode != "" && spec.JudgerCode != j.Code {
		return JudgeInputSnapshot{}, apperr.ErrJudgeSpecUnavailable
	}
	expectation, err := s.snapshotExpectationForJudger(j.Type, spec.Expectation, extra)
	if err != nil {
		return JudgeInputSnapshot{}, err
	}
	return JudgeInputSnapshot{
		ItemCode:            spec.ItemCode,
		ItemVersion:         spec.ItemVersion,
		JudgerCode:          j.Code,
		JudgerType:          j.Type,
		JudgerVersion:       j.ExecutorRef,
		SuiteRef:            spec.SuiteRef,
		SuiteArchiveName:    j.ResourceSpec.SuiteArchiveName,
		VersionHash:         spec.VersionHash,
		RuntimeCode:         j.ResourceSpec.RuntimeCode,
		RuntimeImageVersion: j.ResourceSpec.RuntimeImageVersion,
		GenesisRef:          j.ResourceSpec.GenesisRef,
		ToolCodes:           append([]string(nil), j.ResourceSpec.ToolCodes...),
		InitScriptRef:       j.ResourceSpec.InitScriptRef,
		Command:             append([]string(nil), j.ResourceSpec.Command...),
		TimeoutSec:          timeoutForSnapshot(j),
		MaxRetries:          maxRetriesForJudger(j, s.cfg.DefaultMaxRetries),
		MaxScore:            spec.MaxScore,
		Expectation:         expectation,
		ExtraInput:          extra,
	}, nil
}

// buildSubmissionVector 读取对象存储提交包并计算查重特征。
func (s *Service) buildSubmissionVector(ctx context.Context, objectRef string) (map[string]float64, error) {
	name, data, err := s.readObjectRef(ctx, objectRef)
	if err != nil {
		return nil, apperr.ErrFingerprintSimilarityFailed.WithCause(err)
	}
	return fingerprintVectorFromArchive(name, data, upload.ArchiveLimits{MaxFiles: s.cfg.InputArchiveMaxFiles, MaxUnpackedBytes: s.cfg.InputArchiveMaxUnpackedBytes})
}

// readObjectRef 读取 minio://bucket/key 对象,限制在统一对象存储接口内。
func (s *Service) readObjectRef(ctx context.Context, objectRef string) (string, []byte, error) {
	ref, err := storage.ParseObjectRef(objectRef)
	if err != nil {
		return "", nil, err
	}
	rc, err := s.minio.Get(ctx, ref.Bucket, ref.Key)
	if err != nil {
		return "", nil, err
	}
	defer rc.Close()
	limit := s.cfg.InputArchiveMaxUnpackedBytes
	if limit <= 0 {
		limit = 64 << 20
	}
	data, err := io.ReadAll(io.LimitReader(rc, limit+1))
	if err != nil {
		return "", nil, err
	}
	if int64(len(data)) > limit {
		return "", nil, apperr.ErrJudgeInputArchiveInvalid
	}
	return ref.Key, data, nil
}

// checkSubmitRate 基于最近同题同人任务做提交级限频。
func (s *Service) checkSubmitRate(ctx context.Context, task JudgeTask) error {
	if s.cfg.SubmitRateLimitSec <= 0 {
		return nil
	}
	var recent []JudgeTask
	if err := s.store.TenantTx(ctx, task.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		recent, err = tx.ListJudgeTasksBySourceRef(ctx, task.TenantID, task.SourceRef)
		return err
	}); err != nil {
		return apperr.ErrJudgeTaskEnqueueFailed.WithCause(err)
	}
	for _, item := range recent {
		if item.SubmitterID == task.SubmitterID && item.ProblemRef == task.ProblemRef && time.Since(item.CreatedAt) < time.Duration(s.cfg.SubmitRateLimitSec)*time.Second {
			return apperr.ErrJudgeSubmitRateLimited
		}
	}
	return nil
}

// getTaskInfo 查询任务并转换 no rows 为 M3 错误码。
func (s *Service) getTaskInfo(ctx context.Context, tenantID, taskID int64) (JudgeTaskInfo, error) {
	var info JudgeTaskInfo
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		info, err = tx.GetJudgeTaskInfo(ctx, tenantID, taskID)
		if err != nil {
			return apperr.ErrJudgeTaskNotFound.WithCause(err)
		}
		return nil
	}); err != nil {
		return JudgeTaskInfo{}, err
	}
	return info, nil
}

// publishProgress 向任务进度 topic 广播用户向状态。
func (s *Service) publishProgress(tenantID, taskID int64, status int16, stage, message string) {
	if s.wsHub == nil {
		return
	}
	raw, err := jsonx.AnyBytes(ProgressMessage{TaskID: taskID, Status: status, Stage: stage, Message: message}, apperr.ErrInternal)
	if err != nil {
		return
	}
	s.wsHub.Broadcast(judgeProgressTopic(tenantID, taskID), raw)
}

// judgeProgressTopic 生成判题进度 WebSocket topic。
func judgeProgressTopic(tenantID, taskID int64) string {
	return "judge:" + ids.Format(tenantID) + ":" + ids.Format(taskID) + ":progress"
}

// normalizeJudgerRequest 修剪判题器请求字段。
func normalizeJudgerRequest(req CreateJudgerRequest) CreateJudgerRequest {
	req.Code = strings.TrimSpace(req.Code)
	req.Name = strings.TrimSpace(req.Name)
	req.ExecutorRef = strings.TrimSpace(req.ExecutorRef)
	if req.Status == 0 {
		req.Status = JudgerStatusDisabled
	}
	return req
}

// normalizePriority 限制队列优先级范围。
func normalizePriority(priority int16) int16 {
	if priority < 1 {
		return 1
	}
	if priority > 9 {
		return 9
	}
	return priority
}

// safeFailureReason 返回脱敏失败原因。
func safeFailureReason(err error) string {
	if err == nil {
		return ""
	}
	return logging.SanitizeError(err.Error())
}

// encodeJSONBytes 序列化结构化输入。
func encodeJSONBytes(v any) ([]byte, error) {
	return jsonx.EncodeLineBytes(v)
}
