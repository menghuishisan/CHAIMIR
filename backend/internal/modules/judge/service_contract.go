package judge

import (
	"context"
	"fmt"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/response"
	"chaimir/pkg/apperr"
)

// SubmitJudgeTask 创建判题任务、输入快照和提交指纹。
func (s *Service) SubmitJudgeTask(ctx context.Context, req contracts.JudgeSubmitRequest) (contracts.JudgeTaskInfo, error) {
	if err := validateSubmitRequest(req); err != nil {
		return contracts.JudgeTaskInfo{}, err
	}
	problemRef := req.ItemCode + ":" + req.ItemVersion
	if existing, ok, err := s.findExistingTaskBySourceRef(ctx, req.TenantID, req.SourceRef, problemRef); err != nil {
		return contracts.JudgeTaskInfo{}, err
	} else if ok {
		return contractTaskInfoFromModel(JudgeTaskInfo{Task: existing, Existing: true}), nil
	}
	spec, err := s.content.GetJudgeSpec(ctx, req.TenantID, req.ItemCode, req.ItemVersion)
	if err != nil {
		return contracts.JudgeTaskInfo{}, apperr.ErrJudgeSpecUnavailable.WithCause(err)
	}
	judgerCode := strings.TrimSpace(req.JudgerCode)
	if judgerCode == "" {
		judgerCode = spec.JudgerCode
	}
	if judgerCode == "" || (strings.TrimSpace(req.JudgerCode) != "" && strings.TrimSpace(req.JudgerCode) != spec.JudgerCode) {
		return contracts.JudgeTaskInfo{}, apperr.ErrJudgeSubmitInvalid
	}
	j, err := s.loadAvailableJudger(ctx, judgerCode)
	if err != nil {
		return contracts.JudgeTaskInfo{}, err
	}
	mode, _ := normalizedSandboxMode(req.SandboxMode)
	if j.Type == JudgerTypeManual {
		mode = JudgeSandboxModeFresh
	}
	if err := validateJudgerSandboxMode(j.Type, mode, req.TargetSandboxRef); err != nil {
		return contracts.JudgeTaskInfo{}, err
	}
	if req.RequiresOutcome && j.Type != JudgerTypeOnchainAssert {
		return contracts.JudgeTaskInfo{}, apperr.ErrJudgeSubmitInvalid
	}
	snapshot, err := s.buildInputSnapshot(j, spec, req.ExtraInput)
	if err != nil {
		return contracts.JudgeTaskInfo{}, err
	}
	traceID := response.TraceFromContext(ctx)
	if strings.TrimSpace(traceID) == "" {
		return contracts.JudgeTaskInfo{}, apperr.ErrJudgeSubmitInvalid.WithCause(fmt.Errorf("判题提交缺少 trace_id"))
	}
	snapshot.TraceID = traceID
	snapshot.SandboxSourceRef = strings.TrimSpace(req.SandboxSourceRef)
	snapshot.SandboxCompositionDigest = strings.TrimSpace(req.SandboxCompositionDigest)
	requiresCode := judgerRequiresCode(j.Type, mode)
	codeHash, vector, sanitizedCode, err := s.prepareSubmittedCode(ctx, req, requiresCode)
	if err != nil {
		return contracts.JudgeTaskInfo{}, err
	}
	snapshot.CodeHash = codeHash
	ownership := sourceOwnershipFromRequest(req)
	if err := validateSourceOwnership(ownership); err != nil {
		return contracts.JudgeTaskInfo{}, err
	}
	task := JudgeTask{ID: s.ids.Generate(), TenantID: req.TenantID, JudgerID: j.ID, SourceRef: req.SourceRef, SourceOwnerID: ownership.OwnerID, SourceCourseID: ownership.CourseID, SourceScope: ownership.Scope, SubmitterTenantID: req.SubmitterTenantID, SubmitterID: req.SubmitterID, ProblemRef: problemRef, CodeStorageKey: req.CodeStorageKey, CodeHash: codeHash, InputSnapshot: snapshot, SandboxMode: mode, TargetSandboxRef: strings.TrimSpace(req.TargetSandboxRef), Priority: normalizePriority(req.Priority), Status: JudgeTaskStatusQueued, MaxRetries: maxRetriesForJudger(j, s.cfg.DefaultMaxRetries)}
	if j.Type == JudgerTypeManual {
		task.Status = JudgeTaskStatusJudging
	}
	if mode == JudgeSandboxModeReuse {
		if _, err := s.resolveReuseSandbox(ctx, &task); err != nil {
			return contracts.JudgeTaskInfo{}, err
		}
	}
	if j.Type == JudgerTypeOnchainAssert {
		if _, err := onchainSteps(mode, task.accessProfile, spec.Expectation); err != nil {
			return contracts.JudgeTaskInfo{}, err
		}
		hasOutcome, err := validateOutcomeAssertions(spec.Expectation)
		if err != nil {
			return contracts.JudgeTaskInfo{}, err
		}
		if err := validateOutcomeRequirement(j.Type, mode, task.accessProfile, hasOutcome, req.RequiresOutcome); err != nil {
			return contracts.JudgeTaskInfo{}, err
		}
	}
	if isBattleTask(task) {
		task.MaxRetries = 0
	}
	if err := s.checkSubmitRate(ctx, task); err != nil {
		return contracts.JudgeTaskInfo{}, err
	}
	requestedTaskID := task.ID
	archiveRef := ""
	if requiresCode {
		archiveRef, err = s.storeSanitizedCodeArchive(ctx, task.TenantID, requestedTaskID, sanitizedCode)
		if err != nil {
			return contracts.JudgeTaskInfo{}, err
		}
		task.InputSnapshot.SanitizedCodeArchiveName = "submission.tar"
		task.InputSnapshot.SanitizedCodeArchiveRef = archiveRef
	}
	createdNew := false
	if err := s.store.TenantTx(ctx, task.TenantID, func(ctx context.Context, tx TxStore) error {
		created, err := tx.CreateJudgeTask(ctx, task)
		if err != nil {
			return apperr.ErrJudgeTaskEnqueueFailed.WithCause(err)
		}
		task = created
		createdNew = task.ID == requestedTaskID
		if !createdNew {
			return nil
		}
		if requiresCode {
			if _, err := tx.CreateFingerprint(ctx, SubmissionFingerprint{ID: s.ids.Generate(), TenantID: task.TenantID, SourceRef: task.SourceRef, ProblemRef: task.ProblemRef, SubmitterTenantID: task.SubmitterTenantID, SubmitterID: task.SubmitterID, CodeHash: task.CodeHash, SimVector: vector}); err != nil {
				return apperr.ErrFingerprintSimilarityFailed.WithCause(err)
			}
		}
		return s.writeSystemAudit(ctx, task.TenantID, "judge.submit", "judge_task", task.ID, map[string]any{"source_ref": task.SourceRef, "problem_ref": task.ProblemRef, "submitter_id": task.SubmitterID})
	}); err != nil {
		s.discardStoredCodeArchive(ctx, task.TenantID, requestedTaskID, archiveRef, err)
		return contracts.JudgeTaskInfo{}, err
	}
	if !createdNew {
		s.discardStoredCodeArchive(ctx, task.TenantID, requestedTaskID, archiveRef, nil)
		return contractTaskInfoFromModel(JudgeTaskInfo{Task: task, Existing: true}), nil
	}
	s.publishProgress(ctx, task.TenantID, task.ID, task.Status, ProgressStageQueued, "判题任务已提交")
	return contractTaskInfoFromModel(JudgeTaskInfo{Task: task}), nil
}

