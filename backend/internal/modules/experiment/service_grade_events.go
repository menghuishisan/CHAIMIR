// experiment service_grade_events 文件负责课程绑定与得分的唯一事务outbox生产、领取和发布。
package experiment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/intx"
	"chaimir/internal/platform/response"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
)

// experimentMaximumScore 由真实检查点和应交报告计算满分；协作实例按实际成员报告数量汇总。
func experimentMaximumScore(exp Experiment, memberCount int) float64 {
	total := 0.0
	for _, checkpoint := range exp.Components.Checkpoints {
		total += checkpoint.Score
	}
	if exp.RequireReport {
		total += 100 * float64(memberCount)
	}
	return total
}

// enqueueExperimentSourceEvent 与定义写入同事务固化绑定快照，下架也发送同一来源的失效修订。
func (s *Service) enqueueExperimentSourceEvent(ctx context.Context, tx TxStore, exp Experiment) error {
	eventID := s.ids.Generate()
	maxScore := experimentMaximumScore(exp, 1)
	event := contracts.ExperimentCourseSourceChangedEvent{EventID: ids.Format(eventID), TenantID: exp.TenantID, TraceID: response.TraceFromContext(ctx), ExperimentID: exp.ID, ExperimentName: exp.Name, CourseID: exp.CourseID, SourceRevision: exp.UpdatedAt.UnixMicro(), Available: exp.Status == ExperimentStatusPublished, MaxScore: maxScore}
	if math.IsNaN(maxScore) || math.IsInf(maxScore, 0) || maxScore < 0 || event.TraceID == "" || event.SourceRevision <= 0 {
		return apperr.ErrExperimentEventFailed
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return apperr.ErrExperimentEventFailed.WithCause(err)
	}
	_, err = tx.CreateExperimentEventOutbox(ctx, ExperimentEventOutbox{ID: eventID, TenantID: exp.TenantID, ExperimentID: exp.ID, AggregateID: exp.ID, EventRevision: event.SourceRevision, TraceID: event.TraceID, OccurredAt: exp.UpdatedAt, Subject: contracts.SubjectExperimentCourseSourceChanged, Payload: payload})
	return err
}

// enqueueExperimentEventOutbox 在实例锁内固化本次原始得分、课程和实际成员，SQL生成递增得分修订。
func (s *Service) enqueueExperimentEventOutbox(ctx context.Context, tx TxStore, inst ExperimentInstance) error {
	traceID := strings.TrimSpace(response.TraceFromContext(ctx))
	if inst.TenantID <= 0 || inst.ExperimentID <= 0 || inst.ID <= 0 || inst.OwnerAccountID <= 0 || traceID == "" {
		return apperr.ErrExperimentEventFailed
	}
	exp, err := tx.GetExperiment(ctx, inst.TenantID, inst.ExperimentID)
	if err != nil {
		return apperr.ErrExperimentEventFailed.WithCause(err)
	}
	students := []int64{inst.OwnerAccountID}
	if inst.GroupID > 0 {
		members, err := tx.ListGroupMembers(ctx, inst.TenantID, inst.GroupID)
		if err != nil {
			return apperr.ErrExperimentEventFailed.WithCause(err)
		}
		seen := map[int64]bool{inst.OwnerAccountID: true}
		for _, member := range members {
			if !seen[member.StudentID] {
				students = append(students, member.StudentID)
				seen[member.StudentID] = true
			}
		}
	}
	pending, err := tx.HasPendingInstanceReports(ctx, inst.TenantID, inst.ID)
	if err != nil {
		return apperr.ErrExperimentEventFailed.WithCause(err)
	}
	eventID := s.ids.Generate()
	occurredAt := timex.Now()
	event := contracts.ExperimentScoredEvent{EventID: ids.Format(eventID), TenantID: inst.TenantID, TraceID: traceID, ExperimentID: exp.ID, ExperimentName: exp.Name, InstanceID: inst.ID, StudentID: inst.OwnerAccountID, StudentIDs: students, Pending: pending, Score: inst.Score, ScoredAt: occurredAt, CourseID: exp.CourseID, SourceRevision: exp.UpdatedAt.UnixMicro(), Available: exp.Status == ExperimentStatusPublished, SourceMaxScore: experimentMaximumScore(exp, 1), MaxScore: experimentMaximumScore(exp, len(students))}
	payload, err := json.Marshal(event)
	if err != nil {
		return apperr.ErrExperimentEventFailed.WithCause(err)
	}
	_, err = tx.CreateExperimentEventOutbox(ctx, ExperimentEventOutbox{ID: eventID, TenantID: inst.TenantID, ExperimentID: exp.ID, AggregateID: inst.ID, TraceID: traceID, OccurredAt: occurredAt, Subject: contracts.SubjectExperimentScored, Payload: payload})
	return err
}

