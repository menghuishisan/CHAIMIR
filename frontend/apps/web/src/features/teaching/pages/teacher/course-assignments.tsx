// 作业管理(课程详情页内区块)。
// 作业创建与编辑在同一弹层:后端 AssignmentRequest 两者字段一致。
// 题目从 M5 题库选取(按 item_code + item_version 锁版本),不让教师手填题目编号 ——
// 旧前端「编辑多题作业静默丢弃第 2 题及以后」正是把题目当单值处理造成的。

import { useCallback, useMemo, useState } from 'react'
import { ClipboardList, FileCheck2, Pencil, Plus, Send } from 'lucide-react'
import {
  AssignmentStatus,
  GradingMode,
  LatePolicy,
  type Assignment,
  type AssignmentItemInput,
  type AssignmentRequest,
  type CourseOutline,
} from '@chaimir/api-client'
import {
  Button,
  Callout,
  Empty,
  FormField,
  IconButton,
  Input,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
	PageSection,
	Pagination,
  Select,
  StatusIndicator,
  Table,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { toDateTimeInputValue } from '../../../../utils/dateInput'
import { ResourceState } from '../../../../components/ResourceState'
import { ContentItemPicker } from '../../../content/components/ContentItemPicker'
import { useAsyncResource, usePagedResource } from '../../../../hooks'
import { formatDateTime } from '../../../../utils/formatters'
import {
  assignmentStatusLabel,
  gradingModeLabel,
  latePolicyLabel,
} from '../../../../utils/labels/teaching'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { LATE_POLICIES } from '../../options'

export interface CourseAssignmentsProps {
  courseId: string
  outline: CourseOutline
  onOpenGrading: (assignmentId: string) => void
}

/**
 * CourseAssignments 管理课程作业。
 */
export function CourseAssignments({ courseId, outline, onOpenGrading }: CourseAssignmentsProps) {
  const [formOpen, setFormOpen] = useState<{ assignment?: Assignment } | undefined>()
  const [publishTarget, setPublishTarget] = useState<Assignment>()
  const [working, setWorking] = useState(false)
  const [actionError, setActionError] = useState<string>()

	const assignments = usePagedResource<Assignment>((params) => api.teaching.listCourseAssignments(courseId, params), [courseId])

  /** publishAssignment 发布作业:发布后学生可见,题目与分值不能再改。 */
  const publishAssignment = useCallback(async () => {
    if (!publishTarget) return
    setWorking(true)
    setActionError(undefined)
    try {
      await api.teaching.publishAssignment(publishTarget.id)
      toast.success('作业已发布')
      setPublishTarget(undefined)
      assignments.reload()
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '发布没有成功,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [assignments, publishTarget])

  const columns: TableColumn<Assignment>[] = [
    {
      key: 'title',
      header: '作业',
      render: (assignment) => <span className="font-medium text-ink">{assignment.title}</span>,
    },
    {
      key: 'due_at',
      header: '截止时间',
      render: (assignment) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatDateTime(assignment.due_at)}
        </span>
      ),
    },
    { key: 'max_attempts', header: '可提交次数', align: 'right', mono: true },
    {
      key: 'late_policy',
      header: '迟交规则',
      render: (assignment) => (
        <span className="text-ink-sub">{latePolicyLabel(assignment.late_policy)}</span>
      ),
    },
    {
      key: 'status',
      header: '状态',
      render: (assignment) => (
        <StatusIndicator
          tone={assignment.status === AssignmentStatus.PUBLISHED ? 'success' : 'neutral'}
          label={assignmentStatusLabel(assignment.status)}
        />
      ),
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (assignment) => (
        <div className="flex justify-end gap-1">
          {assignment.status === AssignmentStatus.DRAFT ? (
            <>
              <IconButton
                variant="ghost"
                size="sm"
                icon={Pencil}
                aria-label={`编辑作业 ${assignment.title}`}
                onClick={() => setFormOpen({ assignment })}
              />
              <Button variant="ghost" size="sm" leftIcon={Send} onClick={() => setPublishTarget(assignment)}>
                发布
              </Button>
            </>
          ) : (
            <Button
              variant="ghost"
              size="sm"
              leftIcon={FileCheck2}
              onClick={() => onOpenGrading(assignment.id)}
            >
              去批改
            </Button>
          )}
        </div>
      ),
    },
  ]

  return (
    <PageSection
      title="作业管理"
      description="作业草稿可以随时修改;发布后学生可见,题目与分值不再变动。"
      actions={
        <Button variant="primary" leftIcon={Plus} onClick={() => setFormOpen({})}>
          新建作业
        </Button>
      }
    >
      <div className="flex flex-col gap-4">
        {actionError ? <Callout tone="danger">{actionError}</Callout> : null}

        <ResourceState
          resource={assignments}
          emptyIcon={ClipboardList}
          emptyTitle="还没有作业"
          emptyDescription="从题库选题创建作业,发布后学生就能作答。"
          emptyAction={
            <Button variant="primary" leftIcon={Plus} onClick={() => setFormOpen({})}>
              新建作业
            </Button>
          }
          skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
        >
			{(page) => (
				<div className="flex flex-col gap-4">
					<Table columns={columns} data={page.list} rowKey={(item) => item.id} />
					<Pagination page={assignments.page} pageSize={assignments.pageSize} total={assignments.total} onPageChange={assignments.setPage} />
				</div>
			)}
        </ResourceState>
      </div>

      {formOpen ? (
        <AssignmentFormModal
          courseId={courseId}
          assignment={formOpen.assignment}
          outline={outline}
          onClose={() => setFormOpen(undefined)}
          onSaved={() => {
            setFormOpen(undefined)
            assignments.reload()
          }}
        />
      ) : null}

      <Modal open={publishTarget !== undefined} onOpenChange={(open) => !open && setPublishTarget(undefined)}>
        <ModalContent size="sm">
          <ModalHeader>
            <ModalTitle>确认发布作业</ModalTitle>
            <ModalDescription>
              发布后学生可以看到题目并开始作答,题目与分值不能再修改。
            </ModalDescription>
          </ModalHeader>
          <ModalBody>
            <p className="text-base text-ink">{publishTarget?.title}</p>
            {publishTarget ? (
              <p className="mt-1 text-sm text-ink-sub">
                截止时间 {formatDateTime(publishTarget.due_at)}
              </p>
            ) : null}
          </ModalBody>
          <ModalFooter>
            <Button variant="outline" onClick={() => setPublishTarget(undefined)}>
              再检查一下
            </Button>
            <Button variant="seal" loading={working} onClick={() => void publishAssignment()}>
              确认发布
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>
    </PageSection>
  )
}

interface AssignmentFormModalProps {
  courseId: string
  assignment?: Assignment
  outline: CourseOutline
  onClose: () => void
  onSaved: () => void
}

/**
 * AssignmentFormModal 承载作业创建与编辑,含从题库选题。
 */
function AssignmentFormModal({
  courseId,
  assignment,
  outline,
  onClose,
  onSaved,
}: AssignmentFormModalProps) {
  const editing = assignment !== undefined
  const [title, setTitle] = useState(assignment?.title ?? '')
  const [chapterId, setChapterId] = useState(assignment?.chapter_id ?? '')
  const [dueAt, setDueAt] = useState(toDateTimeInputValue(assignment?.due_at))
  const [maxAttempts, setMaxAttempts] = useState(String(assignment?.max_attempts ?? 1))
  const [latePolicy, setLatePolicy] = useState(String(assignment?.late_policy ?? LatePolicy.REJECT))
  const [items, setItems] = useState<AssignmentItemInput[]>([])
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  // 编辑已有作业时把题目载入:后端 GetAssignment 回题目清单(含分值与判分方式)
  const detail = useAsyncResource(
    () => (editing ? api.teaching.getAssignment(assignment.id) : Promise.resolve(undefined)),
    [assignment?.id, editing],
    () => false,
  )
  const loadedItems = detail.data?.items

  // 首次载入后把服务端题目铺进本地编辑态(只在题目尚未编辑过时同步)
  const syncedRef = useMemo(() => ({ done: false }), [])
  if (loadedItems && !syncedRef.done) {
    syncedRef.done = true
    setItems(
      loadedItems.map((item) => ({
        item_code: item.item_code,
        item_version: item.item_version,
        score: item.score,
        seq: item.seq,
        grading_mode: item.grading_mode,
        judger_code: item.judger_code ?? '',
      })),
    )
  }

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (title.trim() === '') {
        setFormError('请输入作业标题')
        return
      }
      if (dueAt === '') {
        setFormError('请选择截止时间')
        return
      }
      if (items.length === 0) {
        setFormError('请至少添加一道题目')
        return
      }
      setFormError(undefined)
      setWorking(true)
      try {
        const payload: AssignmentRequest = {
          title: title.trim(),
          due_at: new Date(dueAt).toISOString(),
          max_attempts: Number(maxAttempts) || 1,
          late_policy: Number(latePolicy) as LatePolicy,
          // 迟交扣分规则由后端按策略解释;不提供扣分比例输入时传空对象
          late_penalty: {},
          items: items.map((item, index) => ({ ...item, seq: index + 1 })),
        }
        if (chapterId !== '') payload.chapter_id = chapterId
        if (editing) {
          await api.teaching.updateAssignment(assignment.id, payload)
          toast.success('作业已更新')
        } else {
          await api.teaching.createAssignment(courseId, payload)
          toast.success('作业已创建为草稿')
        }
        onSaved()
      } catch (error) {
        setFormError(userFacingErrorMessage(error, '保存没有成功,请稍后重试。'))
      } finally {
        setWorking(false)
      }
    },
    [assignment?.id, chapterId, courseId, dueAt, editing, items, latePolicy, maxAttempts, onSaved, title],
  )

  const chapterOptions = useMemo(
    () => [
      { value: '', label: '不归入章节' },
      ...outline.chapters.map((chapter) => ({ value: chapter.id, label: chapter.title })),
    ],
    [outline.chapters],
  )

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="xl">
        <ModalHeader>
          <ModalTitle>{editing ? '编辑作业' : '新建作业'}</ModalTitle>
          <ModalDescription>
            从题库选题组成作业。客观题与编程题自动判分,主观题由你批改。
          </ModalDescription>
        </ModalHeader>
        <form onSubmit={submit} noValidate>
          <ModalBody className="flex flex-col gap-4">
            <FormField label="作业标题" htmlFor="assignment-title" required>
              <Input
                id="assignment-title"
                value={title}
                onChange={(event) => setTitle(event.target.value)}
              />
            </FormField>

            <div className="grid gap-4 sm:grid-cols-2">
              <FormField label="所属章节" htmlFor="assignment-chapter">
                <Select
                  id="assignment-chapter"
                  options={chapterOptions}
                  value={chapterId}
                  onValueChange={setChapterId}
                />
              </FormField>
              <FormField label="截止时间" htmlFor="assignment-due" required>
                <Input
                  id="assignment-due"
                  type="datetime-local"
                  value={dueAt}
                  onChange={(event) => setDueAt(event.target.value)}
                />
              </FormField>
            </div>

            <div className="grid gap-4 sm:grid-cols-2">
              <FormField label="可提交次数" htmlFor="assignment-attempts" required helper="学生最多能提交几次">
                <Input
                  id="assignment-attempts"
                  type="number"
                  min="1"
                  value={maxAttempts}
                  onChange={(event) => setMaxAttempts(event.target.value)}
                />
              </FormField>
              <FormField label="迟交规则" htmlFor="assignment-late" required>
                <Select
                  id="assignment-late"
                  options={LATE_POLICIES.map((value) => ({
                    value: String(value),
                    label: latePolicyLabel(value),
                  }))}
                  value={latePolicy}
                  onValueChange={setLatePolicy}
                />
              </FormField>
            </div>

            <AssignmentItemsEditor items={items} onChange={setItems} />

            {formError ? <Callout tone="danger">{formError}</Callout> : null}
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" variant="primary" loading={working}>
              {editing ? '保存修改' : '创建作业'}
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  )
}

