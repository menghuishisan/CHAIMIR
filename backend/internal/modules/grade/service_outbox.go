// 投递成绩锁事件并维护有界重试与发布租约。
package grade

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/intx"
	"chaimir/internal/platform/response"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
	"chaimir/pkg/textx"
)

// enqueueLockOutbox 在状态变更同一事务内保存 M6 锁定投影事件。
func (s *Service) enqueueLockOutbox(ctx context.Context, tx TxStore, review ReviewDTO, locked bool, reason string) error {
	_, err := s.createLockOutbox(ctx, tx, review, locked, reason)
	return err
}

// createLockOutbox 保存本次权威锁修订,供同事务申诉精确引用因果边界。
func (s *Service) createLockOutbox(ctx context.Context, tx TxStore, review ReviewDTO, locked bool, reason string) (GradeLockOutbox, error) {
	traceID := strings.TrimSpace(response.TraceFromContext(ctx))
	if review.TenantID <= 0 || review.ID <= 0 || review.CourseID <= 0 || traceID == "" || strings.TrimSpace(reason) == "" {
		return GradeLockOutbox{}, apperr.ErrGradeEventPublishFailed
	}
	out, err := tx.CreateGradeLockOutbox(ctx, s.ids.Generate(), review, locked, reason, traceID)
	if err != nil {
		return GradeLockOutbox{}, apperr.ErrGradeEventPublishFailed.WithCause(err)
	}
	return out, nil
}

// RunOutboxesOnce 投递M11成绩锁事件与预警通知,共用原后台任务和配置预算。
func (s *Service) RunOutboxesOnce(ctx context.Context) error {
	limit, ok := intx.Int32(s.cfg.LockOutboxBatchSize)
	if !ok || limit <= 0 {
		return apperr.ErrGradeEventPublishFailed
	}
	now := timex.Now()
	staleBefore := now
	leaseUntil := now.Add(time.Duration(s.cfg.LockOutboxStaleMs) * time.Millisecond)
	var items []GradeLockOutbox
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		maxAttempts, ok := intx.Int32(s.cfg.LockOutboxMaxAttempts)
		if !ok || maxAttempts <= 0 {
			return apperr.ErrGradeEventPublishFailed
		}
		replayDelay := int32(s.cfg.LockOutboxStaleMs / 1000)
		if replayDelay < 60 {
			replayDelay = 60
		}
		if _, err = tx.RequeueExhaustedGradeLockOutbox(ctx, replayDelay); err != nil {
			return apperr.ErrGradeEventPublishFailed.WithCause(err)
		}
		items, err = tx.ClaimPendingGradeLockOutbox(ctx, limit, maxAttempts, staleBefore, leaseUntil)
		if err != nil {
			return apperr.ErrGradeEventPublishFailed.WithCause(err)
		}
		return nil
	}); err != nil {
		return err
	}
	var firstErr error
	for _, item := range items {
		if err := s.publishLockOutboxItem(ctx, item); err != nil {
			logging.ErrorContext(response.WithTrace(ctx, item.TraceID), "grade lock outbox publish failed", apperr.AsAppError(err).LogString(), slog.Int64("tenant_id", item.TenantID), slog.Int64("review_id", item.ReviewID), slog.Int64("outbox_id", item.ID))
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return errors.Join(firstErr, s.runWarningNotificationsOnce(ctx))
}

// publishLockOutboxItem 发布单条锁定事件并按结果回写 outbox 状态。
func (s *Service) publishLockOutboxItem(ctx context.Context, item GradeLockOutbox) error {
	if item.Revision <= 0 || item.ChangedAt.IsZero() {
		return apperr.ErrGradeEventPublishFailed.WithCause(fmt.Errorf("成绩锁事件缺少权威修订或变更时间: outbox_id=%d", item.ID))
	}
	eventCtx := response.WithTrace(ctx, item.TraceID)
	payload := contracts.GradeReviewLockChangedEvent{TenantID: item.TenantID, TraceID: item.TraceID, ReviewID: item.ReviewID, CourseID: item.CourseID, Locked: item.Locked, Revision: item.Revision, Reason: item.Reason, ChangedAt: item.ChangedAt}
	if err := s.bus.Publish(eventCtx, contracts.SubjectGradeReviewLockChanged, payload); err != nil {
		markErr := s.recordLockOutboxFailure(eventCtx, item, err)
		return apperr.ErrGradeEventPublishFailed.WithCause(errors.Join(err, markErr))
	}
	return s.markLockOutboxPublished(eventCtx, item)
}

// markLockOutboxPublished 用特权事务标记锁定事件投递成功。
func (s *Service) markLockOutboxPublished(ctx context.Context, item GradeLockOutbox) error {
	return s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		_, err := tx.MarkGradeLockOutboxPublished(ctx, item.TenantID, item.ID, item.LeaseToken)
		if err != nil {
			return apperr.ErrGradeEventPublishFailed.WithCause(err)
		}
		return nil
	})
}

// recordLockOutboxFailure 记录锁定事件投递失败并保留脱敏原因供后台重试。
func (s *Service) recordLockOutboxFailure(ctx context.Context, item GradeLockOutbox, cause error) error {
	return s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		reason := textx.TruncateRunes(logging.SanitizeError(apperr.AsAppError(cause).LogString()), 255)
		_, err := tx.MarkGradeLockOutboxFailed(ctx, item.TenantID, item.ID, reason, item.LeaseToken)
		return err
	})
}

// drainLockOutboxBestEffort 在请求提交后尽快投递,失败只记录日志并交给后台任务补偿。
func (s *Service) drainLockOutboxBestEffort(ctx context.Context) {
	if err := s.RunOutboxesOnce(ctx); err != nil {
		logging.ErrorContext(ctx, "grade lock outbox drain failed", apperr.AsAppError(err).LogString())
	}
}
