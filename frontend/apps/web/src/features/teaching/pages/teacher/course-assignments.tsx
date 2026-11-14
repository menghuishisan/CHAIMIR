// 作业管理(课程详情页内区块)。
// 作业创建与编辑在课程内容区使用同一表单,选题时切换为完整分页列表。
// 题目从 M5 题库选取(按 item_code + item_version 锁版本),不让教师手填题目编号 ——
// 旧前端「编辑多题作业静默丢弃第 2 题及以后」正是把题目当单值处理造成的。

import { useCallback, useMemo, useState } from 'react'
import { useNavigate } from 'react-router'
import { ClipboardList, FileCheck2, MoreVertical, Pencil, Plus, Send } from 'lucide-react'
import {
  AssignmentStatus,
  LatePolicy,
  ContentType,
  type Assignment,
  type AssignmentItemInput,
  type AssignmentRequest,
  type CourseOutline,
} from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  DataPanel,
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
  Menu,
  MenuContent,
  MenuItem,
  MenuTrigger,
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
import { useFormValidation } from '../../../../hooks/useFormValidation'
import { formatDateTime, formatRelativeDeadline } from '../../../../utils/formatters'
import { ASSIGNMENT_DUE_TONE } from '../../statusPresentation'
import {
  assignmentStatusLabel,
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
  const navigate = useNavigate()
  /** onOpenGradingHref 整行链接的本页跳转(中键与 Ctrl/⌘ 点击仍在新标签页打开) */
  const onOpenGradingHref = (href: string) => navigate(href)
  const [formOpen, setFormOpen] = useState<{ assignment?: Assignment } | undefined>()
  const [publishTarget, setPublishTarget] = useState<Assignment>()
  const [working, setWorking] = useState(false)
  const [actionError, setActionError] = useState<string>()

  const assignments = usePagedResource<Assignment>(
    (params) => api.teaching.listCourseAssignments(courseId, params),
    [courseId]
  )

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
      primary: true,
      render: (assignment) => <span className="font-medium text-ink">{assignment.title}</span>,
    },
    {
      key: 'due_at',
      header: '截止',
      render: (assignment) => {
        const deadline = formatRelativeDeadline(assignment.due_at)
        return (
          <span className="flex flex-col">
            <Badge tone={ASSIGNMENT_DUE_TONE[deadline.urgency]} className="self-start">
              {deadline.text}
            </Badge>
            <span className="mt-0.5 whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
              {formatDateTime(assignment.due_at)}
            </span>
          </span>
        )
      },
    },
    {
      key: 'submissions',
      header: '已交 · 待批改',
      align: 'right',
      // 按学生去重的全班统计;草稿作业还没有人能交,写「—」
      render: (assignment) =>
        assignment.status === AssignmentStatus.DRAFT || !assignment.submission_stats ? (
          <span className="text-ink-sub">—</span>
        ) : (
          <span className="whitespace-nowrap font-mono tabular-nums">
            {assignment.submission_stats.submitted_count} 人 ·{' '}
            <span className={assignment.submission_stats.pending_count > 0 ? 'text-warning' : 'text-ink-sub'}>
              {assignment.submission_stats.pending_count} 待批改
            </span>
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
        <div className="flex items-center justify-end gap-1">
          {assignment.status === AssignmentStatus.DRAFT ? (
            <Button variant="secondary" size="sm" onClick={() => setPublishTarget(assignment)}>
              发布
            </Button>
          ) : null}
          <AssignmentActionMenu
            assignment={assignment}
            onEdit={() => setFormOpen({ assignment })}
            onPublish={() => setPublishTarget(assignment)}
            onOpenGrading={() => onOpenGrading(assignment.id)}
          />
        </div>
      ),
    },
  ]

  if (formOpen) {
    return (
      <AssignmentForm
        courseId={courseId}
        assignment={formOpen.assignment}
        outline={outline}
        onClose={() => setFormOpen(undefined)}
        onSaved={() => {
          setFormOpen(undefined)
          assignments.reload()
        }}
      />
    )
  }

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

        {/* 列表型页内子视图走 DataPanel 片段(§6.5.5 B):数据表与分页同处一块抬起片 */}
        <DataPanel
          label="课程作业"
          footer={
            <Pagination
              page={assignments.page}
              pageSize={assignments.pageSize}
              total={assignments.total}
              onPageChange={assignments.setPage}
            />
          }
        >
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
            skeleton={
              <Table columns={columns} data={[]} rowKey={() => ''} loading elevated={false} />
            }
          >
            {(page) => (
              <Table
                columns={columns}
                data={page.list}
                rowKey={(item) => item.id}
                elevated={false}
                // 整行链向批改中心并带上这份作业(设计 §7.4)
                getRowHref={(item) => `/teacher/grading?course=${courseId}&assignment=${item.id}`}
                onNavigate={(href) => onOpenGradingHref(href)}
                // <md 换行卡(§6.4.1 规则 3):作业名一行、截止时间与提交情况一行,发布状态在右
                mobileCard={(item) => ({
                  title: item.title,
                  meta: [
                    `${formatDateTime(item.due_at)} 截止`,
                    item.submission_stats && item.status !== AssignmentStatus.DRAFT
                      ? `已交 ${item.submission_stats.submitted_count} 人 · 待批改 ${item.submission_stats.pending_count}`
                      : `可交 ${item.max_attempts} 次 · ${latePolicyLabel(item.late_policy)}`,
                  ].join(' · '),
                  badge: (
                    <StatusIndicator
                      tone={item.status === AssignmentStatus.PUBLISHED ? 'success' : 'neutral'}
                      label={assignmentStatusLabel(item.status)}
                    />
                  ),
                  action: (
                    <AssignmentActionMenu
                      assignment={item}
                      onEdit={() => setFormOpen({ assignment: item })}
                      onPublish={() => setPublishTarget(item)}
                      onOpenGrading={() => onOpenGrading(item.id)}
                    />
                  ),
                })}
              />
            )}
          </ResourceState>
        </DataPanel>
      </div>

      <Modal
        open={publishTarget !== undefined}
        onOpenChange={(open) => !open && setPublishTarget(undefined)}
      >
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

