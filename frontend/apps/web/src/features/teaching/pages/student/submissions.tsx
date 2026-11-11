// 作业结果页(深页,/student/courses/:courseId/assignments/:assignmentId/submissions)。设计 §7.2:
// 顶栏位置「课程名 › 作业名 › 作业结果」;上方「最新一次提交」白卡(大号得分,待批改时写明在等什么;
// 自动得分与人工得分、迟交标记、老师评语、提交时间),下方历次提交列表(第几次、时间、得分、状态)。
// 主操作:还能提交时为「再交一次」(回到作答),否则换成一句原因;判断与作答页同一份规则(submissionWindow)。
//
// 提交编号从 GET /teaching/assignments/{id}/submissions 取回(该路由师生同用按身份分视角,
// 学生只回本人提交),因此刷新后仍能回到自己的结果 —— 不依赖上一次提交响应里的编号(FE-7)。
// 判题结果只经 M6 提交记录的分数与状态字段呈现:M3 的判题任务详情与进度 WS 属教师能力
// (judge 用户组守卫为教师/校管),学生侧不连(对齐清单 §6.6)。自动判分进行中时,页面可见期间每 10 秒
// 静默读一次最新提交,状态变了再刷新列表。

import { useCallback } from 'react'
import { Link, useParams } from 'react-router'
import { ClipboardCheck, Lock, MessageSquare, RotateCcw } from 'lucide-react'
import { SubmissionStatus, type Submission } from '@chaimir/api-client'
import {
  Badge,
  buttonVariants,
  DataPanel,
  Icon,
  ListRow,
  PageHeader,
  PageScaffold,
  Pagination,
  Skeleton,
  StatusIndicator,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceError, ResourceState } from '../../../../components/ResourceState'
import {
  locationLabel,
  useAsyncResource,
  usePagedResource,
  usePageLocation,
  useVisiblePolling,
} from '../../../../hooks'
import { formatDateTime, formatRelativeOccurrence, formatScore } from '../../../../utils/formatters'
import { submissionStatusLabel } from '../../../../utils/labels/teaching'
import { errorDiagnostics, RESOURCE_LOAD_FAILED_MESSAGE } from '../../../../utils/userFacingError'
import { submissionStatusTone } from '../../statusPresentation'
import { submissionWindow } from '../../submissionWindow'

/** 自动判分进行中时的静默检查间隔 */
const JUDGING_POLL_MS = 10_000

/**
 * StudentSubmissionsPage 展示最新一次提交的结果与历次提交。
 */
