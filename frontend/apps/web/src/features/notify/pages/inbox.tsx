// 通知收件箱页(顶栏铃铛进入,{prefix}/notifications)。
// 三端共用同一实现:学生、教师、学校管理端都是租户身份,能力完全一致
// (平台管理端无租户、无收件箱,由该端导航配置 hasNotificationInbox: false 声明)。
//
// 通知偏好只渲染 GET /notify/preferences 返回的类型:该接口回全部可配置类型 + 本人设置 +
// 是否强制,前端不硬编码类型清单(否则后端加类型即失效,见对齐清单 §6.5)。
// 未读数改变后经资源失效协议广播给顶栏角标 —— 壳层与页面不在同一渲染树。

import { useCallback, useState } from 'react'
import { useNavigate } from 'react-router'
import { Bell, BellOff, CheckCheck, Inbox, Lock, Megaphone, Settings2, Trash2 } from 'lucide-react'
import type { Announcement, Notification, NotificationPreference } from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  DataPanel,
  EventTimeline,
  FilterBar,
  FilterField,
  Icon,
  IconButton,
  PageBody,
  PageHeader,
  PageScaffold,
  PageSection,
  Pagination,
  SegmentedControl,
  Skeleton,
  Switch,
  toast,
  type TimelineDay,
} from '@chaimir/ui'
import { api } from '../../../app/api'
import { invalidateAppResource } from '../../../app/resourceInvalidation'
import { ResourceState } from '../../../components/ResourceState'
import { useAsyncResource, usePagedResource } from '../../../hooks'
import { formatDate, formatShortDateTime, formatTime } from '../../../utils/formatters'
import { safeInternalNavigation } from '../../../utils/safeNavigation'
import {
  FORCED_PREFERENCE_HINT,
  announcementScopeLabel,
  notificationTypeLabel,
} from '../../../utils/labels/notify'
import { userFacingErrorMessage } from '../../../utils/userFacingError'

/** 已读筛选项:值为空串表示不过滤。 */
const READ_FILTERS = [
  { value: '', label: '全部' },
  { value: 'unread', label: '未读' },
  { value: 'read', label: '已读' },
] as const

/**
 * NotificationInboxPage 承载站内信、系统公告与通知偏好。
 */
export default function NotificationInboxPage() {
  return (
    <PageScaffold>
      <PageHeader
        title="通知收件箱"
        description="这里是与你相关的站内消息和学校公告。可以在右侧关掉不想收到的提醒类型。"
        icon={Inbox}
      />

      <PageBody rail={<PreferencePanel />}>
        <NotificationList />
        <AnnouncementList />
      </PageBody>
    </PageScaffold>
  )
}

/**
 * NotificationList 展示站内信分页列表,并承载标记已读、全部已读与删除。
 */