interface AssignmentActionMenuProps {
  assignment: Assignment
  onEdit: () => void
  onPublish: () => void
  onOpenGrading: () => void
}

/** AssignmentActionMenu 将作业的对象级动作收进同一入口,避免表格列与窄屏行卡都被按钮撑开。 */
function AssignmentActionMenu({
  assignment,
  onEdit,
  onPublish,
  onOpenGrading,
}: AssignmentActionMenuProps) {
  return (
    <Menu>
      <MenuTrigger asChild>
        <IconButton
          variant="ghost"
          size="sm"
          icon={MoreVertical}
          aria-label={`${assignment.title} 的更多操作`}
        />
      </MenuTrigger>
      <MenuContent align="end">
        {assignment.status === AssignmentStatus.DRAFT ? (
          <>
            <MenuItem icon={Pencil} onSelect={onEdit}>
              编辑作业
            </MenuItem>
            <MenuItem icon={Send} onSelect={onPublish}>
              发布作业
            </MenuItem>
          </>
        ) : (
          <MenuItem icon={FileCheck2} onSelect={onOpenGrading}>
            去批改
          </MenuItem>
        )}
      </MenuContent>
    </Menu>
  )
}

interface AssignmentFormProps {
  courseId: string
  assignment?: Assignment
  outline: CourseOutline
  onClose: () => void
  onSaved: () => void
}

/**
 * AssignmentForm 承载课程内容区内的作业创建、编辑与分页选题。
 */
