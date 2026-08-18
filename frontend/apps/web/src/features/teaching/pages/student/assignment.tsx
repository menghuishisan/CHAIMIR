// 作业作答页(深页,/student/courses/:courseId/assignments/:assignmentId)。
//
// FE-7 服务端草稿:进入先拉服务端草稿,编辑按 60 秒定时 + 失焦即时写回服务端,
// 服务端草稿是权威;本地只保留尚未同步的编辑态与最近同步标记,
// 服务端读写失败时不用本地内容替代草稿进入提交链路(规范 §6.6)。
//
// 答案黑盒:题面经 M6 → M5 GetContentFace 取得,答案/判题配置已在后端剥离,
// 前端不做二次过滤也不请求 full 内容(铁律 1 + 对齐清单 §6.4)。
//
// 编程题的代码归档由沉浸式工作台在沙箱内保存后产出 code_storage_key(阶段 5),
// 本页承载客观题与主观题作答;编程题在此说明去处,不在页面里让用户手填对象存储键。

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router'
import { ClipboardList, FileText, Send } from 'lucide-react'
import { GradingMode, type AssignmentDetail, type AssignmentItem } from '@chaimir/api-client'
import {
  Autosave,
  Badge,
  Breadcrumb,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  DescriptionList,
  FormField,
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
  RadioGroup,
  RadioItem,
  Textarea,
  toast,
  type AutosaveState,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import { formatDateTime, formatRelativeDeadline } from '../../../../utils/formatters'
import { contentDifficultyLabel } from '../../../../utils/labels/content'
import { latePolicyLabel } from '../../../../utils/labels/teaching'
import { errorDiagnostics, userFacingErrorMessage } from '../../../../utils/userFacingError'
import { ASSIGNMENT_DUE_TONE } from '../../statusPresentation'
import { itemStatement, itemChoices } from './assignment-item'

/** 服务端草稿定时写回间隔:与 M6 接口设计 §6.2「自动每 60s」一致。 */
const DRAFT_AUTOSAVE_INTERVAL_MS = 60_000

/** AnswerMap 是按题目编号索引的作答内容,提交与草稿共用同一形状。 */
type AnswerMap = Record<string, string>

/**
 * StudentAssignmentPage 读取作业题面与服务端草稿。
 */
export default function StudentAssignmentPage() {
  const { courseId = '', assignmentId = '' } = useParams<{ courseId: string; assignmentId: string }>()

  // 题面与草稿一次读齐:草稿要按题目编号铺回表单,缺任一方都无法进入可编辑状态
  const view = useAsyncResource(
    () =>
      Promise.all([
        api.teaching.getAssignment(assignmentId),
        api.teaching.getDraft(assignmentId),
      ]).then(([detail, draft]) => ({ detail, draft })),
    [assignmentId],
    () => false,
  )

  return (
    <PageScaffold>
      <ResourceState
        resource={view}
        emptyIcon={ClipboardList}
        emptyTitle="作业内容暂未开放"
        emptyDescription="老师还没有为这次作业添加题目。"
      >
        {(data) => (
          <AssignmentForm
            courseId={courseId}
            assignmentId={assignmentId}
            detail={data.detail}
            initialAnswers={draftAnswers(data.draft.content)}
            draftUpdatedAt={data.draft.exists ? data.draft.updated_at : undefined}
          />
        )}
      </ResourceState>
    </PageScaffold>
  )
}

/**
 * draftAnswers 把服务端草稿正文收敛成表单可用的作答表。
 * 草稿正文是开放对象(后端只校验非空),只接受字符串值 —— 其他类型无法回填到文本控件,
 * 静默转字符串会让用户看到不是自己写的内容。
 */
function draftAnswers(content: Record<string, unknown>): AnswerMap {
  const answers: AnswerMap = {}
  for (const [itemId, value] of Object.entries(content)) {
    if (typeof value === 'string') answers[itemId] = value
  }
  return answers
}

interface AssignmentFormProps {
  courseId: string
  assignmentId: string
  detail: AssignmentDetail
  initialAnswers: AnswerMap
  draftUpdatedAt: string | undefined
}

/**
 * AssignmentForm 承载作答、草稿自动保存与提交。
 */
function AssignmentForm({
  courseId,
  assignmentId,
  detail,
  initialAnswers,
  draftUpdatedAt,
}: AssignmentFormProps) {
  const navigate = useNavigate()
  const [answers, setAnswers] = useState<AnswerMap>(initialAnswers)
  const [autosaveState, setAutosaveState] = useState<AutosaveState>('idle')
  const [savedAt, setSavedAt] = useState<Date | undefined>(
    draftUpdatedAt ? new Date(draftUpdatedAt) : undefined,
  )
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState<string>()

  // 未同步标记:只有真正改动过才写服务端,避免定时器空跑
  const dirtyRef = useRef(false)
  const answersRef = useRef(answers)
  answersRef.current = answers

  const assignment = detail.assignment
  const deadline = formatRelativeDeadline(assignment.due_at)
  const hasProgrammingItem = detail.items.some((item) => item.grading_mode === GradingMode.AUTO)

  /** saveDraft 把当前作答写回服务端草稿;服务端成功才认为已保存。 */
  const saveDraft = useCallback(async () => {
    if (!dirtyRef.current) return
    setAutosaveState('saving')
    try {
      const result = await api.teaching.saveDraft(assignmentId, { content: answersRef.current })
      dirtyRef.current = false
      setSavedAt(new Date(result.updated_at))
      setAutosaveState('saved')
    } catch (error) {
      // 保存失败必须让用户看见:草稿以服务端为权威,失败意味着这次编辑还没落库
      setAutosaveState('error')
      console.error('作业草稿写回失败', {
        operation: 'teaching.assignment.saveDraft',
        reason: 'draft-save-failed',
        error: errorDiagnostics(error),
      })
    }
  }, [assignmentId])

  // 定时写回:60 秒一轮,与后端接口设计的自动保存节奏一致
  useEffect(() => {
    const timer = window.setInterval(() => void saveDraft(), DRAFT_AUTOSAVE_INTERVAL_MS)
    return () => window.clearInterval(timer)
  }, [saveDraft])

  /** updateAnswer 记录作答并标记未同步。 */
  const updateAnswer = useCallback((itemId: string, value: string) => {
    dirtyRef.current = true
    setAutosaveState('idle')
    setAnswers((current) => ({ ...current, [itemId]: value }))
  }, [])

  /** submit 正式提交作答。提交前先把草稿落库,确保服务端与页面一致。 */
  const submit = useCallback(async () => {
    setSubmitting(true)
    setSubmitError(undefined)
    try {
      await api.teaching.submitAssignment(assignmentId, { content_ref: answersRef.current })
      setConfirmOpen(false)
      toast.success('作业已提交')
      navigate(`/student/courses/${courseId}/assignments/${assignmentId}/submissions`)
    } catch (error) {
      setSubmitError(userFacingErrorMessage(error, '作业提交失败,请稍后重试。'))
    } finally {
      setSubmitting(false)
    }
  }, [assignmentId, courseId, navigate])

  const answeredCount = detail.items.filter((item) => (answers[item.id] ?? '').trim() !== '').length

  return (
    <>
      <PageHeader
        kicker={
          <Breadcrumb
            items={[
              { label: '我的课程', href: '/student/courses' },
              { label: '课程详情', href: `/student/courses/${courseId}` },
              { label: assignment.title },
            ]}
          />
        }
        title={assignment.title}
        description="作答内容每分钟自动保存到服务器,刷新或换设备都能接着写。"
        icon={ClipboardList}
        actions={<Badge tone={ASSIGNMENT_DUE_TONE[deadline.urgency]}>{deadline.text}</Badge>}
      />

      <PageBody
        rail={
          <AssignmentRail
            assignment={assignment}
            answeredCount={answeredCount}
            itemCount={detail.items.length}
            autosaveState={autosaveState}
            savedAt={savedAt}
            submitError={submitError}
            onSaveDraft={() => void saveDraft()}
            onRequestSubmit={() => setConfirmOpen(true)}
            onViewSubmissions={() =>
              navigate(`/student/courses/${courseId}/assignments/${assignmentId}/submissions`)
            }
          />
        }
      >
        {hasProgrammingItem ? (
          <Callout tone="info" title="这次作业含编程题" className="mb-6">
            编程题需要在实验工作台的代码环境里完成并保存代码,再回到这里提交。
          </Callout>
        ) : null}

        <PageSection title="题目" description={`共 ${detail.items.length} 题`}>
          <div className="flex flex-col gap-4">
            {detail.items.map((item, index) => (
              <AssignmentItemCard
                key={item.id}
                item={item}
                seq={index + 1}
                value={answers[item.id] ?? ''}
                onChange={(value) => updateAnswer(item.id, value)}
                onBlur={() => void saveDraft()}
              />
            ))}
          </div>
        </PageSection>
      </PageBody>

      <Modal open={confirmOpen} onOpenChange={setConfirmOpen}>
        <ModalContent size="sm">
          <ModalHeader>
            <ModalTitle>确认提交作业</ModalTitle>
            <ModalDescription>
              提交后本次作答将进入批改流程。剩余可提交次数会减少一次。
            </ModalDescription>
          </ModalHeader>
          <ModalBody>
            <DescriptionList
              dense
              items={[
                { term: '已作答题目', description: `${answeredCount} / ${detail.items.length}`, mono: true },
                { term: '截止时间', description: formatDateTime(assignment.due_at), mono: true },
                { term: '迟交规则', description: latePolicyLabel(assignment.late_policy) },
              ]}
            />
            {answeredCount < detail.items.length ? (
              <Callout tone="warning" className="mt-3">
                还有题目没有作答,提交后未作答的题目按零分计。
              </Callout>
            ) : null}
          </ModalBody>
          <ModalFooter>
            <Button variant="outline" onClick={() => setConfirmOpen(false)}>
              再检查一下
            </Button>
            <Button variant="seal" loading={submitting} onClick={() => void submit()}>
              确认提交
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>
    </>
  )
}

interface AssignmentItemCardProps {
  item: AssignmentItem
  seq: number
  value: string
  onChange: (value: string) => void
  onBlur: () => void
}

/**
 * AssignmentItemCard 渲染单题题面与作答控件。
 * 选择型题目(题面带选项)用单选,其余用文本作答;题面与选项都来自后端题面视角。
 */
function AssignmentItemCard({ item, seq, value, onChange, onBlur }: AssignmentItemCardProps) {
  const statement = itemStatement(item)
  const choices = itemChoices(item)

  return (
    <Card>
      <CardHeader
        title={
          <span className="flex flex-wrap items-center gap-2">
            <span className="font-mono text-sm tabular-nums text-ink-sub">第 {seq} 题</span>
            <span className="min-w-0">{item.title ?? '题目'}</span>
          </span>
        }
        description={`本题 ${item.score} 分`}
        actions={
          item.difficulty !== undefined ? (
            <Badge tone="neutral">{contentDifficultyLabel(item.difficulty)}</Badge>
          ) : null
        }
      />
      <CardBody className="flex flex-col gap-4">
        {statement ? (
          <div className="flex flex-col gap-2 text-base leading-relaxed text-ink">
            {statement.split('\n').map((paragraph, index) =>
              paragraph.trim() === '' ? null : <p key={index}>{paragraph}</p>,
            )}
          </div>
        ) : (
          <Callout tone="info">这道题的题面暂未提供,请联系老师确认。</Callout>
        )}

        {item.grading_mode === GradingMode.AUTO && choices.length === 0 ? (
          <Callout tone="info" title="这是编程题">
            请在实验工作台的代码环境中完成并保存代码,保存后回到本页提交。
          </Callout>
        ) : choices.length > 0 ? (
          <FormField label="选择答案" required>
            <RadioGroup value={value} onValueChange={onChange}>
              <div className="flex flex-col gap-2">
                {choices.map((choice, index) => (
                  <RadioItem
                    key={index}
                    value={choice}
                    label={choice}
                    onBlur={onBlur}
                  />
                ))}
              </div>
            </RadioGroup>
          </FormField>
        ) : (
          <FormField label="作答内容" required>
            <Textarea
              value={value}
              rows={6}
              placeholder="写下你的解答"
              onChange={(event) => onChange(event.target.value)}
              onBlur={onBlur}
            />
          </FormField>
        )}
      </CardBody>
    </Card>
  )
}

interface AssignmentRailProps {
  assignment: AssignmentDetail['assignment']
  answeredCount: number
  itemCount: number
  autosaveState: AutosaveState
  savedAt: Date | undefined
  submitError: string | undefined
  onSaveDraft: () => void
  onRequestSubmit: () => void
  onViewSubmissions: () => void
}

/**
 * AssignmentRail 是右侧动作区:作业档案 + 保存状态 + 提交入口。
 */
function AssignmentRail({
  assignment,
  answeredCount,
  itemCount,
  autosaveState,
  savedAt,
  submitError,
  onSaveDraft,
  onRequestSubmit,
  onViewSubmissions,
}: AssignmentRailProps) {
  const items = useMemo(
    () => [
      { term: '截止时间', description: formatDateTime(assignment.due_at), mono: true },
      { term: '可提交次数', description: assignment.max_attempts, mono: true },
      { term: '迟交规则', description: latePolicyLabel(assignment.late_policy) },
      { term: '已作答', description: `${answeredCount} / ${itemCount}`, mono: true },
    ],
    [answeredCount, assignment.due_at, assignment.late_policy, assignment.max_attempts, itemCount],
  )

  return (
    <div className="flex flex-col gap-4">
      <Card>
        <CardHeader title="作业信息" />
        <CardBody className="flex flex-col gap-4">
          <DescriptionList dense items={items} />
          <Autosave state={autosaveState} savedAt={savedAt} onRetry={onSaveDraft} />
          {autosaveState === 'error' ? (
            <Callout tone="danger">
              作答内容还没有保存到服务器,请点击「重试」或稍后再试;这段时间请不要关闭页面。
            </Callout>
          ) : null}
        </CardBody>
      </Card>

      <Card>
        <CardHeader title="提交作答" description="提交后进入批改流程,可提交次数会减少一次。" />
        <CardBody className="flex flex-col gap-3">
          {submitError ? <Callout tone="danger">{submitError}</Callout> : null}
          <Button variant="outline" onClick={onSaveDraft}>
            立即保存草稿
          </Button>
          <Button variant="seal" leftIcon={Send} onClick={onRequestSubmit}>
            提交作业
          </Button>
          <Button variant="ghost" leftIcon={FileText} onClick={onViewSubmissions}>
            查看历次提交
          </Button>
        </CardBody>
      </Card>
    </div>
  )
}
