// 系统公告页(校管侧栏 /school-admin/announcements、平台侧栏 /platform-admin/announcements 共用)。设计 §7.7:
// 页头 +「发布公告」;已发布公告列表行(标题、范围标签:全平台 / 全校 / 指定角色、发布时间),点开就地展开全文;
// 发布弹窗里范围选择在上、正文在下,发布前显示「将通知谁」的一句确认。指标条与常驻说明卡去掉。
//
// 这是「发」的一侧:发布公告。「收」的一侧是顶栏铃铛进入的通知收件箱 ——
// 两者不互相替代(规范 §10)。平台管理端没有收件箱(无租户、站内信在数据模型层不成立),
// 公告能力就落在这一页。
//
// 发布范围按发布方分叉,这是后端的硬边界:
//   学校管理员发本校或定向角色(scope=平台会被 validateAnnouncementRequest 拒);
//   平台管理员发全平台(它的会话没有租户,发本校范围会落到一个不存在的归属上)。
// 故由调用方通过 publisher 显式声明,页面不在运行时判角色枚举。

import { useCallback, useState } from 'react'
import { Megaphone, Plus, Send } from 'lucide-react'
import {
  AnnouncementScope,
  UserRole,
  type Announcement,
} from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  Checkbox,
  FormField,
  Input,
  ListRow,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  PageHeader,
  PageScaffold,
  Pagination,
  SegmentedControl,
  Skeleton,
  Textarea,
  toast,
} from '@chaimir/ui'
import { api } from '../../../app/api'
import { ResourceState } from '../../../components/ResourceState'
import { usePagedResource } from '../../../hooks'
import { useFormValidation } from '../../../hooks/useFormValidation'
import { facetCount } from '../../../utils/facets'
import { formatDateTime, formatRelativeOccurrence } from '../../../utils/formatters'
import { announcementScopeLabel } from '../../../utils/labels/notify'
import { userRoleLabel } from '../../../utils/labels/identity'
import { userFacingErrorMessage } from '../../../utils/userFacingError'

/** 发布方:决定可选的公告范围与页面文案。 */
type AnnouncementPublisher = 'platform' | 'school'

/** 校管可发布的范围:平台范围属平台管理员,不在校管端出现。 */
const SCHOOL_SCOPE_OPTIONS = [
  { value: String(AnnouncementScope.TENANT), label: '全校可见' },
  { value: String(AnnouncementScope.ROLES), label: '指定角色可见' },
] as const

/** 可定向的角色:校内三类身份。 */
const TARGET_ROLE_OPTIONS = [UserRole.TEACHER, UserRole.STUDENT, UserRole.SCHOOL_ADMIN] as const

/** 两个发布方的空列表说明。 */
const PUBLISHER_COPY: Record<AnnouncementPublisher, { emptyDescription: string }> = {
  platform: { emptyDescription: '发布公告后会推送到所有学校师生的通知收件箱。' },
  school: { emptyDescription: '发布公告后会推送到师生的通知收件箱;平台下发的公告也会出现在这里。' },
}

export interface AnnouncementsPageProps {
  /**
   * 发布方身份。
   * 决定可选范围:platform 只能发全平台,school 能发本校与定向角色 ——
   * 后端按会话身份强制这一边界,前端按声明渲染,不给必然被拒的选项。
   */
  publisher: AnnouncementPublisher
}

/**
 * AnnouncementsPage 发布与查看公告。
 */
