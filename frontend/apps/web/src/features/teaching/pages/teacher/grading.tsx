// 批改中心页(教师侧栏,/teacher/grading)。设计 §7.4:
// 顶部一行「课程」「作业」两个下拉(都写进地址;从课程详情进来时已选好)+ 可点的计数
// 「待批改 n / 已出分 n / 迟交 n」;标签「学生提交 / 待人工评分 / 成绩申诉」(写进地址)。
// 「学生提交」为队列 + 详情:左侧队列每行显示学生姓名与学号(用本课成员列表解析)、第几次、
// 是否迟交、状态;右侧逐题显示学生答案、自动得分、人工得分输入和评语,底部「提交批改并看下一份」。
// 上下方向键切换队列项,Ctrl/⌘ + 回车提交;提交后自动跳到下一份待批改,没有则显示
// 「这份作业已全部批完」;当前选中的提交写进地址;窄屏变成队列页和详情页两级。
//
// 判题任务在本页只做「单任务重判、人工评分」两件教师能力(对齐清单 §6.4)。
// 成绩申诉是「学生对判分有异议」的下游动作,作为本页标签(对齐清单 §3.2)。

import { useCallback, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router'
import { CheckCheck, ChevronLeft, ClipboardCheck, RefreshCw, RotateCcw } from 'lucide-react'
import {
  BOOL_FILTER,
  PAGINATION_MAX_SIZE,
  SubmissionStatus,
  TENANT_MODULE,
  type AssignmentDetail,
  type JudgeTask,
  type Submission,
} from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  cn,
  FormField,
  Icon,
  Input,
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
  QueueDetailLayout,
  SegmentedControl,
  Select,
  Skeleton,
  StatusIndicator,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
  Textarea,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { collectAllPages } from '../../../../utils/allPages'
import { useSession } from '../../../../components/RoleGuard/session'
import { ResourceState } from '../../../../components/ResourceState'
import {
  useAsyncResource,
  usePagedResource,
  useReviewQueue,
  useUrlState,
} from '../../../../hooks'
import { useFormValidation } from '../../../../hooks/useFormValidation'
import { GradeAppeals } from '../../../grade/components/GradeAppeals'
import { isJudgeTaskActive } from '../../../judge/rules'
import { judgeTaskStatusTone } from '../../../judge/statusPresentation'
import { useRejudgeTask } from '../../../judge/useRejudgeTask'
import { facetCount, facetSum } from '../../../../utils/facets'
import { formatDateTime, formatRelativeOccurrence, formatScore } from '../../../../utils/formatters'
import { judgeTaskStatusLabel } from '../../../../utils/labels/judge'
import { submissionStatusLabel } from '../../../../utils/labels/teaching'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { submissionStatusTone } from '../../statusPresentation'
import { formatAssignmentAnswer, itemStatement } from '../../assignment-answers'
import { StudentName } from '../../components/StudentName'
import { useCourseMemberNames } from '../../useCourseMemberNames'

/** 待处理判题面板一次展示的条数:它是提示区不是全量列表。 */
const PENDING_JUDGE_SIZE = 10

/** 地址参数:tab 工作区,course / assignment 所选课程与作业,view 队列分段 */
const GRADING_QUERY = {
  tab: { defaultValue: 'submissions', allowed: ['submissions', 'manual', 'appeals'] },
  course: { defaultValue: '' },
  assignment: { defaultValue: '' },
  view: { defaultValue: 'all', allowed: ['all', 'pending', 'graded', 'late'] },
} as const

/** 当前选中的提交写在地址的这个参数里(单独写,不清页码) */
const SELECTED_PARAM = 'submission'

/**
 * TeacherGradingPage 按作业列出提交并承载批改。
 */
export default function TeacherGradingPage() {
  const { me } = useSession()
  const gradesEnabled = me.enabled_modules.includes(TENANT_MODULE.GRADE)
  const [query, setQuery] = useUrlState(GRADING_QUERY)
  const tab = !gradesEnabled && query.tab === 'appeals' ? 'submissions' : query.tab
  return (
    <PageScaffold>
      <PageHeader title="批改中心" />
      <Tabs value={tab} onValueChange={(value) => setQuery({ tab: value })}>
        <TabsList aria-label="批改工作区">
          <TabsTrigger value="submissions">学生提交</TabsTrigger>
          <TabsTrigger value="manual">待人工评分</TabsTrigger>
          {gradesEnabled ? <TabsTrigger value="appeals">成绩申诉</TabsTrigger> : null}
        </TabsList>
        <TabsContent value="submissions">
          <SubmissionsQueue />
        </TabsContent>
        <TabsContent value="manual">
          <JudgeTasksSection />
        </TabsContent>
        {gradesEnabled ? (
          <TabsContent value="appeals">
            <GradeAppeals scope="teacher" />
          </TabsContent>
        ) : null}
      </Tabs>
    </PageScaffold>
  )
}

/** viewFilter 队列分段对应的提交筛选 */
function viewFilter(view: string): { status?: SubmissionStatus; is_late?: typeof BOOL_FILTER.YES } {
  if (view === 'pending') return { status: SubmissionStatus.PENDING }
  if (view === 'graded') return { status: SubmissionStatus.GRADED }
  if (view === 'late') return { is_late: BOOL_FILTER.YES }
  return {}
}

/**
 * SubmissionsQueue 是批改的审阅队列:左队列 + 右详情与评分表单。
 * 两个下拉都一次取齐(PAGINATION_MAX_SIZE):下拉本身就该是可滚动的完整清单。
 * 作业详情(题面与所属课程)在队列层读一次,不随切换提交重复读取。
 */
function SubmissionsQueue() {
  const [query, setQuery] = useUrlState(GRADING_QUERY)
  const [searchParams, setSearchParams] = useSearchParams()
  const selectedParam = searchParams.get(SELECTED_PARAM) ?? undefined
  const [mobileView, setMobileView] = useState<'queue' | 'detail'>('queue')
  const [finished, setFinished] = useState(false)
  const [actionError, setActionError] = useState<string>()
  const assignmentId = query.assignment

  const detail = useAsyncResource<AssignmentDetail | null>(
    () => (assignmentId ? api.teaching.getAssignment(assignmentId) : Promise.resolve(null)),
    [assignmentId],
    () => false
  )
  // 课程可以只由作业推出(从别处链接进来时只带了作业)
  const courseId = query.course || detail.data?.assignment.course_id || ''
  const names = useCourseMemberNames([courseId])

  const courses = useAsyncResource(
    () => collectAllPages((params) => api.teaching.getCourses({ role: 'teacher', ...params }), PAGINATION_MAX_SIZE),
    [],
    () => false
  )
  const assignments = useAsyncResource(
    () =>
      courseId
        ? collectAllPages((params) => api.teaching.listCourseAssignments(courseId, params), PAGINATION_MAX_SIZE)
        : Promise.resolve(null),
    [courseId],
    () => false
  )
  const submissions = usePagedResource<Submission>(
    (params) =>
      assignmentId
        ? api.teaching.getSubmissions(assignmentId, { ...viewFilter(query.view), ...params })
        : Promise.resolve({
            list: [] as Submission[],
            total: 0,
            page: params.page,
            size: params.size,
          }),
    [assignmentId, query.view],
    { urlPageKey: 'page' }
  )
  // 分段计数取列表接口的 facets(接口总览「分段计数」):状态与是否迟交两组,不套用本组筛选。
  // 列表读取中或失败时数量未知,不显示数字。
  const facets = submissions.status === 'success' || submissions.status === 'empty' ? submissions.data?.facets : undefined
  const allCount = facetSum(facets, 'status')
  const pendingCount = facetCount(facets, 'status', SubmissionStatus.PENDING)
  const gradedCount = facetCount(facets, 'status', SubmissionStatus.GRADED)
  const lateCount = facetCount(facets, 'is_late', true)

  const list = useMemo(() => submissions.data?.list ?? [], [submissions.data])
  const selected = finished
    ? undefined
    : (list.find((item) => item.id === selectedParam) ?? list[0])
  const selectedIndex = selected ? list.findIndex((item) => item.id === selected.id) : -1

  /** select 把当前选中的提交写进地址(替换历史记录,不清页码) */
  const select = useCallback(
    (id: string | undefined) => {
      setFinished(false)
      setSearchParams(
        (previous) => {
          const params = new URLSearchParams(previous)
          if (id) params.set(SELECTED_PARAM, id)
          else params.delete(SELECTED_PARAM)
          return params
        },
        { replace: true }
      )
    },
    [setSearchParams]
  )
  const queue = useReviewQueue({
    items: list,
    currentId: selected?.id,
    onSelect: select,
    isPending: (item: Submission) => item.status !== SubmissionStatus.GRADED,
  })

  const courseOptions = (courses.data ?? []).map((course) => ({
    value: course.id,
    label: course.name,
  }))
  const assignmentOptions = (assignments.data ?? []).map((item) => ({
    value: item.id,
    label: item.title,
  }))

  return (
    <>
      <div className="mb-4 flex flex-col gap-3 lg:flex-row lg:items-center">
        <Select
          aria-label="选择课程"
          options={courseOptions}
          value={courseId}
          placeholder={courseOptions.length > 0 ? '选择课程' : '还没有课程'}
          disabled={courseOptions.length === 0}
          onValueChange={(value) => setQuery({ course: value, assignment: '' })}
          className="w-full lg:w-60"
        />
        <Select
          aria-label="选择作业"
          options={assignmentOptions}
          value={assignmentId}
          placeholder={
            courseId === ''
              ? '先选择课程'
              : assignmentOptions.length > 0
                ? '选择作业'
                : '这门课还没有作业'
          }
          disabled={assignmentOptions.length === 0}
          onValueChange={(value) => setQuery({ course: courseId, assignment: value })}
          className="w-full lg:w-72"
        />
        {assignmentId ? (
          <SegmentedControl
            aria-label="按批改状态筛选"
            size="sm"
            options={[
              { value: 'all', label: '全部', count: allCount },
              { value: 'pending', label: '待批改', count: pendingCount },
              { value: 'graded', label: '已出分', count: gradedCount },
              { value: 'late', label: '迟交', count: lateCount },
            ]}
            value={query.view}
            onValueChange={(value) => setQuery({ view: value })}
          />
        ) : null}
      </div>

      {courses.status === 'error' || assignments.status === 'error' ? (
        <Callout tone="danger" className="mb-4">
          <span className="flex flex-wrap items-center gap-3">
            {courses.status === 'error' ? '课程列表暂时读不到。' : '作业列表暂时读不到。'}
            <Button
              variant="outline"
              size="sm"
              onClick={courses.status === 'error' ? courses.reload : assignments.reload}
            >
              重新读取
            </Button>
          </span>
        </Callout>
      ) : null}
      {actionError ? (
        <Callout tone="danger" className="mb-4">
          {actionError}
        </Callout>
      ) : null}

      {assignmentId === '' ? (
        <div className="flex flex-col items-center rounded-lg bg-surface px-6 py-14 text-center shadow-xs">
          <Icon icon={ClipboardCheck} size="lg" className="text-ink-faint" />
          <p className="mt-3 text-md font-semibold text-ink">先选一份作业</p>
          <p className="mt-1.5 text-sm text-ink-sub">
            选好课程和作业后,这里会列出全班的提交,可以逐份批改。
          </p>
        </div>
      ) : (
        <QueueDetailLayout
          view={mobileView}
          detailHeader={
            selected ? (
              <div className="flex items-center gap-2">
                <Button
                  variant="ghost"
                  size="sm"
                  leftIcon={ChevronLeft}
                  onClick={() => setMobileView('queue')}
                >
                  返回队列
                </Button>
                <span className="text-sm tabular-nums text-ink-sub">
                  第 {selectedIndex + 1} 份 / 共 {submissions.total ?? '—'} 份
                </span>
              </div>
            ) : undefined
          }
          queue={
            <div className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-lg bg-surface shadow-xs">
              <ResourceState
                resource={submissions}
                emptyIcon={ClipboardCheck}
                emptyTitle={query.view === 'all' ? '还没有提交' : '这一类里没有提交'}
                emptyDescription={
                  query.view === 'all'
                    ? '学生提交作业后会出现在这里。'
                    : '换一类看看,或查看全部提交。'
                }
                skeleton={
                  <div aria-busy="true" className="p-4">
                    <Skeleton variant="line" lines={6} />
                  </div>
                }
              >
                {(page) => (
                  <div
                    role="listbox"
                    aria-label="提交队列"
                    aria-activedescendant={selected ? `submission-${selected.id}` : undefined}
                    tabIndex={0}
                    onKeyDown={queue.onListKeyDown}
                    className="min-h-0 flex-1 overflow-y-auto focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-accent"
                  >
                    {page.list.map((submission) => {
                      const active = submission.id === selected?.id
                      return (
                        <div
                          key={submission.id}
                          id={`submission-${submission.id}`}
                          role="option"
                          aria-selected={active}
                          onClick={() => {
                            select(submission.id)
                            setMobileView('detail')
                          }}
                          className={cn(
                            'cursor-pointer border-t border-line px-4 py-3 first:border-t-0',
                            active ? 'bg-primary-soft' : 'hover:bg-surface-hover'
                          )}
                        >
                          <div className="flex items-center justify-between gap-2">
                            <StudentName member={names.lookup(submission.student_id)} compact />
                            <StatusIndicator
                              tone={submissionStatusTone(submission.status)}
                              label={submissionStatusLabel(submission.status)}
                            />
                          </div>
                          <p className="mt-1 truncate text-xs text-ink-sub">
                            第 {submission.attempt_no} 次 ·{' '}
                            {formatRelativeOccurrence(submission.submitted_at)}
                            {submission.is_late ? ' · 迟交' : ''}
                            {submission.status === SubmissionStatus.GRADED
                              ? ` · ${formatScore(submission.final_score)} 分`
                              : ''}
                          </p>
                        </div>
                      )
                    })}
                  </div>
                )}
              </ResourceState>
              <div className="border-t border-line px-4 py-2 empty:hidden">
                <Pagination
                  page={submissions.page}
                  pageSize={submissions.pageSize}
                  total={submissions.total}
                  onPageChange={submissions.setPage}
                />
              </div>
              <p className="hidden border-t border-line px-4 py-2 text-xs text-ink-sub lg:block">
                ↑↓ 切换 · Ctrl/⌘ + 回车提交批改
              </p>
            </div>
          }
          detail={
            finished ? (
              <div className="flex min-h-48 flex-col items-center justify-center rounded-lg bg-surface p-6 text-center shadow-xs">
                <Icon icon={CheckCheck} size="lg" className="text-primary" />
                <p className="mt-3 text-md font-semibold text-ink">这份作业已全部批完</p>
                <p className="mt-1 text-sm text-ink-sub">点左侧任意一份可以再看或修改。</p>
              </div>
            ) : selected && detail.data ? (
              <SubmissionGradePane
                key={selected.id}
                submission={selected}
                detail={detail.data}
                student={<StudentName member={names.lookup(selected.student_id)} />}
                submitShortcut={queue.submitShortcut}
                onSaved={() => {
                  setActionError(undefined)
                  if (!queue.advance()) {
                    setFinished(true)
                    setMobileView('queue')
                  }
                  submissions.reload()
                }}
                onError={setActionError}
              />
            ) : (
              <div className="flex min-h-48 items-center justify-center rounded-lg bg-surface p-6 text-sm text-ink-sub shadow-xs">
                {detail.status === 'error'
                  ? '作业题目暂时读不到,请刷新后重试。'
                  : '左侧选一份提交,在这里给分并写评语。'}
              </div>
            )
          }
        />
      )}
    </>
  )
}

interface SubmissionGradePaneProps {
  submission: Submission
  /** 作业题面(队列层读一次) */
  detail: AssignmentDetail
  /** 学生姓名与学号的显示 */
  student: React.ReactNode
  /** Ctrl/⌘ + 回车提交的键盘处理 */
  submitShortcut: (submit: () => void) => (event: React.KeyboardEvent<HTMLElement>) => void
  onSaved: () => void
  onError: (message: string) => void
}

/**
 * SubmissionGradePane 是审阅队列族的右侧详情与处理表单(§6.5.3 第 ⑤ 族)。
 *
 * 评分不走弹窗:批改是本页的主动作,为看清一条提交而开窗、给完分再关窗,
 * 一个班的作业要重复几十次(§7.2 只在不可撤销的批量后果时才要二次确认,给分不是)。
 * 评语是给学生看的真实反馈,不提供模板按钮 ——
 * 旧前端用硬编码模板代替真实理由,那等于不告诉学生原因。
 *
 * key 由调用方按提交编号给出:切换条目时整个表单重建,
 * 上一条的草稿不会串到下一条(评分与评语都是每条独立的)。
 */
function SubmissionGradePane({
  submission,
  detail,
  student,
  submitShortcut,
  onSaved,
  onError,
}: SubmissionGradePaneProps) {
  const [score, setScore] = useState(String(submission.manual_score ?? ''))
  const [comment, setComment] = useState(submission.comment ?? '')
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)
  const { errors, validate } = useFormValidation(
    useCallback(
      () => ({
        score:
          score.trim() === '' ||
          !Number.isInteger(Number(score)) ||
          Number(score) < 0 ||
          Number(score) > submission.manual_max_score
            ? `请输入 0 至 ${submission.manual_max_score} 之间的整数分数`
            : null,
        comment: comment.trim() === '' ? '请写下评语,让学生知道得分依据' : null,
      }),
      [comment, score, submission.manual_max_score]
    )
  )

  const submit = useCallback(async () => {
    if (working || !validate()) return
    const value = Number(score)
    setFormError(undefined)
    setWorking(true)
    try {
      await api.teaching.gradeSubmission(submission.id, { score: value, comment: comment.trim() })
      toast.success('已提交批改结果')
      onSaved()
    } catch (error) {
      const message = userFacingErrorMessage(error, '批改没有保存成功,请稍后重试。')
      setFormError(message)
      onError(message)
    } finally {
      setWorking(false)
    }
  }, [comment, onError, onSaved, score, submission.id, validate, working])

  return (
    <div
      className="flex min-h-0 flex-1 flex-col gap-4 rounded-lg bg-surface p-5 shadow-xs"
      onKeyDown={submitShortcut(() => void submit())}
    >
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <h3 className="text-md">{student}</h3>
          <p className="mt-0.5 text-xs text-ink-sub">
            第 {submission.attempt_no} 次提交 · {formatDateTime(submission.submitted_at)}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-1.5">
          <StatusIndicator
            tone={submissionStatusTone(submission.status)}
            label={submissionStatusLabel(submission.status)}
          />
          {submission.is_late ? <Badge tone="warning">迟交</Badge> : null}
        </div>
      </div>

      <dl className="grid grid-cols-2 gap-3 rounded-md bg-surface-hover p-3 text-sm">
        <div>
          <dt className="text-ink-sub">自动得分</dt>
          <dd className="mt-0.5 font-semibold tabular-nums text-ink">
            {formatScore(submission.auto_score)}
          </dd>
        </div>
        <div>
          <dt className="text-ink-sub">当前最终得分</dt>
          <dd className="mt-0.5 font-semibold tabular-nums text-ink">
            {formatScore(submission.final_score)}
          </dd>
        </div>
      </dl>

      {detail.items.length === 0 ? (
        <p className="text-sm text-ink-sub">这份作业还没有题目。</p>
      ) : (
        <section aria-label="学生作答" className="flex flex-col gap-4">
          {detail.items.map((item, index) => (
            <div key={item.id} className="rounded-md bg-surface-hover p-4">
              <h4 className="text-sm font-medium text-ink">
                第 {index + 1} 题 · {item.title}
              </h4>
              <p className="mt-2 whitespace-pre-wrap break-words text-sm text-ink-sub">
                {itemStatement(item)}
              </p>
              <p className="mt-3 text-xs font-medium text-ink-sub">学生答案</p>
              <p className="mt-1 whitespace-pre-wrap break-words text-base text-ink">
                {formatAssignmentAnswer(item, submission.content[item.id])}
              </p>
            </div>
          ))}
        </section>
      )}

      {submission.manual_max_score > 0 ? (
        <>
          <FormField
            label="简答题评分"
            htmlFor="grade-score"
            required
            error={errors.score}
            helper={`本题组最高 ${submission.manual_max_score} 分`}
          >
            <Input
              id="grade-score"
              type="number"
              min="0"
              max={submission.manual_max_score}
              step="1"
              value={score}
              invalid={Boolean(errors.score)}
              onBlur={() => validate(['score'])}
              onChange={(event) => setScore(event.target.value)}
            />
          </FormField>

          <FormField
            label="评语"
            htmlFor="grade-comment"
            required
            error={errors.comment}
            helper="说清楚扣分点与改进方向,学生会在作业结果页看到"
          >
            <Textarea
              id="grade-comment"
              value={comment}
              rows={6}
              invalid={Boolean(errors.comment)}
              onBlur={() => validate(['comment'])}
              onChange={(event) => setComment(event.target.value)}
            />
          </FormField>

          {formError ? <Callout tone="danger">{formError}</Callout> : null}

          {/* 动作条贴详情片底边;<lg 由 QueueDetailLayout 钉到屏幕底部 */}
          <div className="sticky bottom-0 z-sticky -mx-5 -mb-5 mt-auto flex flex-wrap items-center justify-end gap-2 border-t border-line bg-surface px-5 py-4">
            <span className="mr-auto text-sm text-ink-sub">
              简答题评分将与自动判分合并,再按迟交规则计分。
            </span>
            <Button variant="primary" loading={working} onClick={() => void submit()}>
              提交批改并看下一份
            </Button>
          </div>
        </>
      ) : (
        <Callout tone="info">这份作业由系统自动判分,无需教师评分。</Callout>
      )}
    </div>
  )
}

