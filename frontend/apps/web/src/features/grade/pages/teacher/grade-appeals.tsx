// 成绩申诉处理(批改中心内区块)。
//
// 学生对课程成绩有异议时提交申诉,后端把申诉分配给授课教师或学校管理员处理。
// 受理会解锁该课程已通过的成绩审核(教师随后可以改分并重新报送);驳回则维持原成绩。
// 两种处理的后果不同,故各自说明,并要求写处理说明 —— 学生会看到这个结论。
//
// 归属:申诉是批改/成绩内页(对齐清单 §3.2),不进侧栏。
// 后端 GET /appeals 在 teacher 组,学生侧不建申诉列表(§6.6)。
//
// 校管复用同一区块(校管也在 teacher 组里,能看全校),并额外拿到「重算绩点」——
// 那是 admin 组能力,教师没有,故由调用方通过 canRecompute 显式声明,组件不判角色枚举。

import { useCallback, useMemo, useState } from 'react'
import { CircleCheck, CircleX, RefreshCw, Scale } from 'lucide-react'
import {
  GradeAppealStatus,
  type Course,
  type GradeAppeal,
  type Semester,
} from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  DescriptionList,
  FilterBar,
  FilterField,
  FormField,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  PageSection,
  Pagination,
  SegmentedControl,
  Select,
  StatusIndicator,
  Table,
  Textarea,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, usePagedResource } from '../../../../hooks'
import { formatDateTime } from '../../../../utils/formatters'
import {
  gradeAppealStatusLabel,
  gradeAppealStatusTone,
} from '../../../../utils/labels/grade'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

/** 课程选择器一次取回的条数:后端分页上限 100。 */
const COURSE_PICKER_SIZE = 100

/** 状态筛选项:值为空串表示不过滤。 */
const STATUS_FILTERS = [
  { value: '', label: '全部' },
  { value: String(GradeAppealStatus.PENDING), label: '待处理' },
  { value: String(GradeAppealStatus.ACCEPTED), label: '已受理' },
  { value: String(GradeAppealStatus.COMPLETED), label: '已完成' },
  { value: String(GradeAppealStatus.REJECTED), label: '未通过' },
] as const

/** 处理动作:受理会解锁成绩,驳回维持原成绩。 */
type AppealDecision = 'accept' | 'reject'

const DECISION_COPY: Record<AppealDecision, { title: string; description: string; confirm: string }> = {
  accept: {
    title: '受理申诉',
    description:
      '受理后该课程已通过的成绩审核会被解锁,你可以调整成绩并重新报送。学生会看到你的处理说明。',
    confirm: '确认受理',
  },
  reject: {
    title: '驳回申诉',
    description: '驳回后成绩保持不变。请在说明里写清理由,学生会看到这段话。',
    confirm: '确认驳回',
  },
}

export interface GradeAppealsProps {
  /**
   * 是否显示「重算绩点」动作。
   * 重算是 admin 组能力(POST /students/{id}/recompute),教师没有权限;
   * 由调用方显式声明而不是在组件内判角色枚举 —— 权限边界由后端守卫,前端只按声明渲染。
   */
  canRecompute?: boolean
}

/**
 * GradeAppeals 列出分配给当前处理人的成绩申诉并承载受理与驳回。
 */
