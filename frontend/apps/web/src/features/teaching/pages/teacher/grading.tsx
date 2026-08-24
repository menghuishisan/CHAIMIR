// 批改中心页(教师侧栏,/teacher/grading)。
//
// 一页解决「按作业看提交 → 批改主观题 → 处理判题异常」三件事。
// 作业上下文可从课程详情页带入(?assignment=),也可在本页选课程再选作业 ——
// 不让教师手填作业编号(旧前端批改中心要求手输作业/课程/提交编号,实际不可用)。
//
// 判题任务在本页只做「查询进度、单任务重判、人工评分」三件教师能力,
// 不触碰内部任务创建与批量重判(对齐清单 §6.4)。
//
// 成绩申诉也在本页:它是「学生对判分有异议」的下游动作,与批改是同一件事的两端,
// 故作为本页内区块而不进侧栏(对齐清单 §3.2:成绩申诉是批改/成绩内页)。
//
// 归族:审阅队列族(规范 §6.5.3 第 ⑤ 族)。教师在这里的动作是「逐条把提交批完」,
// 不是「在一堆记录里找一条」,故左队列常驻、右侧是当前提交的详情与评分表单,
// 提交后自动落到下一条待批改项。按该族约束:无指标带(待办数量放队列头一行)、
// 筛选(选课程与作业)并入队列头、键盘上下切条。
//
// 三块工作区用 Tabs 分开而不是纵向堆三个队列:同一页出现两套「左队列右详情」
// 会让人不知道哪一栏在响应自己的操作(§6.5.0 通则 1 不重复表达同一结构)。

import { useCallback, useMemo, useState, type KeyboardEvent } from 'react'
import { useSearchParams } from 'react-router'
import { CheckSquare, ChevronLeft, ClipboardCheck, RefreshCw, RotateCcw } from 'lucide-react'
import {
  BOOL_FILTER,
  type BoolFilter,
  PAGINATION_MAX_SIZE,
  SubmissionStatus,
  type Assignment,
  type Course,
  type JudgeTask,
  type Submission,
} from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  cn,
  DataPanel,
  DescriptionList,
  Empty,
  FilterBar,
  FilterField,
  FormField,
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
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, usePagedResource, useResourceTotal } from '../../../../hooks'
import { GradeAppeals } from '../../../grade/components/GradeAppeals'
import { isJudgeTaskActive } from '../../../judge/rules'
import { judgeTaskStatusTone } from '../../../judge/statusPresentation'
import { useRejudgeTask } from '../../../judge/useRejudgeTask'
import { formatDateTime, formatScore } from '../../../../utils/formatters'
import { judgeTaskStatusLabel } from '../../../../utils/labels/judge'
import { submissionStatusLabel } from '../../../../utils/labels/teaching'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { submissionStatusTone } from '../../statusPresentation'

/** 待处理判题面板一次展示的条数:它是提示区不是全量列表。 */
const PENDING_JUDGE_SIZE = 10

/**
 * TeacherGradingPage 按作业列出提交并承载批改。
 */
export default function TeacherGradingPage() {
  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '教学' }]} />}
        title="批改中心"
        description="选择作业查看学生提交。主观题在这里给分评语,编程题的判题异常可以重判。学生的成绩申诉也在本页处理。"
        icon={CheckSquare}
      />

      <Tabs defaultValue="submissions">
        <TabsList aria-label="批改工作区">
          <TabsTrigger value="submissions">学生提交</TabsTrigger>
          <TabsTrigger value="manual">待人工评分</TabsTrigger>
          <TabsTrigger value="appeals">成绩申诉</TabsTrigger>
        </TabsList>

        <TabsContent value="submissions">
          <SubmissionsQueue />
        </TabsContent>

        <TabsContent value="manual">
          <JudgeTasksSection />
        </TabsContent>

        <TabsContent value="appeals">
          {/* 申诉本身也是审阅队列族,由 GradeAppeals 自带该族骨架 */}
          <GradeAppeals />
        </TabsContent>
      </Tabs>
    </PageScaffold>
  )
}