// GetJudgeInputRequirements 统一决定模式与输入，业务模块无需识别判题器编码。
func (s *Service) GetJudgeInputRequirements(ctx context.Context, tenantID int64, itemCode, itemVersion string, input contracts.JudgeInputContext) (contracts.JudgeInputRequirements, error) {
	sandboxMode := input.SandboxMode
	var out contracts.JudgeInputRequirements
	if tenantID <= 0 || strings.TrimSpace(itemCode) == "" || strings.TrimSpace(itemVersion) == "" {
		return out, apperr.ErrJudgeSubmitInvalid
	}
	if input.RequiresOutcome && input.AccessProfile != contracts.SandboxAccessContestBattle {
		return out, apperr.ErrJudgerConfigInvalid
	}
	spec, err := s.content.GetJudgeSpec(ctx, tenantID, itemCode, itemVersion)
	if err != nil {
		return out, apperr.ErrJudgeSpecUnavailable.WithCause(err)
	}
	if strings.TrimSpace(spec.JudgerCode) == "" {
		return out, apperr.ErrJudgeSpecUnavailable
	}
	j, err := s.loadAvailableJudger(ctx, spec.JudgerCode)
	if err != nil {
		return out, err
	}
	if input.AccessProfile == contracts.SandboxAccessContestBattle {
		if sandboxMode != "" && sandboxMode != contracts.JudgeSandboxModeReuse {
			return out, apperr.ErrJudgeSubmitInvalid
		}
		sandboxMode = contracts.JudgeSandboxModeReuse
	}
	if strings.TrimSpace(sandboxMode) == "" && j.Type == JudgerTypeOnchainAssert {
		steps, err := onchainSteps(JudgeSandboxModeFresh, "", spec.Expectation)
		if err != nil {
			return out, err
		}
		if len(steps) == 0 {
			sandboxMode = contracts.JudgeSandboxModeReuse
		}
	}
	mode, err := normalizedSandboxMode(sandboxMode)
	if err != nil {
		return out, err
	}
	if err := validateJudgerSupportsSandboxMode(j.Type, mode); err != nil {
		return out, err
	}
	if j.Type == JudgerTypeOnchainAssert {
		if _, err := onchainSteps(mode, input.AccessProfile, spec.Expectation); err != nil {
			return out, err
		}
	}
	hasOutcome, err := validateOutcomeAssertions(spec.Expectation)
	if err != nil {
		return out, err
	}
	if err := validateOutcomeRequirement(j.Type, mode, input.AccessProfile, hasOutcome, input.RequiresOutcome); err != nil {
		return out, err
	}
	if mode == JudgeSandboxModeReuse {
		if _, err := s.buildInputSnapshot(j, spec, nil); err != nil {
			return out, err
		}
	}
	out.SandboxMode = contracts.JudgeSandboxModeFresh
	if mode == JudgeSandboxModeReuse {
		out.SandboxMode = contracts.JudgeSandboxModeReuse
	}
	out.RequiresCode = judgerRequiresCode(j.Type, mode)
	return out, nil
}

