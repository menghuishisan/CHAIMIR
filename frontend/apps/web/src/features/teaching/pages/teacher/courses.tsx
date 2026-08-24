// 课程管理页(教师侧栏,/teacher/courses)。
// 课程写操作全部在此页与课程编辑页:创建、发布、结束、归档、克隆、共享、刷新邀请码。
// 状态流转是有向的(草稿→发布→进行中→结课→归档),故动作按当前状态给出,
// 不把全部动作平铺成永远可点的按钮组。

import { useCallback, useState } from 'react'
import { useNavigate } from 'react-router'
import {
  Book,
  BookPlus,
  ClipboardCopy,
  Copy,
  KeyRound,
  MoreVertical,
  Share2,
} from 'lucide-react'
import {
  BOOL_FILTER,
  CourseStatus,
  CourseVisibility,
  type BoolFilter,
  type Course,
} from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  DataPanel,
  FilterBar,
  FilterField,
  FormField,
  IconButton,
  Input,
  Menu,
  MenuContent,
  MenuItem,
  MenuSeparator,
  MenuTrigger,
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
  Pagination,
  SegmentedControl,
  StatusIndicator,
  Table,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { usePagedResource, useResourceTotal } from '../../../../hooks'
import { copyText } from '../../../../utils/clipboard'
import { formatDate } from '../../../../utils/formatters'
import {
  courseStatusLabel,
  courseTypeLabel,
  teachingDifficultyLabel,
} from '../../../../utils/labels/teaching'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { CourseIdentityCell } from '../../components/CourseIdentityCell'
import { courseStatusTone } from '../../statusPresentation'
import { CourseFormModal } from '../../components/CourseFormModal'

/** 状态筛选项:值为空串表示不过滤。 */
const STATUS_FILTERS = [
  { value: '', label: '全部' },
  { value: String(CourseStatus.DRAFT), label: '草稿' },
  { value: String(CourseStatus.RUNNING), label: '进行中' },
  { value: String(CourseStatus.ENDED), label: '已结课' },
] as const

/**
 * SHARED_FILTERS 是「是否已共享」筛选项。
 * 布尔列没法用「缺省即不过滤」表达三种意图,全平台统一用 BoolFilter(0 不限 / 1 是 / 2 否)。
 */
const SHARED_FILTERS = [
  { value: BOOL_FILTER.ANY, label: '不限' },
  { value: BOOL_FILTER.YES, label: '已共享' },
  { value: BOOL_FILTER.NO, label: '未共享' },
] as const

/** 需要二次确认的状态流转:这三个动作不可逆或影响学生可见性。 */
type ConfirmAction = 'publish' | 'end' | 'archive'

const CONFIRM_COPY: Record<ConfirmAction, { title: string; description: string; confirm: string }> = {
  publish: {
    title: '确认发布课程',
    description: '发布后学生可以用邀请码加入,课程基础信息将不能再随意修改。',
    confirm: '确认发布',
  },
  end: {
    title: '确认结束课程',
    description: '结束后学生不能再提交作业,可以开始评价课程。你仍可查看全部数据。',
    confirm: '确认结束',
  },
  archive: {
    title: '确认归档课程',
    description: '归档后课程转为只读,学生不再看到它。归档不可撤销。',
    confirm: '确认归档',
  },
}

/**
 * TeacherCoursesPage 列出本人课程并承载课程生命周期操作。
 */
