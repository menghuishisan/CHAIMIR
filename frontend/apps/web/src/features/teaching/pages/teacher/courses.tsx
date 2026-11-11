// 课程管理页(教师侧栏,/teacher/courses)。设计 §7.4:
// 页头 +「新建课程」;分段「进行中 / 草稿 / 已结课 / 全部」+「共享状态」下拉(都写进地址,默认「进行中」);
// 卡片网格,卡片显示课程名、状态、学期 · 学分、开课区间、邀请码(带复制,只在已发布和进行中时显示)、共享标记;
// 整卡链向课程详情;卡片右上角更多菜单:发布 / 结课 / 归档 / 共享或取消共享 / 刷新邀请码 / 克隆,按状态只显示可用项。
// 创建成功后进入新课程的详情页继续编排章节。状态流转是有向的(草稿→发布→进行中→结课→归档),
// 结课、归档等需确认的操作弹窗写明后果。

import { useCallback, useRef, useState } from 'react'
import { Link, useNavigate } from 'react-router'
import { Book, BookPlus, ClipboardCopy, Copy, KeyRound, MoreVertical, Share2 } from 'lucide-react'
import { BOOL_FILTER, CourseStatus, CourseVisibility, type BoolFilter, type Course } from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  Card,
  Icon,
  IconButton,
  Menu,
  MenuContent,
  MenuItem,
  MenuSeparator,
  MenuTrigger,
  Modal,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  PageHeader,
  PageScaffold,
  Pagination,
  SegmentedControl,
  Select,
  Skeleton,
  StatusIndicator,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceError } from '../../../../components/ResourceState'
import { usePagedResource, useUrlState } from '../../../../hooks'
import { copyText } from '../../../../utils/clipboard'
import { formatDateRange } from '../../../../utils/formatters'
import { courseStatusLabel, courseTypeLabel } from '../../../../utils/labels/teaching'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { courseStatusTone } from '../../statusPresentation'
import { CourseFormModal } from '../../components/CourseFormModal'
import { CloneCourseModal } from '../../components/CloneCourseModal'

/** 地址参数:status 课程状态分段(默认进行中),shared 共享状态 */
const COURSES_QUERY = {
  status: { defaultValue: 'running', allowed: ['running', 'draft', 'ended', 'all'] },
  shared: { defaultValue: 'any', allowed: ['any', 'yes', 'no'] },
} as const

const STATUS_SEGMENTS = [
  { value: 'running', label: '进行中' },
  { value: 'draft', label: '草稿' },
  { value: 'ended', label: '已结课' },
  { value: 'all', label: '全部' },
] as const

/** statusOf 分段对应的课程状态(全部为 undefined) */
function statusOf(segment: string): CourseStatus | undefined {
  if (segment === 'running') return CourseStatus.RUNNING
  if (segment === 'draft') return CourseStatus.DRAFT
  if (segment === 'ended') return CourseStatus.ENDED
  return undefined
}

/** sharedOf 共享下拉对应的筛选值 */
function sharedOf(value: string): BoolFilter {
  if (value === 'yes') return BOOL_FILTER.YES
  if (value === 'no') return BOOL_FILTER.NO
  return BOOL_FILTER.ANY
}

/** 需要二次确认的状态流转:这三个动作不可逆或影响学生可见性。 */
type ConfirmAction = 'publish' | 'end' | 'archive'

const CONFIRM_COPY: Record<ConfirmAction, { title: string; description: string; confirm: string }> =
  {
    publish: {
      title: '发布课程',
      description: '发布后学生可以用邀请码加入,课程基础信息将不能再随意修改。',
      confirm: '发布课程',
    },
    end: {
      title: '结束课程',
      description: '结束后学生不能再提交作业,可以开始评价课程。你仍可查看全部数据。',
      confirm: '结束课程',
    },
    archive: {
      title: '归档课程',
      description: '归档后课程转为只读,学生不再看到它。归档不可撤销。',
      confirm: '归档课程',
    },
  }

/**
 * TeacherCoursesPage 列出本人课程并承载课程生命周期操作。
 */
