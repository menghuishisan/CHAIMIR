// NotificationBell 顶栏通知铃铛(规范 §6.3):未读角标(朱砂点)+ Radix Popover 弹出层。
// 弹出层:顶部「通知 · n 条未读」+「全部已读」,最近 5 条消息(未读圆点、标题、相对时间),底部「查看全部」。
// 公告在通知收件箱的公告标签里,不再挤进铃铛。数据全部来自 M10 notify,前端不自建通知存储;
// 弹出层只在打开时拉取(关闭状态不发请求),点条目就地标记已读并跳转对应链接。

import { useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router'
import { Bell } from 'lucide-react'
import { ApiError, type Notification } from '@chaimir/api-client'
import {
  Button,
  Empty,
  IconButton,
  ListRow,
  Popover,
  PopoverContent,
  PopoverTrigger,
  Skeleton,
  cn,
  toast,
} from '@chaimir/ui'
import { api } from '../../app/api'
import { invalidateAppResource, subscribeAppResource } from '../../app/resourceInvalidation'
import { useAsyncResource } from '../../hooks'
import { formatDateTime, formatRelativeOccurrence } from '../../utils/formatters'
import { safeInternalNavigation } from '../../utils/safeNavigation'
import { userFacingErrorMessage } from '../../utils/userFacingError'

/** 弹出层里的消息条数:这里是「最近」摘要,全量在通知收件箱 */
const RECENT_SIZE = 5

export interface NotificationBellProps {
  /** 通知收件箱路径(角色区内) */
  allPath: string
}

/**
 * NotificationBell 组合角标按钮与弹出层;未读数与弹出层数据分别读取,
 * 未读数在关闭态也需要(角标常显),弹出层数据仅在打开时读。
 */
export function NotificationBell({ allPath }: NotificationBellProps) {
  const [open, setOpen] = useState(false)
  const unread = useAsyncResource(() => api.notify.getUnreadCount(), [], () => false)
  // 首次读取完成前没有未读数,此时铃铛不带角标
  const unreadCount = unread.data ? unread.data.unread : 0

  // 通知收件箱标记已读后角标须同步:壳层与页面不在同一渲染树,经资源失效协议通知
  useEffect(() => subscribeAppResource('notification-unread', unread.reload), [unread.reload])

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <span className="relative inline-flex">
          <IconButton
            variant="on-dark"
            icon={Bell}
            aria-label={unreadCount > 0 ? `通知,${unreadCount} 条未读` : '通知'}
            className="border-transparent"
          />
          {unreadCount > 0 ? (
            // 朱砂点:深底上的点睛,数字由 aria-label 与弹出层承载,点本身对读屏隐藏
            <span
              aria-hidden="true"
              className="pointer-events-none absolute right-1 top-1 size-2 rounded-full bg-seal"
            />
          ) : null}
        </span>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-topbar-popover p-0">
        <NotificationPanel allPath={allPath} unreadCount={unreadCount} onClose={() => setOpen(false)} />
      </PopoverContent>
    </Popover>
  )
}

interface NotificationPanelProps {
  allPath: string
  unreadCount: number
  onClose: () => void
}

/**
 * NotificationPanel 拉取最近消息;三态齐全(载/空/错),错误给报障编号与重试。
 */
function NotificationPanel({ allPath, unreadCount, onClose }: NotificationPanelProps) {
  const navigate = useNavigate()
  const [markingAll, setMarkingAll] = useState(false)
  const recent = useAsyncResource(
    () => api.notify.getNotifications({ page: 1, size: RECENT_SIZE }).then((page) => page.list),
    [],
  )
  const { reload } = recent

  /** openNotification 标记已读后跳转;没有链接的通知去收件箱阅读全文 */
  const openNotification = useCallback(
    async (item: Notification) => {
      if (!item.is_read) {
        try {
          await api.notify.markAsRead(item.id)
          // 未读数同时显示在角标与收件箱:只经资源失效协议广播,不再另设回调双轨
          invalidateAppResource('notification-unread')
        } catch (error) {
          // 标记已读失败不阻断阅读,但记录脱敏诊断字段供排查;未读数下次刷新自然纠正。
          console.error('通知已读状态更新失败', {
            notification_id: item.id,
            error:
              error instanceof ApiError
                ? { code: error.code, trace_id: error.traceId }
                : { kind: error instanceof Error ? error.name : typeof error },
          })
        }
      }
      onClose()
      navigate(safeInternalNavigation(item.link) ?? allPath)
    },
    [allPath, navigate, onClose],
  )

  /** markAllRead 把全部消息标为已读;失败时就近提示,列表保持原样 */
  const markAllRead = useCallback(async () => {
    setMarkingAll(true)
    try {
      await api.notify.markAllAsRead()
      invalidateAppResource('notification-unread')
      reload()
    } catch (error) {
      toast.error(userFacingErrorMessage(error, '暂时没能全部标为已读,请稍后重试。'))
    } finally {
      setMarkingAll(false)
    }
  }, [reload])

  return (
    <div className="flex flex-col">
      <div className="flex items-center justify-between gap-3 border-b border-line px-4 py-2.5">
        <p className="text-sm font-semibold text-ink">
          通知
          {unreadCount > 0 ? <span className="ml-1.5 font-normal text-ink-sub">· {unreadCount} 条未读</span> : null}
        </p>
        {unreadCount > 0 ? (
          <Button variant="ghost" size="sm" loading={markingAll} onClick={() => void markAllRead()}>
            全部已读
          </Button>
        ) : null}
      </div>

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
              <p className="font-mono text-xs text-ink-sub">如需帮助,请提供编号 {recent.error.traceId}</p>
            ) : null}
            <Button variant="outline" size="sm" onClick={reload}>
              重新加载
            </Button>
          </div>
        ) : null}

        {recent.status === 'empty' ? (
          <Empty icon={Bell} title="还没有通知" description="作业、成绩、审核结果等消息会出现在这里。" />
        ) : null}

        {recent.status === 'success' && recent.data ? (
          <ul className="divide-y divide-line">
            {recent.data.map((item) => (
              <li key={item.id}>
                <ListRow
                  onSelect={() => void openNotification(item)}
                  leading={
                    <span
                      aria-hidden="true"
                      className={cn('size-2 rounded-full', item.is_read ? 'bg-transparent' : 'bg-seal')}
                    />
                  }
                  title={
                    <span className={cn(item.is_read && 'font-normal text-ink-sub')}>
                      {item.title}
                      {!item.is_read ? <span className="sr-only">(未读)</span> : null}
                    </span>
                  }
                  meta={item.content}
                  trailing={
                    <time
                      dateTime={item.created_at}
                      title={formatDateTime(item.created_at)}
                      className="whitespace-nowrap text-xs text-ink-sub"
                    >
                      {formatRelativeOccurrence(item.created_at)}
                    </time>
                  }
                />
              </li>
            ))}
          </ul>
        ) : null}
      </div>

      <div className="border-t border-line p-2">
        <Button
          variant="ghost"
          size="sm"
          className="w-full"
          onClick={() => {
            onClose()
            navigate(allPath)
          }}
        >
          查看全部
        </Button>
      </div>
    </div>
  )
}
