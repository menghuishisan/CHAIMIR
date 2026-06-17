// experiment service_checkpoint 文件实现检查点判分、判题事件回写和得分重算。
package experiment

import (
	"context"
	"fmt"
	"math"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/pkg/apperr"
)

// JudgeCheckpoint 触发某个实例检查点判分,判题终态由 M3 事件回写。
func (s *Service) JudgeCheckpoint(ctx context.Context, instanceID int64, checkpointID string, req JudgeCheckpointRequest) (CheckpointDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return CheckpointDTO{}, err
	}
	bindingOutput, err := normalizeBindingOutput(req.BindingOutput)
	if err != nil {
		return CheckpointDTO{}, err
	}
	var inst ExperimentInstance
	var exp Experiment
	var cp CheckpointComponent
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
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
	}); err != nil {
		return CheckpointDTO{}, err
	}
	if s.judge == nil {
		return CheckpointDTO{}, apperr.ErrExperimentJudgeUnavailable
	}
	codeKey, codeHash, err := s.resolveJudgeCodeSnapshot(ctx, inst, req)
	if err != nil {
		return CheckpointDTO{}, err
	}
	extra := cloneAnyMap(cp.ExtraInput)
	for key, value := range req.ExtraInput {
		extra[key] = value
	}
	task, err := s.judge.SubmitJudgeTask(ctx, contracts.JudgeSubmitRequest{TenantID: inst.TenantID, JudgerCode: cp.JudgerCode, ItemCode: cp.ItemCode, ItemVersion: cp.ItemVersion, CodeStorageKey: codeKey, CodeHash: codeHash, SubmitterID: id.AccountID, SourceRef: inst.SourceRef, SourceOwnerID: inst.OwnerAccountID, SourceCourseID: exp.CourseID, SourceScope: "exp", SandboxMode: sandboxModeForCheckpoint(cp), TargetSandboxRef: targetSandboxRef(cp, inst), ExtraInput: extra, Priority: 5})
	if err != nil {
		return CheckpointDTO{}, apperr.ErrExperimentJudgeUnavailable.WithCause(err)
	}
	result := CheckpointResult{ID: s.ids.Generate(), TenantID: inst.TenantID, InstanceID: inst.ID, CheckpointID: cp.ID, JudgeTaskRef: ids.Format(task.TaskID), Passed: task.Result.Passed, Score: scaledCheckpointScore(cp.Score, task.Result.Score, task.Result.MaxScore), DetailRef: task.Result.SnapshotRef, BindingOutput: bindingOutput}
	if err := s.store.TenantTx(ctx, inst.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		result, err = tx.UpsertCheckpoint(ctx, result)
		if err != nil {
			return err
		}
		_, err = tx.TouchInstance(ctx, inst.TenantID, inst.ID)
		return err
	}); err != nil {
		return CheckpointDTO{}, err
	}
	return CheckpointDTO{ID: result.CheckpointID, JudgeTaskRef: result.JudgeTaskRef, Passed: result.Passed, Score: result.Score, DetailRef: result.DetailRef, BindingOutput: result.BindingOutput}, nil
}

// HandleJudgeCompleted 消费 M3 判题完成事件并回写检查点得分。
func (s *Service) HandleJudgeCompleted(ctx context.Context, event contracts.JudgeCompletedEvent) error {
	if event.TenantID <= 0 || event.TaskID <= 0 || !validExperimentSourceRef(event.SourceRef) {
		return apperr.ErrExperimentCheckpointInvalid
	}
	if s.judge == nil {
		return apperr.ErrExperimentJudgeUnavailable
	}
	task, err := s.judge.GetJudgeTask(ctx, event.TenantID, event.TaskID)
	if err != nil {
		return apperr.ErrExperimentJudgeUnavailable.WithCause(err)
	}
	if task.SourceRef != event.SourceRef {
		return apperr.ErrExperimentSourceRefInvalid
	}
	var scored ExperimentInstance
	var inst ExperimentInstance
	var exp Experiment
	shouldPublish := false
	if err := s.store.TenantTx(ctx, event.TenantID, func(ctx context.Context, tx TxStore) error {
		result, err := tx.GetCheckpointByJudgeTask(ctx, event.TenantID, ids.Format(event.TaskID))
		if err != nil {
			return err
		}
		inst, err = tx.GetInstance(ctx, event.TenantID, result.InstanceID)
		if err != nil {
			return err
		}
		if inst.SourceRef != event.SourceRef {
			return apperr.ErrExperimentSourceRefInvalid
		}
		exp, err = tx.GetExperiment(ctx, event.TenantID, inst.ExperimentID)
		if err != nil {
			return err
		}
		cp, ok := checkpointByID(exp, result.CheckpointID)
		if !ok {
			return apperr.ErrExperimentCheckpointInvalid
		}
		if _, err = tx.UpsertCheckpoint(ctx, CheckpointResult{ID: result.ID, TenantID: event.TenantID, InstanceID: result.InstanceID, CheckpointID: result.CheckpointID, JudgeTaskRef: ids.Format(event.TaskID), Passed: task.Result.Passed, Score: scaledCheckpointScore(cp.Score, task.Result.Score, task.Result.MaxScore), DetailRef: task.Result.SnapshotRef, BindingOutput: result.BindingOutput}); err != nil {
			return err
		}
		if inst.Status != InstanceStatusFinished {
			return nil
		}
		score, err := tx.SumScores(ctx, event.TenantID, inst.ID)
		if err != nil {
			return err
		}
		scored, err = tx.UpdateInstanceScore(ctx, event.TenantID, inst.ID, score)
		if err != nil {
			return err
		}
		shouldPublish = true
		return s.enqueueExperimentScoreOutbox(ctx, tx, scored)
	}); err != nil {
		return err
	}
	if shouldPublish {
		s.drainExperimentScoreOutboxBestEffort(ctx)
	}
	return nil
}

