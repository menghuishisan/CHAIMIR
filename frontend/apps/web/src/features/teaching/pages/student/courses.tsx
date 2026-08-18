// 我的课程页(学生侧栏首入口,/student/courses)。
// 数据只用 GET /teaching/courses?role=student 真实返回的字段:完成率需要课时总数,
// 只有单课程 outline 才同时给出总数与进度,按课程逐个调用会让一页产生二十次请求,
// 因此完成率放在课程详情页(见 docs/前端后端功能对齐清单.md §6.6)。

import { useCallback, useId, useState } from 'react'
import { useNavigate } from 'react-router'
import { BookOpen, GraduationCap, Layers, TicketCheck } from 'lucide-react'
import { CourseStatus, type Course } from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Card,
  CardBody,
  CardHeader,
  FilterBar,
  FilterField,
  FormField,
  Input,
  PageBody,
  PageHeader,
  PageScaffold,
  PageSection,
  Pagination,
  SegmentedControl,
  Stat,
  StatusIndicator,
  Table,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { usePagedResource, useResourceTotal } from '../../../../hooks'
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
  )
  const runningCount = useResourceTotal(
    (params) =>
      api.teaching.getCourses({ role: 'student', status: CourseStatus.RUNNING, ...params }),
    [],
  )
  const endedCount = useResourceTotal(
    (params) => api.teaching.getCourses({ role: 'student', status: CourseStatus.ENDED, ...params }),
    [],
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
        kicker={<Breadcrumb items={[{ label: '学习区' }, { label: '我的课程' }]} />}
        title="我的课程"
        description="这里是你已加入的课程。进入课程可以查看章节课时、作业和学习进度。"
        icon={BookOpen}
      />

      <PageSection>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <Stat label="已加入课程" value={totalCount ?? '—'} icon={BookOpen} hint="本学期及历史全部课程" />
          <Stat label="进行中" value={runningCount ?? '—'} icon={Layers} hint="正在上课的课程数" />
          <Stat label="已结课" value={endedCount ?? '—'} icon={GraduationCap} hint="成绩已可查看" />
        </div>
      </PageSection>

      <PageBody rail={<JoinCourseCard onJoined={courses.reload} />}>
        <PageSection title="课程列表" description={`共 ${courses.total} 门课程`}>
          <div className="flex flex-col gap-4">
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

            <ResourceState
              resource={courses}
              emptyIcon={BookOpen}
              emptyTitle="还没有加入任何课程"
              emptyDescription="向老师索取课程邀请码,在右侧填入即可加入课程开始学习。"
              skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
            >
              {(page) => (
                <div className="flex flex-col gap-4">
                  <Table
                    columns={columns}
                    data={page.list}
                    rowKey={(course) => course.id}
                    onRowClick={(course) => navigate(`/student/courses/${course.id}`)}
                  />
                  <Pagination
                    page={courses.page}
                    pageSize={courses.pageSize}
                    total={courses.total}
                    onPageChange={courses.setPage}
                  />
                </div>
              )}
            </ResourceState>
          </div>
        </PageSection>
      </PageBody>
    </PageScaffold>
  )
}

interface JoinCourseCardProps {
  /** 加入成功后刷新课程列表 */
  onJoined: () => void
}

/**
 * JoinCourseCard 是页面级动作卡:邀请码加入课程。
 * 邀请码是一次性短表单,提交即完成,无跨步骤中间态,故不落草稿(FE-7 只约束多步骤流程)。
 */
function JoinCourseCard({ onJoined }: JoinCourseCardProps) {
  const fieldId = useId()
  const [inviteCode, setInviteCode] = useState('')
  const [fieldError, setFieldError] = useState<string>()
  const [submitting, setSubmitting] = useState(false)

  const handleSubmit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      const code = inviteCode.trim()
      if (!code) {
        setFieldError('请输入课程邀请码')
        return
      }
      setFieldError(undefined)
      setSubmitting(true)
      try {
        await api.teaching.joinCourse({ invite_code: code })
        setInviteCode('')
        toast.success('已加入课程')
        onJoined()
      } catch (joinError) {
        setFieldError(userFacingErrorMessage(joinError, '加入课程失败,请确认邀请码后重试。'))
      } finally {
        setSubmitting(false)
      }
    },
    [inviteCode, onJoined],
  )

  return (
    <Card>
      <CardHeader
        title="加入新课程"
        description="老师会给出一串课程邀请码,填入后即可加入。"
      />
      <CardBody>
        <form onSubmit={handleSubmit} noValidate>
          <FormField label="课程邀请码" htmlFor={fieldId} required error={fieldError}>
            <Input
              id={fieldId}
              value={inviteCode}
              autoComplete="off"
              placeholder="请输入邀请码"
              invalid={Boolean(fieldError)}
              onChange={(event) => setInviteCode(event.target.value)}
            />
          </FormField>
          <Button type="submit" variant="primary" leftIcon={TicketCheck} loading={submitting} className="mt-4 w-full">
            加入课程
          </Button>
        </form>
      </CardBody>
    </Card>
  )
}
