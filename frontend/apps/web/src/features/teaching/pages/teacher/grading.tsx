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

import { useCallback, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router'
import { CheckSquare, ClipboardCheck, RefreshCw, RotateCcw } from 'lucide-react'
import {
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
  Card,
  CardBody,
  CardHeader,
  DescriptionList,
  Empty,
  FormField,
  Input,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  PageBody,
  PageHeader,
  PageScaffold,
  PageSection,
  Pagination,
  SegmentedControl,
  Select,
  Skeleton,
  Stat,
  StatusIndicator,
  Table,
  Textarea,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, usePagedResource, useResourceTotal } from '../../../../hooks'
import { GradeAppeals } from '../../../grade/pages/teacher/grade-appeals'
import { formatDateTime, formatScore } from '../../../../utils/formatters'
import {
  isJudgeTaskActive,
  judgeTaskStatusLabel,
  judgeTaskStatusTone,
} from '../../../../utils/labels/judge'
import { submissionStatusLabel, submissionStatusTone } from '../../../../utils/labels/teaching'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

/** 课程与作业选择器一次取回的条数:后端分页上限 100。 */
const PICKER_SIZE = 100

/** 待处理判题面板一次展示的条数:它是提示区不是全量列表。 */
const PENDING_JUDGE_SIZE = 10

/**
 * TeacherGradingPage 按作业列出提交并承载批改。
 */
export default function TeacherGradingPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const assignmentId = searchParams.get('assignment') ?? ''
  const [courseId, setCourseId] = useState<string>('')

  const courses = useAsyncResource(
    () => api.teaching.getCourses({ role: 'teacher', page: 1, size: PICKER_SIZE }),
    [],
    () => false,
  )

  const assignments = useAsyncResource(
    () => (courseId ? api.teaching.listCourseAssignments(courseId) : Promise.resolve([])),
    [courseId],
    () => false,
  )

  const courseOptions = useMemo(
    () => (courses.data?.list ?? []).map((course: Course) => ({ value: course.id, label: course.name })),
    [courses.data],
  )

  const assignmentOptions = useMemo(
    () => (assignments.data ?? []).map((item: Assignment) => ({ value: item.id, label: item.title })),
    [assignments.data],
  )

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '教学' }, { label: '批改中心' }]} />}
        title="批改中心"
        description="选择作业查看学生提交。主观题在这里给分评语,编程题的判题异常可以重判。学生的成绩申诉也在本页处理。"
        icon={CheckSquare}
      />

      <PageSection>
        <Card>
          <CardHeader title="选择要批改的作业" description="先选课程,再选课程下的作业。" />
          <CardBody>
            <div className="grid gap-4 sm:grid-cols-2">
              <FormField label="课程" htmlFor="grading-course">
                <Select
                  id="grading-course"
                  options={courseOptions}
                  value={courseId}
                  placeholder={courseOptions.length > 0 ? '选择课程' : '暂无课程'}
                  disabled={courseOptions.length === 0}
                  onValueChange={(value) => {
                    setCourseId(value)
                    setSearchParams({})
                  }}
                />
              </FormField>
              <FormField label="作业" htmlFor="grading-assignment">
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
                  onValueChange={(value) => setSearchParams({ assignment: value })}
                />
              </FormField>
            </div>
          </CardBody>
        </Card>
      </PageSection>

      {assignmentId ? (
        <SubmissionsPanel assignmentId={assignmentId} />
      ) : (
        <PageSection>
          <Empty
            icon={ClipboardCheck}
            title="请先选择作业"
            description="选定作业后会列出全班提交、得分与批改状态。"
          />
        </PageSection>
      )}

      {/* 成绩申诉:与作业选择无关,始终可处理 */}
      <GradeAppeals />
    </PageScaffold>
  )
}

/**
 * SubmissionsPanel 列出该作业的全班提交并承载批改与重判。
 */
