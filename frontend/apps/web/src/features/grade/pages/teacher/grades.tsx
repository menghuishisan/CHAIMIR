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
import { FileText, Send } from 'lucide-react'
import {
  GradeReviewStatus,
  PAGINATION_MAX_SIZE,
  type Course,
  type GradeReview,
  type Semester,
} from '@chaimir/api-client'
import {
  Breadcrumb,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  DataPanel,
  Empty,
  FilterBar,
  FilterField,
  FormField,
  MetricStrip,
  PageBody,
  PageHeader,
  PageScaffold,
  PageSection,
  Pagination,
  SegmentedControl,
  Select,
  Table,
  Textarea,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, usePagedResource, useResourceTotal } from '../../../../hooks'
import { CourseGrades } from '../../../teaching/components/CourseGrades'
import { formatDateTime } from '../../../../utils/formatters'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { GradeReviewStatusCell } from '../../components/GradeReviewStatusCell'

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
    () => api.teaching.getCourses({ role: 'teacher', page: 1, size: PAGINATION_MAX_SIZE }),
    [],
    () => false
  )

  const semesters = useAsyncResource(
    () => api.grade.listSemesters(),
    [],
    () => false
  )

  const reviews = usePagedResource<GradeReview>(
    (params) =>
      api.grade.listOwnReviews({
        status: statusFilter ? (Number(statusFilter) as GradeReviewStatus) : undefined,
        ...params,
      }),
    [statusFilter]
  )

  const courseOptions = useMemo(
    () =>
      (courses.data?.list ?? []).map((course: Course) => ({
        value: course.id,
        label: course.name,
      })),
    [courses.data]
  )

  const courseNameById = useMemo(
    () => new Map((courses.data?.list ?? []).map((course: Course) => [course.id, course.name])),
    [courses.data]
  )

  // 指标带取服务端全量口径,不随下方状态筛选变化。
  // 「已锁定」没有服务端筛选参数,故不做这张卡:锁定状态在每一行里可见(规范 §6.5)。
  const totalCount = useResourceTotal((params) => api.grade.listOwnReviews(params), [])
  const pendingCount = useResourceTotal(
    (params) => api.grade.listOwnReviews({ status: GradeReviewStatus.PENDING, ...params }),
    []
  )
  const approvedCount = useResourceTotal(
    (params) => api.grade.listOwnReviews({ status: GradeReviewStatus.APPROVED, ...params }),
    []
  )
  const rejectedCount = useResourceTotal(
    (params) => api.grade.listOwnReviews({ status: GradeReviewStatus.REJECTED, ...params }),
    []
  )

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
      render: (review) => <GradeReviewStatusCell review={review} />,
    },
  ]

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '组织与成绩' }]} />}
        title="成绩报送"
        description="确认课程成绩后报送给学校审核。审核通过的成绩会锁定,进入学生的成绩中心。"
        icon={Send}
      />

      {/* 指标降为内联摘要(§6.5.3 第 ① 族):本页主体是报送记录与课程成绩明细 */}
      <MetricStrip
        label="报送总量摘要"
        className="mb-5"
        items={[
          { label: '我的报送', value: totalCount ?? '—', hint: '不受下方筛选影响' },
          { label: '待审核', value: pendingCount ?? '—', hint: '等待学校处理' },
          { label: '已通过', value: approvedCount ?? '—', hint: '成绩已锁定' },
          { label: '已驳回', value: rejectedCount ?? '—', hint: '改分后可重新报送' },
        ]}
      />

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
        {/*
          归族说明:这一页**不是**审阅队列族。教师在这里是报送方而不是处理方 ——
          记录状态由学校审核决定,本页列表是只读历史,真正的动作在右侧报送卡与下方课程成绩里。
          按 §6.5.3 归族判据「要逐条产出处理结论吗」答否,故走资源列表族(第 ① 族)。
        */}
        <DataPanel
          label="报送记录"
          filter={
            <FilterBar label="报送记录筛选">
              <FilterField label="审核状态" group>
                <SegmentedControl
                  aria-label="按审核状态筛选"
                  size="sm"
                  options={STATUS_FILTERS.map((item) => ({
                    value: item.value,
                    label: item.label,
                  }))}
                  value={statusFilter}
                  onValueChange={setStatusFilter}
                />
              </FilterField>
            </FilterBar>
          }
          footer={
            <Pagination
              page={reviews.page}
              pageSize={reviews.pageSize}
              total={reviews.total}
              onPageChange={reviews.setPage}
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
            skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading elevated={false} />}
          >
            {(page) => (
              <Table
                columns={columns}
                data={page.list}
                rowKey={(item) => item.id}
                elevated={false}
                // <md 换行卡(§6.4.1 规则 3):课程名一行、报送时间一行,状态在右
                mobileCard={(item) => ({
                  title: courseNameById.get(item.course_id) ?? '已归档的课程',
                  meta: `报送于 ${formatDateTime(item.submitted_at)}`,
                  badge: <GradeReviewStatusCell review={item} />,
                })}
              />
            )}
          </ResourceState>
        </DataPanel>
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
    [comment, onSubmitted, selectedCourseId, semesterId]
  )

  return (
    <Card>
      <CardHeader title="报送成绩" description="选择课程后先确认下方成绩,再报送给学校审核。" />
      <CardBody>
        <form onSubmit={submit} noValidate>
          <FormField label="课程" htmlFor="review-course" required>
            <Select
              id="review-course"
              options={courseOptions}
              value={selectedCourseId}
              placeholder={courseOptions.length > 0 ? '选择课程' : '暂无课程'}
              disabled={courseOptions.length === 0}
              onValueChange={(value) => {
                onCourseChange(value)
                setFieldError(undefined)
              }}
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