export default function StudentSubmissionsPage() {
  const { courseId = '', assignmentId = '' } = useParams<{
    courseId: string
    assignmentId: string
  }>()

  const assignment = useAsyncResource(
    () => api.teaching.getAssignment(assignmentId),
    [assignmentId],
    () => false
  )
  const outline = useAsyncResource(
    () => api.teaching.getCourseOutline(courseId),
    [courseId],
    () => false
  )
  const submissions = usePagedResource<Submission>(
    (params) => api.teaching.getSubmissions(assignmentId, params),
    [assignmentId],
    { urlPageKey: 'page' }
  )
  // 最新一次单独取(后端按提交时间倒序,第一页第一条):历史列表翻页时它不变
  const latest = useAsyncResource(
    () => api.teaching.getSubmissions(assignmentId, { page: 1, size: 1 }),
    [assignmentId]
  )
  const latestSubmission = latest.data?.list[0]

  usePageLocation([
    {
      label: locationLabel(outline, (data) => data.course.name, '课程详情'),
      to: `/student/courses/${courseId}`,
    },
    {
      label: locationLabel(assignment, (data) => data.assignment.title, '作业'),
      to: `/student/courses/${courseId}/assignments/${assignmentId}`,
    },
    { label: '作业结果' },
  ])

  // 自动判分还在进行:静默读一次,状态变了再刷新两份列表(不闪骨架)
  const judging = latestSubmission?.status === SubmissionStatus.SUBMITTED
  const reloadLatest = latest.reload
  const reloadHistory = submissions.reload
  const checkJudging = useCallback(() => {
    if (!latestSubmission) return
    api.teaching
      .getSubmissions(assignmentId, { page: 1, size: 1 })
      .then((page) => {
        const current = page.list[0]
        if (
          current &&
          (current.id !== latestSubmission.id || current.status !== latestSubmission.status)
        ) {
          reloadLatest()
          reloadHistory()
        }
      })
      .catch((error) => {
        // 检查失败不打扰阅读:下一轮再读,页面继续显示上一次的结果
        console.warn('作业判分进度检查失败', {
          operation: 'teaching.submissions.poll',
          error: errorDiagnostics(error),
        })
      })
  }, [assignmentId, latestSubmission, reloadHistory, reloadLatest])
  useVisiblePolling(judging, checkJudging, JUDGING_POLL_MS)

  const answerHref = `/student/courses/${courseId}/assignments/${assignmentId}`
  const attemptsUsed = latest.data?.total
  const submitState =
    assignment.data && attemptsUsed !== undefined
      ? submissionWindow({
          dueAt: assignment.data.assignment.due_at,
          latePolicy: assignment.data.assignment.late_policy,
          maxAttempts: assignment.data.assignment.max_attempts,
          attemptsUsed,
        })
      : undefined

  return (
    <PageScaffold>
      {assignment.status === 'loading' ? (
        // 页头占位:作业名读到之前不先显示一个通用标题再跳变
        <div aria-busy="true" className="mb-6 flex flex-col gap-2">
          <Skeleton variant="title" width="half" />
          <Skeleton variant="line" width="third" />
        </div>
      ) : (
        <PageHeader
          title={assignment.data?.assignment.title ?? '作业结果'}
          meta={
            assignment.data
              ? [
                  { label: '截止', value: formatDateTime(assignment.data.assignment.due_at) },
                  {
                    value:
                      attemptsUsed !== undefined
                        ? `已提交 ${attemptsUsed} / ${assignment.data.assignment.max_attempts} 次`
                        : undefined,
                  },
                ]
              : undefined
          }
          actions={
            submitState?.open && attemptsUsed !== undefined && attemptsUsed > 0 ? (
              <Link to={answerHref} className={buttonVariants({ variant: 'primary' })}>
                <Icon icon={RotateCcw} size="sm" />
                再交一次
              </Link>
            ) : undefined
          }
        />
      )}

      {latest.status === 'loading' ? (
        <div
          aria-busy="true"
          className="mb-8 flex flex-col gap-3 rounded-lg bg-surface p-6 shadow-xs"
        >
          <Skeleton variant="line" width="third" />
          <Skeleton variant="stat" width="half" />
          <Skeleton variant="line" lines={2} />
        </div>
      ) : latest.status === 'error' ? (
        <div className="mb-8 rounded-lg bg-surface shadow-xs">
          <ResourceError
            message={latest.error?.message ?? RESOURCE_LOAD_FAILED_MESSAGE}
            traceId={latest.error?.traceId}
            onRetry={latest.reload}
          />
        </div>
      ) : latestSubmission ? (
        <LatestSubmissionCard
          submission={latestSubmission}
          closedReason={submitState && !submitState.open ? submitState.reason : undefined}
        />
      ) : (
        <NoSubmissionYet
          answerHref={answerHref}
          closedReason={submitState && !submitState.open ? submitState.reason : undefined}
        />
      )}

      {latestSubmission ? (
        <section aria-labelledby="submission-history-title">
          <h2 id="submission-history-title" className="mb-3 text-md font-semibold text-ink">
            历次提交
          </h2>
          <DataPanel
            label="历次提交"
            footer={
              <Pagination
                page={submissions.page}
                pageSize={submissions.pageSize}
                total={submissions.total}
                onPageChange={submissions.setPage}
              />
            }
          >
            <ResourceState
              resource={submissions}
              emptyIcon={ClipboardCheck}
              emptyTitle="还没有提交记录"
              emptyDescription="完成作答并提交后,每一次提交都会列在这里。"
              skeleton={
                <div aria-busy="true" className="flex flex-col gap-3 p-4">
                  <Skeleton variant="line" lines={3} />
                </div>
              }
            >
              {(page) => (
                <ul className="divide-y divide-line">
                  {page.list.map((item) => (
                    <li key={item.id}>
                      <SubmissionRow submission={item} isLatest={item.id === latestSubmission.id} />
                    </li>
                  ))}
                </ul>
              )}
            </ResourceState>
          </DataPanel>
        </section>
      ) : null}
    </PageScaffold>
  )
}

/** scoreHeadline 给出大号得分位置的内容:出分显示最终得分,否则写明在等什么 */
function scoreHeadline(submission: Submission): {
  score: string | undefined
  waiting: string | undefined
} {
  if (submission.status === SubmissionStatus.GRADED) {
    return { score: formatScore(submission.final_score), waiting: undefined }
  }
  if (submission.status === SubmissionStatus.PENDING)
    return { score: undefined, waiting: '待老师批改' }
  return { score: undefined, waiting: '正在自动判分' }
}

/**
 * LatestSubmissionCard 最新一次提交:大号得分与状态、自动得分与人工得分、迟交标记、老师评语、提交时间。
 * 不能再提交时卡片底部写一句原因(能提交时「再交一次」在页头)。
 */
