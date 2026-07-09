// NotificationsPage 展示当前账号的站内通知和系统公告，数据来自 notify 后端模块。

import React, { useCallback, useMemo, useState } from 'react'
import type { Announcement, ApiError, Notification, PaginatedResponse } from '@chaimir/api-client'
import { Button } from '@chaimir/ui'
import { Bell, CheckCheck, Megaphone, RefreshCw } from 'lucide-react'
import { useLocation } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { EmptyState, ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../shared.module.css'
import { formatDateTime } from '../../../../../utils/index'

type NotificationTab = 'notifications' | 'announcements'

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

      {announcements.status === 'loading' && <LoadingState title="正在获取公告" />}
      {announcements.status === 'error' && (
        <ErrorState error={announcements.error} onRetry={announcements.reload} />
      )}
      {announcements.status === 'empty' && (
        <EmptyState title="暂无公告" description="当前没有需要查看的系统公告。" />
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
                  <span>{item.scope}</span>
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
  const [activeTab, setActiveTab] = useState<NotificationTab>('notifications')
  const [actionError, setActionError] = useState<ApiError | null>(null)
  const [markingAll, setMarkingAll] = useState(false)

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

  const currentStatus = activeTab === 'notifications' ? notifications.status : announcements.status
  const currentError = activeTab === 'notifications' ? notifications.error : announcements.error
  const currentReload = activeTab === 'notifications' ? notifications.reload : announcements.reload
  const notificationItems = useMemo(() => notifications.data?.list || [], [notifications.data])
  const announcementItems = useMemo(() => announcements.data?.list || [], [announcements.data])

  const handleMarkAllRead = useCallback(async () => {
    setActionError(null)
    setMarkingAll(true)
    try {
      await api.notify.markAllAsRead()
      notifications.reload()
    } catch (error) {
      setActionError(error as ApiError)
    } finally {
      setMarkingAll(false)
    }
  }, [notifications])

  return (
    <div className={styles.page}>
      <div className={styles.breadcrumb}>共享功能 / 通知公告</div>

      <div className={styles.header}>
        <h1 className={styles.title}>
          <Bell className={styles.titleIcon} size={28} />
          站内信件与系统公告
        </h1>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={currentReload}>
          刷新
        </Button>
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
        <ErrorState error={actionError} onRetry={() => setActionError(null)} title="操作未完成" />
      )}

      {currentStatus === 'loading' && <LoadingState title="正在获取消息" />}
      {currentStatus === 'error' && (
        <ErrorState error={currentError} onRetry={currentReload} />
      )}
      {currentStatus === 'empty' && (
        <EmptyState title="暂无消息" description="当前没有需要查看的通知或公告。" />
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
                  <span>{item.type}</span>
                  <span>{item.is_read ? '已读' : '未读'}</span>
                  <span>{formatDateTime(item.created_at)}</span>
                </div>
                <p className={styles.content}>{item.content}</p>
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
                  <span>{item.scope}</span>
                  <span>{item.is_read ? '已读' : '未读'}</span>
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