/**
 * JudgeTasksSection 展示待人工评分的判题任务并承载评分与重判。
 * pending_manual 是后端提供的筛选:只列「判题器为人工评分且正在判题中」的任务
 * (SQL 条件是 status=判题中 且 judger.type=人工),这类任务在教师给分之前不会自己完成,
 * 故主动作是人工评分;重判留给判题异常的任务(在实时监控页按状态筛选)。
 *
 * 归族:这一块是页内子视图(§6.5.5 B),不自带页面头。它列的是任务而不是「逐条产出结论」
 * 的同一批对象(评分动作在弹窗里完成,给完即离场),故用资源列表族的片段而不是队列骨架。
 */
function JudgeTasksSection() {
  const [scoreTarget, setScoreTarget] = useState<JudgeTask>()

  const tasks = useAsyncResource(
    () => api.judge.getTasks({ pending_manual: true, page: 1, size: PENDING_JUDGE_SIZE }),
    [],
    (value) => value.list.length === 0
  )
  const { rejudge, rejudgingId, actionError } = useRejudgeTask(tasks.reload)

  return (
    <PageSection
      title="待人工评分"
      description={`判题方式为人工评分的提交在这里给分,给分后判题任务才算完成。一次最多列 ${PENDING_JUDGE_SIZE} 条。`}
    >
      <div className="flex flex-col gap-4">
        {actionError ? <Callout tone="danger">{actionError}</Callout> : null}

        <ResourceState
          resource={tasks}
          emptyIcon={RefreshCw}
          emptyTitle="没有待人工评分的任务"
          emptyDescription="需要教师给分的判题任务会出现在这里。"
          skeleton={<Skeleton variant="line" lines={3} />}
        >
          {(page) => (
            <div className={page.list.length > 1 ? 'grid gap-4 lg:grid-cols-2' : 'grid gap-4'}>
              {page.list.map((task) => (
                // 每条任务自成一块抬起片:任务卡是并列的处理入口,不是表格行
                <div
                  key={task.task_id}
                  className="flex flex-col gap-2 rounded-lg bg-surface p-4 shadow-xs"
                >
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <StatusIndicator
                      tone={judgeTaskStatusTone(task.status)}
                      label={judgeTaskStatusLabel(task.status)}
                      loading={isJudgeTaskActive(task.status)}
                    />
                    {task.result ? (
                      <span className="font-mono text-xs tabular-nums text-ink-sub">
                        {task.result.score} / {task.result.max_score}
                      </span>
                    ) : null}
                  </div>
                  {task.result && task.result.details.length > 0 ? (
                    <p className="line-clamp-2 text-xs text-ink-sub">
                      {task.result.details[0].hint ??
                        task.result.details[0].expected_label ??
                        '判题详情见结果'}
                    </p>
                  ) : null}
                  <div className="flex flex-wrap items-center gap-2">
                    <Button variant="outline" size="sm" onClick={() => setScoreTarget(task)}>
                      人工评分
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      leftIcon={RotateCcw}
                      loading={rejudgingId === task.task_id}
                      onClick={() => void rejudge(task.task_id)}
                    >
                      重新判题
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </ResourceState>
      </div>

      {scoreTarget ? (
        <ManualScoreModal
          task={scoreTarget}
          onClose={() => setScoreTarget(undefined)}
          onSaved={() => {
            setScoreTarget(undefined)
            tasks.reload()
          }}
        />
      ) : null}
    </PageSection>
  )
}

interface ManualScoreModalProps {
  task: JudgeTask
  onClose: () => void
  onSaved: () => void
}

/**
 * ManualScoreModal 给人工判题任务录入分数与结论。
 * 满分由教师给出:人工评分任务在给分前没有结果行,后端也不预设满分,
 * 只要求 0 ≤ 得分 ≤ 满分 且评语非空。
 */
function ManualScoreModal({ task, onClose, onSaved }: ManualScoreModalProps) {
  const [score, setScore] = useState(String(task.result?.score ?? ''))
  const [maxScore, setMaxScore] = useState(String(task.result?.max_score || 100))
  const [passed, setPassed] = useState('true')
  const [comment, setComment] = useState('')
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)
  const { errors, validate } = useFormValidation(
    useCallback(() => {
      const scoreValue = Number(score)
      const maxValue = Number(maxScore)
      return {
        maxScore: !Number.isFinite(maxValue) || maxValue <= 0 ? '满分需要是大于 0 的数字' : null,
        score:
          score.trim() === '' ||
          !Number.isFinite(scoreValue) ||
          scoreValue < 0 ||
          scoreValue > maxValue
            ? '得分需要在 0 与满分之间'
            : null,
        comment: comment.trim() === '' ? '请写下评分说明,学生会看到这段话' : null,
      }
    }, [comment, maxScore, score])
  )

  const submit = useCallback(async () => {
    if (!validate()) return
    const scoreValue = Number(score)
    const maxValue = Number(maxScore)
    setFormError(undefined)
    setWorking(true)
    try {
      await api.judge.manualScore(task.task_id, {
        score: scoreValue,
        max_score: maxValue,
        passed: passed === 'true',
        comment: comment.trim(),
      })
      toast.success('人工评分已提交,判题任务完成')
      onSaved()
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '评分没有保存成功,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [comment, maxScore, onSaved, passed, score, task.task_id, validate])

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="lg">
        <ModalHeader>
          <ModalTitle>人工评分</ModalTitle>
          <ModalDescription>
            这道题由教师判分。提交后判题任务完成,分数会回流到学生的提交记录。
          </ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
          <div className="grid gap-4 sm:grid-cols-2">
            <FormField label="得分" htmlFor="manual-score" required error={errors.score}>
              <Input
                id="manual-score"
                type="number"
                min="0"
                step="0.5"
                value={score}
                invalid={Boolean(errors.score)}
                onBlur={() => validate(['score'])}
                onChange={(event) => setScore(event.target.value)}
              />
            </FormField>
            <FormField
              label="满分"
              htmlFor="manual-max"
              required
              helper="这道题的分值上限"
              error={errors.maxScore}
            >
              <Input
                id="manual-max"
                type="number"
                min="1"
                value={maxScore}
                invalid={Boolean(errors.maxScore)}
                onBlur={() => validate(['maxScore'])}
                onChange={(event) => setMaxScore(event.target.value)}
              />
            </FormField>
          </div>

          <FormField label="判定结论" required helper="决定这次提交记为通过还是未通过">
            <SegmentedControl
              aria-label="判定结论"
              options={[
                { value: 'true', label: '通过' },
                { value: 'false', label: '未通过' },
              ]}
              value={passed}
              onValueChange={setPassed}
            />
          </FormField>

          <FormField
            label="评分说明"
            htmlFor="manual-comment"
            required
            helper="说清得分依据,学生会在提交结果里看到"
            error={errors.comment}
          >
            <Textarea
              id="manual-comment"
              value={comment}
              rows={4}
              invalid={Boolean(errors.comment)}
              onBlur={() => validate(['comment'])}
              onChange={(event) => setComment(event.target.value)}
            />
          </FormField>

          {formError ? <Callout tone="danger">{formError}</Callout> : null}
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            取消
          </Button>
          <Button variant="primary" loading={working} onClick={() => void submit()}>
            提交评分
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}
