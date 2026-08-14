// 成绩报送页(教师侧栏,/teacher/grades)。
//
// 两件事:把某门课程的成绩提交给学校审核、查看自己报送过的记录状态。
// 课程成绩(权重/计算/调分/导出)是本页的内页 —— 选定课程后在下方展开,
// 与课程管理里的「课程成绩」标签共用同一个 CourseGrades 组件,两处入口一个实现。
//
// 报送前需要先算出成绩:后端 SubmitReview 以课程成绩为依据,故本页把「配置权重 → 计算 → 报送」
// 排成同一条动线,而不是让教师在两个页面间来回跳。
//
// 申诉处理在批改中心;本页不重复放入口(对齐清单 §3.2:成绩申诉是批改/成绩内页,不进侧栏)。

import { useCallback, useMemo, useState } from 'react'
import { CircleCheck, ClipboardList, FileText, Lock, Send } from 'lucide-react'
import {
  GradeReviewStatus,
  type Course,
  type GradeReview,
  type Semester,
} from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  Empty,
  FormField,
  PageBody,
  PageHeader,
  PageScaffold,
  PageSection,
  Pagination,
  SegmentedControl,
  Select,
  Stat,
  StatusIndicator,
  Table,
  Textarea,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, usePagedResource } from '../../../../hooks'
import { CourseGrades } from '../../../teaching/pages/teacher/course-grades'
import { formatDateTime } from '../../../../utils/formatters'
import { gradeReviewStatusLabel, gradeReviewStatusTone } from '../../../../utils/labels/grade'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

/** 课程与学期选择器一次取回的条数:后端分页上限 100。 */
const PICKER_SIZE = 100

/** 状态筛选项:值为空串表示不过滤。 */
const STATUS_FILTERS = [
  { value: '', label: '全部' },
  { value: String(GradeReviewStatus.PENDING), label: '待审核' },
  { value: String(GradeReviewStatus.APPROVED), label: '已通过' },
  { value: String(GradeReviewStatus.REJECTED), label: '已驳回' },
] as const

/**
 * TeacherGradesPage 承载课程成绩维护与成绩报送。
 */
export default function TeacherGradesPage() {
  const [courseId, setCourseId] = useState<string>('')
  const [statusFilter, setStatusFilter] = useState<string>('')

  const courses = useAsyncResource(
    () => api.teaching.getCourses({ role: 'teacher', page: 1, size: PICKER_SIZE }),
    [],
    () => false,
  )

  const semesters = useAsyncResource(() => api.grade.listSemesters(), [], () => false)

  const reviews = usePagedResource<GradeReview>(
    (params) =>
      api.grade.listOwnReviews({
        status: statusFilter ? (Number(statusFilter) as GradeReviewStatus) : undefined,
        ...params,
      }),
    [statusFilter],
  )

  const courseOptions = useMemo(
    () => (courses.data?.list ?? []).map((course: Course) => ({ value: course.id, label: course.name })),
    [courses.data],
  )

  const courseNameById = useMemo(
    () => new Map((courses.data?.list ?? []).map((course: Course) => [course.id, course.name])),
    [courses.data],
  )

  const stats = useMemo(() => {
    const list = reviews.data ? reviews.data.list : []
    return {
      pending: list.filter((item) => item.status === GradeReviewStatus.PENDING).length,
      approved: list.filter((item) => item.status === GradeReviewStatus.APPROVED).length,
      locked: list.filter((item) => item.is_locked).length,
    }
  }, [reviews.data])

  const columns: TableColumn<GradeReview>[] = [
    {
      key: 'course_id',
      header: '课程',
      render: (review) => (
        <span className="text-ink">{courseNameById.get(review.course_id) ?? '已归档的课程'}</span>
      ),
    },
    {
      key: 'submitted_at',
      header: '报送时间',
      render: (review) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatDateTime(review.submitted_at)}
        </span>
      ),
    },
    {
      key: 'reviewed_at',
      header: '审核时间',
      render: (review) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {review.reviewed_at ? formatDateTime(review.reviewed_at) : '—'}
        </span>
      ),
    },
    {
      key: 'comment',
      header: '审核意见',
      render: (review) =>
        review.comment ? (
          <span className="line-clamp-2 text-sm text-ink-sub">{review.comment}</span>
        ) : (
          <span className="text-ink-sub">—</span>
        ),
    },
    {
      key: 'status',
      header: '状态',
      render: (review) => (
        <div className="flex flex-wrap items-center gap-1.5">
          <StatusIndicator
            tone={gradeReviewStatusTone(review.status)}
            label={gradeReviewStatusLabel(review.status)}
          />
          {review.is_locked ? <Badge tone="neutral">已锁定</Badge> : null}
        </div>
      ),
    },
  ]

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '组织与成绩' }, { label: '成绩报送' }]} />}
        title="成绩报送"
        description="确认课程成绩后报送给学校审核。审核通过的成绩会锁定,进入学生的成绩中心。"
        icon={Send}
      />

      <PageSection>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <Stat label="我的报送记录" value={reviews.total} icon={ClipboardList} />
          <Stat label="待审核" value={stats.pending} icon={Send} hint="等待学校处理" />
          <Stat label="已通过" value={stats.approved} icon={CircleCheck} />
          <Stat label="已锁定" value={stats.locked} icon={Lock} hint="锁定后不能再调分" />
        </div>
      </PageSection>

      <PageBody
        rail={
          <SubmitReviewCard
            courseOptions={courseOptions}
            semesters={semesters.data ?? []}
            selectedCourseId={courseId}
            onCourseChange={setCourseId}
            onSubmitted={reviews.reload}
          />
        }
      >
        <PageSection
          title="报送记录"
          description={`共 ${reviews.total} 条报送记录`}
          actions={
            <SegmentedControl
              aria-label="按审核状态筛选"
              size="sm"
              options={STATUS_FILTERS.map((item) => ({ value: item.value, label: item.label }))}
              value={statusFilter}
              onValueChange={setStatusFilter}
            />
          }
        >
          <ResourceState
            resource={reviews}
            emptyIcon={Send}
            emptyTitle={statusFilter ? '这个状态下没有记录' : '还没有报送过成绩'}
            emptyDescription={
              statusFilter ? '换个状态看看。' : '在右侧选择课程并确认成绩后即可报送给学校审核。'
            }
            skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
          >
            {(page) => (
              <div className="flex flex-col gap-4">
                <Table columns={columns} data={page.list} rowKey={(item) => item.id} />
                <Pagination
                  page={reviews.page}
                  pageSize={reviews.pageSize}
                  total={reviews.total}
                  onPageChange={reviews.setPage}
                />
              </div>
            )}
          </ResourceState>
        </PageSection>
      </PageBody>

      {/* 课程成绩内页:选定课程后在此维护权重、计算、调分与导出 */}
      {courseId ? (
        <PageSection
          title="课程成绩"
          description={`${courseNameById.get(courseId) ?? '所选课程'} 的成绩明细。报送前请先按权重计算成绩。`}
        >
          <CourseGrades courseId={courseId} />
        </PageSection>
      ) : (
        <PageSection title="课程成绩">
          <Empty
            icon={FileText}
            title="请先选择课程"
            description="在右侧选择要报送的课程,这里会展开该课程的成绩权重与全班成绩。"
          />
        </PageSection>
      )}
    </PageScaffold>
  )
}

