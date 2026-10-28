// grade service_warning_dispatch 从预警原记录可靠投递通知,重试沿用同一业务请求编号。
package grade

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/intx"
	"chaimir/internal/platform/response"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	pkgcrypto "chaimir/pkg/crypto"
	"chaimir/pkg/logging"
	"chaimir/pkg/textx"
)

// runWarningNotificationsOnce 使用原M11任务预算领取预警通知;过期租约可重新领取。
func (s *Service) runWarningNotificationsOnce(ctx context.Context) error {
	limit, ok := intx.Int32(s.cfg.LockOutboxBatchSize)
	if !ok || limit <= 0 || s.cfg.LockOutboxStaleMs <= 0 {
		return apperr.ErrGradeEventPublishFailed
	}
	token, err := pkgcrypto.RandomToken(48)
	if err != nil {
		return apperr.ErrGradeEventPublishFailed.WithCause(err)
	}
	now := timex.Now()
	var items []WarningNotification
	err = s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		items, err = tx.ClaimGradeWarningNotifications(ctx, limit, now, token, now.Add(time.Duration(s.cfg.LockOutboxStaleMs)*time.Millisecond))
		return err
	})
	if err != nil {
		return apperr.ErrGradeEventPublishFailed.WithCause(err)
	}
	var result error
	for _, item := range items {
		if err := s.publishWarningNotification(ctx, item); err != nil {
			logging.ErrorContext(response.WithTrace(ctx, item.TraceID), "grade warning dispatch failed", apperr.ErrGradeEventPublishFailed.WithCause(err).LogString(), slog.String("trace_id", item.TraceID), slog.Int64("tenant_id", item.TenantID), slog.Int64("warning_id", item.ID))
			result = errors.Join(result, err)
		}
	}
	return result
}

// publishWarningNotification 发布原预警事件,投递或确认失败均保留持久记录供重试。
func (s *Service) publishWarningNotification(ctx context.Context, item WarningNotification) error {
	err := s.sendWarningNotification(ctx, item)
	if err == nil {
		return s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
			count, err := tx.MarkGradeWarningNotificationPublished(ctx, item.TenantID, item.ID, item.LeaseToken)
			if err != nil {
				return apperr.ErrGradeEventPublishFailed.WithCause(err)
			}
			if count != 1 {
				return apperr.ErrGradeEventPublishFailed.WithCause(fmt.Errorf("预警通知发布确认已失去租约: warning_id=%d", item.ID))
			}
			return nil
		})
	}
	markErr := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		count, markErr := tx.MarkGradeWarningNotificationFailed(ctx, item.TenantID, item.ID, item.LeaseToken, textx.TruncateRunes(logging.SanitizeError(apperr.ErrGradeEventPublishFailed.WithCause(err).LogString()), 255), timex.Now().Add(time.Duration(s.cfg.LockOutboxStaleMs)*time.Millisecond))
		if markErr != nil {
			return markErr
		}
		if count != 1 {
			return fmt.Errorf("预警通知失败回写已失去租约: warning_id=%d", item.ID)
		}
		return nil
	})
	return apperr.ErrGradeEventPublishFailed.WithCause(errors.Join(err, markErr))
}

// sendWarningNotification 固定创建年份和预警ID作为M10幂等键,不以重试时刻生成新身份。
func (s *Service) sendWarningNotification(ctx context.Context, item WarningNotification) error {
	gpa, err := strconv.ParseFloat(item.GPAText, 64)
	if err != nil || math.IsNaN(gpa) || math.IsInf(gpa, 0) || gpa < 0 || gpa > 4 || item.ID <= 0 || item.TenantID <= 0 || item.StudentID <= 0 || item.CreatedAt.IsZero() || strings.TrimSpace(item.TraceID) == "" || strings.TrimSpace(item.LeaseToken) == "" {
		return apperr.ErrGradeEventPublishFailed.WithCause(errors.Join(fmt.Errorf("预警通知持久载荷无效: warning_id=%d", item.ID), err))
	}
	evt := contracts.NotifySendRequestedEvent{
		RequestID: fmt.Sprintf("grade:%04d:warning:%d", item.CreatedAt.Year(), item.ID),
		TenantID:  item.TenantID, TraceID: item.TraceID, Type: "grade.warning",
		Receivers: []int64{item.StudentID}, Params: map[string]string{"gpa": fmt.Sprintf("%.3f", gpa)},
		Link: "/student/alerts",
	}
	return s.bus.Publish(response.WithTrace(ctx, item.TraceID), contracts.SubjectNotifySendRequested, evt)
}
