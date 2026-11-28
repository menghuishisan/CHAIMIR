// 维护告警规则与事件，周期判定异常并可靠请求通知投递。
package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/intx"
	"chaimir/internal/platform/pagex"

	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
	"chaimir/pkg/textx"
)

// AlertObservation 绑定规则修订和采样时刻，旧采样不能覆盖新规则或新结果。
type AlertObservation struct {
	Rule     AlertRuleDTO
	TenantID int64
	At       time.Time
	Value    float64
	Missing  bool
	MaxGap   time.Duration
	EventID  int64
}

// AlertNotification 是告警原记录的租约快照，不复制通知业务状态。
type AlertNotification struct {
	ID, TenantID, LeaseToken int64
	Message, TraceID         string
	CreatedAt                time.Time
}

// RunAlertsOnce 独立采集各模块汇总；一个指标失败不伪装成零，也不阻塞通知重试。
func (s *Service) RunAlertsOnce(ctx context.Context) error {
	var rules []AlertRuleDTO
	err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		rules, err = tx.ListEvaluationRules(ctx)
		return err
	})
	if err != nil {
		return errors.Join(err, s.RunAlertNotificationsOnce(ctx))
	}
	var result error
	if len(rules) > 0 {
		tenants, err := s.identity.ListTenants(ctx)
		if err != nil {
			result = err
		} else {
			for _, tenant := range tenants {
				if tenant.Status != 1 {
					continue
				}
				if err := s.evaluateTenantAlerts(ctx, tenant.TenantID, rules); err != nil {
					logging.ErrorContext(ctx, "admin alert evaluation failed", apperr.AsAppError(err).LogString(), slog.Int64("tenant_id", tenant.TenantID))
					result = errors.Join(result, err)
				}
			}
		}
	}
	return errors.Join(result, s.RunAlertNotificationsOnce(ctx))
}

// evaluateTenantAlerts 用同一时点读取学校指标，再逐规则原子判定。
func (s *Service) evaluateTenantAlerts(ctx context.Context, tenantID int64, rules []AlertRuleDTO) error {
	ctx = tenant.WithContext(ctx, tenant.Identity{TenantID: tenantID})
	now := timex.Now()
	query := contracts.AlertMetricQuery{TenantID: tenantID, From: now.Add(-time.Duration(s.cfg.AlertWindowSeconds) * time.Second), To: now}
	values := map[string]float64{}
	var result error
	for _, reader := range s.metrics {
		metrics, err := reader.AlertMetrics(ctx, query)
		if err != nil {
			result = errors.Join(result, err)
			continue
		}
		for key, value := range metrics {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				result = errors.Join(result, fmt.Errorf("non-finite alert metric: %s", key))
				continue
			}
			if _, exists := values[key]; exists {
				return fmt.Errorf("duplicate alert metric owner: %s", key)
			}
			values[key] = value
		}
	}
	for _, rule := range rules {
		if rule.Scope == ScopeTenant && rule.TenantID.Int64() != tenantID {
			continue
		}
		value, ok := values[rule.Metric]
		observation := AlertObservation{Rule: rule, TenantID: tenantID, At: now, Value: value, Missing: !ok, MaxGap: 2 * time.Duration(s.cfg.AlertPollSeconds) * time.Second, EventID: s.ids.Generate()}
		if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error { return tx.EvaluateAlert(ctx, observation) }); err != nil {
			result = errors.Join(result, err)
		}
	}
	return result
}

// RunAlertNotificationsOnce 以持久租约领取通知，外部发布不占数据库事务。
func (s *Service) RunAlertNotificationsOnce(ctx context.Context) error {
	limit, ok := intx.Int32(s.cfg.AlertNotifyBatchSize)
	attempts, attemptOK := intx.Int32(s.cfg.AlertNotifyMaxAttempts)
	if !ok || !attemptOK || limit <= 0 || attempts <= 0 || s.cfg.AlertNotifyLeaseSeconds <= 0 {
		return fmt.Errorf("invalid admin alert delivery configuration")
	}
	var items []AlertNotification
	err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		items, err = tx.ClaimAlertNotifications(ctx, limit, attempts, s.ids.Generate(), timex.Now().Add(time.Duration(s.cfg.AlertNotifyLeaseSeconds)*time.Second))
		return err
	})
	if err != nil {
		return err
	}
	var result error
	for _, item := range items {
		if err := s.publishAlertNotification(ctx, item); err != nil {
			logging.ErrorContext(logging.WithTrace(ctx, item.TraceID), "admin alert delivery failed", apperr.AsAppError(err).LogString(), slog.Int64("tenant_id", item.TenantID), slog.Int64("alert_id", item.ID))
			result = errors.Join(result, err)
		}
	}
	return result
}