/**
 * SubmissionsQueue 是批改的审阅队列:左队列 + 右详情与评分表单(§6.5.3 第 ⑤ 族)。
 *
 * 课程与作业是「取哪些数据」的选择,故按 §6.5.2 归入队列头的 FilterBar,
 * 不再单独占一张「选择要批改的作业」卡。两个下拉都一次取齐(PAGINATION_MAX_SIZE):
 * 给下拉配翻页控件等于让教师先翻页再选,而下拉本身就该是可滚动的完整清单。
 */
function SubmissionsQueue() {
  const [searchParams, setSearchParams] = useSearchParams()
  const assignmentId = searchParams.get('assignment') ?? ''
  const [courseId, setCourseId] = useState<string>('')
  const [statusFilter, setStatusFilter] = useState<string>('')
  const [lateFilter, setLateFilter] = useState<BoolFilter>(BOOL_FILTER.ANY)
  const [selectedId, setSelectedId] = useState<string>()
  /** <lg 走两级页面(§6.4.1 规则 4) */
  const [mobileView, setMobileView] = useState<'queue' | 'detail'>('queue')
  const [actionError, setActionError] = useState<string>()

  const courses = useAsyncResource(
    () => api.teaching.getCourses({ role: 'teacher', page: 1, size: PAGINATION_MAX_SIZE }),
    [],
    () => false,
  )

  const assignments = useAsyncResource(
    () =>
      courseId
        ? api.teaching.listCourseAssignments(courseId, { page: 1, size: PAGINATION_MAX_SIZE })
        : Promise.resolve({ list: [] as Assignment[], total: 0, page: 1, size: PAGINATION_MAX_SIZE }),
    [courseId],
    () => false,
  )

  const submissions = usePagedResource<Submission>(
    (params) =>
      assignmentId
        ? api.teaching.getSubmissions(assignmentId, {
            status: statusFilter ? (Number(statusFilter) as SubmissionStatus) : undefined,
            is_late: lateFilter,
            ...params,
          })
        : Promise.resolve({ list: [] as Submission[], total: 0, page: params.page, size: params.size }),
    [assignmentId, lateFilter, statusFilter],
  )

  const courseOptions = useMemo(
    () => (courses.data?.list ?? []).map((course: Course) => ({ value: course.id, label: course.name })),
    [courses.data],
  )

  const assignmentOptions = useMemo(
    () => (assignments.data?.list ?? []).map((item: Assignment) => ({ value: item.id, label: item.title })),
    [assignments.data],
  )

  const list = useMemo(() => submissions.data?.list ?? [], [submissions.data])
  const selected = list.find((item) => item.id === selectedId) ?? list[0]
  const selectedIndex = selected ? list.findIndex((item) => item.id === selected.id) : -1

  // 待办摘要取服务端全量口径:待批改是教师最常看的数字,不能用当前页数出来的近似值。
  // 「迟交」现在也走服务端 is_late 参数,故一并给出全量数(§6.5.4)。
  const pendingCount = useResourceTotal(
    (params) =>
      assignmentId
        ? api.teaching.getSubmissions(assignmentId, { status: SubmissionStatus.PENDING, ...params })
        : Promise.resolve({ list: [] as Submission[], total: 0, page: 1, size: 1 }),
    [assignmentId],
  )
  const gradedCount = useResourceTotal(
    (params) =>
      assignmentId
        ? api.teaching.getSubmissions(assignmentId, { status: SubmissionStatus.GRADED, ...params })
        : Promise.resolve({ list: [] as Submission[], total: 0, page: 1, size: 1 }),
    [assignmentId],
  )
  const lateCount = useResourceTotal(
    (params) =>
      assignmentId
        ? api.teaching.getSubmissions(assignmentId, { is_late: BOOL_FILTER.YES, ...params })
        : Promise.resolve({ list: [] as Submission[], total: 0, page: 1, size: 1 }),
    [assignmentId],
  )

  /** 批完一条直接落到下一条待批改项;没有下一条就退回队列层 */
  const advanceToNext = useCallback(() => {
    const nextPending = list.find(
      (item, index) => index > selectedIndex && item.status !== SubmissionStatus.GRADED,
    )
    setSelectedId(nextPending?.id ?? undefined)
    if (!nextPending) setMobileView('queue')
  }, [list, selectedIndex])

  /** 键盘上下切条:整批批改靠键盘才快 */
  const onQueueKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return
    event.preventDefault()
    const step = event.key === 'ArrowDown' ? 1 : -1
    const next = list[Math.min(Math.max(selectedIndex + step, 0), list.length - 1)]
    if (next) setSelectedId(next.id)
  }

  return (
    <>
      {/* 待办摘要一行(§6.5.3 第 ⑤ 族:审阅队列不做指标带,待办数量放标题下一行) */}
      <p className="mb-4 text-sm text-ink-sub">
        {assignmentId === ''
          ? '先选课程与作业,这里会显示这次作业的待批改数量。'
          : `共 ${submissions.total} 次提交,待批改 ${pendingCount ?? '—'} 次、已出分 ${gradedCount ?? '—'} 次、迟交 ${lateCount ?? '—'} 次。`}
      </p>

      {courses.status === 'error' ? (
        <Callout tone="danger" title="课程目录暂时读不到" className="mb-4">
          <Button variant="outline" size="sm" onClick={courses.reload}>
            重新加载课程
          </Button>
        </Callout>
      ) : null}
      {assignments.status === 'error' ? (
        <Callout tone="danger" title="作业目录暂时读不到" className="mb-4">
          <Button variant="outline" size="sm" onClick={assignments.reload}>
            重新加载作业
          </Button>
        </Callout>
      ) : null}
      {actionError ? (
        <Callout tone="danger" className="mb-4">
          {actionError}
        </Callout>
      ) : null}

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
              <span className="text-sm text-ink-sub tabular-nums">
                第 {selectedIndex + 1} 条 / 共 {submissions.total} 条
              </span>
            </div>
          ) : undefined
        }
        queue={
          <DataPanel
            label="提交队列"
            className="min-h-0 flex-1"
            filter={
              <FilterBar label="提交筛选">
                <FilterField label="课程" htmlFor="grading-course">
                  <Select
                    id="grading-course"
                    options={courseOptions}
                    value={courseId}
                    placeholder={courseOptions.length > 0 ? '选择课程' : '暂无课程'}
                    disabled={courseOptions.length === 0}
                    onValueChange={(value) => {
                      setCourseId(value)
                      setSearchParams({})
                      setSelectedId(undefined)
                    }}
                  />
                </FilterField>
                <FilterField label="作业" htmlFor="grading-assignment">
                  <Select
                    id="grading-assignment"
                    options={assignmentOptions}
                    value={assignmentId}
                    placeholder={
                      courseId === ''
                        ? '请先选择课程'
                        : assignmentOptions.length > 0
                          ? '选择作业'
                          : '该课程暂无作业'
                    }
                    disabled={assignmentOptions.length === 0}
                    onValueChange={(value) => {
                      setSearchParams({ assignment: value })
                      setSelectedId(undefined)
                    }}
                  />
                </FilterField>
                <FilterField label="批改状态" group>
                  <SegmentedControl
                    aria-label="按批改状态筛选"
                    size="sm"
                    options={STATUS_FILTERS.map((item) => ({ value: item.value, label: item.label }))}
                    value={statusFilter}
                    onValueChange={(value) => {
                      setStatusFilter(value)
                      setSelectedId(undefined)
                    }}
                  />
                </FilterField>
                <FilterField label="是否迟交" group>
                  <SegmentedControl
                    aria-label="按是否迟交筛选"
                    size="sm"
                    options={LATE_FILTERS.map((item) => ({
                      value: String(item.value),
                      label: item.label,
                    }))}
                    value={String(lateFilter)}
                    onValueChange={(value) => {
                      setLateFilter(Number(value) as BoolFilter)
                      setSelectedId(undefined)
                    }}
                  />
                </FilterField>
              </FilterBar>
            }
            footer={
              <Pagination
                page={submissions.page}
                pageSize={submissions.pageSize}
                total={submissions.total}
                onPageChange={submissions.setPage}
              />
            }
          >
            {assignmentId === '' ? (
              <Empty
                icon={ClipboardCheck}
                title="请先选择作业"
                description="选定作业后会列出全班提交、得分与批改状态。"
              />
            ) : (
              <ResourceState
                resource={submissions}
                emptyIcon={ClipboardCheck}
                emptyTitle={statusFilter ? '这个状态下没有提交' : '还没有提交'}
                emptyDescription={
                  statusFilter ? '换个状态看看,或查看全部提交。' : '学生提交作业后会出现在这里。'
                }
                skeleton={<Skeleton variant="line" lines={5} />}
              >
                {(page) => (
                  <div
                    role="listbox"
                    aria-label="提交队列"
                    aria-activedescendant={selected ? `submission-${selected.id}` : undefined}
                    tabIndex={0}
                    onKeyDown={onQueueKeyDown}
                    className="focus-visible:outline-2 focus-visible:outline-accent focus-visible:-outline-offset-2"
                  >
                    {page.list.map((submission) => {
                      const isActive = submission.id === selected?.id
                      return (
                        <div
                          key={submission.id}
                          id={`submission-${submission.id}`}
                          role="option"
                          aria-selected={isActive}
                          onClick={() => {
                            setSelectedId(submission.id)
                            setMobileView('detail')
                          }}
                          className={cn(
                            'cursor-pointer border-t border-line px-4 py-3 first:border-t-0',
                            isActive ? 'bg-primary-soft' : 'hover:bg-surface-hover',
                          )}
                        >
                          <div className="flex items-center justify-between gap-2">
                            <span className="truncate text-sm font-medium text-ink">
                              第 {submission.attempt_no} 次提交
                            </span>
                            <StatusIndicator
                              tone={submissionStatusTone(submission.status)}
                              label={submissionStatusLabel(submission.status)}
                            />
                          </div>
                          <p className="mt-1 truncate text-xs text-ink-sub">
                            最终得分 {formatScore(submission.final_score)}
                            {submission.is_late ? ' · 迟交' : ''}
                          </p>
                          <p className="mt-1 font-mono text-xs text-ink-faint">
                            {formatDateTime(submission.submitted_at)}
                          </p>
                        </div>
                      )
                    })}
                    <p className="border-t border-line px-4 py-2 text-xs text-ink-faint">↑↓ 切换条目</p>
                  </div>
                )}
              </ResourceState>
            )}
          </DataPanel>
        }
        detail={
          selected ? (
            <SubmissionGradePane
              key={selected.id}
              submission={selected}
              onSaved={() => {
                advanceToNext()
                submissions.reload()
              }}
              onError={setActionError}
            />
          ) : (
            <div className="flex min-h-48 items-center justify-center rounded-lg bg-surface p-6 text-sm text-ink-sub shadow-xs">
              {assignmentId === ''
                ? '先在左侧选择课程与作业。'
                : '左侧选一次提交,在这里给分并写评语。'}
            </div>
          )
        }
      />
    </>
  )
}