interface AssignmentItemsEditorProps {
  items: AssignmentItemInput[]
  onChange: (items: AssignmentItemInput[]) => void
}

/**
 * AssignmentItemsEditor 从题库选题并设置每题分值与判分方式。
 * 题目按 code + version 锁定版本(后端要求),题目列表全量保留,不静默丢弃。
 */
function AssignmentItemsEditor({ items, onChange }: AssignmentItemsEditorProps) {
  const [pickerOpen, setPickerOpen] = useState(false)

  const updateItem = useCallback(
    (index: number, patch: Partial<AssignmentItemInput>) => {
      onChange(items.map((item, i) => (i === index ? { ...item, ...patch } : item)))
    },
    [items, onChange],
  )

  const removeItem = useCallback(
    (index: number) => onChange(items.filter((_, i) => i !== index)),
    [items, onChange],
  )

  const totalScore = items.reduce((sum, item) => sum + item.score, 0)

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="text-sm font-medium text-ink">
          题目({items.length} 题 · 合计 {totalScore} 分)
        </div>
        <Button type="button" variant="outline" size="sm" leftIcon={Plus} onClick={() => setPickerOpen(true)}>
          从题库选题
        </Button>
      </div>

      {items.length === 0 ? (
        <Empty
          icon={ClipboardList}
          title="还没有题目"
          description="从题库选题后设置每题分值。"
          action={
            <Button type="button" variant="outline" size="sm" leftIcon={Plus} onClick={() => setPickerOpen(true)}>
              从题库选题
            </Button>
          }
        />
      ) : (
        <div className="flex flex-col gap-2">
          {items.map((item, index) => (
            <div
              key={`${item.item_code}-${item.item_version}`}
              className="flex flex-wrap items-end gap-3 well p-3"
            >
              <div className="min-w-0 flex-1">
                <div className="truncate text-base text-ink">
                  第 {index + 1} 题 · {item.item_code}
                </div>
                <div className="truncate font-mono text-xs text-ink-sub">版本 {item.item_version}</div>
              </div>
              <FormField label="分值" htmlFor={`item-score-${index}`} className="mb-0 w-24">
                <Input
                  id={`item-score-${index}`}
                  type="number"
                  min="1"
                  value={String(item.score)}
                  onChange={(event) => updateItem(index, { score: Number(event.target.value) || 0 })}
                />
              </FormField>
              <FormField label="判分方式" htmlFor={`item-mode-${index}`} className="mb-0 w-36">
                <Select
                  id={`item-mode-${index}`}
                  size="sm"
                  options={[
                    { value: String(GradingMode.AUTO), label: gradingModeLabel(GradingMode.AUTO) },
                    { value: String(GradingMode.MANUAL), label: gradingModeLabel(GradingMode.MANUAL) },
                  ]}
                  value={String(item.grading_mode)}
                  onValueChange={(value) =>
                    updateItem(index, { grading_mode: Number(value) as GradingMode })
                  }
                />
              </FormField>
              <Button type="button" variant="ghost" size="sm" onClick={() => removeItem(index)}>
                移除
              </Button>
            </div>
          ))}
        </div>
      )}

      {pickerOpen ? (
        <ContentItemPicker
          selectedCodes={new Set(items.map((item) => item.item_code))}
          targetName="作业"
          onClose={() => setPickerOpen(false)}
          onPick={(picked) => {
            onChange([
              ...items,
              ...picked.map((item, index) => ({
                item_code: item.code,
                item_version: item.version,
                score: 10,
                seq: items.length + index + 1,
                grading_mode: GradingMode.MANUAL,
                judger_code: '',
              })),
            ])
            setPickerOpen(false)
          }}
        />
      ) : null}
    </div>
  )
}
