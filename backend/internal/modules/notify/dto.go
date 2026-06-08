// M10 DTO 定义:站内信、偏好、公告与收件箱查询的数据结构。
package notify

import "time"

const (
	// AnnouncementScopePlatform 表示全平台公告。
	AnnouncementScopePlatform int16 = 1
	// AnnouncementScopeTenant 表示当前学校公告。
	AnnouncementScopeTenant int16 = 2
	// AnnouncementScopeRole 表示指定角色公告。
	AnnouncementScopeRole int16 = 3
)

// TemplateDTO 是通知模板读取结果。
type TemplateDTO struct {
	ID              string
	Type            string
	TitleTemplate   string
	ContentTemplate string
	Channels        []string
	Force           bool
}

// NotificationCreate 是写入站内信的渲染结果。
type NotificationCreate struct {
	ID         int64
	TenantID   int64
	ReceiverID int64
	Type       string
	Title      string
	Content    string
	Link       string
}

// NotificationDTO 是收件箱展示用站内信。
type NotificationDTO struct {
	ID        string     `json:"id"`
	Type      string     `json:"type"`
	Title     string     `json:"title"`
	Content   string     `json:"content"`
	Link      string     `json:"link,omitempty"`
	IsRead    bool       `json:"is_read"`
	ReadAt    *time.Time `json:"read_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// InboxQuery 是收件箱过滤与分页条件。
type InboxQuery struct {
	Type   string
	IsRead *bool
	Page   int
	Size   int
}

// PreferenceDTO 是用户通知接收偏好。
type PreferenceDTO struct {
	Type    string `json:"type"`
	Enabled bool   `json:"enabled"`
	Force   bool   `json:"force"`
}

// PreferenceRequest 是用户更新通知接收偏好的请求项。
type PreferenceRequest struct {
	Type    string `json:"type"`
	Enabled bool   `json:"enabled"`
}

// AnnouncementRequest 是管理员发布公告的请求体。
type AnnouncementRequest struct {
	Title       string  `json:"title"`
	Content     string  `json:"content"`
	Scope       int16   `json:"scope"`
	TargetRoles []int16 `json:"target_roles"`
	ExpireAt    string  `json:"expire_at"`
}

// AnnouncementDTO 是公告列表展示对象,包含当前用户已读状态。
type AnnouncementDTO struct {
	ID          string     `json:"id"`
	TenantID    string     `json:"tenant_id,omitempty"`
	Title       string     `json:"title"`
	Content     string     `json:"content"`
	Scope       int16      `json:"scope"`
	TargetRoles []int16    `json:"target_roles,omitempty"`
	PublisherID string     `json:"publisher_id"`
	PublishedAt time.Time  `json:"published_at"`
	ExpireAt    *time.Time `json:"expire_at,omitempty"`
	IsRead      bool       `json:"is_read"`
}