export default function TeacherCoursesPage() {
  const navigate = useNavigate()
  const [query, setQuery] = useUrlState(COURSES_QUERY)
  const [createOpen, setCreateOpen] = useState(false)
  const [confirm, setConfirm] = useState<{ action: ConfirmAction; course: Course }>()
  const [cloneTarget, setCloneTarget] = useState<Course>()
  const [working, setWorking] = useState(false)
  const [actionError, setActionError] = useState<string>()
  const [sharingCourseId, setSharingCourseId] = useState<string>()
  const sharingCourseRef = useRef<string | undefined>(undefined)

  const courses = usePagedResource<Course>(
    (params) =>
      api.teaching.getCourses({
        role: 'teacher',
        status: statusOf(query.status),
        is_shared: sharedOf(query.shared),
        ...params,
      }),
    [query.shared, query.status],
    { urlPageKey: 'page' },
  )

  /** runStatusAction 执行状态流转并刷新列表。 */
  const runStatusAction = useCallback(async () => {
    if (!confirm) return
    setWorking(true)
    setActionError(undefined)
    try {
      if (confirm.action === 'publish') await api.teaching.publishCourse(confirm.course.id)
      if (confirm.action === 'end') await api.teaching.endCourse(confirm.course.id)
      if (confirm.action === 'archive') await api.teaching.archiveCourse(confirm.course.id)
      toast.success('课程状态已更新')
      setConfirm(undefined)
      courses.reload()
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '操作没有完成,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [confirm, courses])

  /** toggleCourseSharing 按权威可见性切换共享,请求期间只允许一次变更。 */
  const toggleCourseSharing = useCallback(
    async (course: Course) => {
      if (sharingCourseRef.current !== undefined) return
      sharingCourseRef.current = course.id
      setSharingCourseId(course.id)
      const shared = course.visibility === CourseVisibility.SHARED
      setActionError(undefined)
      try {
        if (shared) await api.teaching.unshareCourse(course.id)
        else await api.teaching.shareCourse(course.id)
        toast.success(shared ? '已取消共享' : '已共享到课程库')
        courses.reload()
      } catch (error) {
        setActionError(userFacingErrorMessage(error, '共享设置没有保存,请稍后重试。'))
      } finally {
        sharingCourseRef.current = undefined
        setSharingCourseId(undefined)
      }
    },
    [courses],
  )

  /** refreshInviteCode 刷新邀请码:旧邀请码随之失效。 */
  const refreshInviteCode = useCallback(
    async (course: Course) => {
      setActionError(undefined)
      try {
        const updated = await api.teaching.refreshInviteCode(course.id)
        toast.success(`新邀请码 ${updated.invite_code ?? ''},旧邀请码已失效`)
        courses.reload()
      } catch (error) {
        setActionError(userFacingErrorMessage(error, '刷新邀请码没有成功,请稍后重试。'))
      }
    },
    [courses],
  )

  const list = courses.data?.list ?? []
  const filtered = query.status !== 'all' || query.shared !== 'any'

  return (
    <PageScaffold>
      <PageHeader
        title="课程管理"
        actions={
          <Button variant="primary" leftIcon={BookPlus} onClick={() => setCreateOpen(true)}>
            新建课程
          </Button>
        }
      />

      <div className="mb-5 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <SegmentedControl
          aria-label="按课程状态筛选"
          options={STATUS_SEGMENTS.map((segment) => ({ value: segment.value, label: segment.label }))}
          value={query.status}
          onValueChange={(value) => setQuery({ status: value })}
        />
        <Select
          aria-label="按共享状态筛选"
          options={[
            { value: 'any', label: '共享状态不限' },
            { value: 'yes', label: '已共享' },
            { value: 'no', label: '未共享' },
          ]}
          value={query.shared}
          onValueChange={(value) => setQuery({ shared: value })}
          className="w-full sm:w-44"
        />
      </div>

      {actionError ? (
        <Callout tone="danger" className="mb-4">
          {actionError}
        </Callout>
      ) : null}

      {courses.status === 'error' ? (
        <ResourceError
          message={courses.error?.message ?? '课程暂时读取不到'}
          traceId={courses.error?.traceId}
          onRetry={courses.reload}
        />
      ) : courses.status === 'loading' && list.length === 0 ? (
        <ul className="card-grid gap-4" aria-busy="true" aria-label="正在读取课程">
          {Array.from({ length: 3 }, (_, index) => (
            <li key={index} className="flex flex-col gap-3 rounded-lg bg-surface p-5 shadow-xs">
              <Skeleton variant="title" width="half" />
              <Skeleton variant="line" lines={2} />
            </li>
          ))}
        </ul>
      ) : list.length === 0 ? (
        <div className="flex flex-col items-center rounded-lg bg-surface px-6 py-14 text-center shadow-xs">
          <Icon icon={Book} size="lg" className="text-ink-faint" />
          <p className="mt-3 text-md font-semibold text-ink">{filtered ? '这里暂时没有课程' : '还没有课程'}</p>
          <p className="mt-1.5 text-sm text-ink-sub">
            {filtered ? '切换到「全部」看看其他课程。' : '新建课程后可以添加章节课时、布置作业并邀请学生。'}
          </p>
          <div className="mt-5 flex flex-wrap justify-center gap-3">
            {filtered ? (
              <Button variant="outline" onClick={() => setQuery({ status: 'all', shared: 'any' })}>
                看全部课程
              </Button>
            ) : null}
            <Button variant="primary" leftIcon={BookPlus} onClick={() => setCreateOpen(true)}>
              新建课程
            </Button>
          </div>
        </div>
      ) : (
        <>
          <ul className="card-grid gap-4" aria-busy={courses.status === 'loading' || undefined}>
            {list.map((course) => (
              <li key={course.id}>
                <TeacherCourseCard
                  course={course}
                  menu={
                    <CourseActionMenu
                      course={course}
                      onManage={() => navigate(`/teacher/courses/${course.id}`)}
                      onRequestStatus={(action) => setConfirm({ action, course })}
                      sharing={sharingCourseId === course.id}
                      onShare={() => void toggleCourseSharing(course)}
                      onRefreshInvite={() => void refreshInviteCode(course)}
                      onClone={() => setCloneTarget(course)}
                    />
                  }
                />
              </li>
            ))}
          </ul>
          <div className="mt-6">
            <Pagination page={courses.page} pageSize={courses.pageSize} total={courses.total} onPageChange={courses.setPage} />
          </div>
        </>
      )}

      {createOpen ? (
        <CourseFormModal
          onClose={() => setCreateOpen(false)}
          onSaved={(created) => {
            setCreateOpen(false)
            navigate(`/teacher/courses/${created.id}`)
          }}
        />
      ) : null}

      {cloneTarget ? (
        <CloneCourseModal
          course={cloneTarget}
          onClose={() => setCloneTarget(undefined)}
          onCloned={() => {
            setCloneTarget(undefined)
            courses.reload()
          }}
        />
      ) : null}

      <Modal open={confirm !== undefined} onOpenChange={(open) => !open && setConfirm(undefined)}>
        <ModalContent size="sm">
          {confirm ? (
            <>
              <ModalHeader>
                <ModalTitle>
                  {CONFIRM_COPY[confirm.action].title}「{confirm.course.name}」?
                </ModalTitle>
                <ModalDescription>{CONFIRM_COPY[confirm.action].description}</ModalDescription>
              </ModalHeader>
              <ModalFooter>
                <Button variant="outline" onClick={() => setConfirm(undefined)}>
                  取消
                </Button>
                <Button
                  variant={confirm.action === 'archive' ? 'danger' : 'seal'}
                  loading={working}
                  onClick={() => void runStatusAction()}
                >
                  {CONFIRM_COPY[confirm.action].confirm}
                </Button>
              </ModalFooter>
            </>
          ) : null}
        </ModalContent>
      </Modal>
    </PageScaffold>
  )
}

/**
 * TeacherCourseCard 一门课:课程名(整卡链接)、状态、学期 · 学分 · 类型、开课区间、共享标记;
 * 已发布或进行中时显示邀请码与复制按钮;更多菜单与复制按钮浮在整卡链接之上单独可点。
 */
function TeacherCourseCard({ course, menu }: { course: Course; menu: React.ReactNode }) {
  const showInvite =
    Boolean(course.invite_code) && (course.status === CourseStatus.PUBLISHED || course.status === CourseStatus.RUNNING)
  return (
    <Card interactive className="flex h-full flex-col gap-2 p-5">
      <div className="flex items-start justify-between gap-2">
        <Link to={`/teacher/courses/${course.id}`} className="card-link line-clamp-2 text-lg font-bold text-ink">
          {course.name}
        </Link>
        <div className="relative z-base -mr-2 -mt-1 shrink-0">{menu}</div>
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <StatusIndicator tone={courseStatusTone(course.status)} label={courseStatusLabel(course.status)} />
        {course.visibility === CourseVisibility.SHARED ? <Badge tone="jade">已共享</Badge> : null}
      </div>
      <p className="text-sm text-ink-sub">
        {course.semester} · {course.credits} 学分 · {courseTypeLabel(course.type)}
      </p>
      <p className="font-mono text-xs tabular-nums text-ink-sub">{formatDateRange(course.start_at, course.end_at)}</p>
      {showInvite && course.invite_code ? (
        <div className="relative z-base mt-auto flex items-center justify-between gap-2 rounded-md bg-surface-hover px-3 py-2">
          <span className="text-xs text-ink-sub">
            邀请码 <span className="ml-1 font-mono text-sm tabular-nums text-ink">{course.invite_code}</span>
          </span>
          <IconButton
            variant="ghost"
            size="sm"
            icon={ClipboardCopy}
            aria-label={`复制课程「${course.name}」的邀请码`}
            onClick={() => {
              const code = course.invite_code ?? ''
              void copyText(code, { what: '邀请码', operation: 'teaching.course.copyInviteCode' }).then((ok) => {
                if (ok) toast.success('邀请码已复制')
              })
            }}
          />
        </div>
      ) : null}
    </Card>
  )
}

interface CourseActionMenuProps {
  course: Course
  sharing: boolean
  onManage: () => void
  onRequestStatus: (action: ConfirmAction) => void
  onShare: () => void
  onRefreshInvite: () => void
  onClone: () => void
}

/**
 * CourseActionMenu 按当前状态给出可执行动作。
 * 危险动作(归档)与常规动作用分隔线隔开,不与常规项同排(规范:危险操作视觉分离)。
 */
function CourseActionMenu({
  course,
  sharing,
  onManage,
  onRequestStatus,
  onShare,
  onRefreshInvite,
  onClone,
}: CourseActionMenuProps) {
  const canPublish = course.status === CourseStatus.DRAFT
  const canEnd = course.status === CourseStatus.RUNNING || course.status === CourseStatus.PUBLISHED
  const canArchive = course.status === CourseStatus.ENDED
  const shared = course.visibility === CourseVisibility.SHARED
  const canShare = shared || course.status !== CourseStatus.DRAFT
  const canRefreshInvite = course.status !== CourseStatus.ARCHIVED

  return (
    <Menu>
      <MenuTrigger asChild>
        <IconButton
          variant="ghost"
          size="sm"
          icon={MoreVertical}
          aria-label={`${course.name} 的更多操作`}
        />
      </MenuTrigger>
      <MenuContent align="end">
        <MenuItem icon={Book} onSelect={onManage}>
          管理内容
        </MenuItem>
        {canPublish ? (
          <MenuItem onSelect={() => onRequestStatus('publish')}>发布课程</MenuItem>
        ) : null}
        {canEnd ? <MenuItem onSelect={() => onRequestStatus('end')}>结束课程</MenuItem> : null}
        {canRefreshInvite ? (
          <MenuItem icon={KeyRound} onSelect={onRefreshInvite}>
            刷新邀请码
          </MenuItem>
        ) : null}
        {canShare ? (
          <MenuItem icon={Share2} disabled={sharing} onSelect={onShare}>
            {shared ? '取消共享' : '共享到课程库'}
          </MenuItem>
        ) : null}
        <MenuItem icon={Copy} onSelect={onClone}>
          克隆为新课程
        </MenuItem>
        {canArchive ? (
          <>
            <MenuSeparator />
            <MenuItem danger onSelect={() => onRequestStatus('archive')}>
              归档课程
            </MenuItem>
          </>
        ) : null}
      </MenuContent>
    </Menu>
  )
}