export default function AnnouncementsPage({ publisher }: AnnouncementsPageProps) {
  const [createOpen, setCreateOpen] = useState(false)
  const [expanded, setExpanded] = useState<string>()
  const copy = PUBLISHER_COPY[publisher]

  const announcements = usePagedResource<Announcement>(
    (params) => api.notify.getAnnouncements(params),
    [],
    { urlPageKey: 'page' },
  )

  // 按范围分桶取后端 facets.scope:全量分组计数,不用当前页切片去数(§6.5.4)
  const facets = announcements.total === undefined ? undefined : announcements.data?.facets
  const platformCount = facetCount(
    facets,
    'scope',
    AnnouncementScope.PLATFORM,
  )
  const tenantCount = facetCount(facets, 'scope', AnnouncementScope.TENANT)
  const roleCount = facetCount(facets, 'scope', AnnouncementScope.ROLES)

  return (
    <PageScaffold>
      <PageHeader
        title="系统公告"
        meta={
          announcements.total
            ? [
                { value: `共 ${announcements.total} 条` },
                { label: '全平台', value: platformCount || undefined },
                { label: '全校', value: tenantCount || undefined },
                { label: '指定角色', value: roleCount || undefined },
              ]
            : undefined
        }
        actions={
          <Button variant="primary" leftIcon={Plus} onClick={() => setCreateOpen(true)}>
            发布公告
          </Button>
        }
      />

      <ResourceState
        resource={announcements}
        emptyIcon={Megaphone}
        emptyTitle="还没有公告"
        emptyDescription={copy.emptyDescription}
        emptyAction={
          <Button variant="primary" leftIcon={Plus} onClick={() => setCreateOpen(true)}>
            发布公告
          </Button>
        }
        skeleton={<Skeleton variant="line" lines={5} />}
      >
        {(page) => (
          <>
            <ul className="divide-y divide-line rounded-lg bg-surface px-2 py-1 shadow-xs">
              {page.list.map((announcement) => {
                const open = expanded === announcement.id
                return (
                  <li key={announcement.id}>
                    <ListRow
                      title={announcement.title}
                      meta={
                        <span className="flex flex-wrap items-center gap-x-2" title={formatDateTime(announcement.published_at)}>
                          {formatRelativeOccurrence(announcement.published_at)}发布
                          {announcement.expire_at ? <span>· 有效期至 {formatDateTime(announcement.expire_at)}</span> : null}
                        </span>
                      }
                      trailing={
                        <span className="flex flex-wrap items-center justify-end gap-1.5">
                          <Badge tone="neutral">{announcementScopeLabel(announcement.scope)}</Badge>
                          {(announcement.target_roles ?? []).map((role) => (
                            <Badge key={role} tone="jade">
                              {userRoleLabel(role)}
                            </Badge>
                          ))}
                        </span>
                      }
                      selected={open}
                      onSelect={() => setExpanded(open ? undefined : announcement.id)}
                    />
                    {open ? (
                      <p className="whitespace-pre-wrap px-3 pb-4 pt-1 text-sm leading-relaxed text-ink">{announcement.content}</p>
                    ) : null}
                  </li>
                )
              })}
            </ul>
            <div className="mt-4">
              <Pagination
                page={announcements.page}
                pageSize={announcements.pageSize}
                total={announcements.total}
                onPageChange={announcements.setPage}
              />
            </div>
          </>
        )}
      </ResourceState>

      {createOpen ? (
        <AnnouncementFormModal
          publisher={publisher}
          onClose={() => setCreateOpen(false)}
          onPublished={() => {
            setCreateOpen(false)
            announcements.reload()
          }}
        />
      ) : null}
    </PageScaffold>
  )
}

interface AnnouncementFormModalProps {
  publisher: AnnouncementPublisher
  onClose: () => void
  onPublished: () => void
}

/**
 * AnnouncementFormModal 发布一条公告。
 * 定向公告必须选至少一个角色(后端 validateAnnouncementRequest 要求),
 * 全校公告则不能带角色 —— 故按范围切换时清空角色选择。
 * 平台发布方只有一种范围(全平台),此时不渲染范围选择器。
 */
