// grade repo_warning_dispatch 只读写academic_warning原记录中的通知投递元数据。
package grade

import (
	"context"
	"time"

	"chaimir/internal/modules/grade/internal/sqlcgen"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
)

// ClaimGradeWarningNotifications 原子领取未投递、失败或租约超时的预警。
func (t *txStore) ClaimGradeWarningNotifications(ctx context.Context, limit int32, now time.Time, token string, until time.Time) ([]WarningNotification, error) {
	rows, err := t.q.ClaimGradeWarningNotifications(ctx, sqlcgen.ClaimGradeWarningNotificationsParams{NowAt: timex.RequiredTimestamptz(now), PageLimit: limit, LeaseToken: token, LeaseUntil: timex.RequiredTimestamptz(until)})
	if err != nil {
		return nil, err
	}
	items := make([]WarningNotification, 0, len(rows))
	for _, row := range rows {
		items = append(items, WarningNotification{ID: row.ID, TenantID: row.TenantID, StudentID: row.StudentID, SemesterID: row.SemesterID, GPAText: row.GpaText, TraceID: row.NotifyTraceID, CreatedAt: timex.FromTimestamptz(row.CreatedAt), LeaseToken: row.NotifyLeaseToken})
	}
	return items, nil
}

// MarkGradeWarningNotificationPublished 只有当前未过期租约可确认通知已持久发布。
func (t *txStore) MarkGradeWarningNotificationPublished(ctx context.Context, tenantID, warningID int64, token string) (int64, error) {
	return t.q.MarkGradeWarningNotificationPublished(ctx, sqlcgen.MarkGradeWarningNotificationPublishedParams{TenantID: tenantID, ID: warningID, NotifyLeaseToken: token})
}

// MarkGradeWarningNotificationFailed 保存脱敏原因和下次尝试时间,不影响学生确认状态。
func (t *txStore) MarkGradeWarningNotificationFailed(ctx context.Context, tenantID, warningID int64, token, reason string, next time.Time) (int64, error) {
	return t.q.MarkGradeWarningNotificationFailed(ctx, sqlcgen.MarkGradeWarningNotificationFailedParams{TenantID: tenantID, ID: warningID, NotifyLeaseToken: token, NotifyLastError: pgtypex.Text(reason), NextAt: timex.RequiredTimestamptz(next)})
}