export function GradeAppeals({ canRecompute = false }: GradeAppealsProps) {
  const [statusFilter, setStatusFilter] = useState<string>('')
  const [decisionTarget, setDecisionTarget] = useState<{ appeal: GradeAppeal; decision: AppealDecision }>()
  const [recomputeTarget, setRecomputeTarget] = useState<GradeAppeal>()

  const appeals = usePagedResource<GradeAppeal>(
    (params) =>
      api.grade.listAppeals({
        status: statusFilter ? (Number(statusFilter) as GradeAppealStatus) : undefined,
        ...params,
      }),
    [statusFilter],
  )

  // 申诉记录只回 course_id,课程名从课程列表解析,不把内部编号抛到界面上。
  // 校管能看全校申诉,故按处理人身份取对应范围的课程:教师取自己授课的,校管取全校的。
  const courses = useAsyncResource(
    () =>
      api.teaching.getCourses(
        canRecompute ? { page: 1, size: COURSE_PICKER_SIZE } : { role: 'teacher', page: 1, size: COURSE_PICKER_SIZE },
      ),
    [canRecompute],
    () => false,
  )

  const courseNameById = useMemo(
    () => new Map((courses.data?.list ?? []).map((course: Course) => [course.id, course.name])),
    [courses.data],
  )

  const columns: TableColumn<GradeAppeal>[] = [
    {
      key: 'course_id',
      header: '课程',
      render: (appeal) => (
        <span className="text-ink">{courseNameById.get(appeal.course_id) ?? '其他课程'}</span>
      ),
    },
    {
      key: 'reason',
      header: '申诉理由',
      render: (appeal) => <span className="line-clamp-2 text-sm text-ink">{appeal.reason}</span>,
    },
    {
      key: 'created_at',
      header: '提交时间',
      render: (appeal) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatDateTime(appeal.created_at)}
        </span>
      ),
    },
    {
      key: 'result_comment',
      header: '处理说明',
      render: (appeal) =>
        appeal.result_comment ? (
          <span className="line-clamp-2 text-sm text-ink-sub">{appeal.result_comment}</span>
        ) : (
          <span className="text-ink-sub">尚未处理</span>
        ),
    },
    {
      key: 'status',
      header: '状态',
      render: (appeal) => (
        <div className="flex flex-wrap items-center gap-1.5">
          <StatusIndicator
            tone={gradeAppealStatusTone(appeal.status)}
            label={gradeAppealStatusLabel(appeal.status)}
          />
          {appeal.handled_at ? (
            <Badge tone="neutral">{formatDateTime(appeal.handled_at)}</Badge>
          ) : null}
        </div>
      ),
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (appeal) =>
        appeal.status === GradeAppealStatus.PENDING ? (
          <div className="flex items-center justify-end gap-1">
            <Button
              variant="ghost"
              size="sm"
              leftIcon={CircleCheck}
              onClick={() => setDecisionTarget({ appeal, decision: 'accept' })}
            >
              受理
            </Button>
            <Button
              variant="ghost"
              size="sm"
              leftIcon={CircleX}
              onClick={() => setDecisionTarget({ appeal, decision: 'reject' })}
            >
              驳回
            </Button>
          </div>
        ) : canRecompute && appeal.status === GradeAppealStatus.ACCEPTED ? (
          <Button
            variant="ghost"
            size="sm"
            leftIcon={RefreshCw}
            onClick={() => setRecomputeTarget(appeal)}
          >
            重算绩点
          </Button>
        ) : (
          <span className="text-sm text-ink-faint">已处理</span>
        ),
    },
  ]

  return (
    <PageSection
      title="成绩申诉"
      description={`共 ${appeals.total} 条申诉。受理会解锁该课程成绩,处理结论学生可见。`}
    >
      <div className="flex flex-col gap-4">
        <FilterBar label="成绩申诉筛选">
          <FilterField label="申诉状态" group>
            <SegmentedControl
              aria-label="按申诉状态筛选"
              size="sm"
              options={STATUS_FILTERS.map((item) => ({ value: item.value, label: item.label }))}
              value={statusFilter}
              onValueChange={setStatusFilter}
            />
          </FilterField>
        </FilterBar>

        {canRecompute ? (
          <Callout tone="info">
            受理后由授课教师改分,改分事件会自动触发绩点重算。如果学生反馈绩点没更新,可以在已受理的申诉上手动重算一次。
          </Callout>
        ) : null}

        <ResourceState
          resource={appeals}
          emptyIcon={Scale}
          emptyTitle={statusFilter ? '这个状态下没有申诉' : '没有待处理的申诉'}
          emptyDescription={
            statusFilter ? '换个状态看看。' : '学生对成绩有异议时提交的申诉会分配到这里。'
          }
          skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
        >
          {(page) => (
            <>
              <Table columns={columns} data={page.list} rowKey={(item) => item.id} />
              <Pagination
                page={appeals.page}
                pageSize={appeals.pageSize}
                total={appeals.total}
                onPageChange={appeals.setPage}
              />
            </>
          )}
        </ResourceState>
      </div>

      {decisionTarget ? (
        <AppealDecisionModal
          appeal={decisionTarget.appeal}
          decision={decisionTarget.decision}
          courseName={courseNameById.get(decisionTarget.appeal.course_id) ?? '该课程'}
          onClose={() => setDecisionTarget(undefined)}
          onSaved={() => {
            setDecisionTarget(undefined)
            appeals.reload()
          }}
        />
      ) : null}

      {recomputeTarget ? (
        <RecomputeModal
          appeal={recomputeTarget}
          courseName={courseNameById.get(recomputeTarget.course_id) ?? '该课程'}
          onClose={() => setRecomputeTarget(undefined)}
          onDone={() => {
            setRecomputeTarget(undefined)
            appeals.reload()
          }}
        />
      ) : null}
    </PageSection>
  )
}

