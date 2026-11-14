// 通知收件箱页(顶栏铃铛进入,{prefix}/notifications)。设计 §7.7:
// 标签「消息 / 公告」(都带未读数),右上角「全部已读」与「提醒设置」。消息为按天分组的列表行
// (未读圆点、标题加粗、内容摘要一行、相对时间),分段「全部 / 未读」;点击一条标为已读,有链接的
// 跳转到对应页面,没有链接的就地展开全文;行内删除在悬停时出现。公告标签为公告列表,点开展开全文
// 并标为已读。「提醒设置」在右侧抽屉里列出提醒类型开关,强制类显示为不可关闭并说明原因。
// 标签与分段写进地址。
//
// 三端共用同一实现:学生、教师、学校管理端都是租户身份(平台管理端无收件箱)。
// 通知偏好只渲染 GET /notify/preferences 返回的类型,前端不硬编码类型清单(对齐清单 §6.5)。
// 未读数改变后经资源失效协议广播给顶栏角标 —— 壳层与页面不在同一渲染树。
// 公告的未读数取公告列表的 facets.is_read:公告列表在页面层读一次,标签数字与列表共用这次请求;
// 公告页码写进地址的 notice_page(消息列表用 page),两个标签互不干扰。

import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router'
import { Bell, CheckCheck, Inbox, Lock, Megaphone, Settings2, Trash2 } from 'lucide-react'
import type { Announcement, Notification, NotificationPreference } from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  cn,
  Drawer,
  DrawerBody,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
  Icon,
  IconButton,
  PageHeader,
  PageScaffold,
  Pagination,
  SegmentedControl,
  Skeleton,
  Switch,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
  toast,
} from '@chaimir/ui'
import { api } from '../../../app/api'
import { invalidateAppResource, subscribeAppResource } from '../../../app/resourceInvalidation'
import { ResourceError, ResourceState } from '../../../components/ResourceState'
import { useAsyncResource, usePagedResource, usePageLocation, useUrlState, type PagedResourceState } from '../../../hooks'
import { facetCount, facetSum } from '../../../utils/facets'
import { formatDateTime, formatRelativeOccurrence } from '../../../utils/formatters'
import { safeInternalNavigation } from '../../../utils/safeNavigation'
import { FORCED_PREFERENCE_HINT, announcementScopeLabel, notificationTypeLabel } from '../../../utils/labels/notify'
import { userFacingErrorMessage } from '../../../utils/userFacingError'

/** 地址参数:tab 是标签,filter 是消息分段 */
const INBOX_QUERY = {
  tab: { defaultValue: 'messages', allowed: ['messages', 'announcements'] },
  filter: { defaultValue: 'all', allowed: ['all', 'unread'] },
} as const

/**
 * NotificationInboxPage 承载站内消息、公告与提醒设置。
 */
