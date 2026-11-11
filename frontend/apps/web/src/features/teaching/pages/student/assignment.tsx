// 作业作答页(深页,/student/courses/:courseId/assignments/:assignmentId)。设计 §7.2:
// 页头为作业名 + 截止倒计时 + 一行属性(截止时间 · 可提交次数 · 迟交规则),右侧「历次提交」;
// ≥lg 左侧答题卡(已作答实底、当前题描边、可跳转),右侧题目卡片依次排列;底部固定操作条
// (保存状态、已作答 n / m、立即保存、提交作业)。不能再提交时(次数用完,或截止后不接收)页面只读,
// 操作条换成一句原因和「查看结果」;截止后仍接收的作业照常作答,提交按迟交处理。
//
// FE-7 服务端草稿:进入先拉服务端草稿,编辑按 60 秒定时 + 失焦即时写回服务端,
// 服务端草稿是权威;本地只保留尚未同步的编辑态与最近同步标记,
// 服务端读写失败时不用本地内容替代草稿进入提交链路(规范 §6.6)。有未保存改动时离开页面先提醒。
//
// 答案黑盒:题面经 M6 → M5 GetContentFace 取得,答案/判题配置已在后端剥离,
// 前端不做二次过滤也不请求 full 内容(铁律 1 + 对齐清单 §6.4)。
//
// 作业只引用 M5 理论题;题型和答案结构决定作答控件与判分路径。

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import { ClipboardList, History, Info } from 'lucide-react'
import type { AssignmentDetail, TheoryAnswer } from '@chaimir/api-client'
import {
  Badge,
  Button,
  buttonVariants,
  Callout,
  Icon,
  PageHeader,
  PageScaffold,
  Skeleton,
  toast,
  useReducedMotion,
  type AutosaveState,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { UnsavedChangesGuard } from '../../../../components/UnsavedChangesGuard'
import { useActiveAnchor, useAsyncResource, usePageLocation, locationLabel } from '../../../../hooks'
import { formatDateTime, formatRelativeDeadline } from '../../../../utils/formatters'
import { latePolicyLabel } from '../../../../utils/labels/teaching'
import { errorDiagnostics, userFacingErrorMessage } from '../../../../utils/userFacingError'
import { ASSIGNMENT_DUE_TONE } from '../../statusPresentation'
import { submissionWindow } from '../../submissionWindow'
import {
  draftAnswers,
  serializeAnswers,
  isAssignmentAnswer,
  isAnswerEmpty,
  type AssignmentAnswers as AnswerMap,
} from '../../assignment-answers'
import { AnswerSheet } from '../../components/assignment/AnswerSheet'
import {
  AssignmentActionBar,
  AssignmentClosedBar,
} from '../../components/assignment/AssignmentActionBar'
import {
  QuestionCard,
  questionAnchorId,
  questionHeadingId,
} from '../../components/assignment/QuestionCard'
import { SubmitConfirmModal } from '../../components/assignment/SubmitConfirmModal'

/** 服务端草稿定时写回间隔:与 M6 接口设计 §6.2「自动每 60s」一致。 */
const DRAFT_AUTOSAVE_INTERVAL_MS = 60_000

interface LocalAssignmentDraft {
  studentId: string
  assignmentId: string
  serverUpdatedAt: string | null
  answers: AnswerMap
}

/** localDraftKey 将未同步作答限定到当前学生和作业。 */
function localDraftKey(studentId: string, assignmentId: string) {
  return `chaimir:teaching-draft:${studentId}:${assignmentId}`
}

/** readLocalDraft 只读取归属和结构均有效的本机记录,显式报告恢复失败。 */
function readLocalDraft(
  studentId: string,
  assignmentId: string
): { draft?: LocalAssignmentDraft; failed: boolean } {
  try {
    const raw = window.localStorage.getItem(localDraftKey(studentId, assignmentId))
    if (!raw) return { failed: false }
    const parsed = JSON.parse(raw) as Partial<LocalAssignmentDraft>
    if (
      parsed.studentId !== studentId ||
      parsed.assignmentId !== assignmentId ||
      !(parsed.serverUpdatedAt === null || typeof parsed.serverUpdatedAt === 'string') ||
      !parsed.answers ||
      typeof parsed.answers !== 'object' ||
      Array.isArray(parsed.answers) ||
      Object.values(parsed.answers).some((value) => !isAssignmentAnswer(value))
    ) {
      return { failed: true }
    }
    return { draft: parsed as LocalAssignmentDraft, failed: false }
  } catch {
    return { failed: true }
  }
}

/** writeLocalDraft 仅保存尚未写入服务端草稿的编辑。 */
function writeLocalDraft(key: string, draft: LocalAssignmentDraft) {
  window.localStorage.setItem(key, JSON.stringify(draft))
}

/**
 * StudentAssignmentPage 读取作业题面、服务端草稿与已提交次数。
 */
export default function StudentAssignmentPage() {
  const { courseId = '', assignmentId = '' } = useParams<{
    courseId: string
    assignmentId: string
  }>()

  // 题面、草稿与已提交次数一次读齐:草稿要按题目编号铺回表单,提交次数决定还能不能作答
  const view = useAsyncResource(
    () =>
      Promise.all([
        api.teaching.getAssignment(assignmentId),
        api.teaching.getDraft(assignmentId),
        // 只用总数:后端按身份只回本人提交,总数即已用提交次数
        api.teaching.getSubmissions(assignmentId, { page: 1, size: 1 }),
      ]).then(([detail, draft, submissions]) => {
        const local = readLocalDraft(draft.student_id, assignmentId)
        const serverUpdatedAt = draft.exists ? draft.updated_at : null
        const hasLocalConflict = Boolean(
          local.draft && local.draft.serverUpdatedAt !== serverUpdatedAt
        )
        return {
          detail,
          draft,
          attemptsUsed: submissions.total,
          localDraft: local.draft,
          hasLocalConflict,
          localStorageUnavailable: local.failed,
        }
      }),
    [assignmentId],
    () => false
  )
  // 课程名只用于顶栏位置;读不到时顶栏用通用名称,不影响作答
  const outline = useAsyncResource(
    () => api.teaching.getCourseOutline(courseId),
    [courseId],
    () => false
  )

  usePageLocation([
    {
      label: locationLabel(outline, (data) => data.course.name, '课程详情'),
      to: `/student/courses/${courseId}`,
    },
    { label: locationLabel(view, (data) => data.detail.assignment.title, '作业') },
  ])

  return (
    <PageScaffold>
      <ResourceState
        resource={view}
        emptyIcon={ClipboardList}
        emptyTitle="作业内容暂未开放"
        emptyDescription="老师还没有为这次作业添加题目。"
        skeleton={<AssignmentSkeleton />}
      >
        {(data) => (
          <AssignmentForm
            key={assignmentId + ':' + data.draft.student_id}
            courseId={courseId}
            assignmentId={assignmentId}
            detail={data.detail}
            attemptsUsed={data.attemptsUsed}
            studentId={data.draft.student_id}
            serverAnswers={draftAnswers(data.draft.content, data.detail.items)}
            initialAnswers={draftAnswers(
              data.draft.content,
              data.detail.items,
              data.hasLocalConflict ? undefined : data.localDraft?.answers
            )}
            draftUpdatedAt={data.draft.exists ? data.draft.updated_at : undefined}
            localDraftKey={localDraftKey(data.draft.student_id, assignmentId)}
            pendingLocalAnswers={data.hasLocalConflict ? data.localDraft?.answers : undefined}
            localDraftConflict={data.hasLocalConflict}
            localDraftNeedsRestore={Boolean(data.localDraft && !data.hasLocalConflict)}
            localStorageUnavailable={data.localStorageUnavailable}
          />
        )}
      </ResourceState>
    </PageScaffold>
  )
}

/** AssignmentSkeleton 与真实内容同形:页头、左侧答题卡、右侧题目卡 */
function AssignmentSkeleton() {
  return (
    <div aria-busy="true" className="flex flex-col gap-6">
      <div className="flex flex-col gap-2">
        <Skeleton variant="title" width="half" />
        <Skeleton variant="line" width="third" />
      </div>
      <div className="flex flex-col gap-4 lg:flex-row lg:gap-6">
        <div className="lg:w-60 lg:shrink-0">
          <Skeleton variant="block" />
        </div>
        <div className="flex min-w-0 flex-1 flex-col gap-4">
          <Skeleton variant="block" />
          <Skeleton variant="block" />
        </div>
      </div>
    </div>
  )
}

interface AssignmentFormProps {
  courseId: string
  assignmentId: string
  detail: AssignmentDetail
  /** 已用提交次数(本人在这份作业下的提交总数) */
  attemptsUsed: number
  studentId: string
  serverAnswers: AnswerMap
  initialAnswers: AnswerMap
  draftUpdatedAt: string | undefined
  localDraftKey: string
  pendingLocalAnswers?: AnswerMap
  localDraftConflict: boolean
  localDraftNeedsRestore: boolean
  localStorageUnavailable: boolean
}

/**
 * AssignmentForm 承载作答、草稿自动保存与提交;不能再提交时只读。
 */
function AssignmentForm({
  courseId,
  assignmentId,
  detail,
  attemptsUsed,
  studentId,
  serverAnswers,
  initialAnswers,
  draftUpdatedAt,
  localDraftKey: localKey,
  pendingLocalAnswers,
  localDraftConflict,
  localDraftNeedsRestore,
  localStorageUnavailable,
}: AssignmentFormProps) {
  const navigate = useNavigate()
  const assignment = detail.assignment
  // 还能不能提交:与服务端提交校验同口径(次数用完,或截止后不接收即只读)
  const submitState = submissionWindow({
    dueAt: assignment.due_at,
    latePolicy: assignment.late_policy,
    maxAttempts: assignment.max_attempts,
    attemptsUsed,
  })
  const readOnly = !submitState.open
  // 只读时不恢复本机记录、不再写服务端草稿,页面显示服务器上的作答
  const restoreLocal = localDraftNeedsRestore && !readOnly
  const [answers, setAnswers] = useState<AnswerMap>(
    localDraftNeedsRestore && !restoreLocal ? serverAnswers : initialAnswers
  )
  const [autosaveState, setAutosaveState] = useState<AutosaveState>(
    restoreLocal ? 'saving' : 'idle'
  )
  const [savedAt, setSavedAt] = useState<Date | undefined>(
    draftUpdatedAt ? new Date(draftUpdatedAt) : undefined
  )
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState<string>()
  const [recoveryConflict, setRecoveryConflict] = useState(localDraftConflict && !readOnly)
  const [localCacheError, setLocalCacheError] = useState(localStorageUnavailable)
  /** 已落库的作答快照:与当前作答逐题比对,得出「有几处改动未保存」 */
  const [savedAnswers, setSavedAnswers] = useState<AnswerMap>(serverAnswers)

  // 未同步标记:只有真正改动过才写服务端,避免定时器空跑
  const dirtyRef = useRef(restoreLocal)
  const editVersionRef = useRef(restoreLocal ? 1 : 0)
  const inFlightSaveRef = useRef<Promise<boolean> | null>(null)
  const serverUpdatedAtRef = useRef(draftUpdatedAt ?? null)
  const answersRef = useRef(answers)
  answersRef.current = answers

  const deadline = formatRelativeDeadline(assignment.due_at)
  const hasShortAnswer = detail.items.some((item) => item.body?.q_type === 'short_answer')

  /** saveDraft 把当前作答写回服务端草稿;服务端成功才认为已保存。 */
  const saveDraft = useCallback((): Promise<boolean> => {
    if (inFlightSaveRef.current) return inFlightSaveRef.current
    if (!dirtyRef.current) return Promise.resolve(true)
    const operation = (async () => {
      while (dirtyRef.current) {
        const snapshot = { ...answersRef.current }
        const snapshotVersion = editVersionRef.current
        setAutosaveState('saving')
        try {
          const result = await api.teaching.saveDraft(assignmentId, {
            content: serializeAnswers(detail.items, snapshot),
          })
          setSavedAnswers(snapshot)
          setSavedAt(new Date(result.updated_at))
          serverUpdatedAtRef.current = result.updated_at
          if (snapshotVersion === editVersionRef.current) {
            dirtyRef.current = false
            setAutosaveState('saved')
            try {
              window.localStorage.removeItem(localKey)
              setLocalCacheError(false)
            } catch (error) {
              setLocalCacheError(true)
              console.error('已保存作答,但清除本机恢复记录失败', {
                operation: 'teaching.assignment.clearLocalDraft',
                error: errorDiagnostics(error),
              })
            }
          } else {
            try {
              writeLocalDraft(localKey, {
                studentId,
                assignmentId,
                serverUpdatedAt: result.updated_at,
                answers: answersRef.current,
              })
            } catch (error) {
              setLocalCacheError(true)
              console.error('本机作答恢复记录更新失败', {
                operation: 'teaching.assignment.writeLocalDraft',
                error: errorDiagnostics(error),
              })
            }
          }
        } catch (error) {
          setAutosaveState('error')
          console.error('作业草稿写回失败', {
            operation: 'teaching.assignment.saveDraft',
            reason: 'draft-save-failed',
            error: errorDiagnostics(error),
          })
          return false
        }
      }
      return true
    })()
    const tracked = operation.finally(() => {
      if (inFlightSaveRef.current === tracked) inFlightSaveRef.current = null
    })
    inFlightSaveRef.current = tracked
    return tracked
  }, [assignmentId, detail.items, localKey, studentId])

  useEffect(() => {
    if (restoreLocal) void saveDraft()
  }, [restoreLocal, saveDraft])

  // 定时写回:60 秒一轮,与后端接口设计的自动保存节奏一致
  useEffect(() => {
    if (readOnly) return
    const timer = window.setInterval(() => void saveDraft(), DRAFT_AUTOSAVE_INTERVAL_MS)
    return () => window.clearInterval(timer)
  }, [readOnly, saveDraft])

  /** updateAnswer 记录作答并标记未同步。 */
  const updateAnswer = useCallback(
    (itemId: string, value: TheoryAnswer) => {
      if (readOnly || submitting || recoveryConflict) return
      const next = { ...answersRef.current, [itemId]: value }
      answersRef.current = next
      editVersionRef.current += 1
      dirtyRef.current = true
      setAutosaveState('idle')
      setAnswers(next)
      try {
        writeLocalDraft(localKey, {
          studentId,
          assignmentId,
          serverUpdatedAt: serverUpdatedAtRef.current,
          answers: next,
        })
        setLocalCacheError(false)
      } catch (error) {
        setLocalCacheError(true)
        console.error('本机作答恢复记录写入失败', {
          operation: 'teaching.assignment.writeLocalDraft',
          error: errorDiagnostics(error),
        })
      }
    },
    [assignmentId, localKey, readOnly, recoveryConflict, studentId, submitting]
  )

  const discardConflictingLocalDraft = useCallback(() => {
    try {
      window.localStorage.removeItem(localKey)
      setLocalCacheError(false)
      setRecoveryConflict(false)
    } catch (error) {
      setLocalCacheError(true)
      console.error('本机作答恢复记录清除失败', {
        operation: 'teaching.assignment.clearLocalDraft',
        error: errorDiagnostics(error),
      })
    }
  }, [localKey])

  const restoreConflictingLocalDraft = useCallback(() => {
    if (!pendingLocalAnswers) return
    const next = draftAnswers({}, detail.items, pendingLocalAnswers)
    answersRef.current = next
    editVersionRef.current += 1
    dirtyRef.current = true
    setAnswers(next)
    setSavedAnswers(serverAnswers)
    setRecoveryConflict(false)
    try {
      writeLocalDraft(localKey, {
        studentId,
        assignmentId,
        serverUpdatedAt: serverUpdatedAtRef.current,
        answers: next,
      })
      setLocalCacheError(false)
    } catch (error) {
      setLocalCacheError(true)
      console.error('本机作答恢复记录写入失败', {
        operation: 'teaching.assignment.writeLocalDraft',
        error: errorDiagnostics(error),
      })
    }
    void saveDraft()
  }, [
    assignmentId,
    detail.items,
    localKey,
    pendingLocalAnswers,
    saveDraft,
    serverAnswers,
    studentId,
  ])

  /** submit 正式提交作答。提交前先把草稿落库,确保服务端与页面一致。 */
  const submit = useCallback(async () => {
    setSubmitting(true)
    setSubmitError(undefined)
    try {
      const submissionAnswers = serializeAnswers(detail.items, answersRef.current)
      if (!(await saveDraft())) {
        setSubmitError('作答尚未保存,请重试后再提交。')
        return
      }
      await api.teaching.submitAssignment(assignmentId, { answers: submissionAnswers })
      setConfirmOpen(false)
      toast.success('作业已提交')
      navigate(`/student/courses/${courseId}/assignments/${assignmentId}/submissions`)
    } catch (error) {
      setSubmitError(userFacingErrorMessage(error, '作业提交失败,请稍后重试。'))
    } finally {
      setSubmitting(false)
    }
  }, [assignmentId, courseId, detail.items, navigate, saveDraft])

  /** openConfirm 打开提交确认;上一次的失败原因不带进新的一轮 */
  const openConfirm = useCallback(() => {
    setSubmitError(undefined)
    setConfirmOpen(true)
  }, [])

  /** 答题卡的格子:判断题选「错误」也是作答,填空题至少填了一空才算 */
  const sheetItems = useMemo(
    () =>
      detail.items.map((item, index) => ({
        id: item.id,
        seq: index + 1,
        answered: !isAnswerEmpty(answers[item.id]),
      })),
    [answers, detail.items]
  )
  const answeredCount = sheetItems.filter((item) => item.answered).length

  /** 未保存改动数:逐题与已落库快照比对 —— 「改了 2 题」比「有改动」信息量更足 */
  const dirtyCount = useMemo(
    () =>
      detail.items.filter(
        (item) =>
          JSON.stringify(answers[item.id] ?? '') !== JSON.stringify(savedAnswers[item.id] ?? '')
      ).length,
    [answers, detail.items, savedAnswers]
  )

  // 当前题跟随滚动;点答题卡或弹窗里的题号时跳过去,焦点落到那题的标题上
  const anchorIds = useMemo(
    () => detail.items.map((item) => questionAnchorId(item.id)),
    [detail.items]
  )
  const [currentAnchor, setCurrentAnchor] = useActiveAnchor(anchorIds)
  const currentId = detail.items.find((item) => questionAnchorId(item.id) === currentAnchor)?.id
  const reducedMotion = useReducedMotion()
  const jumpTo = useCallback(
    (itemId: string) => {
      const anchor = questionAnchorId(itemId)
      setCurrentAnchor(anchor)
      document
        .getElementById(anchor)
        ?.scrollIntoView({ behavior: reducedMotion ? 'auto' : 'smooth', block: 'start' })
      document.getElementById(questionHeadingId(itemId))?.focus({ preventScroll: true })
    },
    [reducedMotion, setCurrentAnchor]
  )

  const resultsHref = `/student/courses/${courseId}/assignments/${assignmentId}/submissions`
  const attemptsText =
    submitState.remaining > 0
      ? `还能提交 ${submitState.remaining} 次(共 ${assignment.max_attempts} 次)`
      : `提交次数已用完(共 ${assignment.max_attempts} 次)`

  return (
    <>
      <PageHeader
        title={assignment.title}
        status={<Badge tone={ASSIGNMENT_DUE_TONE[deadline.urgency]}>{deadline.text}</Badge>}
        meta={[
          { label: '截止', value: formatDateTime(assignment.due_at) },
          { value: attemptsText },
          { value: latePolicyLabel(assignment.late_policy) },
        ]}
        actions={
          attemptsUsed > 0 ? (
            <Link to={resultsHref} className={buttonVariants({ variant: 'ghost' })}>
              <Icon icon={History} size="sm" />
              历次提交
            </Link>
          ) : undefined
        }
      />

      {recoveryConflict ? (
        <Callout tone="warning" title="服务器上的作答已有更新" className="mb-5">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <span>本机还保存着另一份没有同步的作答,选一份继续。</span>
            <div className="flex flex-wrap gap-2">
              <Button variant="outline" onClick={discardConflictingLocalDraft}>
                使用服务器版本
              </Button>
              <Button variant="primary" onClick={restoreConflictingLocalDraft}>
                保留本机作答
              </Button>
            </div>
          </div>
        </Callout>
      ) : null}
      {localCacheError ? (
        <Callout tone="warning" className="mb-5">
          本机恢复记录暂时无法使用,服务器上的作答仍会继续保存。
        </Callout>
      ) : null}

      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:gap-6">
        <AnswerSheet
          items={sheetItems}
          currentId={currentId}
          onJump={jumpTo}
          className="lg:sticky lg:top-4 lg:w-60 lg:shrink-0"
        />

        <div className="flex min-w-0 flex-1 flex-col gap-4">
          {hasShortAnswer ? (
            <p className="flex items-center gap-2 text-sm text-ink-sub">
              <Icon icon={Info} size="sm" className="shrink-0" />
              简答题由老师批改,其余题目提交后自动判分。
            </p>
          ) : null}
          {detail.items.map((item, index) => (
            <QuestionCard
              key={item.id}
              item={item}
              seq={index + 1}
              value={answers[item.id]}
              onChange={(value: TheoryAnswer) => updateAnswer(item.id, value)}
              onBlur={() => void saveDraft()}
              disabled={readOnly || submitting || recoveryConflict}
            />
          ))}
        </div>
      </div>

      {readOnly ? (
        <AssignmentClosedBar
          reason={submitState.reason ?? ''}
          resultsHref={attemptsUsed > 0 ? resultsHref : undefined}
        />
      ) : (
        <AssignmentActionBar
          autosaveState={autosaveState}
          dirtyCount={dirtyCount}
          savedAt={savedAt}
          conflict={recoveryConflict}
          answered={answeredCount}
          total={detail.items.length}
          late={submitState.late}
          busy={submitting}
          onSave={() => void saveDraft()}
          onSubmit={openConfirm}
        />
      )}

      <SubmitConfirmModal
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        total={detail.items.length}
        unanswered={sheetItems.filter((item) => !item.answered)}
        remainingAfter={Math.max(0, submitState.remaining - 1)}
        late={submitState.late}
        latePolicyText={latePolicyLabel(assignment.late_policy)}
        submitting={submitting}
        error={submitError}
        onConfirm={() => void submit()}
        onJump={jumpTo}
      />

      <UnsavedChangesGuard
        when={!readOnly && dirtyCount > 0}
        description="不保存就离开,这次的改动会丢失。保存到服务器后,换台设备也能接着写。"
        onSave={saveDraft}
      />
    </>
  )
}