// RunExperimentEventOutboxOnce 通过同一租约任务发布绑定与得分，不在事务提交后重组业务事实。
func (s *Service) RunExperimentEventOutboxOnce(ctx context.Context) error {
	limit, ok := intx.Int32(s.cfg.EventOutboxBatchSize)
	maxAttempts, valid := intx.Int32(s.cfg.EventOutboxMaxAttempts)
	if !ok || !valid || limit <= 0 || maxAttempts <= 0 {
		return apperr.ErrExperimentEventFailed
	}
	now := timex.Now()
	var items []ExperimentEventOutbox
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		items, err = tx.ClaimPendingExperimentEventOutbox(ctx, limit, maxAttempts, now, now.Add(time.Duration(s.cfg.EventOutboxStaleMs)*time.Millisecond))
		return err
	}); err != nil {
		return apperr.ErrExperimentEventFailed.WithCause(err)
	}
	var failures []error
	for _, item := range items {
		if err := s.publishExperimentEvent(ctx, item); err != nil {
			failures = append(failures, fmt.Errorf("发布实验事件失败: tenant_id=%d outbox_id=%d subject=%s: %w", item.TenantID, item.ID, item.Subject, err))
		}
	}
	return errors.Join(failures...)
}

// publishExperimentEvent 按原载荷发布并受租约保护回写状态，失败链不被回写错误覆盖。
func (s *Service) publishExperimentEvent(ctx context.Context, item ExperimentEventOutbox) error {
	eventCtx := response.WithTrace(ctx, item.TraceID)
	var publishErr error
	if item.Subject != contracts.SubjectExperimentScored && item.Subject != contracts.SubjectExperimentCourseSourceChanged || !json.Valid(item.Payload) {
		publishErr = apperr.ErrExperimentEventFailed.WithCause(fmt.Errorf("实验outbox主题或载荷无效: outbox_id=%d subject=%s", item.ID, item.Subject))
	} else {
		publishErr = s.bus.Publish(eventCtx, item.Subject, json.RawMessage(item.Payload))
	}
	if publishErr != nil {
		markErr := s.store.PrivilegedTx(eventCtx, func(ctx context.Context, tx TxStore) error {
			_, err := tx.MarkExperimentEventOutboxFailed(ctx, item.TenantID, item.ID, logging.SanitizeError(publishErr.Error()), item.LeaseToken)
			return err
		})
		return apperr.ErrExperimentEventFailed.WithCause(errors.Join(publishErr, markErr))
	}
	return s.store.PrivilegedTx(eventCtx, func(ctx context.Context, tx TxStore) error {
		_, err := tx.MarkExperimentEventOutboxPublished(ctx, item.TenantID, item.ID, item.LeaseToken)
		return err
	})
}

// drainExperimentEventOutboxBestEffort 在请求提交后尽快投递，失败保留给同一后台worker重试。
func (s *Service) drainExperimentEventOutboxBestEffort(ctx context.Context) {
	if err := s.RunExperimentEventOutboxOnce(ctx); err != nil {
		logging.ErrorContext(ctx, "experiment event outbox drain failed", apperr.AsAppError(err).LogString())
	}
}