export default function NotificationInboxPage() {
  usePageLocation([{ label: '通知收件箱' }])
  const [query, setQuery] = useUrlState(INBOX_QUERY)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [markingAll, setMarkingAll] = useState(false)
  const [actionError, setActionError] = useState<string>()
  const unread = useAsyncResource(() => api.notify.getUnreadCount(), [], () => false)
  useEffect(() => subscribeAppResource('notification-unread', unread.reload), [unread.reload])
  const unreadCount = unread.data?.unread
  const announcements = usePagedResource<Announcement>((params) => api.notify.getAnnouncements(params), [], {
    urlPageKey: 'notice_page',
  })
  const unreadAnnouncements = facetCount(announcements.data?.facets, 'is_read', false)

  const markAllRead = useCallback(async () => {
    setMarkingAll(true)
    setActionError(undefined)
    try {
      await api.notify.markAllAsRead()
      toast.success('已全部标记为已读')
      invalidateAppResource('notification-unread')
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '操作没有完成,请稍后重试。'))
    } finally {
      setMarkingAll(false)
    }
  }, [])

  return (
    <PageScaffold>
      <div className="max-w-4xl">
        <PageHeader
          title="通知收件箱"
          actions={
            <>
              <Button
                variant="ghost"
                leftIcon={CheckCheck}
                loading={markingAll}
                disabled={unreadCount === 0}
                onClick={() => void markAllRead()}
              >
                全部已读
              </Button>
              <Button variant="outline" leftIcon={Settings2} onClick={() => setSettingsOpen(true)}>
                提醒设置
              </Button>
            </>
          }
        />

        {actionError ? (
          <Callout tone="danger" className="mb-4">
            {actionError}
          </Callout>
        ) : null}

        <Tabs value={query.tab} onValueChange={(value) => setQuery({ tab: value })}>
          <TabsList aria-label="消息与公告">
            <TabsTrigger value="messages" icon={Inbox} count={unreadCount && unreadCount > 0 ? unreadCount : undefined}>
              消息
            </TabsTrigger>
            <TabsTrigger
              value="announcements"
              icon={Megaphone}
              count={unreadAnnouncements && unreadAnnouncements > 0 ? unreadAnnouncements : undefined}
            >
              公告
            </TabsTrigger>
          </TabsList>
          <TabsContent value="messages">
            <MessageList filter={query.filter} onFilterChange={(value) => setQuery({ filter: value })} />
          </TabsContent>
          <TabsContent value="announcements">
            <AnnouncementList announcements={announcements} />
          </TabsContent>
        </Tabs>
      </div>

      <Drawer open={settingsOpen} onOpenChange={setSettingsOpen}>
        <DrawerContent>
          <DrawerHeader>
            <DrawerTitle>提醒设置</DrawerTitle>
            <DrawerDescription>关掉不想收到的提醒类型。</DrawerDescription>
          </DrawerHeader>
          <DrawerBody>
            <PreferenceList />
          </DrawerBody>
        </DrawerContent>
      </Drawer>
    </PageScaffold>
  )
}

/** dayLabel 按天分组的标题:今天 / 昨天 / 具体日期 */
function dayLabel(value: string, now: Date): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '时间未知'
  const startOf = (input: Date) => new Date(input.getFullYear(), input.getMonth(), input.getDate()).getTime()
  const days = Math.round((startOf(now) - startOf(date)) / 86_400_000)
  if (days === 0) return '今天'
  if (days === 1) return '昨天'
  return date.getFullYear() === now.getFullYear()
    ? `${date.getMonth() + 1}月${date.getDate()}日`
    : `${date.getFullYear()}年${date.getMonth() + 1}月${date.getDate()}日`
}

/**
 * MessageList 站内消息:分段「全部 / 未读」+ 按天分组的列表行 + 分页。
 */