function SubmissionsPanel({ assignmentId }: { assignmentId: string }) {
  const [gradeTarget, setGradeTarget] = useState<Submission>()
  const [actionError, setActionError] = useState<string>()

  const submissions = usePagedResource<Submission>(
    (params) => api.teaching.getSubmissions(assignmentId, params),
    [assignmentId],
  )

  // 指标带取服务端全量口径:待批改是教师最常看的数字,不能用当前页数出来的近似值。
  // 「迟交」没有服务端筛选参数,故不做这张卡:迟交在每一行的标签里可见(规范 §6.5)。
  const totalCount = useResourceTotal(
    (params) => api.teaching.getSubmissions(assignmentId, params),
    [assignmentId],
  )
  const pendingCount = useResourceTotal(
    (params) =>
      api.teaching.getSubmissions(assignmentId, { status: SubmissionStatus.PENDING, ...params }),
    [assignmentId],
  )
  const gradedCount = useResourceTotal(
    (params) =>
      api.teaching.getSubmissions(assignmentId, { status: SubmissionStatus.GRADED, ...params }),
    [assignmentId],
  )

  const columns: TableColumn<Submission>[] = [
    {
      key: 'attempt_no',
      header: '提交',
      // 提交记录只回 student_id;按提交次序呈现,不把内部编号当学生名显示
      render: (submission) => <span className="text-ink">第 {submission.attempt_no} 次提交</span>,
    },
    {
      key: 'submitted_at',
      header: '提交时间',
      render: (submission) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatDateTime(submission.submitted_at)}
        </span>
      ),
    },
    {
      key: 'auto_score',
      header: '自动判分',
      align: 'right',
      mono: true,
      render: (submission) => formatScore(submission.auto_score),
    },
    {
      key: 'manual_score',
      header: '我的评分',
      align: 'right',
      mono: true,
      render: (submission) => formatScore(submission.manual_score),
    },
    {
      key: 'final_score',
      header: '最终得分',
      align: 'right',
      mono: true,
      render: (submission) => (
        <span className="font-medium text-ink">{formatScore(submission.final_score)}</span>
      ),
    },
    {
      key: 'status',
      header: '状态',
      render: (submission) => (
        <div className="flex flex-wrap items-center gap-1.5">
          <StatusIndicator
            tone={submissionStatusTone(submission.status)}
            label={submissionStatusLabel(submission.status)}
          />
          {submission.is_late ? <Badge tone="warning">迟交</Badge> : null}
        </div>
      ),
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (submission) => (
        <Button variant="ghost" size="sm" onClick={() => setGradeTarget(submission)}>
          批改
        </Button>
      ),
    },
  ]

  return (
    <>
      <PageSection>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <Stat label="提交总数" value={totalCount ?? '—'} icon={ClipboardCheck} />
          <Stat label="待批改" value={pendingCount ?? '—'} icon={CheckSquare} />
          <Stat label="已出分" value={gradedCount ?? '—'} icon={ClipboardCheck} />
        </div>
      </PageSection>

      <PageBody rail={<JudgeTasksPanel assignmentId={assignmentId} />}>
        <PageSection title="学生提交" description={`共 ${submissions.total} 次提交`}>
          <div className="flex flex-col gap-4">
            {actionError ? <Callout tone="danger">{actionError}</Callout> : null}

            <ResourceState
              resource={submissions}
              emptyIcon={ClipboardCheck}
              emptyTitle="还没有提交"
              emptyDescription="学生提交作业后会出现在这里。"
              skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
            >
              {(page) => (
                <>
                  <Table columns={columns} data={page.list} rowKey={(item) => item.id} />
                  <Pagination
                    page={submissions.page}
                    pageSize={submissions.pageSize}
                    total={submissions.total}
                    onPageChange={submissions.setPage}
                  />
                </>
              )}
            </ResourceState>
          </div>
        </PageSection>
      </PageBody>

      {gradeTarget ? (
        <GradeSubmissionModal
          submission={gradeTarget}
          onClose={() => setGradeTarget(undefined)}
          onSaved={() => {
            setGradeTarget(undefined)
            submissions.reload()
          }}
          onError={setActionError}
        />
      ) : null}
    </>
  )
}