function LatestSubmissionCard({
  submission,
  closedReason,
}: {
  submission: Submission
  closedReason: string | undefined
}) {
  const headline = scoreHeadline(submission)
  const hasManualPart = submission.manual_max_score > 0
  // 自动判分的部分:已出自动分,或还在判;只含简答题的作业没有这一项
  const hasAutoPart =
    submission.auto_score !== undefined || submission.status === SubmissionStatus.SUBMITTED
  return (
    <section
      aria-labelledby="latest-submission-title"
      className="mb-8 rounded-lg bg-surface p-5 shadow-xs md:p-6"
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 id="latest-submission-title" className="text-sm font-medium text-ink-sub">
          最新一次提交 · 第 {submission.attempt_no} 次
        </h2>
        <div className="flex items-center gap-2">
          {submission.is_late ? <Badge tone="warning">迟交</Badge> : null}
          <StatusIndicator
            tone={submissionStatusTone(submission.status)}
            label={submissionStatusLabel(submission.status)}
            loading={submission.status === SubmissionStatus.SUBMITTED}
          />
        </div>
      </div>

      <div className="mt-4 flex flex-wrap items-end gap-x-10 gap-y-4">
        <p className="flex items-baseline gap-1.5">
          {headline.score !== undefined ? (
            <>
              <span className="text-stat font-bold tabular-nums text-ink">{headline.score}</span>
              <span className="text-sm text-ink-sub">分</span>
            </>
          ) : (
            <span className="text-2xl font-bold text-ink">{headline.waiting}</span>
          )}
        </p>
        <dl className="flex flex-wrap gap-x-8 gap-y-2 text-sm">
          {hasAutoPart ? (
            <div>
              <dt className="text-ink-sub">自动得分</dt>
              <dd className="mt-0.5 font-medium tabular-nums text-ink">
                {submission.auto_score !== undefined
                  ? formatScore(submission.auto_score)
                  : '判分中'}
              </dd>
            </div>
          ) : null}
          {hasManualPart ? (
            <div>
              <dt className="text-ink-sub">老师评分</dt>
              <dd className="mt-0.5 font-medium tabular-nums text-ink">
                {submission.manual_score !== undefined
                  ? `${formatScore(submission.manual_score)} / ${submission.manual_max_score}`
                  : '还没批改'}
              </dd>
            </div>
          ) : null}
          <div>
            <dt className="text-ink-sub">提交时间</dt>
            <dd className="mt-0.5 font-medium tabular-nums text-ink">
              {formatDateTime(submission.submitted_at)}
            </dd>
          </div>
        </dl>
      </div>

      {submission.is_late ? (
        <p className="mt-4 text-sm text-ink-sub">
          这次在截止时间之后提交,得分已按课程的迟交规则计算。
        </p>
      ) : null}

      {submission.comment ? (
        <div className="mt-5 rounded-md bg-surface-hover p-4">
          <p className="flex items-center gap-2 text-sm font-medium text-ink">
            <Icon icon={MessageSquare} size="sm" className="text-primary" />
            老师评语
          </p>
          <p className="mt-2 whitespace-pre-wrap text-base leading-relaxed text-ink">
            {submission.comment}
          </p>
        </div>
      ) : null}

      {closedReason ? (
        <p className="mt-5 flex items-center gap-2 text-sm text-ink-sub">
          <Icon icon={Lock} size="sm" className="shrink-0" />
          {closedReason}
        </p>
      ) : null}
    </section>
  )
}

/** NoSubmissionYet 还没有提交过:能作答时给「去作答」,否则写明原因 */
function NoSubmissionYet({
  answerHref,
  closedReason,
}: {
  answerHref: string
  closedReason: string | undefined
}) {
  return (
    <div className="flex flex-col items-center rounded-lg bg-surface px-6 py-14 text-center shadow-xs">
      <Icon icon={ClipboardCheck} size="lg" className="text-ink-faint" />
      <p className="mt-3 text-md font-semibold text-ink">还没有提交过这份作业</p>
      {closedReason ? (
        <p className="mt-1 text-sm text-ink-sub">{closedReason}。</p>
      ) : (
        <>
          <p className="mt-1 text-sm text-ink-sub">提交后,得分和老师的评语会显示在这里。</p>
          <Link to={answerHref} className={`${buttonVariants({ variant: 'primary' })} mt-5`}>
            去作答
          </Link>
        </>
      )}
    </div>
  )
}

/** SubmissionRow 历次提交里的一行:第几次、时间、得分、状态 */
function SubmissionRow({ submission, isLatest }: { submission: Submission; isLatest: boolean }) {
  const graded = submission.status === SubmissionStatus.GRADED
  return (
    <ListRow
      title={
        <span>
          第 {submission.attempt_no} 次提交
          {isLatest ? <span className="ml-2 text-sm font-normal text-ink-sub">最新</span> : null}
        </span>
      }
      meta={
        <time dateTime={submission.submitted_at} title={formatDateTime(submission.submitted_at)}>
          {formatRelativeOccurrence(submission.submitted_at)}
          {submission.is_late ? ' · 迟交' : ''}
        </time>
      }
      trailing={
        <>
          <span className="w-16 text-right text-base font-semibold tabular-nums text-ink">
            {graded ? `${formatScore(submission.final_score)} 分` : '—'}
          </span>
          <StatusIndicator
            tone={submissionStatusTone(submission.status)}
            label={submissionStatusLabel(submission.status)}
            loading={submission.status === SubmissionStatus.SUBMITTED}
            className="hidden sm:inline-flex"
          />
        </>
      }
    />
  )
}
