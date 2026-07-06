// ===== M10 Notify 模块 =====

import type { UserRole } from '../constants/identity'
import type { AnnouncementScope } from '../constants/notify'

export interface Notification {
  id: string
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
}

export interface Announcement {
  id: string
  tenant_id?: string
  title: string
  content: string
  scope: AnnouncementScope
  target_roles?: UserRole[]
  publisher_id: string
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
