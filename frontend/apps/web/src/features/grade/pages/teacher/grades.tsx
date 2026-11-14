// 成绩报送页(教师侧栏,/teacher/grades)。设计 §7.4:
// 左栏本学期课程列表,每行显示报送状态(未报送 / 待审核 / 已通过 / 已驳回),驳回的显示原因;
// 学期下拉在列表上方。右栏是所选课程的权重与全班成绩(学生姓名用成员列表解析),
// 底部「报送成绩」(可填备注,朱砂落印按钮);待审核或已通过时不可用并写原因。
// 所选课程与学期写进地址;默认选中第一门未报送的课。
//
// 报送以课程成绩为依据:「配置权重 → 计算 → 报送」在同一右栏里完成,与课程详情的「成绩」标签共用
// CourseGrades 组件。报送时不指定学期,由后端按课程学期名解析;课程学期未登记时写明原因。
// 学校暂未开放课程教学时,只显示自己的报送记录。申诉处理在批改中心,本页不重复放入口。

import { useCallback, useMemo, useState } from 'react'
import { Send } from 'lucide-react'
import { GradeReviewStatus, PAGINATION_MAX_SIZE, TENANT_MODULE, type Course, type GradeReview, type Semester } from '@chaimir/api-client'
import {
  Button,
  Callout,
  FormField,
  Icon,
  ListRow,
  PageHeader,
  PageScaffold,
  Select,
  Skeleton,
  StatusIndicator,
  Textarea,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceError } from '../../../../components/ResourceState'
import { useSession } from '../../../../components/RoleGuard/session'
import { useAsyncResource, useUrlState } from '../../../../hooks'
import { collectAllPages } from '../../../../utils/allPages'
import { formatDateTime } from '../../../../utils/formatters'
import { gradeReviewStatusLabel } from '../../../../utils/labels/grade'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { CourseGrades } from '../../../teaching/components/CourseGrades'
import { courseReportStates, defaultReportCourse, reportBlockedReason } from '../../reportStatus'
import { gradeReviewStatusTone } from '../../statusPresentation'

/** 地址参数:semester 学期(空为当前学期,all 为全部),course 所选课程 */
const GRADES_QUERY = {
  semester: { defaultValue: '' },
  course: { defaultValue: '' },
} as const

/**
 * TeacherGradesPage 承载课程成绩维护与成绩报送;教学模块未开放时只显示报送记录。
 */
export default function TeacherGradesPage() {
  const { me } = useSession()
  return (
    <PageScaffold>
      <PageHeader title="成绩报送" />
      {me.enabled_modules.includes(TENANT_MODULE.TEACHING) ? <ReportWorkspace /> : <ReviewHistoryOnly />}
    </PageScaffold>
  )
}

