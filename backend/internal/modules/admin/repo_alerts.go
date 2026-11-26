// 持久化告警规则与事件，锁定持续状态并维护通知租约。
package admin

import (
	"context"
	"fmt"
	"time"

	"chaimir/internal/modules/admin/internal/sqlcgen"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/textx"
)

// ListEvaluationRules 仅供受控后台任务读取所有已启用规则。
func (t *txStore) ListEvaluationRules(ctx context.Context) ([]AlertRuleDTO, error) {
	rows, err := t.q.ListEvaluationRules(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]AlertRuleDTO, 0, len(rows))
	for _, row := range rows {
		item, err := alertRuleDTO(row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// EvaluateAlert 同事务保存持续状态和事件，规则更新与其他副本不能插入重复事件。
func (t *txStore) EvaluateAlert(ctx context.Context, o AlertObservation) error {
	rule, err := t.q.LockEvaluationRule(ctx, o.Rule.ID.Int64())
	if err != nil {
		return err
	}
	if !rule.Enabled || rule.Revision != o.Rule.revision {
		return nil
	}
	if rule.Scope == ScopeTenant && pgtypex.Int8Value(rule.TenantID) != o.TenantID {
		return fmt.Errorf("alert rule tenant mismatch")
	}
	if err := t.q.EnsureAlertEvaluation(ctx, sqlcgen.EnsureAlertEvaluationParams{RuleID: rule.ID, TenantID: o.TenantID}); err != nil {
		return err
	}
	state, err := t.q.LockAlertEvaluation(ctx, sqlcgen.LockAlertEvaluationParams{RuleID: rule.ID, TenantID: o.TenantID})
	if err != nil {
		return err
	}
	next, fire, err := nextAlertEvaluation(state, o)
	if err != nil {
		return err
	}
	if next == nil {
		return nil
	}
	if err := t.q.SaveAlertEvaluation(ctx, *next); err != nil {
		return err
	}
	if !fire {
		return nil
	}
	message := textx.TruncateRunes(fmt.Sprintf("%s：监测值 %g 已达到告警条件，请检查相关业务。", rule.Name, o.Value), 500)
	if _, err := t.CreateAlertEvent(ctx, o.EventID, rule.ID, o.TenantID, rule.Level, message); err != nil {
		return err
	}
	return t.q.SetAlertNotificationTrace(ctx, sqlcgen.SetAlertNotificationTraceParams{ID: o.EventID, NotifyTraceID: fmt.Sprintf("admin-alert-%d", o.EventID)})
}

// nextAlertEvaluation 保留跨重启状态；采样中断不为未触发的异常累计未知时间。
func nextAlertEvaluation(state sqlcgen.AlertEvaluation, o AlertObservation) (*sqlcgen.SaveAlertEvaluationParams, bool, error) {
	if err := validateAlertCondition(o.Rule.Metric, o.Rule.Condition); err != nil {
		return nil, false, err
	}
	last := timex.FromTimestamptz(state.ObservedAt)
	if state.Revision > o.Rule.revision || (state.Revision == o.Rule.revision && !o.At.After(last)) {
		return nil, false, nil
	}
	next := &sqlcgen.SaveAlertEvaluationParams{RuleID: o.Rule.ID.Int64(), TenantID: o.TenantID, Revision: o.Rule.revision, ObservedAt: timex.RequiredTimestamptz(o.At), BreachSince: state.BreachSince, Fired: state.Fired}
	if state.Revision != o.Rule.revision {
		next.BreachSince = timex.Timestamptz(time.Time{})
		next.Fired = false
	}
	if o.Missing {
		if !next.Fired {
			next.BreachSince = timex.Timestamptz(time.Time{})
		}
		return next, false, nil
	}
	threshold, _ := alertConditionNumber(o.Rule.Condition["threshold"])
	operator := o.Rule.Condition["operator"].(string)
	breach := operator == "gt" && o.Value > threshold || operator == "gte" && o.Value >= threshold || operator == "lt" && o.Value < threshold || operator == "lte" && o.Value <= threshold
	if !breach {
		next.BreachSince = timex.Timestamptz(time.Time{})
		next.Fired = false
		return next, false, nil
	}
	if !next.BreachSince.Valid || (!next.Fired && o.At.Sub(last) > o.MaxGap) {
		next.BreachSince = timex.RequiredTimestamptz(o.At)
	}
	duration, _ := alertConditionNumber(o.Rule.Condition["duration_seconds"])
	fire := !next.Fired && o.At.Sub(timex.FromTimestamptz(next.BreachSince)).Seconds() >= duration
	next.Fired = next.Fired || fire
	return next, fire, nil
}

// ClaimAlertNotifications 使用跳过锁定行的批量领取，租约过期后可重试。
func (t *txStore) ClaimAlertNotifications(ctx context.Context, limit, attempts int32, token int64, until time.Time) ([]AlertNotification, error) {
	rows, err := t.q.ClaimAlertNotifications(ctx, sqlcgen.ClaimAlertNotificationsParams{BatchSize: limit, MaxAttempts: attempts, LeaseToken: token, LeaseUntil: timex.RequiredTimestamptz(until)})
	if err != nil {
		return nil, err
	}
	out := make([]AlertNotification, 0, len(rows))
	for _, row := range rows {
		out = append(out, AlertNotification{ID: row.ID, TenantID: pgtypex.Int8Value(row.TenantID), LeaseToken: row.NotifyLeaseToken, Message: row.Message, TraceID: row.NotifyTraceID, CreatedAt: timex.FromTimestamptz(row.TriggeredAt)})
	}
	return out, nil
}

// FinishAlertNotification 只确认本次租约，失败原因脱敏后保留在原事件。
func (t *txStore) FinishAlertNotification(ctx context.Context, id, token int64, success bool, reason string) error {
	n, err := t.q.FinishAlertNotification(ctx, sqlcgen.FinishAlertNotificationParams{ID: id, LeaseToken: token, Success: success, LastError: reason})
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("alert notification lease lost: %d", id)
	}
	return nil
}

// CreateAlertRule 创建告警规则。
func (t *txStore) CreateAlertRule(ctx context.Context, id int64, req AlertRuleRequest) (AlertRuleDTO, error) {
	condition, err := jsonx.ObjectBytes(req.Condition, apperr.ErrAdminAlertInvalid)
	if err != nil {
		return AlertRuleDTO{}, err
	}
	row, err := t.q.CreateAlertRule(ctx, sqlcgen.CreateAlertRuleParams{ID: id, Scope: req.Scope, TenantID: pgtypex.Int8When(req.TenantID.Int64(), req.TenantID > 0), Name: req.Name, Metric: req.Metric, Condition: condition, Level: req.Level, Enabled: req.Enabled})
	if err != nil {
		return AlertRuleDTO{}, err
	}
	return alertRuleDTO(row)
}

// ListAlertRules 查询告警规则。
func (t *txStore) ListAlertRules(ctx context.Context, scope int16, tenantID int64) ([]AlertRuleDTO, error) {
	rows, err := t.q.ListAlertRules(ctx, sqlcgen.ListAlertRulesParams{Scope: scope, TenantID: pgtypex.Int8(tenantID)})
	if err != nil {
		return nil, err
	}
	out := make([]AlertRuleDTO, 0, len(rows))
	for _, row := range rows {
		item, err := alertRuleDTO(row)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// UpdateAlertRule 更新告警规则。
func (t *txStore) UpdateAlertRule(ctx context.Context, id int64, req AlertRuleRequest) (AlertRuleDTO, error) {
	condition, err := jsonx.ObjectBytes(req.Condition, apperr.ErrAdminAlertInvalid)
	if err != nil {
		return AlertRuleDTO{}, err
	}
	row, err := t.q.UpdateAlertRule(ctx, sqlcgen.UpdateAlertRuleParams{ID: id, Name: req.Name, Metric: req.Metric, Condition: condition, Level: req.Level, Enabled: req.Enabled, Scope: req.Scope, TenantID: pgtypex.Int8When(req.TenantID.Int64(), req.TenantID > 0)})
	if err != nil {
		return AlertRuleDTO{}, err
	}
	return alertRuleDTO(row)
}

// CreateAlertEvent 创建告警事件。
func (t *txStore) CreateAlertEvent(ctx context.Context, id, ruleID, tenantID int64, level int16, message string) (AlertEventDTO, error) {
	row, err := t.q.CreateAlertEvent(ctx, sqlcgen.CreateAlertEventParams{ID: id, RuleID: ruleID, TenantID: pgtypex.Int8When(tenantID, tenantID > 0), Level: level, Message: message})
	if err != nil {
		return AlertEventDTO{}, err
	}
	return alertEventDTO(row), nil
}

// CountAlertEventsByStatus 按状态聚合告警(套用级别与学校范围,不套用状态)。
func (t *txStore) CountAlertEventsByStatus(ctx context.Context, level int16, tenantID int64) (pagex.FacetGroup, error) {
	rows, err := t.q.CountAlertEventsByStatus(ctx, sqlcgen.CountAlertEventsByStatusParams{Level: level, TenantID: pgtypex.Int8(tenantID)})
	if err != nil {
		return nil, err
	}
	group := pagex.FacetGroup{}
	for _, row := range rows {
		group.Add(row.Status, row.Count)
	}
	return group, nil
}

// ListAlertEvents 查询告警事件和总数。
func (t *txStore) ListAlertEvents(ctx context.Context, status, level int16, tenantID int64, page, size int) ([]AlertEventDTO, int64, error) {
	limit, offset := pagex.LimitOffset(page, size)
	rows, err := t.q.ListAlertEvents(ctx, sqlcgen.ListAlertEventsParams{Status: status, Level: level, TenantID: pgtypex.Int8(tenantID), PageOffset: offset, PageLimit: limit})
	if err != nil {
		return nil, 0, err
	}
	out := make([]AlertEventDTO, 0, len(rows))
	for _, row := range rows {
		item := alertEventDTO(row.AlertEvent)
		item.RuleName = row.RuleName
		out = append(out, item)
	}
	total, err := t.q.CountAlertEvents(ctx, sqlcgen.CountAlertEventsParams{Status: status, Level: level, TenantID: pgtypex.Int8(tenantID)})
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// HandleAlertEvent 处理告警事件。
func (t *txStore) HandleAlertEvent(ctx context.Context, id, tenantID int64, status int16, handlerID int64) (AlertEventDTO, error) {
	row, err := t.q.HandleAlertEvent(ctx, sqlcgen.HandleAlertEventParams{ID: id, Status: status, HandlerID: pgtypex.Int8(handlerID), TenantID: pgtypex.Int8When(tenantID, tenantID > 0)})
	if err != nil {
		return AlertEventDTO{}, err
	}
	return alertEventDTO(row), nil
}
