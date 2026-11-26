// 校验 M9 公开输入与外接监控地址。
package admin

import (
	"encoding/json"
	"math"
	"net/url"
	"strconv"
	"strings"

	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
)

// alertMetricSet 是 M9 告警规则允许保存的指标键集合;规则触发器与前端选项共用这份契约。
var alertMetricSet = map[string]struct{}{
	"sandbox.active_count":       {},
	"sandbox.pending_seconds":    {},
	"sandbox.failed_rate":        {},
	"judge.queue_length":         {},
	"judge.failed_rate":          {},
	"account.login_failed_count": {},
	"transfer.failed_count":      {},
	"grade.warning_count":        {},
}

// alertConditionOperatorSet 是告警条件允许的比较运算符集合。
var alertConditionOperatorSet = map[string]struct{}{
	"gt":  {},
	"gte": {},
	"lt":  {},
	"lte": {},
}

// ParseMonitoringPanels 校验受控外部监控入口及允许的来源。
func ParseMonitoringPanels(raw string, allowedOrigins []string) ([]MonitoringPanel, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, apperr.ErrAdminMonitoringInvalid
	}
	var panels []MonitoringPanel
	if err := jsonx.DecodeStrictKnownFields([]byte(raw), &panels); err != nil {
		return nil, apperr.ErrAdminMonitoringInvalid.WithCause(err)
	}
	for i := range panels {
		panels[i].Name = strings.TrimSpace(panels[i].Name)
		panels[i].URL = strings.TrimSpace(panels[i].URL)
		if panels[i].Name == "" || !safePanelURL(panels[i].URL, allowedOrigins) {
			return nil, apperr.ErrAdminMonitoringInvalid
		}
	}
	return panels, nil
}

// safePanelURL 校验面板 URL 仅含 HTTPS scheme/host/path,不携带令牌类信息。
func safePanelURL(raw string, allowedOrigins []string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	for _, allowed := range allowedOrigins {
		origin, parseErr := url.Parse(strings.TrimSpace(allowed))
		if parseErr == nil && strings.EqualFold(origin.Scheme, u.Scheme) && strings.EqualFold(origin.Host, u.Host) {
			return true
		}
	}
	return false
}

// validateScopeTenant 校验全局/租户范围与 tenant_id 的对应关系。
func validateScopeTenant(scope int16, tenantID int64) error {
	switch scope {
	case ScopeGlobal:
		if tenantID != 0 {
			return apperr.ErrAdminAlertInvalid
		}
	case ScopeTenant:
		if tenantID <= 0 {
			return apperr.ErrAdminAlertInvalid
		}
	default:
		return apperr.ErrAdminAlertInvalid
	}
	return nil
}

// validateDateRange 校验统计查询必须使用闭区间自然日范围。
func validateDateRange(fromDate, toDate string) error {
	fromDate = strings.TrimSpace(fromDate)
	toDate = strings.TrimSpace(toDate)
	if fromDate == "" || toDate == "" {
		return apperr.ErrAdminStatisticsInvalid
	}
	from, fromErr := timex.ParseDate(fromDate)
	to, toErr := timex.ParseDate(toDate)
	if fromErr != nil || toErr != nil || to.Before(from) {
		return apperr.ErrAdminStatisticsInvalid
	}
	return nil
}

// validateAlertCondition 校验告警规则 condition 的唯一 JSON 结构。
func validateAlertCondition(metric string, condition map[string]any) error {
	if _, ok := alertMetricSet[metric]; !ok || condition == nil {
		return apperr.ErrAdminAlertInvalid
	}
	for key := range condition {
		switch key {
		case "operator", "threshold", "duration_seconds":
		default:
			return apperr.ErrAdminAlertInvalid
		}
	}
	operator, ok := condition["operator"].(string)
	if !ok {
		return apperr.ErrAdminAlertInvalid
	}
	if _, ok := alertConditionOperatorSet[operator]; !ok {
		return apperr.ErrAdminAlertInvalid
	}
	threshold, ok := alertConditionNumber(condition["threshold"])
	if !ok || !finiteAlertConditionNumber(threshold) {
		return apperr.ErrAdminAlertInvalid
	}
	duration, ok := alertConditionNumber(condition["duration_seconds"])
	if !ok || !finiteAlertConditionNumber(duration) || duration < 0 || math.Trunc(duration) != duration {
		return apperr.ErrAdminAlertInvalid
	}
	return nil
}

// alertConditionNumber 统一处理 JSON 解码后的数字类型,不改变条件契约本身。
func alertConditionNumber(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		parsed, err := strconv.ParseFloat(string(typed), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

// finiteAlertConditionNumber 拒绝 NaN 和无穷值,避免不可持久化或不可比较的阈值进入规则表。
func finiteAlertConditionNumber(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

// validateAlertRule 校验告警规则范围、指标、条件和等级。
func validateAlertRule(req AlertRuleRequest) error {
	if err := validateScopeTenant(req.Scope, req.TenantID.Int64()); err != nil {
		return apperr.ErrAdminAlertInvalid
	}
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Metric) != req.Metric || req.Level < AlertLevelNotice || req.Level > AlertLevelCritical {
		return apperr.ErrAdminAlertInvalid
	}
	if err := validateAlertCondition(req.Metric, req.Condition); err != nil {
		return apperr.ErrAdminAlertInvalid
	}
	return nil
}
