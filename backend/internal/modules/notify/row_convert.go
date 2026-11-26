// 将通知查询行转换为内部模型和响应。
package notify

import (
	"chaimir/internal/modules/notify/internal/sqlcgen"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
)

// templateFromRow 转换模板行。
func templateFromRow(row sqlcgen.NotificationTemplate) notificationTemplate {
	return notificationTemplate{Type: row.Type, TitleTpl: row.TitleTpl, ContentTpl: row.ContentTpl, Force: row.Force}
}

// notificationDTO 转换通知行。
func notificationDTO(row sqlcgen.Notification) NotificationDTO {
	return NotificationDTO{ID: ids.ID(row.ID), Type: row.Type, Title: row.Title, Content: row.Content, Link: pgtypex.TextValue(row.Link), IsRead: row.IsRead, ReadAt: timex.PtrFromTimestamptz(row.ReadAt), CreatedAt: timex.FromTimestamptz(row.CreatedAt)}
}

// announcementDTO 转换公告行。
func announcementDTO(row sqlcgen.SystemAnnouncement, isRead bool) AnnouncementDTO {
	return AnnouncementDTO{ID: ids.ID(row.ID), TenantID: ids.ID(pgtypex.Int8Value(row.TenantID)), Title: row.Title, Content: row.Content, Scope: row.Scope, TargetRoles: row.TargetRoles, PublisherID: ids.ID(row.PublisherID), PublishedAt: timex.FromTimestamptz(row.PublishedAt), ExpireAt: timex.PtrFromTimestamptz(row.ExpireAt), IsRead: isRead}
}

// announcementRowDTO 转换公告列表行。
func announcementRowDTO(row sqlcgen.ListAnnouncementsRow) AnnouncementDTO {
	return AnnouncementDTO{ID: ids.ID(row.ID), TenantID: ids.ID(pgtypex.Int8Value(row.TenantID)), Title: row.Title, Content: row.Content, Scope: row.Scope, TargetRoles: row.TargetRoles, PublisherID: ids.ID(row.PublisherID), PublishedAt: timex.FromTimestamptz(row.PublishedAt), ExpireAt: timex.PtrFromTimestamptz(row.ExpireAt), IsRead: row.IsRead}
}

// announcementVisibleRowDTO 转换可见公告查询行。
func announcementVisibleRowDTO(row sqlcgen.GetVisibleAnnouncementRow) AnnouncementDTO {
	return AnnouncementDTO{ID: ids.ID(row.ID), TenantID: ids.ID(pgtypex.Int8Value(row.TenantID)), Title: row.Title, Content: row.Content, Scope: row.Scope, TargetRoles: row.TargetRoles, PublisherID: ids.ID(row.PublisherID), PublishedAt: timex.FromTimestamptz(row.PublishedAt), ExpireAt: timex.PtrFromTimestamptz(row.ExpireAt), IsRead: row.IsRead}
}
