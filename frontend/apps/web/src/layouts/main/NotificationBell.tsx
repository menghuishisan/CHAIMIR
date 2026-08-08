// NotificationBell 顶栏通知铃铛(规范 §6.3):未读角标(朱砂点)+ Radix Popover 下拉,
// 下拉内容为「最近通知 + 系统公告 + 查看全部」;数据全部来自 M10 notify,前端不自建通知存储。
// 面板只在打开时拉取(关闭状态不发请求),点条目跳转对应链接并就地标记已读。

import { useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router'
import { Bell, Megaphone } from 'lucide-react'
import type { Announcement, Notification } from '@chaimir/api-client'
import {
  Button,
  Empty,
  Icon,
  IconButton,
  Popover,
  PopoverContent,
  PopoverTrigger,
  Skeleton,
  cn,
} from '@chaimir/ui'
import { api } from '../../app/api'
import { invalidateAppResource, subscribeAppResource } from '../../app/resourceInvalidation'
import { useAsyncResource } from '../../hooks'
import { formatShortDateTime } from '../../utils/formatters'
import { safeInternalNavigation } from '../../utils/safeNavigation'

/** 下拉内每类条目的展示条数:面板是「最近」摘要,全量在通知中心页 */
const RECENT_SIZE = 5

export interface NotificationBellProps {
  /** 通知中心页路径(角色区内) */
  allPath: string
}

/**
 * NotificationBell 组合角标按钮与下拉面板;未读数与面板数据分别读取,
 * 未读数在关闭态也需要(角标常显),面板数据仅在打开时读。
 */
export function NotificationBell({ allPath }: NotificationBellProps) {
  const [open, setOpen] = useState(false)
  const unread = useAsyncResource(() => api.notify.getUnreadCount(), [], () => false)
  // 首次读取完成前没有未读数,此时铃铛不带角标
  const unreadCount = unread.data ? unread.data.unread : 0

  // 通知中心页标记已读后角标须同步:壳层与页面不在同一渲染树,经资源失效协议通知
  useEffect(() => subscribeAppResource('notification-unread', unread.reload), [unread.reload])

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <span className="relative inline-flex">
          <IconButton
            variant="on-dark"
            icon={Bell}
            aria-label={unreadCount > 0 ? `通知,${unreadCount} 条未读` : '通知'}
          />
          {unreadCount > 0 ? (
            // 朱砂点:深底上的点睛,数字由 aria-label 与面板承载,点本身对读屏隐藏
            <span
              aria-hidden="true"
              className="pointer-events-none absolute right-1 top-1 size-2 rounded-full bg-seal"
            />
          ) : null}
        </span>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-80 p-0">
        <NotificationPanel allPath={allPath} onNavigate={() => setOpen(false)} />
      </PopoverContent>
    </Popover>
  )
}

interface NotificationPanelProps {
  allPath: string
  onNavigate: () => void
}

/**
 * NotificationPanel 拉取最近通知与公告;三态齐全(载/空/错),错误给报障编号与重试。
 */