interface GradeSubmissionModalProps {
  submission: Submission
  onClose: () => void
  onSaved: () => void
  onError: (message: string) => void
}

/**
 * GradeSubmissionModal 给单次提交打分与写评语。
 * 评语是给学生看的真实反馈,不提供模板按钮 ——
 * 旧前端用硬编码模板代替真实理由,那等于不告诉学生原因。
 */
function GradeSubmissionModal({ submission, onClose, onSaved, onError }: GradeSubmissionModalProps) {
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
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="lg">
        <ModalHeader>
          <ModalTitle>批改提交</ModalTitle>
          <ModalDescription>
            评语会展示给学生。自动判分的部分已由系统给出,你的评分会覆盖为最终得分。
          </ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
          <DescriptionList
            dense
            columns={2}
            items={[
              { term: '提交次数', description: `第 ${submission.attempt_no} 次`, mono: true },
              { term: '提交时间', description: formatDateTime(submission.submitted_at), mono: true },
              { term: '自动判分', description: formatScore(submission.auto_score), mono: true },
              { term: '是否迟交', description: submission.is_late ? '是' : '否' },
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
            helper="说清楚扣分点与改进方向,学生会在作业结果页看到"
          >
            <Textarea
              id="grade-comment"
              value={comment}
              rows={5}
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
            提交批改
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}

/**
 * JudgeTasksPanel 展示待人工评分的判题任务并承载评分与重判。
 * pending_manual 是后端提供的筛选:只列「判题器为人工评分且正在判题中」的任务
 * (SQL 条件是 status=判题中 且 judger.type=人工),这类任务在教师给分之前不会自己完成,
 * 故主动作是人工评分;重判留给判题异常的任务(在实时监控页按状态筛选)。
 */
function JudgeTasksPanel({ assignmentId }: { assignmentId: string }) {
  const [scoreTarget, setScoreTarget] = useState<JudgeTask>()
  const [rejudgingId, setRejudgingId] = useState<string>()
  const [actionError, setActionError] = useState<string>()

  const tasks = useAsyncResource(
    () => api.judge.getTasks({ pending_manual: true, page: 1, size: PENDING_JUDGE_SIZE }),
    [assignmentId],
    (value) => value.list.length === 0,
  )

  const rejudge = useCallback(
    async (task: JudgeTask) => {
      setRejudgingId(task.task_id)
      setActionError(undefined)
      try {
        await api.judge.rejudgeTask(task.task_id)
        toast.success('已重新提交判题')
        tasks.reload()
      } catch (error) {
        setActionError(userFacingErrorMessage(error, '重判没有成功,请稍后重试。'))
      } finally {
        setRejudgingId(undefined)
      }
    },
    [tasks],
  )

  return (
    <Card>
      <CardHeader
        title="待人工评分"
        description="判题方式为人工评分的提交在这里给分。给分后判题任务才算完成。"
      />
      <CardBody>
        <ResourceState
          resource={tasks}
          emptyIcon={RefreshCw}
          emptyTitle="没有待人工评分的任务"
          emptyDescription="需要教师给分的判题任务会出现在这里。"
          skeleton={<Skeleton variant="line" lines={3} />}
        >
          {(page) => (
            <div className="flex flex-col gap-3">
              {actionError ? <Callout tone="danger">{actionError}</Callout> : null}
              {page.list.map((task) => (
                <div key={task.task_id} className="flex flex-col gap-2 well p-3">
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
                      onClick={() => void rejudge(task)}
                    >
                      重新判题
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </ResourceState>
      </CardBody>

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
    </Card>
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
