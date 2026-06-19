// Notify API：通知与实时推送
// 对应后端 M10 模块

import { ApiClient } from '../client'
import type {
  Notification,
  NotificationPreference,
  Announcement,
  AnnouncementRequest,
  PaginatedResponse,
} from '../types'

export class NotifyApi {
  constructor(private client: ApiClient) {}

  // ===== 通知列表 =====

  /**
   * 获取我的通知列表
   */
  async getNotifications(params?: {
    type?: string
    is_read?: boolean
    page?: number
    size?: number
  }): Promise<PaginatedResponse<Notification>> {
    return this.client.get('/notify/inbox', params)
  }

  /**
   * 获取未读数量
   */
  async getUnreadCount(): Promise<{ unread: number }> {
    return this.client.get('/notify/inbox/unread-count')
  }

  /**
   * 标记为已读
   */
  async markAsRead(notificationId: string): Promise<Notification> {
    return this.client.post(`/notify/inbox/${notificationId}/read`)
  }

  /**
   * 全部标记为已读
   */
  async markAllAsRead(): Promise<void> {
    return this.client.post('/notify/inbox/read-all')
  }

  /**
   * 删除通知
   */
  async deleteNotification(notificationId: string): Promise<void> {
    return this.client.delete(`/notify/inbox/${notificationId}`)
  }

  // ===== 通知偏好 =====

  /**
   * 获取通知偏好设置
   */
  async getPreferences(): Promise<NotificationPreference[]> {
    return this.client.get('/notify/preferences')
  }

  /**
   * 更新通知偏好
   */
  async updatePreference(type: string, data: { enabled: boolean }): Promise<NotificationPreference> {
    return this.client.put('/notify/preferences', { type, enabled: data.enabled })
  }

  // ===== 公告 =====

  /**
   * 获取公告列表
   */
  async getAnnouncements(params?: { page?: number; size?: number }): Promise<PaginatedResponse<Announcement>> {
    return this.client.get('/notify/announcements', params)
  }

  /**
   * 发布公告（管理员）
   */
  async createAnnouncement(data: AnnouncementRequest): Promise<Announcement> {
    return this.client.post('/notify/announcements', data)
  }

  /**
   * 标记公告已读
   */
  async markAnnouncementRead(announcementId: string): Promise<void> {
    return this.client.post(`/notify/announcements/${announcementId}/read`)
  }

  // ===== WebSocket =====

  /**
   * 获取 WebSocket URL
   */
  getWebSocketUrl(): string {
    const baseUrl = this.client['config'].baseURL || ''
    const wsProtocol = baseUrl.startsWith('https') ? 'wss' : 'ws'
    const wsBaseUrl = baseUrl.replace(/^https?/, wsProtocol)
    return `${wsBaseUrl}/api/ws`
  }
}
