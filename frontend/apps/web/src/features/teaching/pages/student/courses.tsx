// 我的课程页(学生侧栏首入口,/student/courses)。
// 数据只用 GET /teaching/courses?role=student 真实返回的字段:完成率需要课时总数,
// 只有单课程 outline 才同时给出总数与进度,按课程逐个调用会让一页产生二十次请求,
// 因此完成率放在课程详情页(见 docs/前端后端功能对齐清单.md §6.6)。

import { useCallback, useId, useState } from 'react'
import { useNavigate } from 'react-router'
import { BookOpen, TicketCheck } from 'lucide-react'
import { CourseStatus, type Course } from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  DataPanel,
  FilterBar,
  FilterField,
  FormField,
  Input,
  MetricStrip,
  PageHeader,
  PageScaffold,
  Pagination,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalHeader,
  ModalTitle,
  SegmentedControl,
  StatusIndicator,
  Table,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { usePagedResource, useResourceTotal } from '../../../../hooks'
import { useFormValidation } from '../../../../hooks/useFormValidation'
import { formatDate } from '../../../../utils/formatters'
import {
  courseStatusLabel,
  courseTypeLabel,
  teachingDifficultyLabel,
} from '../../../../utils/labels/teaching'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { CourseIdentityCell } from '../../components/CourseIdentityCell'
import { courseStatusTone } from '../../statusPresentation'

/** 状态筛选项:值为空串表示不过滤(后端 status=0 即不过滤,由调用处转换)。 */
const STATUS_FILTERS = [
  { value: '', label: '全部' },
  { value: String(CourseStatus.RUNNING), label: '进行中' },
  { value: String(CourseStatus.PUBLISHED), label: '待开课' },
  { value: String(CourseStatus.ENDED), label: '已结课' },
] as const

/**
 * StudentCoursesPage 呈现已加入课程,并提供邀请码加入入口。
 */
export default function StudentCoursesPage() {
  const navigate = useNavigate()
  const [statusFilter, setStatusFilter] = useState<string>('')
  const [joinOpen, setJoinOpen] = useState(false)

  const courses = usePagedResource<Course>(
    (params) =>
      api.teaching.getCourses({
        role: 'student',
        status: statusFilter ? (Number(statusFilter) as CourseStatus) : undefined,
        ...params,
      }),
    [statusFilter],
  )

  // 指标带取服务端全量口径,不随下方状态筛选变化。
  // 「学分合计」需要跨全部课程求和,服务端没有这个聚合,故不做这张卡 ——
  // 原先带的「以当前页课程累计」只是给错数加了个说明(规范 §6.5);每门课的学分在行内可见。
  const totalCount = useResourceTotal(
    (params) => api.teaching.getCourses({ role: 'student', ...params }),
    [],
    courses.refreshVersion,
  )
  const runningCount = useResourceTotal(
    (params) =>
      api.teaching.getCourses({ role: 'student', status: CourseStatus.RUNNING, ...params }),
    [],
    courses.refreshVersion,
  )
  const endedCount = useResourceTotal(
    (params) => api.teaching.getCourses({ role: 'student', status: CourseStatus.ENDED, ...params }),
    [],
    courses.refreshVersion,
  )

  const columns: TableColumn<Course>[] = [
    {
      key: 'name',
      header: '课程',
      render: (course) => <CourseIdentityCell course={course} />,
    },
    {
      key: 'type',
      header: '类型',
      render: (course) => (
        <div className="flex flex-wrap items-center gap-1.5">
          <Badge tone="neutral">{courseTypeLabel(course.type)}</Badge>
          <Badge tone="jade">{teachingDifficultyLabel(course.difficulty)}</Badge>
        </div>
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
      key: 'status',
      header: '状态',
      render: (course) => (
        <StatusIndicator tone={courseStatusTone(course.status)} label={courseStatusLabel(course.status)} />
      ),
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (course) => (
        <Button variant="ghost" size="sm" onClick={() => navigate(`/student/courses/${course.id}`)}>
          进入学习
        </Button>
      ),
    },
  ]

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '学习区' }]} />}
        title="我的课程"
        description="这里是你已加入的课程。进入课程可以查看章节课时、作业和学习进度。"
        icon={BookOpen}
        actions={
          <Button variant="outline" leftIcon={TicketCheck} onClick={() => setJoinOpen(true)}>
            加入课程
          </Button>
        }
      />

      {/* 指标降为内联摘要(§6.5.3 第 ① 族):本页主体是课程列表,不是这三个数字 */}
      <MetricStrip
        label="课程总量摘要"
        className="mb-5"
        items={[
          { label: '已加入', value: totalCount ?? '—', hint: '本学期及历史全部课程' },
          { label: '进行中', value: runningCount ?? '—', hint: '正在上课的课程数' },
          { label: '已结课', value: endedCount ?? '—', hint: '成绩已可查看' },
        ]}
      />

      <div className="min-w-0">
        {/* 筛选井、数据表、分页同处一块抬起片(§6.5.2) */}
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
              emptyIcon={BookOpen}
              emptyTitle="还没有加入任何课程"
              emptyDescription="向老师索取课程邀请码,填入后即可开始学习。"
              emptyAction={
                <Button variant="primary" leftIcon={TicketCheck} onClick={() => setJoinOpen(true)}>
                  加入课程
                </Button>
              }
              skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading elevated={false} />}
            >
              {(page) => (
                <Table
                  columns={columns}
                  data={page.list}
                  rowKey={(course) => course.id}
                  elevated={false}
                  onRowClick={(course) => navigate(`/student/courses/${course.id}`)}
                  // <md 换行卡(§6.4.1 规则 3):课程名一行、学期与学分一行,状态在右
                  mobileCard={(course) => ({
                    title: course.name,
                    meta: `${course.semester} · ${course.credits} 学分`,
                    badge: (
                      <StatusIndicator
                        tone={courseStatusTone(course.status)}
                        label={courseStatusLabel(course.status)}
                      />
                    ),
                    action: (
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => navigate(`/student/courses/${course.id}`)}
                      >
                        进入
                      </Button>
                    ),
                  })}
                />
              )}
            </ResourceState>
        </DataPanel>
      </div>

      <Modal open={joinOpen} onOpenChange={setJoinOpen}>
        <ModalContent size="sm">
          <ModalHeader>
            <ModalTitle>加入新课程</ModalTitle>
            <ModalDescription>老师会给出一串课程邀请码,填入后即可加入。</ModalDescription>
          </ModalHeader>
          <ModalBody>
            <JoinCourseForm
              onJoined={() => {
                setJoinOpen(false)
                courses.reload()
              }}
            />
          </ModalBody>
        </ModalContent>
      </Modal>
    </PageScaffold>
  )
}

