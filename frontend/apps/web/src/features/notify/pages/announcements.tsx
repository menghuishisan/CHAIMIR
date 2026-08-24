// 系统公告页(校管侧栏 /school-admin/announcements、平台侧栏 /platform-admin/announcements 共用)。
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
  Breadcrumb,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  Checkbox,
  FormField,
  Input,
  MetricStrip,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  PageHeader,
  PageScaffold,
  PageSection,
  Pagination,
  SegmentedControl,
  Skeleton,
  Textarea,
  toast,
} from '@chaimir/ui'
import { api } from '../../../app/api'
import { ResourceState } from '../../../components/ResourceState'
import { usePagedResource } from '../../../hooks'
import { facetCount } from '../../../utils/facets'
import { formatDateTime } from '../../../utils/formatters'
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

/** 两个发布方的页面文案。 */
const PUBLISHER_COPY: Record<
  AnnouncementPublisher,
  { group: string; description: string; listDescription: string; emptyDescription: string }
> = {
  platform: {
    group: '底层资源',
    description: '向全平台所有学校发布公告。发布后会推送到各校师生的通知收件箱。',
    listDescription: '按发布时间从新到旧排列,这里是你发布过的平台公告。',
    emptyDescription: '发布公告后会推送到所有学校师生的通知收件箱。',
  },
  school: {
    group: '系统配置',
    description: '向全校或指定角色发布公告。发布后会推送到对应人员的通知收件箱。',
    listDescription: '按发布时间从新到旧排列,包含平台下发的公告。',
    emptyDescription: '发布公告后会推送到师生的通知收件箱。',
  },
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
  const copy = PUBLISHER_COPY[publisher]

  const announcements = usePagedResource<Announcement>(
    (params) => api.notify.getAnnouncements(params),
    [],
  )

  // 按范围分桶取后端 facets.scope:全量分组计数,不用当前页切片去数(§6.5.4)
  const platformCount = facetCount(
    announcements.data?.facets,
    'scope',
    AnnouncementScope.PLATFORM,
  )
  const tenantCount = facetCount(announcements.data?.facets, 'scope', AnnouncementScope.TENANT)
  const roleCount = facetCount(announcements.data?.facets, 'scope', AnnouncementScope.ROLES)

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: copy.group }]} />}
        title="系统公告"
        description={copy.description}
        icon={Megaphone}
        actions={
          <Button variant="primary" leftIcon={Plus} onClick={() => setCreateOpen(true)}>
            发布公告
          </Button>
        }
      />

      {/*
        归族:资源列表族的卡片列表形态(§6.5.3 第 ① 族 + §6.5.2 第二条出路)。
        为什么不是时间流族(第 ⑥):⑥ 的每条是「时间 + 状态点 + 一句主文」,
        而公告的主体是整段正文,压不进一行事件条。故按卡片列表排,卡本身就是抬起片,
        不再套 DataPanel(那会成为片里套片)。

        指标退为一行内联摘要:按范围分桶(平台/全校/定向)走后端聚合契约(facets.scope),
        是全量口径而非当前页切片(§6.5.4)。每条公告的定向角色仍在卡片标题的标签里可见。
      */}
      <MetricStrip
        label="公告总量摘要"
        className="mb-5"
        items={[
          { label: '公告总数', value: announcements.total, hint: '你能看到的全部' },
          { label: '平台公告', value: platformCount, hint: '所有学校可见' },
          { label: '全校公告', value: tenantCount, hint: '本校全员可见' },
          { label: '定向公告', value: roleCount, hint: '只发给指定角色' },
        ]}
      />

      <PageSection title="已发布公告" description={copy.listDescription}>
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
          skeleton={<Skeleton variant="line" lines={4} />}
        >
          {(page) => (
            <div className="flex flex-col gap-4">
              <div className="flex flex-col gap-3">
                {page.list.map((announcement) => (
                  <Card key={announcement.id}>
                    <CardHeader
                      title={
                        <span className="flex flex-wrap items-center gap-2">
                          <span className="min-w-0 truncate">{announcement.title}</span>
                          <Badge tone="neutral">{announcementScopeLabel(announcement.scope)}</Badge>
                          {announcement.target_roles && announcement.target_roles.length > 0 ? (
                            <span className="flex flex-wrap gap-1">
                              {announcement.target_roles.map((role) => (
                                <Badge key={role} tone="jade">
                                  {userRoleLabel(role)}
                                </Badge>
                              ))}
                            </span>
                          ) : null}
                        </span>
                      }
                      description={
                        announcement.expire_at
                          ? `${formatDateTime(announcement.published_at)} · 有效期至 ${formatDateTime(announcement.expire_at)}`
                          : formatDateTime(announcement.published_at)
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
            </div>
          )}
        </ResourceState>
      </PageSection>

      <Callout tone="info" className="mt-4">
        公告发布后不能撤回或修改。如果内容有误,请发一条新公告说明更正。
      </Callout>

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

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (title.trim() === '') {
        setFormError('请输入公告标题')
        return
      }
      if (content.trim() === '') {
        setFormError('请输入公告内容')
        return
      }
      if (isRoleScoped && roles.length === 0) {
        setFormError('定向公告要选至少一个角色')
        return
      }
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
    [content, expireAt, isRoleScoped, onPublished, roles, scopeValue, title],
  )

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="xl">
        <ModalHeader>
          <ModalTitle>发布公告</ModalTitle>
          <ModalDescription>
            公告会推送到对应人员的通知收件箱。发布后不能撤回,请确认内容无误。
          </ModalDescription>
        </ModalHeader>
        <form onSubmit={submit} noValidate>
          <ModalBody className="flex flex-col gap-4">
            <FormField label="公告标题" htmlFor="announcement-title" required>
              <Input
                id="announcement-title"
                value={title}
                onChange={(event) => setTitle(event.target.value)}
              />
            </FormField>

            <FormField
              label="公告内容"
              htmlFor="announcement-content"
              required
              helper="换行会原样保留"
            >
              <Textarea
                id="announcement-content"
                value={content}
                rows={8}
                onChange={(event) => setContent(event.target.value)}
              />
            </FormField>

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
              <FormField label="定向角色" required helper="只有选中角色的人能看到这条公告">
                <div className="flex flex-col gap-2">
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
