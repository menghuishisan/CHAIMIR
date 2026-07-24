// NotificationsPage 展示当前账号的站内通知和系统公告，数据来自 notify 后端模块。

import React, { useCallback, useMemo, useState } from 'react'
import type { Announcement, ApiError, Notification, PaginatedResponse } from '@chaimir/api-client'
import { Button, Callout, useConfirm, ResourceState } from '@chaimir/ui'
import { Bell, Check, CheckCheck, Megaphone, RefreshCw, Settings, Trash2 } from 'lucide-react'
import { useLocation } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { invalidateAppResource } from '../../../../../app/resourceInvalidation'
import { useAsyncResource, useTicketedWebSocket } from '../../../../../hooks'
import styles from '../shared.module.css'
import { announcementScopeLabel, formatDateTime, notificationTypeLabel } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'
import { NotificationPreferences } from './NotificationPreferences'

type NotificationTab = 'notifications' | 'announcements' | 'preferences'

const PAGE_SIZE = 20


/**
 * isAnnouncementResponseEmpty 判断公告分页响应是否为空。
 */
function isAnnouncementResponseEmpty(value: PaginatedResponse<Announcement>): boolean {
  return value.list.length === 0
}

/**
 * isNotificationResponseEmpty 判断通知分页响应是否为空。
 */
function isNotificationResponseEmpty(value: PaginatedResponse<Notification>): boolean {
  return value.list.length === 0
}

/**
 * PlatformAnnouncementsPage 只读取平台身份可见的全局公告。
 */
const PlatformAnnouncementsPage: React.FC = () => {
  const announcements = useAsyncResource(
    () => api.notify.getAnnouncements({ page: 1, size: PAGE_SIZE }),
    [],
    isAnnouncementResponseEmpty
  )
  const announcementItems = useMemo(() => announcements.data?.list || [], [announcements.data])

  return (
    <div className={styles.page}>
      <div className={styles.breadcrumb}>共享功能 / 系统公告</div>

      <div className={styles.header}>
        <h1 className={styles.title}>
          <Megaphone className={styles.titleIcon} size={28} />
          系统公告
        </h1>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={announcements.reload}>
          刷新
        </Button>
      </div>

      {announcements.status === 'loading' && <ResourceState status="loading" title="正在获取公告" />}
      {announcements.status === 'error' && (
        <ResourceState status="error" error={announcements.error} onRetry={announcements.reload} />
      )}
      {announcements.status === 'empty' && (
        <ResourceState status="empty" title="暂无公告" description="当前没有需要查看的系统公告。" />
      )}
      {announcements.status === 'success' && (
        <div className={styles.list}>
          {announcementItems.map((item) => (
            <article
              className={`${styles.card} ${styles.notification} ${item.is_read ? '' : styles.notificationUnread}`}
              key={item.id}
            >
              <Megaphone className={styles.titleIcon} size={20} aria-hidden="true" />
              <div className={styles.cardMain}>
                <h2 className={styles.cardTitle}>{item.title}</h2>
                <div className={styles.meta}>
                  <span>{announcementScopeLabel(item.scope)}</span>
                  <span>{formatDateTime(item.published_at)}</span>
                </div>
                <p className={styles.content}>{item.content}</p>
              </div>
            </article>
          ))}
        </div>
      )}
    </div>
  )
}

/**
 * TenantNotificationsPage 读取租户账号的站内通知和系统公告，并支持一键标记站内信已读。
 */