/** ReportWorkspace 左栏课程与报送状态,右栏所选课程的成绩与报送 */
function ReportWorkspace() {
  const [query, setQuery] = useUrlState(GRADES_QUERY)
  const courses = useAsyncResource(
    () => collectAllPages((params) => api.teaching.getCourses({ role: 'teacher', ...params }), PAGINATION_MAX_SIZE),
    [],
    () => false,
  )
  const semesters = useAsyncResource(() => api.grade.listSemesters(), [], () => false)
  const reviews = useAsyncResource(
    () => collectAllPages((params) => api.grade.listOwnReviews(params), PAGINATION_MAX_SIZE),
    [],
    () => false,
  )
  const states = useMemo(() => courseReportStates(reviews.data ?? []), [reviews.data])

  const currentSemester = semesters.data?.find((semester) => semester.is_current)
  const semesterValue = query.semester || currentSemester?.id || 'all'
  const semesterName = semesters.data?.find((semester) => semester.id === semesterValue)?.name
  const visibleCourses = useMemo(
    () => (courses.data ?? []).filter((course) => semesterValue === 'all' || course.semester === semesterName),
    [courses.data, semesterName, semesterValue],
  )
  const selectedId =
    visibleCourses.find((course) => course.id === query.course)?.id ??
    defaultReportCourse(
      visibleCourses.map((course) => course.id),
      states,
    )
  const selected = visibleCourses.find((course) => course.id === selectedId)

  if (courses.status === 'error') {
    return <ResourceError message={courses.error?.message ?? '课程暂时读取不到'} traceId={courses.error?.traceId} onRetry={courses.reload} />
  }

  return (
    <div className="flex flex-col gap-6 lg:flex-row lg:items-start">
      <section aria-label="课程与报送状态" className="flex flex-col gap-3 lg:w-80 lg:shrink-0">
        <Select
          aria-label="按学期筛选课程"
          options={[
            { value: 'all', label: '全部学期' },
            ...(semesters.data ?? []).map((semester: Semester) => ({
              value: semester.id,
              label: semester.is_current ? `${semester.name}(本学期)` : semester.name,
            })),
          ]}
          value={semesterValue}
          onValueChange={(value) => setQuery({ semester: value, course: '' })}
        />
        <div className="overflow-hidden rounded-lg bg-surface shadow-xs">
          {courses.status === 'loading' || reviews.status === 'loading' ? (
            <div aria-busy="true" className="p-4">
              <Skeleton variant="line" lines={4} />
            </div>
          ) : visibleCourses.length === 0 ? (
            <p className="px-4 py-8 text-center text-sm text-ink-sub">这个学期没有你的课程。</p>
          ) : (
            <ul className="divide-y divide-line">
              {visibleCourses.map((course) => {
                const state = states.get(course.id)
                return (
                  <li key={course.id}>
                    <ListRow
                      title={course.name}
                      meta={
                        state?.comment && state.status === GradeReviewStatus.REJECTED
                          ? `驳回原因:${state.comment}`
                          : state
                            ? `${formatDateTime(state.submitted_at)} 报送`
                            : course.semester
                      }
                      trailing={
                        state ? (
                          <StatusIndicator tone={gradeReviewStatusTone(state.status)} label={gradeReviewStatusLabel(state.status)} />
                        ) : (
                          <span className="text-sm text-ink-sub">未报送</span>
                        )
                      }
                      selected={course.id === selectedId}
                      onSelect={() => setQuery({ semester: query.semester, course: course.id })}
                    />
                  </li>
                )
              })}
            </ul>
          )}
        </div>
        {reviews.status === 'error' ? (
          <Callout tone="warning">
            <span className="flex flex-wrap items-center gap-2">
              报送状态暂时读不到。
              <Button variant="outline" size="sm" onClick={reviews.reload}>
                重新读取
              </Button>
            </span>
          </Callout>
        ) : null}
      </section>

      <section aria-label="所选课程的成绩" className="min-w-0 flex-1">
        {selected ? (
          <SelectedCourse
            key={selected.id}
            course={selected}
            semesters={semesters.data ?? []}
            semestersReady={semesters.status === 'success' || semesters.status === 'empty'}
            review={states.get(selected.id)}
            onSubmitted={reviews.reload}
          />
        ) : courses.status === 'loading' ? (
          <Skeleton variant="block" />
        ) : (
          <div className="rounded-lg bg-surface px-6 py-12 text-center text-sm text-ink-sub shadow-xs">在左侧选一门课。</div>
        )}
      </section>
    </div>
  )
}

/**
 * SelectedCourse 所选课程:课程名与报送状态、成绩权重与全班成绩、底部报送。
 * 报送时不指定学期(后端按课程学期名解析);课程没填学期或学期未登记时按钮不可用并写明原因。
 */