function NotificationList() {
  const navigate = useNavigate()
  const [readFilter, setReadFilter] = useState<string>('')
  const [actionError, setActionError] = useState<string>()
  const [markingAll, setMarkingAll] = useState(false)

  const notifications = usePagedResource<Notification>(
    (params) =>
      api.notify.getNotifications({
        is_read: readFilter === '' ? undefined : readFilter === 'read',
        ...params,
      }),
    [readFilter],
  )

  /** refreshAll 同时刷新本页与顶栏角标:未读数在两处显示,来源只有服务端。 */
  const refreshAll = useCallback(() => {
    notifications.reload()
    invalidateAppResource('notification-unread')
  }, [notifications])

  /** openNotification 标记已读并跳转;无链接的通知只标记已读。 */
  const openNotification = useCallback(
    async (item: Notification) => {
      if (!item.is_read) {
        try {
          await api.notify.markAsRead(item.id)
          refreshAll()
        } catch (error) {
          setActionError(userFacingErrorMessage(error, '标记已读没有成功,请稍后重试。'))
          return
        }
      }
      const safeLink = safeInternalNavigation(item.link)
      if (safeLink) navigate(safeLink)
    },
    [navigate, refreshAll],
  )

  const markAllRead = useCallback(async () => {
    setMarkingAll(true)
    setActionError(undefined)
    try {
      await api.notify.markAllAsRead()
      toast.success('已全部标记为已读')
      refreshAll()
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '操作没有完成,请稍后重试。'))
    } finally {
      setMarkingAll(false)
    }
  }, [refreshAll])

  const removeNotification = useCallback(
    async (item: Notification) => {
      setActionError(undefined)
      try {
        await api.notify.deleteNotification(item.id)
        refreshAll()
      } catch (error) {
        setActionError(userFacingErrorMessage(error, '删除没有成功,请稍后重试。'))
      }
    },
    [refreshAll],
  )

  /**
   * groupByDay 把消息按天分组成事件轴的输入(§6.5.3 第 ⑥ 族)。
   * 收件箱的读法是「从新到旧扫一遍」而不是「按列比对」,等宽四列表格逼读者横向读完才拼出一条消息;
   * 时间轴把「什么时候、发生了什么、要不要点进去」纵向排成三层。
   * 分组标签用「今天 / 昨天 / 具体日期」,比裸日期更快定位。
   */
  const groupByDay = (list: Notification[]): TimelineDay[] => {
    const today = new Date()
    const dayKey = (value: Date) =>
      `${value.getFullYear()}-${value.getMonth()}-${value.getDate()}`
    const todayKey = dayKey(today)
    const yesterday = new Date(today)
    yesterday.setDate(today.getDate() - 1)
    const yesterdayKey = dayKey(yesterday)

    const days: TimelineDay[] = []
    for (const item of list) {
      const at = new Date(item.created_at)
      const key = dayKey(at)
      const label =
        key === todayKey
          ? `今天 · ${formatDate(item.created_at)}`
          : key === yesterdayKey
            ? `昨天 · ${formatDate(item.created_at)}`
            : formatDate(item.created_at)
      let day = days.find((entry) => entry.label === label)
      if (!day) {
        day = { label, events: [] }
        days.push(day)
      }
      day.events.push({
        id: item.id,
        time: formatTime(item.created_at),
        // 未读用 success 点(玉)+ 加粗标题双通道表达,不靠颜色单一传达(FE-2)
        tone: item.is_read ? 'normal' : 'success',
        title: (
          <span className={item.is_read ? 'text-ink-sub' : 'font-medium text-ink'}>
            {item.title}
            {!item.is_read ? <span className="sr-only">(未读)</span> : null}
          </span>
        ),
        detail: `${notificationTypeLabel(item.type)} · ${item.content}`,
        action: (
          <div className="flex items-center gap-1">
            {item.link || !item.is_read ? (
              <Button variant="ghost" size="sm" onClick={() => void openNotification(item)}>
                {item.link ? '查看' : '标记已读'}
              </Button>
            ) : null}
            <IconButton
              variant="ghost"
              size="sm"
              icon={Trash2}
              aria-label={`删除消息 ${item.title}`}
              onClick={() => void removeNotification(item)}
            />
          </div>
        ),
      })
    }
    return days
  }

  return (
    <PageSection
      title="站内消息"
      actions={
        <Button variant="outline" size="sm" leftIcon={CheckCheck} loading={markingAll} onClick={() => void markAllRead()}>
          全部已读
        </Button>
      }
    >
      {/* 动作失败就近内联(§6.7 C) */}
      {actionError ? (
        <Callout tone="danger" className="mb-4">
          {actionError}
        </Callout>
      ) : null}

      {/* 时间流族:筛选井 + 按天事件轴 + 分页同处一块抬起片(§6.5.3 第 ⑥ 族) */}
      <DataPanel
        label="站内消息"
        filter={
          <FilterBar label="站内消息筛选">
            <FilterField label="已读状态" group>
              <SegmentedControl
                aria-label="按已读状态筛选"
                size="sm"
                options={READ_FILTERS.map((item) => ({ value: item.value, label: item.label }))}
                value={readFilter}
                onValueChange={setReadFilter}
              />
            </FilterField>
          </FilterBar>
        }
        footer={
          <Pagination
            page={notifications.page}
            pageSize={notifications.pageSize}
            total={notifications.total}
            onPageChange={notifications.setPage}
          />
        }
      >
        <ResourceState
          resource={notifications}
          emptyIcon={Inbox}
          emptyTitle={readFilter === 'unread' ? '没有未读消息' : '暂无站内消息'}
          emptyDescription={
            readFilter === 'unread'
              ? '所有消息都已读完。'
              : '作业发布、成绩更新、竞赛开始等消息会出现在这里。'
          }
          skeleton={<Skeleton variant="line" lines={6} />}
        >
          {(page) => <EventTimeline label="站内消息" days={groupByDay(page.list)} />}
        </ResourceState>
      </DataPanel>
    </PageSection>
  )
}

/**
 * AnnouncementList 展示系统公告并支持标记已读。
 * 公告是「收」的一侧:发布在校管/平台端的侧栏「系统公告」页,两者不互相替代。
 */
