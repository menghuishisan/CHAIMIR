// admin rules 文件集中实现 M9 输入校验和监控入口安全规则。
package admin

import (
	"net/url"
	"strings"

	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
)

// ParseMonitoringPanels 解析并校验外接监控面板嵌入地址。
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
			return apperr.ErrAdminConfigInvalid
		}
	case ScopeTenant:
		if tenantID <= 0 {
			return apperr.ErrAdminConfigInvalid
		}
	default:
		return apperr.ErrAdminConfigInvalid
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