function AssignmentForm({ courseId, assignment, outline, onClose, onSaved }: AssignmentFormProps) {
  const editing = assignment !== undefined
  const [title, setTitle] = useState(assignment?.title ?? '')
  const [chapterId, setChapterId] = useState(assignment?.chapter_id ?? '')
  const [dueAt, setDueAt] = useState(toDateTimeInputValue(assignment?.due_at))
  const [maxAttempts, setMaxAttempts] = useState(String(assignment?.max_attempts ?? 1))
  const [latePolicy, setLatePolicy] = useState(String(assignment?.late_policy ?? LatePolicy.REJECT))
  const [items, setItems] = useState<AssignmentItemInput[]>([])
  const [pickerOpen, setPickerOpen] = useState(false)
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  // 必填提示由当前输入计算,与全站表单共用展示规则;请求失败独立保存在formError。
  const { errors, validate } = useFormValidation(useCallback(() => ({
    title: title.trim() === '' ? '请输入作业标题' : null,
    dueAt: dueAt === '' ? '请选择截止时间' : null,
    items: items.length === 0 ? '请至少添加一道题目' : null,
  }), [dueAt, items.length, title]))

  // 编辑已有作业时把题目载入:后端 GetAssignment 回题目清单(含分值与判分方式)
  const detail = useAsyncResource(
    () => (editing ? api.teaching.getAssignment(assignment.id) : Promise.resolve(undefined)),
    [assignment?.id, editing],
    () => false
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
      }))
    )
  }

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (!validate()) return
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
    [
      assignment?.id,
      chapterId,
      courseId,
      dueAt,
      editing,
      items,
      latePolicy,
      maxAttempts,
      onSaved,
      title,
      validate,
    ]
  )

  const chapterOptions = useMemo(
    () => [
      { value: '', label: '不归入章节' },
      ...outline.chapters.map((chapter) => ({ value: chapter.id, label: chapter.title })),
    ],
    [outline.chapters]
  )

  if (pickerOpen) {
    return (
      <PageSection title="从题库选题" description="选择已发布版本后继续填写作业。">
        <ContentItemPicker
          selectedCodes={new Set(items.map((item) => item.item_code))}
          targetName="作业"
          type={ContentType.THEORY_QUESTION}
          onClose={() => setPickerOpen(false)}
          onPick={async (picked) => {
            setWorking(true)
            setFormError(undefined)
            try {
              const faces = await Promise.all(
                picked.map((item) => api.content.getItemFace(item.code, item.version))
              )
              setItems((current) => [
                ...current,
                ...picked.map((item, index) => ({
                  item_code: item.code,
                  item_version: item.version,
                  score: Number(faces[index].body.max_score),
                  seq: current.length + index + 1,
                })),
              ])
              setPickerOpen(false)
            } catch (error) {
              setFormError(userFacingErrorMessage(error, '题目详情没有读取成功,请重试。'))
            } finally {
              setWorking(false)
            }
          }}
        />
      </PageSection>
    )
  }

  return (
    <PageSection
      title={editing ? '编辑作业' : '新建作业'}
      description="从题库选择理论题。选择、判断和填空题自动判分,简答题由你批改。"
    >
      <form onSubmit={submit} noValidate>
        <div className="flex flex-col gap-4 rounded-lg bg-surface p-5 shadow-xs">
          <FormField label="作业标题" htmlFor="assignment-title" required error={errors.title}>
            <Input
              id="assignment-title"
              value={title}
              onChange={(event) => setTitle(event.target.value)}
              invalid={Boolean(errors.title)}
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
            <FormField label="截止时间" htmlFor="assignment-due" required error={errors.dueAt}>
              <Input
                id="assignment-due"
                type="datetime-local"
                value={dueAt}
                onChange={(event) => setDueAt(event.target.value)}
                invalid={Boolean(errors.dueAt)}
              />
            </FormField>
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <FormField
              label="可提交次数"
              htmlFor="assignment-attempts"
              required
              helper="学生最多能提交几次"
            >
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

          <AssignmentItemsEditor
            items={items}
            onChange={setItems}
            onOpenPicker={() => setPickerOpen(true)}
          />

          {errors.items ? <Callout tone="danger">{errors.items}</Callout> : null}

          {formError ? <Callout tone="danger">{formError}</Callout> : null}
        </div>
        <div className="mt-5 flex flex-wrap justify-end gap-3">
          <Button type="button" variant="outline" onClick={onClose}>
            取消
          </Button>
          <Button type="submit" variant="primary" loading={working}>
            {editing ? '保存修改' : '创建作业'}
          </Button>
        </div>
      </form>
    </PageSection>
  )
}

interface AssignmentItemsEditorProps {
  items: AssignmentItemInput[]
  onChange: (items: AssignmentItemInput[]) => void
  onOpenPicker: () => void
}

/**
 * AssignmentItemsEditor 从题库选择理论题,题目分值由 M5 锁定版本决定。
 * 题目按 code + version 锁定版本(后端要求),题目列表全量保留,不静默丢弃。
 */
function AssignmentItemsEditor({ items, onChange, onOpenPicker }: AssignmentItemsEditorProps) {
  const removeItem = useCallback(
    (index: number) => onChange(items.filter((_, i) => i !== index)),
    [items, onChange]
  )

  const totalScore = items.reduce((sum, item) => sum + item.score, 0)

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="text-sm font-medium text-ink">
          题目({items.length} 题 · 合计 {totalScore} 分)
        </div>
        <Button type="button" variant="outline" size="sm" leftIcon={Plus} onClick={onOpenPicker}>
          从题库选题
        </Button>
      </div>

      {items.length === 0 ? (
        <Empty
          icon={ClipboardList}
          title="还没有题目"
          description="从题库选题,分值跟随题目版本。"
          action={
            <Button
              type="button"
              variant="outline"
              size="sm"
              leftIcon={Plus}
              onClick={onOpenPicker}
            >
              从题库选题
            </Button>
          }
        />
      ) : (
        <div className="flex flex-col gap-2">
          {items.map((item, index) => (
            <div
              key={`${item.item_code}-${item.item_version}`}
              className="flex flex-wrap items-end gap-3 rounded-md bg-surface-hover p-3"
            >
              <div className="min-w-0 flex-1">
                <div className="truncate text-base text-ink">
                  第 {index + 1} 题 · {item.item_code}
                </div>
                <div className="truncate font-mono text-xs text-ink-sub">
                  版本 {item.item_version}
                </div>
              </div>
              <div className="w-24 pb-2">
                <div className="text-xs text-ink-sub">本题分值</div>
                <div className="font-mono text-sm tabular-nums text-ink">{item.score} 分</div>
              </div>
              <Button type="button" variant="ghost" size="sm" onClick={() => removeItem(index)}>
                移除
              </Button>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
