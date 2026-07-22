// notify model 文件定义 M10 通知、公告和实时主题响应模型。
package notify

import (
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
