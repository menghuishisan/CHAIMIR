// 通知收件箱页(顶栏铃铛进入,{prefix}/notifications)。
// 三端共用同一实现:学生、教师、学校管理端都是租户身份,能力完全一致
// (平台管理端无租户、无收件箱,由该端导航配置 hasNotificationInbox: false 声明)。
//
// 通知偏好只渲染 GET /notify/preferences 返回的类型:该接口回全部可配置类型 + 本人设置 +
// 是否强制,前端不硬编码类型清单(否则后端加类型即失效,见对齐清单 §6.5)。
// 未读数改变后经资源失效协议广播给顶栏角标 —— 壳层与页面不在同一渲染树。

import { useCallback, useState } from 'react'
import { useNavigate } from 'react-router'
import { Bell, BellOff, CheckCheck, Inbox, Megaphone, Settings2, Trash2 } from 'lucide-react'
import type { Announcement, Notification, NotificationPreference } from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  IconButton,
  PageBody,
  PageHeader,
  PageScaffold,
  PageSection,
  Pagination,
  SegmentedControl,
  Skeleton,
  Switch,
  Table,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../app/api'
import { invalidateAppResource } from '../../../app/resourceInvalidation'
import { ResourceState } from '../../../components/ResourceState'
import { useAsyncResource, usePagedResource } from '../../../hooks'
import { formatShortDateTime } from '../../../utils/formatters'
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
        kicker={<Breadcrumb items={[{ label: '通知收件箱' }]} />}
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
      if (item.link) navigate(item.link)
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

  const columns: TableColumn<Notification>[] = [
    {
      key: 'title',
      header: '消息',
      render: (item) => (
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            {!item.is_read ? (
              <span aria-hidden="true" className="size-1.5 shrink-0 rounded-full bg-seal" />
            ) : null}
            <span className={item.is_read ? 'truncate text-ink-sub' : 'truncate font-medium text-ink'}>
              {item.title}
            </span>
            {!item.is_read ? <span className="sr-only">未读</span> : null}
          </div>
          <div className="line-clamp-2 text-xs text-ink-sub">{item.content}</div>
        </div>
      ),
    },
    {
      key: 'type',
      header: '类型',
      render: (item) => <Badge tone="neutral">{notificationTypeLabel(item.type)}</Badge>,
    },
    {
      key: 'created_at',
      header: '时间',
      render: (item) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatShortDateTime(item.created_at)}
        </span>
      ),
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (item) => (
        <div className="flex justify-end gap-1">
          {item.link ? (
            <Button variant="ghost" size="sm" onClick={() => void openNotification(item)}>
              查看
            </Button>
          ) : !item.is_read ? (
            <Button variant="ghost" size="sm" onClick={() => void openNotification(item)}>
              标记已读
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
    },
  ]

  return (
    <PageSection
      title="站内消息"
      description={`共 ${notifications.total} 条`}
      actions={
        <div className="flex flex-wrap items-center gap-2">
          <SegmentedControl
            aria-label="按已读状态筛选"
            size="sm"
            options={READ_FILTERS.map((item) => ({ value: item.value, label: item.label }))}
            value={readFilter}
            onValueChange={setReadFilter}
          />
          <Button variant="outline" size="sm" leftIcon={CheckCheck} loading={markingAll} onClick={() => void markAllRead()}>
            全部已读
          </Button>
        </div>
      }
    >
      <div className="flex flex-col gap-4">
        {actionError ? <Callout tone="danger">{actionError}</Callout> : null}

        <ResourceState
          resource={notifications}
          emptyIcon={Inbox}
          emptyTitle={readFilter === 'unread' ? '没有未读消息' : '暂无站内消息'}
          emptyDescription={
            readFilter === 'unread'
              ? '所有消息都已读完。'
              : '作业发布、成绩更新、竞赛开始等消息会出现在这里。'
          }
          skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
        >
          {(page) => (
            <>
              <Table
                columns={columns}
                data={page.list}
                rowKey={(item) => item.id}
                onRowClick={(item) => void openNotification(item)}
              />
              <Pagination
                page={notifications.page}
                pageSize={notifications.pageSize}
                total={notifications.total}
                onPageChange={notifications.setPage}
              />
            </>
          )}
        </ResourceState>
      </div>
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
        description="关掉不想收到的提醒类型。重要通知不能关闭。"
      />
      <CardBody>
        <ResourceState
          resource={preferences}
          emptyIcon={BellOff}
          emptyTitle="暂无可设置的类型"
          emptyDescription="平台还没有开放可关闭的通知类型。"
          skeleton={<Skeleton variant="line" lines={4} />}
        >
          {(list) => (
            <div className="flex flex-col gap-4">
              {actionError ? <Callout tone="danger">{actionError}</Callout> : null}
              <div className="flex flex-col gap-3">
                {list.map((preference) => (
                  <PreferenceRow
                    key={preference.type}
                    preference={preference}
                    pending={pendingType === preference.type}
                    onToggle={(enabled) => void togglePreference(preference, enabled)}
                  />
                ))}
              </div>
            </div>
          )}
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
 * 强制类型的开关禁用并给出原因,而不是让用户点了才发现关不掉。
 */
function PreferenceRow({ preference, pending, onToggle }: PreferenceRowProps) {
  const label = notificationTypeLabel(preference.type)

  return (
    <div className="flex items-start justify-between gap-3">
      <div className="min-w-0">
        <div className="flex items-center gap-1.5 text-base text-ink">
          <Bell aria-hidden="true" className="size-3.5 shrink-0 text-ink-faint" />
          <span className="min-w-0 truncate">{label}</span>
        </div>
        {preference.force ? (
          <p className="mt-0.5 text-xs text-ink-sub">{FORCED_PREFERENCE_HINT}</p>
        ) : null}
      </div>
      <Switch
        checked={preference.enabled}
        disabled={preference.force || pending}
        aria-label={`接收${label}通知`}
        onCheckedChange={onToggle}
      />
    </div>
  )
}