interface AppealDecisionModalProps {
  appeal: GradeAppeal
  decision: AppealDecision
  courseName: string
  onClose: () => void
  onSaved: () => void
}

/**
 * AppealDecisionModal 提交受理或驳回结论。
 */
function AppealDecisionModal({
  appeal,
  decision,
  courseName,
  onClose,
  onSaved,
}: AppealDecisionModalProps) {
  const [comment, setComment] = useState('')
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const copy = DECISION_COPY[decision]

  const submit = useCallback(async () => {
    if (comment.trim() === '') {
      setFormError('请写下处理说明,学生会看到这段话')
      return
    }
    setFormError(undefined)
    setWorking(true)
    try {
      const payload = { comment: comment.trim() }
      if (decision === 'accept') {
        await api.grade.acceptAppeal(appeal.id, payload)
        toast.success('申诉已受理,该课程成绩已解锁')
      } else {
        await api.grade.rejectAppeal(appeal.id, payload)
        toast.success('申诉已驳回')
      }
      onSaved()
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '处理没有成功,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [appeal.id, comment, decision, onSaved])

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="lg">
        <ModalHeader>
          <ModalTitle>{copy.title}</ModalTitle>
          <ModalDescription>{copy.description}</ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
          <DescriptionList
            dense
            items={[
              { term: '课程', description: courseName },
              { term: '提交时间', description: formatDateTime(appeal.created_at), mono: true },
              { term: '申诉理由', description: appeal.reason },
            ]}
          />

          {decision === 'accept' ? (
            <Callout tone="warning" title="受理会解锁成绩">
              解锁后这门课的成绩回到可调整状态,调整完需要重新报送给学校审核。
            </Callout>
          ) : null}

          <FormField
            label="处理说明"
            htmlFor="appeal-comment"
            required
            helper="写清处理结论与依据,学生会看到这段话"
            error={formError}
          >
            <Textarea
              id="appeal-comment"
              value={comment}
              rows={4}
              invalid={Boolean(formError)}
              onChange={(event) => setComment(event.target.value)}
            />
          </FormField>
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            取消
          </Button>
          <Button
            variant={decision === 'accept' ? 'seal' : 'danger'}
            loading={working}
            onClick={() => void submit()}
          >
            {copy.confirm}
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}

interface RecomputeModalProps {
  appeal: GradeAppeal
  courseName: string
  onClose: () => void
  onDone: () => void
}

/**
 * RecomputeModal 手动重算学生的学期绩点。
 * 这是接口设计 §8 时序的兜底路径:正常情况下教师改分会发事件驱动重算,
 * 事件未到达时管理员在这里手动补一次。后端要求指定学期。
 */
function RecomputeModal({ appeal, courseName, onClose, onDone }: RecomputeModalProps) {
  const [semesterId, setSemesterId] = useState('')
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const semesters = useAsyncResource(() => api.grade.listSemesters(), [], () => false)

  const submit = useCallback(async () => {
    if (semesterId === '') {
      setFormError('请选择要重算的学期')
      return
    }
    setFormError(undefined)
    setWorking(true)
    try {
      await api.grade.recomputeStudentGrade(appeal.student_id, { semester_id: semesterId })
      toast.success('绩点已重算')
      onDone()
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '重算没有完成,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [appeal.student_id, onDone, semesterId])

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="sm">
        <ModalHeader>
          <ModalTitle>重算绩点</ModalTitle>
          <ModalDescription>
            按当前已锁定的课程成绩重新计算这名学生的学期绩点与累计绩点。
          </ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
          <DescriptionList
            dense
            items={[
              { term: '申诉课程', description: courseName },
              { term: '提交时间', description: formatDateTime(appeal.created_at), mono: true },
            ]}
          />

          <FormField
            label="重算学期"
            htmlFor="recompute-semester"
            required
            error={formError}
            helper="选申诉课程所属的学期"
          >
            <Select
              id="recompute-semester"
              options={(semesters.data ?? []).map((semester: Semester) => ({
                value: semester.id,
                label: semester.name,
              }))}
              value={semesterId}
              placeholder="选择学期"
              onValueChange={setSemesterId}
            />
          </FormField>

          <Callout tone="info">
            正常情况下教师改分会自动触发重算。这里是事件未到达时的补救,重复重算不会有副作用。
          </Callout>
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            取消
          </Button>
          <Button variant="primary" leftIcon={RefreshCw} loading={working} onClick={() => void submit()}>
            开始重算
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}