// HandleJudgeFailed 消费 M3 判题失败事件并把检查点记录为失败零分。
func (s *Service) HandleJudgeFailed(ctx context.Context, event contracts.JudgeFailedEvent) error {
	if event.TenantID <= 0 || event.TaskID <= 0 || !validExperimentSourceRef(event.SourceRef) {
		return apperr.ErrExperimentCheckpointInvalid
	}
	var scored ExperimentInstance
	shouldPublish := false
	if err := s.store.TenantTx(ctx, event.TenantID, func(ctx context.Context, tx TxStore) error {
		result, err := tx.GetCheckpointByJudgeTask(ctx, event.TenantID, ids.Format(event.TaskID))
		if err != nil {
			return err
		}
		inst, err := tx.GetInstance(ctx, event.TenantID, result.InstanceID)
		if err != nil {
			return err
		}
		if inst.SourceRef != event.SourceRef {
			return apperr.ErrExperimentSourceRefInvalid
		}
		if _, err = tx.UpsertCheckpoint(ctx, CheckpointResult{ID: result.ID, TenantID: event.TenantID, InstanceID: result.InstanceID, CheckpointID: result.CheckpointID, JudgeTaskRef: ids.Format(event.TaskID), Passed: false, Score: 0, DetailRef: "judge_failed"}); err != nil {
			return err
		}
		if inst.Status != InstanceStatusFinished {
			return nil
		}
		score, err := tx.SumScores(ctx, event.TenantID, inst.ID)
		if err != nil {
			return err
		}
		scored, err = tx.UpdateInstanceScore(ctx, event.TenantID, inst.ID, score)
		if err != nil {
			return err
		}
		shouldPublish = true
		return s.enqueueExperimentScoreOutbox(ctx, tx, scored)
	}); err != nil {
		return err
	}
	if shouldPublish {
		s.drainExperimentScoreOutboxBestEffort(ctx)
	}
	return nil
}

// resolveJudgeCodeSnapshot 优先由 M2 保存当前沙箱工作区,客户端传入仅用于无沙箱检查点。
func (s *Service) resolveJudgeCodeSnapshot(ctx context.Context, inst ExperimentInstance, req JudgeCheckpointRequest) (string, string, error) {
	if len(inst.SandboxRefs) > 0 {
		if s.sandbox == nil {
			return "", "", apperr.ErrExperimentSandboxUnavailable
		}
		codeKey, codeHash, err := s.sandbox.SaveSandboxFiles(ctx, contracts.SandboxSaveRequest{TenantID: inst.TenantID, SandboxID: inst.SandboxRefs[0].SandboxID, SourceRef: inst.SourceRef})
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

// sandboxModeForCheckpoint 选择 M3 判题沙箱模式,默认复用现场沙箱。
func sandboxModeForCheckpoint(cp CheckpointComponent) string {
	if cp.Mode != "" {
		return cp.Mode
	}
	if cp.EnvID != "" {
		return contracts.JudgeSandboxModeReuse
	}
	return contracts.JudgeSandboxModeFresh
}

// targetSandboxRef 返回指定检查点绑定的沙箱引用。
func targetSandboxRef(cp CheckpointComponent, inst ExperimentInstance) string {
	if len(inst.SandboxRefs) == 0 {
		return ""
	}
	if cp.EnvID != "" {
		for _, ref := range inst.SandboxRefs {
			if ref.ComponentID == cp.EnvID {
				return fmt.Sprintf("%d", ref.SandboxID)
			}
		}
	}
	return fmt.Sprintf("%d", inst.SandboxRefs[0].SandboxID)
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

// cloneAnyMap 复制动态输入,避免修改组件定义中的原始 map。
func cloneAnyMap(in map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range in {
		out[key] = value
	}
	return out
}