interface JoinCourseFormProps {
  /** 加入成功后刷新课程列表 */
  onJoined: () => void
}

/**
 * JoinCourseForm 是页面级动作表单:邀请码加入课程。
 * 邀请码是一次性短表单,提交即完成,无跨步骤中间态,故不落草稿(FE-7 只约束多步骤流程)。
 */
function JoinCourseForm({ onJoined }: JoinCourseFormProps) {
  const fieldId = useId()
  const [inviteCode, setInviteCode] = useState('')
  const [submitError, setSubmitError] = useState<string>()
  const [submitting, setSubmitting] = useState(false)
  const { errors, validate, reset } = useFormValidation(() => ({
    inviteCode: !inviteCode.trim() ? '请输入课程邀请码' : null,
  }))

  const handleSubmit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      const code = inviteCode.trim()
      if (!validate()) return
      setSubmitError(undefined)
      setSubmitting(true)
      try {
        await api.teaching.joinCourse({ invite_code: code })
        setInviteCode('')
        reset()
        toast.success('已加入课程')
        onJoined()
      } catch (joinError) {
        setSubmitError(userFacingErrorMessage(joinError, '加入课程失败,请确认邀请码后重试。'))
      } finally {
        setSubmitting(false)
      }
    },
    [inviteCode, onJoined, reset, validate],
  )

  return (
    <form onSubmit={handleSubmit} noValidate>
      <FormField label="课程邀请码" htmlFor={fieldId} required error={errors.inviteCode}>
        <Input
          id={fieldId}
          value={inviteCode}
          autoComplete="off"
          placeholder="请输入邀请码"
          invalid={Boolean(errors.inviteCode)}
          onChange={(event) => setInviteCode(event.target.value)}
          onBlur={() => validate(['inviteCode'])}
        />
      </FormField>
      {submitError ? <Callout tone="danger" className="mt-4">{submitError}</Callout> : null}
      <Button
        type="submit"
        variant="primary"
        leftIcon={TicketCheck}
        loading={submitting}
        className="mt-4 w-full"
      >
        加入课程
      </Button>
    </form>
  )
}