// publishAlertNotification 重试使用同一请求编号；失去租约不能确认新 worker 的结果。
func (s *Service) publishAlertNotification(ctx context.Context, item AlertNotification) error {
	ctx = logging.WithTrace(ctx, item.TraceID)
	receivers, sendErr := s.recipients.AlertRecipients(ctx, item.TenantID)
	if sendErr == nil && len(receivers) == 0 {
		sendErr = fmt.Errorf("tenant has no active alert recipients: %d", item.TenantID)
	}
	if sendErr == nil {
		sendErr = s.bus.Publish(ctx, contracts.SubjectNotifySendRequested, contracts.NotifySendRequestedEvent{RequestID: fmt.Sprintf("admin:%04d:alert:%d", item.CreatedAt.Year(), item.ID), TenantID: item.TenantID, TraceID: item.TraceID, Type: "system.alert", Receivers: receivers, Params: map[string]string{"message": item.Message}, Link: "/school-admin/system-alerts"})
	}
	reason := ""
	if sendErr != nil {
		reason = textx.TruncateRunes(logging.SanitizeError(apperr.AsAppError(sendErr).LogString()), 255)
	}
	markErr := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		return tx.FinishAlertNotification(ctx, item.ID, item.LeaseToken, sendErr == nil, reason)
	})
	return errors.Join(sendErr, markErr)
}

// ListAlertRules 查询告警规则。
func (s *Service) ListAlertRules(ctx context.Context, scope int16) ([]AlertRuleDTO, error) {
	id, err := s.currentAdminIdentity(ctx)
	if err != nil {
		return nil, err
	}
	tenantID := int64(0)
	if id.IsPlatform {
		if scope == 0 {
			scope = ScopeGlobal
		}
		if scope != ScopeGlobal {
			return nil, apperr.ErrAdminAlertInvalid
		}
	} else {
		scope = ScopeTenant
		tenantID = id.TenantID
	}
	return runAdminRead(ctx, s.store, tenantID, func(ctx context.Context, tx TxStore) ([]AlertRuleDTO, error) {
		return tx.ListAlertRules(ctx, scope, tenantID)
	})
}

// CreateAlertRule 创建告警规则。
func (s *Service) CreateAlertRule(ctx context.Context, req AlertRuleRequest) (AlertRuleDTO, error) {
	id, err := s.currentAdminIdentity(ctx)
	if err != nil {
		return AlertRuleDTO{}, err
	}
	if !id.IsPlatform {
		req.Scope = ScopeTenant
		req.TenantID = ids.ID(id.TenantID)
	}
	if id.IsPlatform && req.Scope == ScopeTenant {
		return AlertRuleDTO{}, apperr.ErrAdminAlertInvalid
	}
	if err := validateAlertRule(req); err != nil {
		return AlertRuleDTO{}, err
	}
	var out AlertRuleDTO
	err = s.runWrite(ctx, req.TenantID.Int64(), func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.CreateAlertRule(ctx, s.ids.Generate(), req)
		if err != nil {
			return apperr.ErrAdminAlertInvalid.WithCause(err)
		}
		return s.writeAudit(ctx, id, "admin.alert_rule.create", "alert_rule", out.ID.Int64(), map[string]any{"scope": out.Scope, "tenant_id": out.TenantID.String(), "metric": out.Metric, "level": out.Level})
	})
	if err != nil {
		return AlertRuleDTO{}, err
	}
	return out, nil
}

