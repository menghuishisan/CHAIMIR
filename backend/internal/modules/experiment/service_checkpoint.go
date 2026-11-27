package experiment

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
)

// JudgeCheckpoint 按当前输入创建独立判题任务，并在实例锁内绑定检查点结果。
func (s *Service) JudgeCheckpoint(ctx context.Context, instanceID int64, checkpointID string, req JudgeCheckpointRequest) (CheckpointDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return CheckpointDTO{}, err
	}
	bindingOutput, err := normalizeBindingOutput(req.BindingOutput)
	if err != nil {
		return CheckpointDTO{}, err
	}
	if s.judge == nil {
		return CheckpointDTO{}, apperr.ErrExperimentJudgeUnavailable
	}
	var result CheckpointResult
	var inst ExperimentInstance
	var exp Experiment
	var cp CheckpointComponent
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		inst, err = tx.GetInstance(ctx, id.TenantID, instanceID)
		if err != nil {
			return err
		}
		if err := ensureInstanceAccess(ctx, tx, id.AccountID, inst); err != nil {
			return err
		}
		if inst.Status != InstanceStatusRunning && inst.Status != InstanceStatusPaused {
			return apperr.ErrExperimentInstanceStateInvalid
		}
		exp, err = tx.GetExperiment(ctx, id.TenantID, inst.ExperimentID)
		if err != nil {
			return err
		}
		var ok bool
		cp, ok = checkpointByID(exp, checkpointID)
		if !ok {
			return apperr.ErrExperimentCheckpointInvalid
		}
		return nil
	})
	if err != nil {
		return CheckpointDTO{}, err
	}
	mode := sandboxModeForCheckpoint(cp)
	sandbox, hasSandbox := sandboxRefForCheckpoint(cp, inst)
	if !hasSandbox && (cp.EnvID != "" || mode == contracts.JudgeSandboxModeReuse) {
		return CheckpointDTO{}, apperr.ErrExperimentInstanceStateInvalid
	}
	codeKey, codeHash, err := s.resolveJudgeCodeSnapshot(ctx, inst, cp, mode, sandbox.SandboxID, req)
	if err != nil {
		return CheckpointDTO{}, err
	}
	extra, err := jsonx.CloneObjectStrict(cp.ExtraInput)
	if err != nil {
		return CheckpointDTO{}, apperr.ErrExperimentCheckpointInvalid.WithCause(err)
	}
	for key, value := range req.ExtraInput {
		extra[key] = value
	}
	var sandboxID, sandboxSource string
	if mode == contracts.JudgeSandboxModeReuse {
		sandboxID = ids.Format(sandbox.SandboxID.Int64())
		sandboxSource = inst.SourceRef
	}
	submissionID := s.ids.Generate()
	sourceRef := strings.Replace(inst.SourceRef, ":instance:", ":judge:", 1) + "-" + ids.Format(submissionID) + "-" + cp.ID
	task, err := s.judge.SubmitJudgeTask(ctx, contracts.JudgeSubmitRequest{TenantID: inst.TenantID, ItemCode: cp.ItemCode, ItemVersion: cp.ItemVersion, CodeStorageKey: codeKey, CodeHash: codeHash, SubmitterTenantID: id.TenantID, SubmitterID: id.AccountID, SourceRef: sourceRef, SourceOwnerID: inst.OwnerAccountID, SourceCourseID: exp.CourseID, SourceScope: "experiment", SandboxMode: mode, TargetSandboxRef: sandboxID, SandboxSourceRef: sandboxSource, ExtraInput: extra, Priority: 5})
	if err != nil {
		return CheckpointDTO{}, apperr.ErrExperimentJudgeUnavailable.WithCause(err)
	}
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		// 与成员调整共用小组锁，避免鉴权通过后、绑定提交前被移除。
		if inst.GroupID > 0 {
			if _, err := tx.GetGroupForUpdate(ctx, id.TenantID, inst.GroupID); err != nil {
				return err
			}
		}
		current, err := tx.GetInstanceForUpdate(ctx, id.TenantID, instanceID)
		if err != nil {
			return err
		}
		if (current.Status != InstanceStatusRunning && current.Status != InstanceStatusPaused) || current.SourceRef != inst.SourceRef || current.ScopeRef != inst.ScopeRef {
			return apperr.ErrExperimentInstanceStateInvalid
		}
		if err := ensureInstanceAccess(ctx, tx, id.AccountID, current); err != nil {
			return err
		}
		result, err = tx.UpsertCheckpoint(ctx, checkpointWithJudgeTask(CheckpointResult{ID: submissionID, TenantID: inst.TenantID, InstanceID: inst.ID, CheckpointID: cp.ID, JudgeTaskRef: ids.Format(task.TaskID), BindingOutput: bindingOutput}, task, cp.Score))
		if err != nil {
			return err
		}
		_, err = tx.TouchInstance(ctx, inst.TenantID, inst.ID)
		return err
	})
	if err != nil {
		// 请求已经取消时仍须有界清理已创建的任务。
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Duration(s.cfg.CleanupTimeoutSeconds)*time.Second)
		defer cancel()
		return CheckpointDTO{}, errors.Join(err, s.judge.CancelJudgeTask(cleanupCtx, id.TenantID, task.TaskID))
	}
	result.JudgeStatus = task.Status
	s.publishInstanceChanged(ctx, inst.TenantID, inst.ID)
	return checkpointDTOFromModel(result), nil
}

