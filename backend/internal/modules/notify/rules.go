// notify rules 文件集中实现 M10 输入校验、模板渲染和实时主题授权。
package notify

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/pkg/apperr"
)

var (
	tenantTopicPrefixPattern = regexp.MustCompile(`^tenant:([1-9][0-9]*):`)
	businessTopicPattern     = regexp.MustCompile(`^tenant:([1-9][0-9]*):(contest|sandbox|sim|experiment|course|judge):[1-9][0-9]*:[a-z][a-z0-9_-]*$`)
	notifyTopicPattern       = regexp.MustCompile(`^tenant:([1-9][0-9]*):notify:([1-9][0-9]*)$`)
	templateVarPattern       = regexp.MustCompile(`\{\{[a-zA-Z0-9_.-]+\}\}`)
	linkPattern              = regexp.MustCompile(`^/[A-Za-z0-9/_?&=.#:%+-]*$`)
)

// AuthorizeTopic 校验实时 topic 语法和 M10 可独立判断的租户/个人边界。
func AuthorizeTopic(tenantID, accountID int64, topic string) error {
	topic = strings.TrimSpace(topic)
	if tenantID <= 0 || accountID <= 0 || topic == "" {
		return apperr.ErrNotifySubscribeInvalid
	}
	if want := personalNotifyTopic(tenantID, accountID); strings.Contains(topic, ":notify:") {
		if topic != want {
			return apperr.ErrNotifyTopicForbidden
		}
		return nil
	}
	if want := tenantAlertTopic(tenantID); strings.HasSuffix(topic, ":alert") {
		if topic != want {
			return apperr.ErrNotifyTopicForbidden
		}
		return nil
	}
	if businessTopicTenantID(topic) == tenantID {
		return nil
	}
	if parsed := topicTenantID(topic); parsed > 0 && parsed != tenantID {
		return apperr.ErrNotifyTopicForbidden
	}
	return apperr.ErrNotifySubscribeInvalid
}

// ValidatePushTopic 校验内部推送 topic 语法和 M10 可独立判断的租户边界。
func ValidatePushTopic(tenantID int64, topic string) error {
	topic = strings.TrimSpace(topic)
	if tenantID <= 0 || topic == "" {
		return apperr.ErrNotifySubscribeInvalid
	}
	if matches := notifyTopicPattern.FindStringSubmatch(topic); len(matches) == 3 {
		if topicTenantID(topic) != tenantID {
			return apperr.ErrNotifyTopicForbidden
		}
		accountID, err := strconv.ParseInt(matches[2], 10, 64)
		if err != nil || accountID <= 0 {
			return apperr.ErrNotifySubscribeInvalid
		}
		return nil
	}
	if want := tenantAlertTopic(tenantID); strings.HasSuffix(topic, ":alert") {
		if topic != want {
			return apperr.ErrNotifyTopicForbidden
		}
		return nil
	}
	if businessTopicTenantID(topic) == tenantID {
		return nil
	}
	if parsed := topicTenantID(topic); parsed > 0 && parsed != tenantID {
		return apperr.ErrNotifyTopicForbidden
	}
	return apperr.ErrNotifySubscribeInvalid
}

// personalNotifyTopic 生成当前租户下个人红点 topic,避免账号 ID 单独暴露为跨租户通道名。
func personalNotifyTopic(tenantID, accountID int64) string {
	return fmt.Sprintf("tenant:%d:notify:%d", tenantID, accountID)
}

// tenantAlertTopic 生成当前租户告警 topic。
func tenantAlertTopic(tenantID int64) string {
	return fmt.Sprintf("tenant:%d:alert", tenantID)
}

// topicTenantID 解析所有 M10 统一实时 topic 的租户前缀,不命中时返回 0。
func topicTenantID(topic string) int64 {
	matches := tenantTopicPrefixPattern.FindStringSubmatch(strings.TrimSpace(topic))
	if len(matches) != 2 {
		return 0
	}
	tenantID, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return 0
	}
	return tenantID
}

// businessTopicTenantID 解析统一业务实时 topic 中的租户前缀,不命中时返回 0。
func businessTopicTenantID(topic string) int64 {
	matches := businessTopicPattern.FindStringSubmatch(strings.TrimSpace(topic))
	if len(matches) != 3 {
		return 0
	}
	tenantID, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return 0
	}
	return tenantID
}

// validateSendRequest 校验内部通知发送请求。
func validateSendRequest(req SendRequest) (SendRequest, error) {
	req.Type = strings.TrimSpace(req.Type)
	req.Link = strings.TrimSpace(req.Link)
	if req.TenantID <= 0 || req.Type == "" || len(req.Receivers) == 0 {
		return SendRequest{}, apperr.ErrNotifyRequestInvalid
	}
	if req.Link != "" && !linkPattern.MatchString(req.Link) {
		return SendRequest{}, apperr.ErrNotifyRequestInvalid
	}
	if req.Params == nil {
		req.Params = map[string]string{}
	}
	seen := map[int64]struct{}{}
	out := req.Receivers[:0]
	for _, id := range req.Receivers {
		if id <= 0 {
			return SendRequest{}, apperr.ErrNotifyRequestInvalid
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	req.Receivers = out
	return req, nil
}

// validateAnnouncementRequest 校验公告发布请求和平台/租户边界。
func validateAnnouncementRequest(req AnnouncementRequest, isPlatform bool) error {
	if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Content) == "" {
		return apperr.ErrNotifyAnnouncementInvalid
	}
	switch req.Scope {
	case AnnouncementScopePlatform:
		if !isPlatform {
			return apperr.ErrForbidden
		}
	case AnnouncementScopeTenant:
		if len(req.TargetRoles) != 0 {
			return apperr.ErrNotifyAnnouncementInvalid
		}
	case AnnouncementScopeRoles:
		if len(req.TargetRoles) == 0 {
			return apperr.ErrNotifyAnnouncementInvalid
		}
		for _, role := range req.TargetRoles {
			if contracts.RoleCode(role) == "unknown" {
				return apperr.ErrNotifyAnnouncementInvalid
			}
		}
	default:
		return apperr.ErrNotifyAnnouncementInvalid
	}
	return nil
}

// renderNotificationTemplate 渲染标题和正文,并拒绝缺失变量的模板输出。
func renderNotificationTemplate(tpl notificationTemplate, params map[string]string) (string, string, error) {
	title := renderTemplate(tpl.TitleTpl, params)
	content := renderTemplate(tpl.ContentTpl, params)
	if templateVarPattern.MatchString(title) || templateVarPattern.MatchString(content) {
		return "", "", apperr.ErrNotifyTemplateUnavailable
	}
	return title, content, nil
}

// renderTemplate 用参数替换 {{key}} 模板变量。
func renderTemplate(tpl string, params map[string]string) string {
	out := tpl
	for key, value := range params {
		out = strings.ReplaceAll(out, "{{"+key+"}}", value)
	}
	return out
}
