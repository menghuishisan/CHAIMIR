// 课程生命周期与结课回收请求的持久投递。
package teaching

import (
	"context"
	"errors"
	"fmt"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/intx"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/logging"
)

// AdvanceCourseStatusesOnce 推进到期课程，再投递已经提交的结课回收请求。
func (s *Service) AdvanceCourseStatusesOnce(ctx context.Context, now time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if now.IsZero() {
		now = timex.Now()
	}
	limit, ok := intx.Int32(s.cfg.CourseLifecycleBatchSize)
	if !ok || limit <= 0 {
		return fmt.Errorf("课程生命周期批大小无效")
	}
	err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		return tx.AdvanceCourseStatuses(ctx, now, limit)
	})
	if err != nil {
		err = fmt.Errorf("推进课程生命周期失败: %w", err)
	}
	return errors.Join(err, s.RunCourseEndedEventsOnce(ctx))
}

// RunCourseEndedEventsOnce 在短事务内逐条发送待发请求；消息确认后的提交失败允许重投。
func (s *Service) RunCourseEndedEventsOnce(ctx context.Context) error {
	limit, ok := intx.Int32(s.cfg.CourseLifecycleBatchSize)
	if !ok || limit <= 0 {
		return fmt.Errorf("课程生命周期批大小无效")
	}
	for i := int32(0); i < limit; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		var event contracts.TeachingCourseEndedEvent
		if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
			var err error
			event, err = tx.ClaimNextCourseEndedEvent(ctx)
			if isNoRows(err) {
				return nil
			}
			if err != nil {
				return fmt.Errorf("领取课程回收事件失败: %w", err)
			}
			event.TraceID = logging.TraceFromContext(ctx)
			if err := s.bus.Publish(ctx, contracts.SubjectTeachingCourseEnded, event); err != nil {
				return fmt.Errorf("发布课程回收事件失败: tenant_id=%d course_id=%d: %w", event.TenantID, event.CourseID, err)
			}
			return tx.MarkCourseEndedEventPublished(ctx, event.TenantID, event.CourseID)
		}); err != nil {
			return errors.Join(fmt.Errorf("投递课程回收事件失败: tenant_id=%d course_id=%d: %w", event.TenantID, event.CourseID, err), ctx.Err())
		}
		if event.CourseID == 0 {
			break
		}
	}
	return ctx.Err()
}
