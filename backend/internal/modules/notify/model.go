// notify model 文件定义 M10 通知、公告和实时主题响应模型。
package notify

import "time"

// NotificationDTO 是站内信响应。
type NotificationDTO struct {
	ID        int64      `json:"id,string"`
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
	ID          int64      `json:"id,string"`
	TenantID    int64      `json:"tenant_id,omitempty,string"`
	Title       string     `json:"title"`
	Content     string     `json:"content"`
	Scope       int16      `json:"scope"`
	TargetRoles []int16    `json:"target_roles,omitempty"`
	PublisherID int64      `json:"publisher_id,string"`
	PublishedAt time.Time  `json:"published_at"`
	ExpireAt    *time.Time `json:"expire_at,omitempty"`
	IsRead      bool       `json:"is_read"`
}

// AnnouncementListDTO 是公告分页列表响应。
type AnnouncementListDTO struct {
	List  []AnnouncementDTO `json:"list"`
	Total int64             `json:"total"`
	Page  int               `json:"page"`
	Size  int               `json:"size"`
}