// GetJudgeTask 读取任务状态与结果摘要。
func (s *Service) GetJudgeTask(ctx context.Context, tenantID, taskID int64) (contracts.JudgeTaskInfo, error) {
	info, err := s.getTaskInfo(ctx, tenantID, taskID)
	if err != nil {
		return contracts.JudgeTaskInfo{}, err
	}
	return contractTaskInfoFromModel(info), nil
}

// Rejudge 按原输入快照重新判题。
func (s *Service) Rejudge(ctx context.Context, tenantID, taskID int64) (contracts.JudgeTaskInfo, error) {
	info, err := s.RejudgeTask(ctx, tenantID, taskID)
	if err != nil {
		return contracts.JudgeTaskInfo{}, err
	}
	return contractTaskInfoFromModel(info), nil
}

// FindSimilarity 实现跨模块相似度查重契约。
func (s *Service) FindSimilarity(ctx context.Context, req contracts.FingerprintSimilarityRequest) ([]contracts.FingerprintMatch, error) {
	return s.Similarity(ctx, req.TenantID, FingerprintSimilarityRequest{ProblemRef: req.ProblemRef, SourceRefs: req.SourceRefs, CodeHash: req.CodeHash, ExcludeSourceRef: req.ExcludeSourceRef, Threshold: req.Threshold})
}