export default function TeacherCoursesPage() {
  const navigate = useNavigate()
  const [statusFilter, setStatusFilter] = useState<string>('')
  const [sharedFilter, setSharedFilter] = useState<BoolFilter>(BOOL_FILTER.ANY)
  const [createOpen, setCreateOpen] = useState(false)
  const [confirm, setConfirm] = useState<{ action: ConfirmAction; course: Course }>()
  const [cloneTarget, setCloneTarget] = useState<Course>()
  const [working, setWorking] = useState(false)
  const [actionError, setActionError] = useState<string>()

  const courses = usePagedResource<Course>(
    (params) =>
      api.teaching.getCourses({
        role: 'teacher',
        status: statusFilter ? (Number(statusFilter) as CourseStatus) : undefined,
        is_shared: sharedFilter,
        ...params,
      }),
    [sharedFilter, statusFilter],
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

  /** shareCourse 把课程共享到课程库,供其他教师克隆复用。 */
  const shareCourse = useCallback(
    async (course: Course) => {
      setActionError(undefined)
      try {
        await api.teaching.shareCourse(course.id)
        toast.success('已共享到课程库')
        courses.reload()
      } catch (error) {
        setActionError(userFacingErrorMessage(error, '共享没有成功,请稍后重试。'))
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
        toast.success(`新邀请码 ${updated.invite_code ?? ''}`)
        courses.reload()
      } catch (error) {
        setActionError(userFacingErrorMessage(error, '刷新邀请码没有成功,请稍后重试。'))
      }
    },
    [courses],
  )

  // 指标带取服务端全量口径,不随下方筛选变化(§6.5.4)。
  // 「已共享」现在也走服务端 is_shared 参数,不再用当前页切片数 —— 那是错数。
  const totalCount = useResourceTotal(
    (params) => api.teaching.getCourses({ role: 'teacher', ...params }),
    [],
  )
  const runningCount = useResourceTotal(
    (params) =>
      api.teaching.getCourses({ role: 'teacher', status: CourseStatus.RUNNING, ...params }),
    [],
  )
  const draftCount = useResourceTotal(
    (params) => api.teaching.getCourses({ role: 'teacher', status: CourseStatus.DRAFT, ...params }),
    [],
  )
  const endedCount = useResourceTotal(
    (params) => api.teaching.getCourses({ role: 'teacher', status: CourseStatus.ENDED, ...params }),
    [],
  )
  const sharedCount = useResourceTotal(
    (params) => api.teaching.getCourses({ role: 'teacher', is_shared: BOOL_FILTER.YES, ...params }),
    [],
  )

  const columns: TableColumn<Course>[] = [
    {
      key: 'name',
      header: '课程',
      render: (course) => (
        <CourseIdentityCell
          course={course}
          details={`${course.semester} · ${courseTypeLabel(course.type)} · ${teachingDifficultyLabel(course.difficulty)}`}
        />
      ),
    },
    { key: 'credits', header: '学分', align: 'right', mono: true },
    {
      key: 'schedule',
      header: '开课时间',
      render: (course) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatDate(course.start_at)} — {formatDate(course.end_at)}
        </span>
      ),
    },
    {
      key: 'invite_code',
      header: '邀请码',
      mono: true,
      // 本页的用途就是「把邀请码发给学生」,所以邀请码必须能一键复制,
      // 而不是让老师用鼠标框选一串等宽字符(规范 §6.5 动作就近)。
      render: (course) => {
        const inviteCode = course.invite_code
        if (!inviteCode) return '—'
        return (
          <span className="flex items-center gap-1">
            <span className="whitespace-nowrap">{inviteCode}</span>
            <IconButton
              variant="ghost"
              size="sm"
              icon={ClipboardCopy}
              aria-label={`复制课程「${course.name}」的邀请码`}
              onClick={() => {
                void copyText(inviteCode, {
                  what: '邀请码',
                  operation: 'teaching.course.copyInviteCode',
                }).then((ok) => {
                  if (ok) toast.success('邀请码已复制')
                })
              }}
            />
          </span>
        )
      },
    },
    {
      key: 'status',
      header: '状态',
      render: (course) => (
        <div className="flex flex-wrap items-center gap-1.5">
          <StatusIndicator tone={courseStatusTone(course.status)} label={courseStatusLabel(course.status)} />
          {course.visibility === CourseVisibility.SHARED ? <Badge tone="jade">已共享</Badge> : null}
        </div>
      ),
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (course) => (
        <div className="flex items-center justify-end gap-1">
          <Button variant="ghost" size="sm" onClick={() => navigate(`/teacher/courses/${course.id}`)}>
            管理内容
          </Button>
          <CourseActionMenu
            course={course}
            onRequestStatus={(action) => setConfirm({ action, course })}
            onShare={() => void shareCourse(course)}
            onRefreshInvite={() => void refreshInviteCode(course)}
            onClone={() => setCloneTarget(course)}
          />
        </div>
      ),
    },
  ]

  return (
    <PageScaffold>
      <PageHeader
        // 面包屑只到父级:末节与 h1 同名等于白占一行(§6.5.0 通则 1)
        kicker={<Breadcrumb items={[{ label: '教学' }]} />}
        title="课程管理"
        description="创建课程、组织章节课时与作业,把邀请码发给学生即可开课。"
        icon={Book}
        actions={
          <Button variant="primary" leftIcon={BookPlus} onClick={() => setCreateOpen(true)}>
            新建课程
          </Button>
        }
      />

      {/* 指标降为内联摘要(§6.5.3 第 ① 族):本页主体是列表,四张 Display 大卡会把表格推到折叠线以下。
          口径仍是服务端全量,不随下方状态筛选变化(§6.5.4) */}
      <MetricStrip
        label="课程总量摘要"
        className="mb-5"
        items={[
          { label: '课程总数', value: totalCount ?? '—', hint: '不受下方筛选影响' },
          { label: '进行中', value: runningCount ?? '—', hint: '学生可进入学习' },
          { label: '草稿', value: draftCount ?? '—', hint: '发布后学生才可加入' },
          { label: '已结课', value: endedCount ?? '—', hint: '可归档结算成绩' },
          { label: '已共享', value: sharedCount ?? '—', hint: '其他学校可复用' },
        ]}
      />

      {/* 动作失败就近内联(§6.7 C),排在数据区之前,不被表格滚动带走 */}
      {actionError ? (
        <Callout tone="danger" className="mb-4">
          {actionError}
        </Callout>
      ) : null}

      {/* 筛选井、数据表、分页同处一块抬起片(§6.5.2 / §6.5.3 第 ① 族):
          井不得直接摆在光面上,一个逻辑数据区也不该被渲染成三个并排的盒子 */}
      <DataPanel
        label="课程列表"
        filter={
          <FilterBar label="课程筛选">
            <FilterField label="课程状态" group>
              <SegmentedControl
                aria-label="按课程状态筛选"
                size="sm"
                options={STATUS_FILTERS.map((item) => ({ value: item.value, label: item.label }))}
                value={statusFilter}
                onValueChange={setStatusFilter}
              />
            </FilterField>
            <FilterField label="是否已共享" group>
              <SegmentedControl
                aria-label="按是否已共享筛选"
                size="sm"
                options={SHARED_FILTERS.map((item) => ({
                  value: String(item.value),
                  label: item.label,
                }))}
                value={String(sharedFilter)}
                onValueChange={(value) => setSharedFilter(Number(value) as BoolFilter)}
              />
            </FilterField>
          </FilterBar>
        }
        footer={
          <Pagination
            page={courses.page}
            pageSize={courses.pageSize}
            total={courses.total}
            onPageChange={courses.setPage}
          />
        }
      >
        <ResourceState
          resource={courses}
          emptyIcon={Book}
          emptyTitle={statusFilter ? '这个状态下没有课程' : '还没有课程'}
          emptyDescription={
            statusFilter ? '换个状态看看。' : '新建课程后可以添加章节课时、布置作业并邀请学生。'
          }
          emptyAction={
            statusFilter ? undefined : (
              <Button variant="primary" leftIcon={BookPlus} onClick={() => setCreateOpen(true)}>
                新建课程
              </Button>
            )
          }
          skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading elevated={false} />}
        >
          {(page) => (
            <Table
              columns={columns}
              data={page.list}
              rowKey={(course) => course.id}
              // 表格在 DataPanel 内部,抬起片由容器给(§6.5.1 不出现第三级)
              elevated={false}
              // <md 六列挤不进 343px,换行卡(§6.4.1 规则 3):课程名一行、次要信息一行、状态在右
              mobileCard={(course) => ({
                title: course.name,
                meta: `${course.credits} 学分 · ${course.semester} · ${course.invite_code ?? '无邀请码'}`,
                badge: (
                  <StatusIndicator
                    tone={courseStatusTone(course.status)}
                    label={courseStatusLabel(course.status)}
                  />
                ),
              })}
              onRowClick={(course) => navigate(`/teacher/courses/${course.id}`)}
            />
          )}
        </ResourceState>
      </DataPanel>

      {createOpen ? (
        <CourseFormModal
          onClose={() => setCreateOpen(false)}
          onSaved={() => {
            setCreateOpen(false)
            courses.reload()
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
                <ModalTitle>{CONFIRM_COPY[confirm.action].title}</ModalTitle>
                <ModalDescription>{CONFIRM_COPY[confirm.action].description}</ModalDescription>
              </ModalHeader>
              <ModalBody>
                <p className="text-base text-ink">{confirm.course.name}</p>
              </ModalBody>
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

interface CourseActionMenuProps {
  course: Course
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
  onRequestStatus,
  onShare,
  onRefreshInvite,
  onClone,
}: CourseActionMenuProps) {
  const canPublish = course.status === CourseStatus.DRAFT
  const canEnd = course.status === CourseStatus.RUNNING || course.status === CourseStatus.PUBLISHED
  const canArchive = course.status === CourseStatus.ENDED
  const canShare = course.visibility !== CourseVisibility.SHARED && course.status !== CourseStatus.DRAFT
  const canRefreshInvite = course.status !== CourseStatus.ARCHIVED

  return (
    <Menu>
      <MenuTrigger asChild>
        <IconButton variant="ghost" size="sm" icon={MoreVertical} aria-label={`${course.name} 的更多操作`} />
      </MenuTrigger>
      <MenuContent align="end">
        {canPublish ? <MenuItem onSelect={() => onRequestStatus('publish')}>发布课程</MenuItem> : null}
        {canEnd ? <MenuItem onSelect={() => onRequestStatus('end')}>结束课程</MenuItem> : null}
        {canRefreshInvite ? (
          <MenuItem icon={KeyRound} onSelect={onRefreshInvite}>
            刷新邀请码
          </MenuItem>
        ) : null}
        {canShare ? (
          <MenuItem icon={Share2} onSelect={onShare}>
            共享到课程库
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

interface CloneCourseModalProps {
  course: Course
  onClose: () => void
  onCloned: () => void
}

/**
 * CloneCourseModal 克隆课程为新草稿。
 * 克隆会复制章节课时与作业外壳(不含学生数据),结果落草稿态。
 */
function CloneCourseModal({ course, onClose, onCloned }: CloneCourseModalProps) {
  const [name, setName] = useState(`${course.name}(副本)`)
  const [fieldError, setFieldError] = useState<string>()
  const [working, setWorking] = useState(false)

  const submit = useCallback(async () => {
    if (name.trim() === '') {
      setFieldError('请输入新课程名称')
      return
    }
    setFieldError(undefined)
    setWorking(true)
    try {
      await api.teaching.cloneCourse(course.id, { name: name.trim() })
      toast.success('已克隆为新课程草稿')
      onCloned()
    } catch (error) {
      setFieldError(userFacingErrorMessage(error, '克隆没有成功,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [course.id, name, onCloned])

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="sm">
        <ModalHeader>
          <ModalTitle>克隆课程</ModalTitle>
          <ModalDescription>
            会复制章节课时与作业设置,不复制学生、提交与成绩。新课程为草稿状态。
          </ModalDescription>
        </ModalHeader>
        <ModalBody>
          <FormField label="新课程名称" htmlFor="clone-course-name" required error={fieldError}>
            <Input
              id="clone-course-name"
              value={name}
              invalid={Boolean(fieldError)}
              onChange={(event) => setName(event.target.value)}
            />
          </FormField>
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            取消
          </Button>
          <Button variant="primary" loading={working} onClick={() => void submit()}>
            确认克隆
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}