// UpdateAlertRule 更新告警规则。
func (s *Service) UpdateAlertRule(ctx context.Context, ruleID int64, req AlertRuleRequest) (AlertRuleDTO, error) {
	id, err := s.currentAdminIdentity(ctx)
	if err != nil {
		return AlertRuleDTO{}, err
	}
	if !id.IsPlatform {
		req.Scope = ScopeTenant
		req.TenantID = ids.ID(id.TenantID)
	}
	if id.IsPlatform && req.Scope == ScopeTenant {
		return AlertRuleDTO{}, apperr.ErrAdminAlertInvalid
	}
	if err := validateAlertRule(req); err != nil {
		return AlertRuleDTO{}, err
	}
	var out AlertRuleDTO
	err = s.runWrite(ctx, req.TenantID.Int64(), func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.UpdateAlertRule(ctx, ruleID, req)
		if err != nil {
			return apperr.ErrAdminAlertNotFound.WithCause(err)
		}
		return s.writeAudit(ctx, id, "admin.alert_rule.update", "alert_rule", out.ID.Int64(), map[string]any{"scope": out.Scope, "tenant_id": out.TenantID.String(), "metric": out.Metric, "level": out.Level})
	})
	if err != nil {
		return AlertRuleDTO{}, err
	}
	return out, nil
}

// ListAlertEvents 查询告警事件。
func (s *Service) ListAlertEvents(ctx context.Context, status, level int16, page, size int) ([]AlertEventDTO, int64, int, int, pagex.Facets, error) {
	id, err := s.currentAdminIdentity(ctx)
	if err != nil {
		return nil, 0, page, size, nil, err
	}
	page, size = pagex.Normalize(page, size)
	tenantID := int64(0)
	if !id.IsPlatform {
		tenantID = id.TenantID
	}
	var total int64
	var byStatus pagex.FacetGroup
	rows, err := runAdminRead(ctx, s.store, tenantID, func(ctx context.Context, tx TxStore) ([]AlertEventDTO, error) {
		out, count, err := tx.ListAlertEvents(ctx, status, level, tenantID, page, size)
		if err != nil {
			return nil, err
		}
		total = count
		// 状态分段计数:套用级别与学校范围,不套用状态筛选
		byStatus, err = tx.CountAlertEventsByStatus(ctx, level, tenantID)
		return out, err
	})
	return rows, total, page, size, pagex.Facets{"status": byStatus}, err
}

// HandleAlertEvent 处理告警事件。
func (s *Service) HandleAlertEvent(ctx context.Context, eventID int64, req AlertEventRequest) (AlertEventDTO, error) {
	id, err := s.currentAdminIdentity(ctx)
	if err != nil {
		return AlertEventDTO{}, err
	}
	if req.Status != AlertStatusHandled && req.Status != AlertStatusIgnored {
		return AlertEventDTO{}, apperr.ErrAdminAlertStateInvalid
	}
	var out AlertEventDTO
	err = s.runWrite(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.HandleAlertEvent(ctx, eventID, id.TenantID, req.Status, id.AccountID)
		if err != nil {
			return apperr.ErrAdminAlertNotFound.WithCause(err)
		}
		return s.writeAudit(ctx, id, "admin.alert.handle", "alert_event", out.ID.Int64(), map[string]any{"status": out.Status})
	})
	if err != nil {
		return AlertEventDTO{}, err
	}
	if out.TenantID > 0 {
		// 状态推送在提交后发送，发布失败记日志；站内信另由事件原记录持久重试。
		payload, marshalErr := json.Marshal(AlertEventPushDTO{EventID: out.ID, RuleID: out.RuleID, Level: out.Level, Status: out.Status, HandlerID: out.HandlerID})
		if marshalErr != nil {
			logging.ErrorContext(ctx, "admin alert notify serialization failed", marshalErr.Error(), slog.Int64("tenant_id", out.TenantID.Int64()), slog.Int64("alert_event_id", out.ID.Int64()))
		} else {
			evt := contracts.NotifyPushRequestedEvent{
				TenantID: out.TenantID.Int64(),
				TraceID:  logging.TraceFromContext(ctx),
				Topic:    "tenant:" + ids.Format(out.TenantID.Int64()) + ":alert",
				Payload:  payload,
			}
			if pubErr := s.bus.Publish(ctx, contracts.SubjectNotifyPushRequested, evt); pubErr != nil {
				logging.ErrorContext(ctx, "admin alert notify publish failed", pubErr.Error(), slog.Int64("tenant_id", out.TenantID.Int64()), slog.Int64("alert_event_id", out.ID.Int64()))
			}
		}
	}
	return out, nil
}
