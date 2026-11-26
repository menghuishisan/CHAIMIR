import { ApiClient, encodePathSegment } from '../client'
import type { PaginatedResponse } from '../types/common'
import type {
  Notification,
  NotificationPreference,
  Announcement,
  AnnouncementRequest,
} from '../types/notify'

export class NotifyApi {
  constructor(private client: ApiClient) {}

  async getNotifications(params?: {
    type?: string
    is_read?: boolean
    page?: number
    size?: number
  }): Promise<PaginatedResponse<Notification>> {
    return this.client.get('/notify/inbox', params)
  }

  async getUnreadCount(): Promise<{ unread: number }> {
    return this.client.get('/notify/inbox/unread-count')
  }

  async markAsRead(notificationId: string): Promise<Notification> {
    return this.client.post(`/notify/inbox/${encodePathSegment(notificationId)}/read`)
  }

  async markAllAsRead(): Promise<void> {
    return this.client.post('/notify/inbox/read-all')
  }

  async deleteNotification(notificationId: string): Promise<Notification> {
    return this.client.delete(`/notify/inbox/${encodePathSegment(notificationId)}`)
  }

  /** 返回后端配置的全部通知类型；未设置时默认启用。 */
  async getPreferences(): Promise<NotificationPreference[]> {
    return this.client.get('/notify/preferences')
  }

  async updatePreference(type: string, data: { enabled: boolean }): Promise<NotificationPreference> {
    return this.client.put('/notify/preferences', { type, enabled: data.enabled })
  }

  async getAnnouncements(params?: { page?: number; size?: number }): Promise<PaginatedResponse<Announcement>> {
    return this.client.get('/notify/announcements', params)
  }

  async createAnnouncement(data: AnnouncementRequest): Promise<Announcement> {
    return this.client.post('/notify/announcements', data)
  }

  async markAnnouncementRead(announcementId: string): Promise<void> {
    return this.client.post(`/notify/announcements/${encodePathSegment(announcementId)}/read`)
  }
}
