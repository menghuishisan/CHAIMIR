// M10 行转换:把 notify 自有表的 sqlc 行模型转换为服务层 DTO。
package notify

import (
	"chaimir/internal/modules/notify/internal/sqlcgen"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/timex"
)

// templateDTOFromRow 转换通知模板行。
func templateDTOFromRow(row sqlcgen.NotificationTemplate) TemplateDTO {
	return TemplateDTO{
		ID:              ids.Format(row.ID),
		Type:            row.Type,
		TitleTemplate:   row.TitleTpl,
		ContentTemplate: row.ContentTpl,
		Channels:        row.Channels,
		Force:           row.Force,
	}
}

// notificationDTOFromRow 转换站内信行。
func notificationDTOFromRow(row sqlcgen.Notification) NotificationDTO {
	return NotificationDTO{
		ID:        ids.Format(row.ID),
		Type:      row.Type,
		Title:     row.Title,
		Content:   row.Content,
		Link:      textValue(row.Link),
		IsRead:    row.IsRead,
		ReadAt:    timex.PtrFromTimestamptz(row.ReadAt),
		CreatedAt: timex.FromTimestamptz(row.CreatedAt),
	}
}

// notificationDTOsFromRows 批量转换站内信行。
func notificationDTOsFromRows(rows []sqlcgen.Notification) []NotificationDTO {
	out := make([]NotificationDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, notificationDTOFromRow(row))
	}
	return out
}

// preferenceDTOFromRow 转换偏好查询行。
func preferenceDTOFromRow(row sqlcgen.ListPreferencesRow) PreferenceDTO {
	return PreferenceDTO{Type: row.Type, Enabled: row.Enabled, Force: row.Force}
}

// preferenceDTOsFromRows 批量转换偏好查询行。
func preferenceDTOsFromRows(rows []sqlcgen.ListPreferencesRow) []PreferenceDTO {
	out := make([]PreferenceDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, preferenceDTOFromRow(row))
	}
	return out
}

// announcementDTOFromRow 转换公告行。
func announcementDTOFromRow(row sqlcgen.SystemAnnouncement) AnnouncementDTO {
	return AnnouncementDTO{
		ID:          ids.Format(row.ID),
		TenantID:    optionalID(row.TenantID),
		Title:       row.Title,
		Content:     row.Content,
		Scope:       row.Scope,
		TargetRoles: row.TargetRoles,
		PublisherID: ids.Format(row.PublisherID),
		PublishedAt: timex.FromTimestamptz(row.PublishedAt),
		ExpireAt:    timex.PtrFromTimestamptz(row.ExpireAt),
	}
}

// announcementDTOFromListRow 转换公告列表行并保留已读状态。
func announcementDTOFromListRow(row sqlcgen.ListAnnouncementsRow) AnnouncementDTO {
	dto := AnnouncementDTO{
		ID:          ids.Format(row.ID),
		TenantID:    optionalID(row.TenantID),
		Title:       row.Title,
		Content:     row.Content,
		Scope:       row.Scope,
		TargetRoles: row.TargetRoles,
		PublisherID: ids.Format(row.PublisherID),
		PublishedAt: timex.FromTimestamptz(row.PublishedAt),
		ExpireAt:    timex.PtrFromTimestamptz(row.ExpireAt),
		IsRead:      row.IsRead,
	}
	return dto
}

// announcementDTOsFromListRows 批量转换公告列表行。
func announcementDTOsFromListRows(rows []sqlcgen.ListAnnouncementsRow) []AnnouncementDTO {
	out := make([]AnnouncementDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, announcementDTOFromListRow(row))
	}
	return out
}