function MessageList({ filter, onFilterChange }: { filter: string; onFilterChange: (value: string) => void }) {
  const navigate = useNavigate()
  const [expandedId, setExpandedId] = useState<string>()
  const [actionError, setActionError] = useState<string>()
  const notifications = usePagedResource<Notification>(
    (params) => api.notify.getNotifications({ is_read: filter === 'unread' ? false : undefined, ...params }),
    [filter],
    { urlPageKey: 'page' },
  )
  useEffect(() => subscribeAppResource('notification-unread', notifications.reload), [notifications.reload])

  /** openMessage 标为已读;有站内链接的跳转,没有的就地展开或收起全文 */
  const openMessage = useCallback(
    async (item: Notification) => {
      setActionError(undefined)
      if (!item.is_read) {
        try {
          await api.notify.markAsRead(item.id)
          invalidateAppResource('notification-unread')
        } catch (error) {
          setActionError(userFacingErrorMessage(error, '标记已读没有成功,请稍后重试。'))
          return
        }
      }
      const link = safeInternalNavigation(item.link)
      if (link) navigate(link)
      else setExpandedId((current) => (current === item.id ? undefined : item.id))
    },
    [navigate],
  )

  const removeMessage = useCallback(async (item: Notification) => {
    setActionError(undefined)
    try {
      await api.notify.deleteNotification(item.id)
      invalidateAppResource('notification-unread')
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '删除没有成功,请稍后重试。'))
    }
  }, [])

  // 「全部 / 未读」的数字取列表接口的 facets(套用类型筛选,不套用已读筛选)
  const facets = notifications.status === 'success' || notifications.status === 'empty' ? notifications.data?.facets : undefined
  const allCount = facetSum(facets, 'is_read')
  const unreadInList = facetCount(facets, 'is_read', false)

  const groups = useMemo(() => {
    const now = new Date()
    const result: { label: string; items: Notification[] }[] = []
    for (const item of notifications.data?.list ?? []) {
      const label = dayLabel(item.created_at, now)
      const last = result[result.length - 1]
      if (last && last.label === label) last.items.push(item)
      else result.push({ label, items: [item] })
    }
    return result
  }, [notifications.data])

  return (
    <div className="flex flex-col gap-4">
      <SegmentedControl
        aria-label="按是否已读筛选"
        size="sm"
        options={[
          { value: 'all', label: '全部', count: allCount },
          { value: 'unread', label: '未读', count: unreadInList },
        ]}
        value={filter}
        onValueChange={onFilterChange}
        className="self-start"
      />
      {actionError ? <Callout tone="danger">{actionError}</Callout> : null}

      {notifications.status === 'error' ? (
        <ResourceError
          message={notifications.error?.message ?? '消息暂时读取不到'}
          traceId={notifications.error?.traceId}
          onRetry={notifications.reload}
        />
      ) : notifications.status === 'loading' && !notifications.data ? (
        <div aria-busy="true" className="rounded-lg bg-surface p-4 shadow-xs">
          <Skeleton variant="line" lines={6} />
        </div>
      ) : groups.length === 0 ? (
        <div className="flex flex-col items-center rounded-lg bg-surface px-6 py-14 text-center shadow-xs">
          <Icon icon={Inbox} size="lg" className="text-ink-faint" />
          <p className="mt-3 text-md font-semibold text-ink">{filter === 'unread' ? '没有未读消息' : '还没有消息'}</p>
          <p className="mt-1.5 text-sm text-ink-sub">
            {filter === 'unread' ? '所有消息都读完了。' : '作业发布、成绩更新、竞赛开始等消息会出现在这里。'}
          </p>
        </div>
      ) : (
        <>
          {groups.map((group) => (
            <section key={group.label} aria-label={group.label}>
              <h2 className="mb-2 text-sm font-medium text-ink-sub">{group.label}</h2>
              <ul className="divide-y divide-line overflow-hidden rounded-lg bg-surface shadow-xs">
                {group.items.map((item) => (
                  <li key={item.id}>
                    <MessageRow
                      item={item}
                      expanded={expandedId === item.id}
                      onOpen={() => void openMessage(item)}
                      onRemove={() => void removeMessage(item)}
                    />
                  </li>
                ))}
              </ul>
            </section>
          ))}
          <Pagination
            page={notifications.page}
            pageSize={notifications.pageSize}
            total={notifications.total}
            onPageChange={notifications.setPage}
          />
        </>
      )}
    </div>
  )
}

