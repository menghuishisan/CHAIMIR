// notify dto 文件定义 M10 HTTP 请求结构。
package notify

import "chaimir/internal/platform/ids"

// SendRequest 是内部通知发送请求。
type SendRequest struct {
	TenantID  ids.ID            `json:"tenant_id"`
	Type      string            `json:"type"`
	Receivers []int64           `json:"receivers"`
	Params    map[string]string `json:"params"`
	Link      string            `json:"link"`
}

// PushRequest 是内部实时推送请求。
type PushRequest struct {
	TenantID ids.ID         `json:"tenant_id"`
	Topic    string         `json:"topic"`
	Payload  map[string]any `json:"payload"`
}

// PreferenceRequest 是通知偏好设置请求。
type PreferenceRequest struct {
	Type    string `json:"type"`
	Enabled bool   `json:"enabled"`
}

// AnnouncementRequest 是公告发布请求。
type AnnouncementRequest struct {
	Title       string  `json:"title"`
	Content     string  `json:"content"`
	Scope       int16   `json:"scope"`
	TargetRoles []int16 `json:"target_roles"`
	ExpireAt    string  `json:"expire_at"`
}

// SubscribeMessage 是 WebSocket 客户端订阅消息。
type SubscribeMessage struct {
	Action string   `json:"action"`
	Topics []string `json:"topics"`
}

// PreferenceDTO 表示用户通知偏好响应。
type PreferenceDTO struct {
	Type    string `json:"type"`
	Enabled bool   `json:"enabled"`
}

// UnreadDTO 表示站内信未读数量响应。
type UnreadDTO struct {
	Unread int64 `json:"unread"`
}