function AnnouncementList() {
  const [actionError, setActionError] = useState<string>()

  const announcements = usePagedResource<Announcement>(
    (params) => api.notify.getAnnouncements(params),
    [],
  )

  const markRead = useCallback(
    async (announcement: Announcement) => {
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
    <PageSection title="系统公告" description={`共 ${announcements.total} 条`}>
      <div className="flex flex-col gap-4">
        {actionError ? <Callout tone="danger">{actionError}</Callout> : null}

        <ResourceState
          resource={announcements}
          emptyIcon={Megaphone}
          emptyTitle="暂无公告"
          emptyDescription="学校或平台发布公告后会显示在这里。"
          skeleton={<Skeleton variant="line" lines={3} />}
        >
          {(page) => (
            <>
              <div className="flex flex-col gap-3">
                {page.list.map((announcement) => (
                  <Card key={announcement.id}>
                    <CardHeader
                      title={
                        <span className="flex flex-wrap items-center gap-2">
                          <span className="min-w-0 truncate">{announcement.title}</span>
                          <Badge tone="neutral">{announcementScopeLabel(announcement.scope)}</Badge>
                          {!announcement.is_read ? <Badge tone="cinnabar">未读</Badge> : null}
                        </span>
                      }
                      description={formatShortDateTime(announcement.published_at)}
                      actions={
                        !announcement.is_read ? (
                          <Button
                            variant="ghost"
                            size="sm"
                            leftIcon={CheckCheck}
                            onClick={() => void markRead(announcement)}
                          >
                            标记已读
                          </Button>
                        ) : null
                      }
                    />
                    <CardBody>
                      <p className="whitespace-pre-wrap text-base leading-relaxed text-ink">
                        {announcement.content}
                      </p>
                    </CardBody>
                  </Card>
                ))}
              </div>
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
    </PageSection>
  )
}

/**
 * PreferencePanel 渲染通知偏好开关。
 * 类型清单来自服务端:强制类型显示为不可关闭并说明原因,
 * 后端 UpsertPreference 仍独立校验(A0005),不依赖前端自律。
 */
function PreferencePanel() {
  const [pendingType, setPendingType] = useState<string>()
  const [actionError, setActionError] = useState<string>()

  const preferences = useAsyncResource(() => api.notify.getPreferences(), [])

  const togglePreference = useCallback(
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
    <Card>
      <CardHeader
        title={
          <span className="flex items-center gap-2">
            <Settings2 aria-hidden="true" className="size-4 shrink-0 text-primary" />
            通知偏好
          </span>
        }
        description="关掉不想收到的提醒类型。"
      />
      <CardBody>
        <ResourceState
          resource={preferences}
          emptyIcon={BellOff}
          emptyTitle="暂无可设置的类型"
          emptyDescription="平台还没有开放可关闭的通知类型。"
          skeleton={<Skeleton variant="line" lines={4} />}
        >
          {(list) => {
            // 强制类型单独成组:同一句「不能关闭」说明只写一次,避免在一张卡里重复八遍
            const optional = list.filter((preference) => !preference.force)
            const forced = list.filter((preference) => preference.force)
            return (
              <div className="flex flex-col gap-4">
                {actionError ? <Callout tone="danger">{actionError}</Callout> : null}
                {optional.length > 0 ? (
                  <div className="flex flex-col gap-3">
                    {optional.map((preference) => (
                      <PreferenceRow
                        key={preference.type}
                        preference={preference}
                        pending={pendingType === preference.type}
                        onToggle={(enabled) => void togglePreference(preference, enabled)}
                      />
                    ))}
                  </div>
                ) : null}
                {forced.length > 0 ? (
                  <div className="flex flex-col gap-3 border-t border-line pt-4">
                    <p className="text-xs text-ink-sub">{FORCED_PREFERENCE_HINT}</p>
                    {forced.map((preference) => (
                      <PreferenceRow
                        key={preference.type}
                        preference={preference}
                        pending={pendingType === preference.type}
                        onToggle={(enabled) => void togglePreference(preference, enabled)}
                      />
                    ))}
                  </div>
                ) : null}
              </div>
            )
          }}
        </ResourceState>
      </CardBody>
    </Card>
  )
}

interface PreferenceRowProps {
  preference: NotificationPreference
  pending: boolean
  onToggle: (enabled: boolean) => void
}

/**
 * PreferenceRow 渲染单个偏好开关。
 * 强制类型换锁形图标并禁用开关:不可关闭这件事不能只靠开关变淡一档表达(规范 §3.2 色非唯一);
 * 原因说明由所在分组统一给出,不在每一行重复同一句话。
 */
function PreferenceRow({ preference, pending, onToggle }: PreferenceRowProps) {
  const label = notificationTypeLabel(preference.type)

  return (
    <div className="flex items-start justify-between gap-3">
      <div className="min-w-0">
        <div className="flex items-center gap-1.5 text-base text-ink">
          <Icon
            icon={preference.force ? Lock : Bell}
            size="xs"
            className="shrink-0 text-ink-sub"
            aria-hidden
          />
          <span className="min-w-0 truncate">{label}</span>
        </div>
      </div>
      <Switch
        checked={preference.enabled}
        disabled={preference.force || pending}
        aria-label={preference.force ? `${label}通知不能关闭` : `接收${label}通知`}
        onCheckedChange={onToggle}
      />
    </div>
  )
}