/** 状态筛选项:值为空串表示不过滤。 */
const STATUS_FILTERS = [
  { value: '', label: '全部' },
  { value: String(SubmissionStatus.PENDING), label: '待批改' },
  { value: String(SubmissionStatus.GRADED), label: '已出分' },
] as const

/**
 * LATE_FILTERS 是「是否迟交」筛选项。
 * 布尔列没法用「缺省即不过滤」表达三种意图,全平台统一用 BoolFilter(0 不限 / 1 是 / 2 否)。
 */
const LATE_FILTERS = [
  { value: BOOL_FILTER.ANY, label: '不限' },
  { value: BOOL_FILTER.YES, label: '仅迟交' },
  { value: BOOL_FILTER.NO, label: '仅按时' },
] as const

interface SubmissionGradePaneProps {
  submission: Submission
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
function SubmissionGradePane({ submission, onSaved, onError }: SubmissionGradePaneProps) {
  const [score, setScore] = useState(String(submission.manual_score ?? ''))
  const [comment, setComment] = useState(submission.comment ?? '')
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const submit = useCallback(async () => {
    const normalizedScore = score.trim()
    const value = Number(normalizedScore)
    if (normalizedScore === '' || !Number.isFinite(value) || value < 0) {
      setFormError('请输入 0 或更大的评分')
      return
    }
    if (comment.trim() === '') {
      setFormError('请写下评语,让学生知道得分依据')
      return
    }
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
  }, [comment, onError, onSaved, score, submission.id])

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-4 rounded-lg bg-surface p-5 shadow-xs">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <h3 className="text-md font-semibold text-ink">第 {submission.attempt_no} 次提交</h3>
          <p className="mt-0.5 text-xs text-ink-sub">{formatDateTime(submission.submitted_at)}</p>
        </div>
        <div className="flex flex-wrap items-center gap-1.5">
          <StatusIndicator
            tone={submissionStatusTone(submission.status)}
            label={submissionStatusLabel(submission.status)}
          />
          {submission.is_late ? <Badge tone="warning">迟交</Badge> : null}
        </div>
      </div>

      <DescriptionList
        dense
        columns={2}
        items={[
          { term: '自动判分', description: formatScore(submission.auto_score), mono: true },
          { term: '当前最终得分', description: formatScore(submission.final_score), mono: true },
        ]}
      />

      <FormField label="我的评分" htmlFor="grade-score" required>
        <Input
          id="grade-score"
          type="number"
          min="0"
          step="0.5"
          value={score}
          onChange={(event) => setScore(event.target.value)}
        />
      </FormField>

      <FormField
        label="评语"
        htmlFor="grade-comment"
        required
        error={formError}
        helper="说清楚扣分点与改进方向,学生会在作业结果页看到"
      >
        <Textarea
          id="grade-comment"
          value={comment}
          rows={6}
          invalid={Boolean(formError)}
          onChange={(event) => setComment(event.target.value)}
        />
      </FormField>

      {/* 动作条贴详情片底边;<lg 由 QueueDetailLayout 钉到屏幕底部 */}
      <div className="mt-auto flex flex-wrap items-center justify-end gap-2 border-t border-line pt-4">
        <span className="mr-auto text-sm text-ink-sub">你的评分会覆盖为最终得分。</span>
        <Button variant="primary" loading={working} onClick={() => void submit()}>
          提交批改并看下一条
        </Button>
      </div>
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
    (value) => value.list.length === 0,
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
            <div className="grid gap-4 lg:grid-cols-2">
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

  const submit = useCallback(async () => {
    const scoreValue = Number(score)
    const maxValue = Number(maxScore)
    if (!Number.isFinite(maxValue) || maxValue <= 0) {
      setFormError('满分需要是大于 0 的数字')
      return
    }
    if (!Number.isFinite(scoreValue) || scoreValue < 0 || scoreValue > maxValue) {
      setFormError('得分需要在 0 与满分之间')
      return
    }
    if (comment.trim() === '') {
      setFormError('请写下评分说明,学生会看到这段话')
      return
    }
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
  }, [comment, maxScore, onSaved, passed, score, task.task_id])

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
            <FormField label="得分" htmlFor="manual-score" required>
              <Input
                id="manual-score"
                type="number"
                min="0"
                step="0.5"
                value={score}
                onChange={(event) => setScore(event.target.value)}
              />
            </FormField>
            <FormField label="满分" htmlFor="manual-max" required helper="这道题的分值上限">
              <Input
                id="manual-max"
                type="number"
                min="1"
                value={maxScore}
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
          >
            <Textarea
              id="manual-comment"
              value={comment}
              rows={4}
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