func (s *Service) instanceWithCurrentJudgements(ctx context.Context, inst ExperimentInstance, exp Experiment, results []CheckpointResult) (InstanceDTO, error) {
	current, err := s.currentCheckpointResults(ctx, inst, exp, results)
	if err != nil {
		return InstanceDTO{}, err
	}
	return instanceDTOFromModel(inst, current, stageDTOs(exp, inst, current)), nil
}

// currentCheckpointResults 统一读取、解锁与结算使用的 M3 当前结果，不持久化任务状态。
func (s *Service) currentCheckpointResults(ctx context.Context, inst ExperimentInstance, exp Experiment, results []CheckpointResult) ([]CheckpointResult, error) {
	current := append([]CheckpointResult(nil), results...)
	for index, result := range current {
		if s.judge == nil {
			return nil, apperr.ErrExperimentJudgeUnavailable
		}
		taskID, valid := ids.Parse(result.JudgeTaskRef)
		cp, found := checkpointByID(exp, result.CheckpointID)
		if !valid || !found {
			return nil, apperr.ErrExperimentCheckpointInvalid
		}
		task, err := s.judge.GetJudgeTask(ctx, inst.TenantID, taskID)
		if err != nil {
			return nil, apperr.ErrExperimentJudgeUnavailable.WithCause(err)
		}
		current[index] = checkpointWithJudgeTask(result, task, cp.Score)
	}
	return current, nil
}

func checkpointWithJudgeTask(result CheckpointResult, task contracts.JudgeTaskInfo, maxScore float64) CheckpointResult {
	result.JudgeStatus = task.Status
	result.Passed, result.Score, result.DetailRef = false, 0, ""
	if task.Status == contracts.JudgeTaskStatusDone {
		result.Passed = task.Result.Passed
		result.Score = scaledCheckpointScore(maxScore, task.Result.Score, task.Result.MaxScore)
		result.DetailRef = task.Result.ResultRef
	} else if task.Status == contracts.JudgeTaskStatusFailed {
		result.DetailRef = "judge_failed"
	}
	return result
}

// HandleJudgeCompleted 消费 M3 完成事件，仅更新仍绑定该任务的检查点。
func (s *Service) HandleJudgeCompleted(ctx context.Context, event contracts.JudgeCompletedEvent) error {
	return s.recordCheckpointJudgement(ctx, event.TenantID, event.TaskID, event.SourceRef, contracts.JudgeTaskStatusDone)
}

// HandleJudgeFailed 撤回执行失败任务的旧结果，保留当前任务绑定供重判。
func (s *Service) HandleJudgeFailed(ctx context.Context, event contracts.JudgeFailedEvent) error {
	return s.recordCheckpointJudgement(ctx, event.TenantID, event.TaskID, event.SourceRef, contracts.JudgeTaskStatusFailed)
}

func (s *Service) recordCheckpointJudgement(ctx context.Context, tenantID, taskID int64, sourceRef string, status int16) error {
	submission, err := parseCheckpointSubmission(sourceRef)
	if tenantID <= 0 || taskID <= 0 || err != nil {
		return apperr.ErrExperimentCheckpointInvalid
	}
	if s.judge == nil {
		return apperr.ErrExperimentJudgeUnavailable
	}
	task, err := s.judge.GetJudgeTask(ctx, tenantID, taskID)
	if err != nil {
		return apperr.ErrExperimentJudgeUnavailable.WithCause(err)
	}
	if task.SourceRef != sourceRef {
		return apperr.ErrExperimentSourceRefInvalid
	}
	if task.Status != status {
		return nil
	}
	shouldPublish := false
	checkpointChanged := false
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		inst, err := tx.GetInstanceForUpdate(ctx, tenantID, submission.instanceID)
		if err != nil {
			return err
		}
		if inst.SourceRef != submission.instanceSource {
			return apperr.ErrExperimentSourceRefInvalid
		}
		results, err := tx.ListCheckpoints(ctx, tenantID, inst.ID)
		if err != nil {
			return err
		}
		var result CheckpointResult
		for _, candidate := range results {
			if candidate.CheckpointID == submission.checkpointID {
				result = candidate
				break
			}
		}
		// 先于绑定的事件必须重投；较旧提交的事件可以直接确认。
		if result.ID < submission.id {
			return apperr.ErrExperimentCheckpointInvalid.WithCause(fmt.Errorf("checkpoint submission %d not bound yet", submission.id))
		}
		if result.ID > submission.id {
			return nil
		}
		if result.InstanceID != inst.ID || result.JudgeTaskRef != ids.Format(taskID) {
			return apperr.ErrExperimentSourceRefInvalid
		}
		var maxScore float64
		if status == contracts.JudgeTaskStatusDone {
			exp, err := tx.GetExperiment(ctx, tenantID, inst.ExperimentID)
			if err != nil {
				return err
			}
			cp, ok := checkpointByID(exp, result.CheckpointID)
			if !ok {
				return apperr.ErrExperimentCheckpointInvalid
			}
			maxScore = cp.Score
		}
		if _, err = tx.UpsertCheckpoint(ctx, checkpointWithJudgeTask(result, task, maxScore)); err != nil {
			return err
		}
		checkpointChanged = true
		if inst.Status != InstanceStatusFinished && inst.Status != InstanceStatusRecycled {
			return nil
		}
		score, err := tx.SumScores(ctx, tenantID, inst.ID)
		if err != nil {
			return err
		}
		scored, changed, err := tx.UpdateInstanceScoreIfChanged(ctx, tenantID, inst.ID, score)
		if err != nil || !changed {
			return err
		}
		shouldPublish = true
		return s.enqueueExperimentEventOutbox(ctx, tx, scored)
	}); err != nil {
		return err
	}
	if shouldPublish {
		s.drainExperimentEventOutboxBestEffort(ctx)
	}
	if checkpointChanged {
		s.publishInstanceChanged(ctx, tenantID, submission.instanceID)
	}
	return nil
}