interface SubmitReviewCardProps {
  courseOptions: { value: string; label: string }[]
  semesters: Semester[]
  selectedCourseId: string
  onCourseChange: (courseId: string) => void
  onSubmitted: () => void
}

/**
 * SubmitReviewCard 提交课程成绩审核。
 * 学期可缺省(后端按课程学期归属处理),选了则随请求带上。
 */
function SubmitReviewCard({
  courseOptions,
  semesters,
  selectedCourseId,
  onCourseChange,
  onSubmitted,
}: SubmitReviewCardProps) {
  const [semesterId, setSemesterId] = useState<string>('')
  const [comment, setComment] = useState('')
  const [fieldError, setFieldError] = useState<string>()
  const [submitting, setSubmitting] = useState(false)

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (!selectedCourseId) {
        setFieldError('请选择要报送的课程')
        return
      }
      setFieldError(undefined)
      setSubmitting(true)
      try {
        await api.grade.submitReview({
          course_id: selectedCourseId,
          semester_id: semesterId || undefined,
          comment: comment.trim() || undefined,
        })
        toast.success('成绩已报送,等待学校审核')
        setComment('')
        onSubmitted()
      } catch (error) {
        setFieldError(userFacingErrorMessage(error, '报送没有成功,请稍后重试。'))
      } finally {
        setSubmitting(false)
      }
    },
    [comment, onSubmitted, selectedCourseId, semesterId],
  )

  return (
    <Card>
      <CardHeader
        title="报送成绩"
        description="选择课程后先确认下方成绩,再报送给学校审核。"
      />
      <CardBody>
        <form onSubmit={submit} noValidate>
          <FormField label="课程" htmlFor="review-course" required>
            <Select
              id="review-course"
              options={courseOptions}
              value={selectedCourseId}
              placeholder={courseOptions.length > 0 ? '选择课程' : '暂无课程'}
              disabled={courseOptions.length === 0}
              onValueChange={onCourseChange}
            />
          </FormField>

          <FormField
            label="学期"
            htmlFor="review-semester"
            helper="不选则按课程所属学期归档"
            className="mt-4"
          >
            <Select
              id="review-semester"
              options={[
                { value: '', label: '按课程学期' },
                ...semesters.map((semester) => ({ value: semester.id, label: semester.name })),
              ]}
              value={semesterId}
              placeholder="按课程学期"
              onValueChange={setSemesterId}
            />
          </FormField>

          <FormField
            label="报送说明"
            htmlFor="review-comment"
            helper="可以写明调分依据,学校审核时会看到"
            error={fieldError}
            className="mt-4"
          >
            <Textarea
              id="review-comment"
              value={comment}
              rows={3}
              invalid={Boolean(fieldError)}
              onChange={(event) => setComment(event.target.value)}
            />
          </FormField>

          <Button
            type="submit"
            variant="seal"
            leftIcon={Send}
            loading={submitting}
            disabled={courseOptions.length === 0}
            className="mt-4 w-full"
          >
            报送成绩
          </Button>
        </form>

        <Callout tone="info" className="mt-4">
          报送后学校可以通过或驳回。通过即锁定,需要改分要请学校先解锁。
        </Callout>
      </CardBody>
    </Card>
  )
}