/** MessageRow 一条消息:未读圆点、标题(未读加粗)、类型与摘要一行、相对时间;删除在悬停或聚焦时出现 */
function MessageRow({
  item,
  expanded,
  onOpen,
  onRemove,
}: {
  item: Notification
  expanded: boolean
  onOpen: () => void
  onRemove: () => void
}) {
  const hasLink = Boolean(safeInternalNavigation(item.link))
  return (
    <div className="group relative flex items-start gap-3 px-4 py-3 transition-colors duration-fast hover:bg-surface-hover">
      <span
        aria-hidden="true"
        className={cn('mt-2 size-2 shrink-0 rounded-full', item.is_read ? 'bg-transparent' : 'bg-primary')}
      />
      <button
        type="button"
        onClick={onOpen}
        aria-expanded={hasLink ? undefined : expanded}
        className="min-w-0 flex-1 text-left outline-hidden before:absolute before:inset-0 focus-visible:before:outline-2 focus-visible:before:-outline-offset-2 focus-visible:before:outline-accent"
      >
        <span className={cn('block truncate text-base', item.is_read ? 'text-ink' : 'font-semibold text-ink')}>
          {item.title}
          {!item.is_read ? <span className="sr-only">(未读)</span> : null}
        </span>
        <span className={cn('mt-0.5 block text-sm text-ink-sub', expanded ? 'whitespace-pre-wrap' : 'truncate')}>
          {notificationTypeLabel(item.type)} · {item.content}
        </span>
      </button>
      <span className="flex shrink-0 items-center gap-1">
        <time dateTime={item.created_at} title={formatDateTime(item.created_at)} className="text-xs text-ink-sub">
          {formatRelativeOccurrence(item.created_at)}
        </time>
        <IconButton
          variant="ghost"
          size="sm"
          icon={Trash2}
          aria-label={`删除消息 ${item.title}`}
          onClick={onRemove}
          className="relative z-base opacity-0 transition-opacity duration-fast focus-visible:opacity-100 group-hover:opacity-100 pointer-coarse:opacity-100"
        />
      </span>
    </div>
  )
}

/**
 * AnnouncementList 系统公告:点开展开全文并标为已读。
 * 公告是「收」的一侧:发布在校管/平台端的侧栏「系统公告」页,两者不互相替代。
 * 列表由页面层读取后传入:标签上的未读数与列表共用同一次请求,标为已读后一起刷新。
 */
function AnnouncementList({ announcements }: { announcements: PagedResourceState<Announcement> }) {
  const [expandedId, setExpandedId] = useState<string>()
  const [actionError, setActionError] = useState<string>()

  const toggle = useCallback(
    async (announcement: Announcement) => {
      setExpandedId((current) => (current === announcement.id ? undefined : announcement.id))
      if (announcement.is_read) return
      setActionError(undefined)
      try {
        await api.notify.markAnnouncementRead(announcement.id)
        announcements.reload()
      } catch (error) {
        setActionError(userFacingErrorMessage(error, '标记已读没有成功,请稍后重试。'))
      }
    },
    [announcements],
  )

  return (
    <div className="flex flex-col gap-4">
      {actionError ? <Callout tone="danger">{actionError}</Callout> : null}
      <ResourceState
        resource={announcements}
        emptyIcon={Megaphone}
        emptyTitle="还没有公告"
        emptyDescription="学校或平台发布公告后会显示在这里。"
        skeleton={
          <div aria-busy="true" className="rounded-lg bg-surface p-4 shadow-xs">
            <Skeleton variant="line" lines={4} />
          </div>
        }
      >
        {(page) => (
          <>
            <ul className="divide-y divide-line overflow-hidden rounded-lg bg-surface shadow-xs">
              {page.list.map((announcement) => {
                const expanded = expandedId === announcement.id
                return (
                  <li key={announcement.id} className="relative px-4 py-3 transition-colors duration-fast hover:bg-surface-hover">
                    <button
                      type="button"
                      aria-expanded={expanded}
                      onClick={() => void toggle(announcement)}
                      className="flex w-full items-start gap-3 text-left outline-hidden before:absolute before:inset-0 focus-visible:before:outline-2 focus-visible:before:-outline-offset-2 focus-visible:before:outline-accent"
                    >
                      <span
                        aria-hidden="true"
                        className={cn('mt-2 size-2 shrink-0 rounded-full', announcement.is_read ? 'bg-transparent' : 'bg-primary')}
                      />
                      <span className="min-w-0 flex-1">
                        <span className="flex flex-wrap items-center gap-2">
                          <span className={cn('min-w-0 text-base text-ink', !announcement.is_read && 'font-semibold')}>
                            {announcement.title}
                          </span>
                          <Badge tone="neutral">{announcementScopeLabel(announcement.scope)}</Badge>
                          {!announcement.is_read ? <span className="sr-only">(未读)</span> : null}
                        </span>
                        <span className={cn('mt-1 block text-sm text-ink-sub', expanded ? 'whitespace-pre-wrap text-ink' : 'truncate')}>
                          {announcement.content}
                        </span>
                      </span>
                      <time
                        dateTime={announcement.published_at}
                        title={formatDateTime(announcement.published_at)}
                        className="shrink-0 text-xs text-ink-sub"
                      >
                        {formatRelativeOccurrence(announcement.published_at)}
                      </time>
                    </button>
                  </li>
                )
              })}
            </ul>
            <Pagination
              page={announcements.page}
              pageSize={announcements.pageSize}
              total={announcements.total}
              onPageChange={announcements.setPage}
            />
          </>
        )}
      </ResourceState>
    </div>
  )
}

