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
import {
  useActiveAnchor,
  useAsyncResource,
  usePageLocation,
  locationLabel,
} from '../../../../hooks'
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

const DRAFT_AUTOSAVE_INTERVAL_MS = 60_000

interface LocalAssignmentDraft {
  studentId: string
  assignmentId: string
  serverUpdatedAt: string | null
  answers: AnswerMap
}

function localDraftKey(studentId: string, assignmentId: string) {
  return `chaimir:teaching-draft:${studentId}:${assignmentId}`
}

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

export default function StudentAssignmentPage() {
  const { courseId = '', assignmentId = '' } = useParams<{
    courseId: string
    assignmentId: string
  }>()

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
  const submitState = submissionWindow({
    dueAt: assignment.due_at,
    latePolicy: assignment.late_policy,
    maxAttempts: assignment.max_attempts,
    attemptsUsed,
  })
  const readOnly = !submitState.open
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
  const [savedAnswers, setSavedAnswers] = useState<AnswerMap>(serverAnswers)

  const dirtyRef = useRef(restoreLocal)
  const editVersionRef = useRef(restoreLocal ? 1 : 0)
  const inFlightSaveRef = useRef<Promise<boolean> | null>(null)
  const serverUpdatedAtRef = useRef(draftUpdatedAt ?? null)
  const answersRef = useRef(answers)
  answersRef.current = answers

  const deadline = formatRelativeDeadline(assignment.due_at)
  const hasShortAnswer = detail.items.some((item) => item.body?.q_type === 'short_answer')

  const cacheDraft = useCallback(
    (draft: AnswerMap | null) => {
      try {
        if (draft === null) {
          window.localStorage.removeItem(localKey)
        } else {
          const record: LocalAssignmentDraft = {
            studentId,
            assignmentId,
            serverUpdatedAt: serverUpdatedAtRef.current,
            answers: draft,
          }
          window.localStorage.setItem(localKey, JSON.stringify(record))
        }
        setLocalCacheError(false)
        return true
      } catch (error) {
        setLocalCacheError(true)
        console.error('本机作答恢复记录无法更新', {
          operation:
            draft === null
              ? 'teaching.assignment.clearLocalDraft'
              : 'teaching.assignment.writeLocalDraft',
          error: errorDiagnostics(error),
        })
        return false
      }
    },
    [assignmentId, localKey, studentId]
  )

  // 保存期间有新编辑时继续同步最新快照；提交会等待同一个保存任务。
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
            cacheDraft(null)
          } else {
            cacheDraft(answersRef.current)
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
  }, [assignmentId, cacheDraft, detail.items])

  useEffect(() => {
    if (restoreLocal) void saveDraft()
  }, [restoreLocal, saveDraft])

  // 定时写回:60 秒一轮,与后端接口设计的自动保存节奏一致
  useEffect(() => {
    if (readOnly) return
    const timer = window.setInterval(() => void saveDraft(), DRAFT_AUTOSAVE_INTERVAL_MS)
    return () => window.clearInterval(timer)
  }, [readOnly, saveDraft])

  const updateAnswer = useCallback(
    (itemId: string, value: TheoryAnswer) => {
      if (readOnly || submitting || recoveryConflict) return
      const next = { ...answersRef.current, [itemId]: value }
      answersRef.current = next
      editVersionRef.current += 1
      dirtyRef.current = true
      setAutosaveState('idle')
      setAnswers(next)
      cacheDraft(next)
    },
    [cacheDraft, readOnly, recoveryConflict, submitting]
  )

  const discardConflictingLocalDraft = useCallback(() => {
    if (cacheDraft(null)) setRecoveryConflict(false)
  }, [cacheDraft])

  const restoreConflictingLocalDraft = useCallback(() => {
    if (!pendingLocalAnswers) return
    const next = draftAnswers({}, detail.items, pendingLocalAnswers)
    answersRef.current = next
    editVersionRef.current += 1
    dirtyRef.current = true
    setAnswers(next)
    setSavedAnswers(serverAnswers)
    setRecoveryConflict(false)
    cacheDraft(next)
    void saveDraft()
  }, [cacheDraft, detail.items, pendingLocalAnswers, saveDraft, serverAnswers])

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

  const dirtyCount = useMemo(
    () =>
      detail.items.filter(
        (item) =>
          JSON.stringify(answers[item.id] ?? '') !== JSON.stringify(savedAnswers[item.id] ?? '')
      ).length,
    [answers, detail.items, savedAnswers]
  )

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