function AnnouncementFormModal({ publisher, onClose, onPublished }: AnnouncementFormModalProps) {
  const isPlatform = publisher === 'platform'
  const [title, setTitle] = useState('')
  const [content, setContent] = useState('')
  const [scope, setScope] = useState(
    String(isPlatform ? AnnouncementScope.PLATFORM : AnnouncementScope.TENANT),
  )
  const [roles, setRoles] = useState<UserRole[]>([])
  const [expireAt, setExpireAt] = useState('')
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const scopeValue = Number(scope) as AnnouncementScope
  const isRoleScoped = scopeValue === AnnouncementScope.ROLES
  // 发布前的「将通知谁」:按范围与选中的角色写成一句话
  const audienceText = isPlatform
    ? '所有学校的师生与管理员'
    : isRoleScoped
      ? roles.length > 0
        ? `本校的${roles.map((role) => userRoleLabel(role)).join('、')}`
        : '(请先选择角色)'
      : '本校全部师生与管理员'
  const { errors, validate } = useFormValidation(useCallback(() => ({
    title: title.trim() === '' ? '请输入公告标题' : null,
    content: content.trim() === '' ? '请输入公告内容' : null,
    roles: isRoleScoped && roles.length === 0 ? '定向公告要选至少一个角色' : null,
  }), [content, isRoleScoped, roles, title]))

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (!validate()) return
      setFormError(undefined)
      setWorking(true)
      try {
        await api.notify.createAnnouncement({
          title: title.trim(),
          content: content.trim(),
          scope: scopeValue,
          // 只有定向公告能带角色(全校与全平台带角色会被后端拒绝)
          target_roles: isRoleScoped ? roles : [],
          expire_at: expireAt ? new Date(expireAt).toISOString() : undefined,
        })
        toast.success('公告已发布')
        onPublished()
      } catch (error) {
        setFormError(userFacingErrorMessage(error, '发布没有成功,请稍后重试。'))
      } finally {
        setWorking(false)
      }
    },
    [content, expireAt, isRoleScoped, onPublished, roles, scopeValue, title, validate],
  )

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="xl">
        <ModalHeader>
          <ModalTitle>发布公告</ModalTitle>
          <ModalDescription>公告会推送到对应人员的通知收件箱。</ModalDescription>
        </ModalHeader>
        <form onSubmit={submit} noValidate>
          <ModalBody className="flex flex-col gap-4">
            {isPlatform ? (
              <Callout tone="info" title="这条公告对全平台可见">
                所有学校的师生与管理员都会收到。平台公告不支持只发给某些角色。
              </Callout>
            ) : (
              <FormField label="可见范围" required>
                <SegmentedControl
                  aria-label="公告可见范围"
                  options={SCHOOL_SCOPE_OPTIONS.map((item) => ({
                    value: item.value,
                    label: item.label,
                  }))}
                  value={scope}
                  onValueChange={(value) => {
                    setScope(value)
                    setRoles([])
                  }}
                />
              </FormField>
            )}

            {isRoleScoped ? (
              <FormField label="定向角色" required helper="只有选中角色的人能看到这条公告" error={errors.roles}>
                <div
                  className="flex flex-col gap-2"
                  role="group"
                  aria-label="定向角色"
                  aria-invalid={Boolean(errors.roles)}
                  onBlur={() => validate(['roles'])}
                >
                  {TARGET_ROLE_OPTIONS.map((role) => (
                    <Checkbox
                      key={role}
                      checked={roles.includes(role)}
                      label={userRoleLabel(role)}
                      onCheckedChange={(checked) =>
                        setRoles((current) =>
                          checked === true ? [...current, role] : current.filter((item) => item !== role),
                        )
                      }
                    />
                  ))}
                </div>
              </FormField>
            ) : null}

            <FormField label="公告标题" htmlFor="announcement-title" required error={errors.title}>
              <Input
                id="announcement-title"
                value={title}
                invalid={Boolean(errors.title)}
                onBlur={() => validate(['title'])}
                onChange={(event) => setTitle(event.target.value)}
              />
            </FormField>

            <FormField
              label="公告内容"
              htmlFor="announcement-content"
              required
              helper="换行会原样保留"
              error={errors.content}
            >
              <Textarea
                id="announcement-content"
                value={content}
                rows={8}
                invalid={Boolean(errors.content)}
                onBlur={() => validate(['content'])}
                onChange={(event) => setContent(event.target.value)}
              />
            </FormField>

            <FormField
              label="有效期至"
              htmlFor="announcement-expire"
              helper="留空表示长期有效。过期后不再显示给师生"
            >
              <Input
                id="announcement-expire"
                type="datetime-local"
                value={expireAt}
                onChange={(event) => setExpireAt(event.target.value)}
              />
            </FormField>

            <p className="rounded-md bg-surface-hover px-4 py-3 text-sm text-ink">
              将通知:<span className="font-medium">{audienceText}</span>。发布后不能撤回或修改,有误请发一条新公告更正。
            </p>

            {formError ? <Callout tone="danger">{formError}</Callout> : null}
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" variant="seal" leftIcon={Send} loading={working}>
              确认发布
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  )
}
