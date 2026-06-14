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
    read?: boolean
    page?: number
    size?: number
  }): Promise<PaginatedResponse<Notification>> {
    return this.client.get('/notify/notifications', params)
  }

  /**
   * 获取未读数量
   */
  async getUnreadCount(): Promise<{ unread: number }> {
    return this.client.get('/notify/notifications/unread')
  }

  /**
   * 标记为已读
   */
  async markAsRead(notificationIds: string[]): Promise<void> {
    return this.client.post('/notify/notifications/mark-read', { notification_ids: notificationIds })
  }

  /**
   * 全部标记为已读
   */
  async markAllAsRead(): Promise<void> {
    return this.client.post('/notify/notifications/mark-all-read')
  }

  /**
   * 删除通知
   */
  async deleteNotification(notificationId: string): Promise<void> {
    return this.client.delete(`/notify/notifications/${notificationId}`)
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
  async updatePreference(type: string, data: { enabled: boolean }): Promise<void> {
    return this.client.put(`/notify/preferences/${type}`, data)
  }

  // ===== 公告 =====

  /**
   * 获取公告列表
   */
  async getAnnouncements(params?: {
    scope?: number
    page?: number
    size?: number
  }): Promise<PaginatedResponse<Announcement>> {
    return this.client.get('/notify/announcements', params)
  }

  /**
   * 获取公告详情
   */
  async getAnnouncement(announcementId: string): Promise<Announcement> {
    return this.client.get(`/notify/announcements/${announcementId}`)
  }

  /**
   * 发布公告（管理员）
   */
  async createAnnouncement(data: AnnouncementRequest): Promise<Announcement> {
    return this.client.post('/notify/announcements', data)
  }

  /**
   * 更新公告
   */
  async updateAnnouncement(announcementId: string, data: AnnouncementRequest): Promise<Announcement> {
    return this.client.put(`/notify/announcements/${announcementId}`, data)
  }

  /**
   * 删除公告
   */
  async deleteAnnouncement(announcementId: string): Promise<void> {
    return this.client.delete(`/notify/announcements/${announcementId}`)
  }

  // ===== WebSocket =====

  /**
   * 获取 WebSocket URL
   */
  getWebSocketUrl(): string {
    const baseUrl = this.client['config'].baseURL || ''
    const wsProtocol = baseUrl.startsWith('https') ? 'wss' : 'ws'
    const wsBaseUrl = baseUrl.replace(/^https?/, wsProtocol)
    return `${wsBaseUrl}/ws`
  }
}