function NotificationPanel({ allPath, onNavigate }: NotificationPanelProps) {
  const navigate = useNavigate()
  const recent = useAsyncResource(
    () =>
      Promise.all([
        api.notify.getNotifications({ page: 1, size: RECENT_SIZE }),
        api.notify.getAnnouncements({ page: 1, size: RECENT_SIZE }),
      ]).then(([notifications, announcements]) => ({
        notifications: notifications.list,
        announcements: announcements.list,
      })),
    [],
    (value) => value.notifications.length === 0 && value.announcements.length === 0,
  )

  /** openNotification 标记已读后跳转;无链接的通知只标记已读 */
  const openNotification = useCallback(
    async (item: Notification) => {
      if (!item.is_read) {
        try {
          await api.notify.markAsRead(item.id)
          // 未读数同时显示在角标与通知中心页:只经资源失效协议广播,不再另设回调双轨
          invalidateAppResource('notification-unread')
        } catch {
          // 标记已读失败不阻断阅读:用户仍可打开通知内容,未读数下次刷新自然纠正
        }
      }
      const safeLink = safeInternalNavigation(item.link)
      if (safeLink) {
        onNavigate()
        navigate(safeLink)
      }
    },
    [navigate, onNavigate],
  )

  return (
    <div className="flex flex-col">
      <div className="border-b border-line px-4 py-3 text-sm font-medium text-ink">通知与公告</div>

      <div className="max-h-96 overflow-y-auto">
        {recent.status === 'loading' ? (
          <div className="flex flex-col gap-3 p-4">
            <Skeleton variant="line" lines={4} />
          </div>
        ) : null}

        {recent.status === 'error' ? (
          <div className="flex flex-col items-center gap-3 px-4 py-8 text-center">
            <p className="text-sm text-ink-sub">暂时无法获取通知,请稍后重试。</p>
            {recent.error?.traceId ? (
              <p className="font-mono text-xs text-ink-faint">
                如需帮助,请提供编号 {recent.error.traceId}
              </p>
            ) : null}
            <Button variant="outline" size="sm" onClick={recent.reload}>
              重新加载
            </Button>
          </div>
        ) : null}

        {recent.status === 'empty' ? (
          <Empty icon={Bell} title="暂无通知" description="有新的通知或公告时会显示在这里。" />
        ) : null}

        {recent.status === 'success' && recent.data ? (
          <>
            {recent.data.notifications.map((item) => (
              <button
                key={item.id}
                type="button"
                onClick={() => void openNotification(item)}
                className="pressable flex w-full flex-col items-start gap-0.5 border-b border-line px-4 py-3 text-left last:border-b-0 hover:bg-surface-hover focus-visible:outline-2 focus-visible:outline-accent focus-visible:-outline-offset-2"
              >
                <span className="flex w-full items-center gap-2">
                  {!item.is_read ? (
                    <span aria-hidden="true" className="size-1.5 shrink-0 rounded-full bg-seal" />
                  ) : null}
                  <span
                    className={cn(
                      'min-w-0 flex-1 truncate text-sm',
                      item.is_read ? 'text-ink-sub' : 'font-medium text-ink',
                    )}
                  >
                    {item.title}
                  </span>
                  {!item.is_read ? <span className="sr-only">未读</span> : null}
                </span>
                <span className="line-clamp-2 text-xs text-ink-sub">{item.content}</span>
                <span className="font-mono text-xs text-ink-faint">
                  {formatShortDateTime(item.created_at)}
                </span>
              </button>
            ))}

            {recent.data.announcements.length > 0 ? (
              <div className="border-t border-line bg-surface-sunken px-4 py-2 text-xs font-medium text-ink-sub">
                系统公告
              </div>
            ) : null}
            {recent.data.announcements.map((item) => (
              <AnnouncementRow key={item.id} item={item} />
            ))}
          </>
        ) : null}
      </div>

      <div className="border-t border-line p-2">
        <Button
          variant="ghost"
          size="sm"
          className="w-full"
          onClick={() => {
            onNavigate()
            navigate(allPath)
          }}
        >
          查看全部
        </Button>
      </div>
    </div>
  )
}

/**
 * AnnouncementRow 展示单条公告;公告只读不跳转,展开阅读在通知中心页。
 */
function AnnouncementRow({ item }: { item: Announcement }) {
  return (
    <div className="flex flex-col gap-0.5 border-b border-line px-4 py-3 last:border-b-0">
      <span className="flex items-center gap-2 text-sm font-medium text-ink">
        <Icon icon={Megaphone} size="xs" className="shrink-0 text-primary" />
        <span className="min-w-0 flex-1 truncate">{item.title}</span>
      </span>
      <span className="line-clamp-2 text-xs text-ink-sub">{item.content}</span>
      <span className="font-mono text-xs text-ink-faint">
        {formatShortDateTime(item.published_at)}
      </span>
    </div>
  )
}