/**
 * PreferenceList 提醒类型开关:类型清单来自服务端;强制类单独成组,显示为不可关闭并说明原因。
 * 后端 UpsertPreference 仍独立校验强制类型,不依赖前端自律。
 */
function PreferenceList() {
  const [pendingType, setPendingType] = useState<string>()
  const [actionError, setActionError] = useState<string>()
  const preferences = useAsyncResource(() => api.notify.getPreferences(), [])

  const toggle = useCallback(
    async (preference: NotificationPreference, enabled: boolean) => {
      setPendingType(preference.type)
      setActionError(undefined)
      try {
        await api.notify.updatePreference(preference.type, { enabled })
        preferences.reload()
      } catch (error) {
        setActionError(userFacingErrorMessage(error, '设置没有保存成功,请稍后重试。'))
      } finally {
        setPendingType(undefined)
      }
    },
    [preferences],
  )

  return (
    <ResourceState
      resource={preferences}
      emptyIcon={Bell}
      emptyTitle="暂无可设置的提醒"
      emptyDescription="平台还没有开放可关闭的提醒类型。"
      skeleton={<Skeleton variant="line" lines={4} />}
    >
      {(list) => {
        const optional = list.filter((preference) => !preference.force)
        const forced = list.filter((preference) => preference.force)
        return (
          <div className="flex flex-col gap-5">
            {actionError ? <Callout tone="danger">{actionError}</Callout> : null}
            {optional.length > 0 ? (
              <ul className="flex flex-col gap-4">
                {optional.map((preference) => (
                  <PreferenceRow
                    key={preference.type}
                    preference={preference}
                    pending={pendingType === preference.type}
                    onToggle={(enabled) => void toggle(preference, enabled)}
                  />
                ))}
              </ul>
            ) : null}
            {forced.length > 0 ? (
              <div className="flex flex-col gap-4 border-t border-line pt-5">
                <p className="text-sm text-ink-sub">{FORCED_PREFERENCE_HINT}</p>
                <ul className="flex flex-col gap-4">
                  {forced.map((preference) => (
                    <PreferenceRow
                      key={preference.type}
                      preference={preference}
                      pending={pendingType === preference.type}
                      onToggle={(enabled) => void toggle(preference, enabled)}
                    />
                  ))}
                </ul>
              </div>
            ) : null}
          </div>
        )
      }}
    </ResourceState>
  )
}

/** PreferenceRow 一个提醒类型的开关;强制类换锁形图标并禁用开关(不只靠变淡表达) */
function PreferenceRow({
  preference,
  pending,
  onToggle,
}: {
  preference: NotificationPreference
  pending: boolean
  onToggle: (enabled: boolean) => void
}) {
  const label = notificationTypeLabel(preference.type)
  return (
    <li className="flex items-center justify-between gap-3">
      <span className="flex min-w-0 items-center gap-2 text-base text-ink">
        <Icon icon={preference.force ? Lock : Bell} size="sm" className="shrink-0 text-ink-sub" />
        <span className="min-w-0 truncate">{label}</span>
      </span>
      <Switch
        checked={preference.enabled}
        disabled={preference.force || pending}
        aria-label={preference.force ? `${label}提醒不能关闭` : `接收${label}提醒`}
        onCheckedChange={onToggle}
      />
    </li>
  )
}
