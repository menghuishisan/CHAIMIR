package notify

import (
	"encoding/json"
	"time"

	"chaimir/internal/platform/ids"
)

// NotificationDTO 是站内信响应。
type NotificationDTO struct {
	ID        ids.ID     `json:"id"`
	Type      string     `json:"type"`
	Title     string     `json:"title"`
	Content   string     `json:"content"`
	Link      string     `json:"link,omitempty"`
	IsRead    bool       `json:"is_read"`
	ReadAt    *time.Time `json:"read_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// AnnouncementDTO 是系统公告响应。
type AnnouncementDTO struct {
	ID          ids.ID     `json:"id"`
	TenantID    ids.ID     `json:"tenant_id,omitempty"`
	Title       string     `json:"title"`
	Content     string     `json:"content"`
	Scope       int16      `json:"scope"`
	TargetRoles []int16    `json:"target_roles,omitempty"`
	PublisherID ids.ID     `json:"publisher_id"`
	PublishedAt time.Time  `json:"published_at"`
	ExpireAt    *time.Time `json:"expire_at,omitempty"`
	IsRead      bool       `json:"is_read"`
}

// SendRequest 是内部通知发送请求。
type SendRequest struct {
	RequestID string            `json:"request_id,omitempty"`
	TenantID  ids.ID            `json:"tenant_id"`
	Type      string            `json:"type"`
	Receivers []int64           `json:"receivers"`
	Params    map[string]string `json:"params"`
	Link      string            `json:"link"`
}

// PushRequest 是内部实时推送请求。
type PushRequest struct {
	TenantID ids.ID          `json:"tenant_id"`
	Topic    string          `json:"topic"`
	Payload  json.RawMessage `json:"payload"`
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

// PreferenceDTO 返回用户偏好；模板正文和投递通道仅供服务端使用。
type PreferenceDTO struct {
	Type    string `json:"type"`
	Enabled bool   `json:"enabled"`
	Force   bool   `json:"force"` // 强制通知不允许关闭。
}

// UnreadDTO 表示站内信未读数量响应。
type UnreadDTO struct {
	Unread int64 `json:"unread"`
}