const TenantNotificationsPage: React.FC = () => {
  const confirm = useConfirm()
  const [activeTab, setActiveTab] = useState<NotificationTab>('notifications')
  const [actionError, setActionError] = useState<ApiError | null>(null)
  const [actionMessage, setActionMessage] = useState<string | null>(null)
  const [markingAll, setMarkingAll] = useState(false)
  const [busyItemId, setBusyItemId] = useState<string | null>(null)

  const notifications = useAsyncResource(
    () => api.notify.getNotifications({ page: 1, size: PAGE_SIZE }),
    [],
    isNotificationResponseEmpty
  )
  const announcements = useAsyncResource(
    () => api.notify.getAnnouncements({ page: 1, size: PAGE_SIZE }),
    [],
    isAnnouncementResponseEmpty
  )
  const me = useAsyncResource(() => api.identity.getMe(), [])
  const realtimeTopic = me.data
    ? `tenant:${me.data.account.tenant_id}:notify:${me.data.account.id}`
    : null
  const realtimeSubscription = useMemo(
    () => realtimeTopic ? { action: 'subscribe', topics: [realtimeTopic] } : undefined,
    [realtimeTopic],
  )
  const handleRealtimeMessage = useCallback(() => {
    notifications.reload()
  }, [notifications])
  const realtime = useTicketedWebSocket({
    url: realtimeTopic ? api.eventWebSocketUrl() : null,
    subscribeMessage: realtimeSubscription,
    onMessage: handleRealtimeMessage,
  })

  const currentStatus = activeTab === 'notifications' ? notifications.status : announcements.status
  const currentError = activeTab === 'notifications' ? notifications.error : announcements.error
  const currentReload = activeTab === 'notifications' ? notifications.reload : announcements.reload
  const notificationItems = useMemo(() => notifications.data?.list || [], [notifications.data])
  const announcementItems = useMemo(() => announcements.data?.list || [], [announcements.data])

  const handleMarkAllRead = useCallback(async () => {
    setActionError(null)
    setActionMessage(null)
    setMarkingAll(true)
    try {
      await api.notify.markAllAsRead()
      setActionMessage('全部通知已标记为已读。')
      invalidateAppResource('notification-unread')
      notifications.reload()
    } catch (error) {
      setActionError({ message: userFacingErrorMessage(error, '标记通知失败，请稍后重试。') })
    } finally {
      setMarkingAll(false)
    }
  }, [notifications])

  /** runItemAction 执行单条通知或公告动作并刷新对应列表。 */
  const runItemAction = useCallback(async (id: string, action: () => Promise<unknown>, success: string, reload: () => void) => {
    setBusyItemId(id)
    setActionError(null)
    setActionMessage(null)
    try {
      await action()
      setActionMessage(success)
      invalidateAppResource('notification-unread')
      reload()
    } catch (actionFailure) {
      setActionError({ message: userFacingErrorMessage(actionFailure, '操作未完成，请稍后重试。') })
    } finally {
      setBusyItemId(null)
    }
  }, [])

  return (
    <div className={styles.page}>
      <div className={styles.breadcrumb}>共享功能 / 通知公告</div>

      <div className={styles.header}>
        <h1 className={styles.title}>
          <Bell className={styles.titleIcon} size={28} />
          站内信件与系统公告
        </h1>
        <div className={styles.cardActions}>
          <span className={styles.connectionStatus} role="status">实时连接：{realtime.status === 'open' ? '已连接' : realtime.status === 'reconnecting' ? '正在重连' : '未连接'}</span>
          <Button variant="outline" icon={<RefreshCw size={16} />} onClick={currentReload}>刷新</Button>
        </div>
      </div>

      <div className={styles.tabs} role="tablist" aria-label="通知公告类型">
        <button
          className={`${styles.tab} ${activeTab === 'notifications' ? styles.tabActive : ''}`}
          type="button"
          role="tab"
          aria-selected={activeTab === 'notifications'}
          onClick={() => setActiveTab('notifications')}
        >
          我的通知
        </button>
        <button
          className={`${styles.tab} ${activeTab === 'announcements' ? styles.tabActive : ''}`}
          type="button"
          role="tab"
          aria-selected={activeTab === 'announcements'}
          onClick={() => setActiveTab('announcements')}
        >
          系统公告
        </button>
        <button
          className={`${styles.tab} ${activeTab === 'preferences' ? styles.tabActive : ''}`}
          type="button"
          role="tab"
          aria-selected={activeTab === 'preferences'}
          onClick={() => setActiveTab('preferences')}
        >
          <Settings size={15} /> 通知偏好
        </button>
        {activeTab === 'notifications' && (
          <Button
            variant="ghost"
            size="sm"
            icon={<CheckCheck size={16} />}
            loading={markingAll}
            onClick={handleMarkAllRead}
          >
            全部标记已读
          </Button>
        )}
      </div>

      {actionError && (
        <ResourceState status="error" error={actionError} onRetry={() => setActionError(null)} title="操作未完成" />
      )}
      {actionMessage && <Callout variant="success" title="操作完成">{actionMessage}</Callout>}

      {activeTab !== 'preferences' && currentStatus === 'loading' && <ResourceState status="loading" title="正在获取消息" />}
      {activeTab !== 'preferences' && currentStatus === 'error' && (
        <ResourceState status="error" error={currentError} onRetry={currentReload} />
      )}
      {activeTab !== 'preferences' && currentStatus === 'empty' && (
        <ResourceState status="empty" title="暂无消息" description="当前没有需要查看的通知或公告。" />
      )}
      {currentStatus === 'success' && activeTab === 'notifications' && (
        <div className={styles.list}>
          {notificationItems.map((item) => (
            <article
              className={`${styles.card} ${styles.notification} ${item.is_read ? '' : styles.notificationUnread}`}
              key={item.id}
            >
              <div className={styles.cardMain}>
                <h2 className={styles.cardTitle}>{item.title}</h2>
                <div className={styles.meta}>
                  <span>{notificationTypeLabel(item.type)}</span>
                  <span>{item.is_read ? '已读' : '未读'}</span>
                  <span>{formatDateTime(item.created_at)}</span>
                </div>
                <p className={styles.content}>{item.content}</p>
              </div>
              <div className={styles.cardActions}>
                {!item.is_read && (
                  <Button size="sm" variant="outline" icon={<Check size={14} />} loading={busyItemId === item.id} onClick={() => void runItemAction(item.id, () => api.notify.markAsRead(item.id), '通知已标记为已读。', notifications.reload)}>标记已读</Button>
                )}
                <Button size="sm" variant="ghost" icon={<Trash2 size={14} />} loading={busyItemId === item.id} onClick={async () => {
                  const confirmed = await confirm({ title: '删除通知', description: `删除“${item.title}”后将不能再次查看。`, confirmLabel: '确认删除' })
                  if (confirmed) await runItemAction(item.id, () => api.notify.deleteNotification(item.id), '通知已删除。', notifications.reload)
                }}>删除</Button>
              </div>
            </article>
          ))}
        </div>
      )}
      {currentStatus === 'success' && activeTab === 'announcements' && (
        <div className={styles.list}>
          {announcementItems.map((item) => (
            <article
              className={`${styles.card} ${styles.notification} ${item.is_read ? '' : styles.notificationUnread}`}
              key={item.id}
            >
              <Megaphone className={styles.titleIcon} size={20} aria-hidden="true" />
              <div className={styles.cardMain}>
                <h2 className={styles.cardTitle}>{item.title}</h2>
                <div className={styles.meta}>
                  <span>{announcementScopeLabel(item.scope)}</span>
                  <span>{item.is_read ? '已读' : '未读'}</span>
                  <span>{formatDateTime(item.published_at)}</span>
                </div>
                <p className={styles.content}>{item.content}</p>
              </div>
              {!item.is_read && (
                <Button size="sm" variant="outline" icon={<Check size={14} />} loading={busyItemId === item.id} onClick={() => void runItemAction(item.id, () => api.notify.markAnnouncementRead(item.id), '公告已标记为已读。', announcements.reload)}>标记已读</Button>
              )}
            </article>
          ))}
        </div>
      )}
      {activeTab === 'preferences' && <NotificationPreferences />}
    </div>
  )
}

/**
 * NotificationsPage 根据身份所在路由选择真实可用的通知数据边界。
 */
const NotificationsPage: React.FC = () => {
  const location = useLocation()
  if (location.pathname.startsWith('/platform-admin')) {
    return <PlatformAnnouncementsPage />
  }
  return <TenantNotificationsPage />
}

export default NotificationsPage
