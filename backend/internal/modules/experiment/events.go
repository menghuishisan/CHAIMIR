// M7 事件处理:订阅 M3 判题结果与 M2 沙箱回收事件,并发布实验得分事件。
package experiment

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/ids"
	"chaimir/pkg/apperr"
)

// HandleJudgeCompleted 处理 M3 判题完成事件并回写检查点结果。
func (s *Service) HandleJudgeCompleted(ctx context.Context, event contracts.JudgeCompletedEvent) error {
	pending, err := s.store.PendingCheckpointByJudgeTask(ctx, event.TenantID, event.TaskID)
	if err != nil {
		return err
	}
	if pending.SourceRef != event.SourceRef {
		return apperr.ErrExperimentEventInvalid
	}
	_, err = s.store.UpsertCheckpointResult(ctx, CheckpointResultDTO{
		ID: ids.Format(s.nextID()), TenantID: event.TenantID, InstanceID: pending.InstanceID, CheckpointID: pending.CheckpointID,
		JudgeTaskRef: ids.Format(event.TaskID), Passed: event.Score > 0, Score: float64(event.Score),
	})
	return err
}

// HandleJudgeFailed 处理 M3 判题失败事件并保留 0 分检查点结果。
func (s *Service) HandleJudgeFailed(ctx context.Context, event contracts.JudgeFailedEvent) error {
	pending, err := s.store.PendingCheckpointByJudgeTask(ctx, event.TenantID, event.TaskID)
	if err != nil {
		return err
	}
	if pending.SourceRef != event.SourceRef {
		return apperr.ErrExperimentEventInvalid
	}
	_, err = s.store.UpsertCheckpointResult(ctx, CheckpointResultDTO{
		ID: ids.Format(s.nextID()), TenantID: event.TenantID, InstanceID: pending.InstanceID, CheckpointID: pending.CheckpointID,
		JudgeTaskRef: ids.Format(event.TaskID), Passed: false, Score: 0, DetailRef: "judge_failed",
	})
	return err
}

// HandleSandboxRecycled 处理 M2 沙箱回收事件并标记相关实例为环境已释放。
func (s *Service) HandleSandboxRecycled(ctx context.Context, event contracts.SandboxRecycledEvent) error {
	_, err := s.store.MarkInstancesReleasedBySandbox(ctx, event.TenantID, event.SandboxID)
	return err
}

// SubscribeEvents 订阅 M3 判题事件与 M2 沙箱回收事件。
func (s *Service) SubscribeEvents() error {
	if s.bus == nil {
		return apperr.ErrExperimentEventFailed
	}
	if _, err := s.bus.Subscribe(contracts.SubjectJudgeCompleted, "experiment", s.onJudgeCompleted); err != nil {
		return apperr.ErrExperimentEventFailed.WithCause(err)
	}
	if _, err := s.bus.Subscribe(contracts.SubjectJudgeFailed, "experiment", s.onJudgeFailed); err != nil {
		return apperr.ErrExperimentEventFailed.WithCause(err)
	}
	if _, err := s.bus.Subscribe(contracts.SubjectSandboxRecycled, "experiment", s.onSandboxRecycled); err != nil {
		return apperr.ErrExperimentEventFailed.WithCause(err)
	}
	return nil
}

// onJudgeCompleted 解码判题完成事件。
func (s *Service) onJudgeCompleted(ctx context.Context, data []byte) error {
	var event contracts.JudgeCompletedEvent
	if err := eventbus.Decode(data, &event, apperr.ErrExperimentEventInvalid); err != nil {
		return err
	}
	return s.HandleJudgeCompleted(ctx, event)
}

// onJudgeFailed 解码判题失败事件。
func (s *Service) onJudgeFailed(ctx context.Context, data []byte) error {
	var event contracts.JudgeFailedEvent
	if err := eventbus.Decode(data, &event, apperr.ErrExperimentEventInvalid); err != nil {
		return err
	}
	return s.HandleJudgeFailed(ctx, event)
}

// onSandboxRecycled 解码沙箱回收事件。
func (s *Service) onSandboxRecycled(ctx context.Context, data []byte) error {
	var event contracts.SandboxRecycledEvent
	if err := eventbus.Decode(data, &event, apperr.ErrExperimentEventInvalid); err != nil {
		return err
	}
	return s.HandleSandboxRecycled(ctx, event)
}