function SelectedCourse({
  course,
  semesters,
  semestersReady,
  review,
  onSubmitted,
}: {
  course: Course
  semesters: Semester[]
  semestersReady: boolean
  review: GradeReview | undefined
  onSubmitted: () => void
}) {
  const [comment, setComment] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState<string>()
  const blocked = reportBlockedReason(review)
  const semesterProblem = !semestersReady
    ? undefined
    : !course.semester
      ? '这门课还没填学期,请先在课程信息里补上'
      : !semesters.some((semester) => semester.name === course.semester)
        ? `学期「${course.semester}」还没有在学校登记,请联系学校管理员核对`
        : undefined
  const reason = blocked ?? semesterProblem

  const submit = useCallback(async () => {
    setSubmitting(true)
    setSubmitError(undefined)
    try {
      await api.grade.submitReview({ course_id: course.id, comment: comment.trim() || undefined })
      toast.success('成绩已报送,等待学校审核')
      setComment('')
      onSubmitted()
    } catch (error) {
      setSubmitError(userFacingErrorMessage(error, '报送没有成功,请稍后重试。'))
    } finally {
      setSubmitting(false)
    }
  }, [comment, course.id, onSubmitted])

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-wrap items-center gap-3">
        <h2 className="text-lg font-semibold text-ink">{course.name}</h2>
        {review ? (
          <StatusIndicator tone={gradeReviewStatusTone(review.status)} label={gradeReviewStatusLabel(review.status)} />
        ) : (
          <span className="text-sm text-ink-sub">未报送</span>
        )}
      </div>
      {review?.status === GradeReviewStatus.REJECTED && review.comment ? (
        <Callout tone="warning" title="学校驳回了上次报送">
          {review.comment}。按意见改好成绩后可以重新报送。
        </Callout>
      ) : null}

      <CourseGrades courseId={course.id} />

      <div className="sticky bottom-4 z-sticky flex flex-col gap-3 rounded-lg bg-surface p-4 shadow-md">
        {reason ? null : (
          <FormField label="报送说明" htmlFor="review-comment" helper="可以写明调分依据,学校审核时会看到(选填)">
            <Textarea id="review-comment" value={comment} rows={2} onChange={(event) => setComment(event.target.value)} />
          </FormField>
        )}
        {submitError ? <Callout tone="danger">{submitError}</Callout> : null}
        <div className="flex flex-wrap items-center justify-between gap-3">
          <span className="text-sm text-ink-sub">
            {reason ?? '报送前请先按权重计算成绩。学校通过后成绩会锁定,并进入学生的成绩中心。'}
          </span>
          <Button variant="seal" leftIcon={Send} loading={submitting} disabled={Boolean(reason)} onClick={() => void submit()}>
            报送成绩
          </Button>
        </div>
      </div>
    </div>
  )
}

/**
 * ReviewHistoryOnly 学校暂未开放课程教学时只列自己的报送记录(不从关闭的教学模块取课程名,
 * 按学期与报送时间区分)。
 */
function ReviewHistoryOnly() {
  const reviews = useAsyncResource(
    () => collectAllPages((params) => api.grade.listOwnReviews(params), PAGINATION_MAX_SIZE),
    [],
  )
  const semesters = useAsyncResource(() => api.grade.listSemesters(), [], () => false)
  const semesterName = useMemo(
    () => new Map((semesters.data ?? []).map((semester) => [semester.id, semester.name])),
    [semesters.data],
  )
  return (
    <div className="flex max-w-3xl flex-col gap-4">
      <Callout tone="info">学校暂未开放课程教学,可以查看已有的报送记录;开放后可以维护课程成绩并报送。</Callout>
      {reviews.status === 'error' ? (
        <ResourceError message={reviews.error?.message ?? '报送记录暂时读取不到'} traceId={reviews.error?.traceId} onRetry={reviews.reload} />
      ) : reviews.status === 'loading' ? (
        <Skeleton variant="line" lines={4} />
      ) : (reviews.data ?? []).length === 0 ? (
        <div className="flex flex-col items-center rounded-lg bg-surface px-6 py-12 text-center shadow-xs">
          <Icon icon={Send} size="lg" className="text-ink-faint" />
          <p className="mt-3 text-md font-semibold text-ink">还没有报送过成绩</p>
        </div>
      ) : (
        <ul className="divide-y divide-line overflow-hidden rounded-lg bg-surface shadow-xs">
          {[...(reviews.data ?? [])]
            .sort((a, b) => b.submitted_at.localeCompare(a.submitted_at))
            .map((review) => (
              <li key={review.id}>
                <ListRow
                  title={`${review.semester_id ? (semesterName.get(review.semester_id) ?? '未登记学期') : '未登记学期'} · 课程成绩`}
                  meta={review.comment ? `审核意见:${review.comment}` : `${formatDateTime(review.submitted_at)} 报送`}
                  trailing={<StatusIndicator tone={gradeReviewStatusTone(review.status)} label={gradeReviewStatusLabel(review.status)} />}
                />
              </li>
            ))}
        </ul>
      )}
    </div>
  )
}
