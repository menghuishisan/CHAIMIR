// ===== M10 Notify 模块 =====

import type { UserRole } from '../constants/identity'
import type { AnnouncementScope } from '../constants/notify'
import type { SnowflakeID } from './common'

export interface Notification {
  id: SnowflakeID
  type: string
  title: string
  content: string
  link?: string
  is_read: boolean
  read_at?: string
  created_at: string
}

export interface NotificationPreference {
  type: string
  enabled: boolean
  /** 强制类通知不允许关闭（由后端通知模板声明），界面渲染为不可操作并说明原因。 */
  force: boolean
}

export interface Announcement {
  id: SnowflakeID
  tenant_id?: SnowflakeID
  title: string
  content: string
  scope: AnnouncementScope
  target_roles?: UserRole[]
  publisher_id: SnowflakeID
  published_at: string
  expire_at?: string
  is_read: boolean
}

export interface AnnouncementRequest {
  title: string
  content: string
  scope: AnnouncementScope
  target_roles: UserRole[]
  expire_at?: string
}