type checkpointSubmission struct {
	instanceID     int64
	id             int64
	checkpointID   string
	instanceSource string
}

func parseCheckpointSubmission(sourceRef string) (checkpointSubmission, error) {
	if !validExperimentSourceRef(sourceRef) {
		return checkpointSubmission{}, apperr.ErrExperimentSourceRefInvalid
	}
	parts := strings.Split(sourceRef, ":")
	refs := strings.SplitN(parts[3], "-", 3)
	if parts[2] != "judge" || len(refs) != 3 || !componentIDPattern.MatchString(refs[2]) {
		return checkpointSubmission{}, apperr.ErrExperimentSourceRefInvalid
	}
	instanceID, instanceOK := ids.Parse(refs[0])
	submissionID, submissionOK := ids.Parse(refs[1])
	if !instanceOK || !submissionOK {
		return checkpointSubmission{}, apperr.ErrExperimentSourceRefInvalid
	}
	parts[2], parts[3] = "instance", refs[0]
	return checkpointSubmission{instanceID: instanceID, id: submissionID, checkpointID: refs[2], instanceSource: strings.Join(parts, ":")}, nil
}

// resolveJudgeCodeSnapshot 仅为 M3 要求代码的判题保存工作区，显式引用只用于无沙箱检查点。
func (s *Service) resolveJudgeCodeSnapshot(ctx context.Context, inst ExperimentInstance, cp CheckpointComponent, mode string, sandboxID ids.ID, req JudgeCheckpointRequest) (string, string, error) {
	input, err := s.judge.GetJudgeInputRequirements(ctx, inst.TenantID, cp.ItemCode, cp.ItemVersion, contracts.JudgeInputContext{SandboxMode: mode, AccessProfile: contracts.SandboxAccessExperiment})
	if err != nil {
		return "", "", apperr.ErrExperimentJudgeUnavailable.WithCause(err)
	}
	if !input.RequiresCode {
		return "", "", nil
	}
	if sandboxID > 0 {
		if s.sandbox == nil {
			return "", "", apperr.ErrExperimentSandboxUnavailable
		}
		codeKey, codeHash, err := s.sandbox.SaveSandboxFiles(ctx, contracts.SandboxSaveRequest{TenantID: inst.TenantID, SandboxID: sandboxID.Int64(), SourceRef: inst.SourceRef})
		if err != nil {
			return "", "", apperr.ErrExperimentSandboxUnavailable.WithCause(err)
		}
		return codeKey, codeHash, nil
	}
	if req.CodeStorageKey == "" || req.CodeHash == "" {
		return "", "", apperr.ErrExperimentCheckpointInvalid
	}
	return req.CodeStorageKey, req.CodeHash, nil
}

func sandboxModeForCheckpoint(cp CheckpointComponent) string {
	if cp.Mode != "" {
		return cp.Mode
	}
	if cp.EnvID != "" {
		return contracts.JudgeSandboxModeReuse
	}
	return contracts.JudgeSandboxModeFresh
}

// sandboxRefForCheckpoint 解析检查点声明的环境;显式绑定缺失时不回退到其他沙箱。
func sandboxRefForCheckpoint(cp CheckpointComponent, inst ExperimentInstance) (SandboxRef, bool) {
	if len(inst.SandboxRefs) == 0 {
		return SandboxRef{}, false
	}
	if cp.EnvID == "" {
		return inst.SandboxRefs[0], true
	}
	for _, ref := range inst.SandboxRefs {
		if ref.ComponentID == cp.EnvID {
			return ref, true
		}
	}
	return SandboxRef{}, false
}

// scaledCheckpointScore 将 M3 原始分按检查点配置分值归一。
func scaledCheckpointScore(maxScore float64, score, judgeMax int32) float64 {
	if judgeMax <= 0 {
		if float64(score) > maxScore {
			return maxScore
		}
		return float64(score)
	}
	return math.Round((float64(score)/float64(judgeMax))*maxScore*100) / 100
}
