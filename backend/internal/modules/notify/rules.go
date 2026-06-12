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

var businessTopicPattern = regexp.MustCompile(`^(contest|sandbox|sim|exp|experiment|course):[1-9][0-9]*:[a-z][a-z0-9_-]*$`)

// AuthorizeTopic 校验实时 topic 语法和 M10 可独立判断的租户/个人边界。
func AuthorizeTopic(tenantID, accountID int64, topic string) error {
	topic = strings.TrimSpace(topic)
	if tenantID <= 0 || accountID <= 0 || topic == "" {
		return apperr.ErrNotifySubscribeInvalid
	}
	if want := fmt.Sprintf("notify:%d", accountID); strings.HasPrefix(topic, "notify:") {
		if topic != want {
			return apperr.ErrNotifyTopicForbidden
		}
		return nil
	}
	if want := fmt.Sprintf("alert:%d", tenantID); strings.HasPrefix(topic, "alert:") {
		if topic != want {
			return apperr.ErrNotifyTopicForbidden
		}
		return nil
	}
	if businessTopicPattern.MatchString(topic) {
		return nil
	}
	return apperr.ErrNotifySubscribeInvalid
}

// ValidatePushTopic 校验内部推送 topic 语法和 M10 可独立判断的租户边界。
func ValidatePushTopic(tenantID int64, topic string) error {
	topic = strings.TrimSpace(topic)
	if tenantID <= 0 || topic == "" {
		return apperr.ErrNotifySubscribeInvalid
	}
	if raw, ok := strings.CutPrefix(topic, "notify:"); ok {
		accountID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || accountID <= 0 {
			return apperr.ErrNotifySubscribeInvalid
		}
		return nil
	}
	if want := fmt.Sprintf("alert:%d", tenantID); strings.HasPrefix(topic, "alert:") {
		if topic != want {
			return apperr.ErrNotifyTopicForbidden
		}
		return nil
	}
	if businessTopicPattern.MatchString(topic) {
		return nil
	}
	return apperr.ErrNotifySubscribeInvalid
}

// validateSendRequest 校验内部通知发送请求。
func validateSendRequest(req SendRequest) (SendRequest, error) {
	req.Type = strings.TrimSpace(req.Type)
	req.Link = strings.TrimSpace(req.Link)
	if req.TenantID <= 0 || req.Type == "" || len(req.Receivers) == 0 {
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

// renderTemplate 用参数替换 {{key}} 模板变量。
func renderTemplate(tpl string, params map[string]string) string {
	out := tpl
	for key, value := range params {
		out = strings.ReplaceAll(out, "{{"+key+"}}", value)
	}
	return out
}
