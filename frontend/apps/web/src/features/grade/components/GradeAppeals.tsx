// 成绩领域的申诉处理复用区块。
//
// 学生对课程成绩有异议时提交申诉,后端把申诉分配给授课教师或学校管理员处理。
// 受理会解锁该课程已通过的成绩审核(教师随后可以改分并重新报送);驳回则维持原成绩。
// 两种处理的后果不同,故各自说明,并要求写处理说明 —— 学生会看到这个结论。
//
// 归属:申诉是批改/成绩内页(对齐清单 §3.2),不进侧栏。
// 批改中心和校管申诉页共享本组件，页面只提供各自的入口与壳层。
// 后端 GET /appeals 在 teacher 组,学生侧不建申诉列表(§6.6)。
//
// 校管复用同一区块(校管也在 teacher 组里,能看全校),并额外拿到「重算绩点」——
// 那是 admin 组能力,教师没有,故由调用方通过 canRecompute 显式声明,组件不判角色枚举。

import { useCallback, useMemo, useState, type KeyboardEvent } from 'react'
import { ChevronLeft, CircleCheck, CircleX, RefreshCw, Scale } from 'lucide-react'
import {
  GradeAppealStatus,
  PAGINATION_MAX_SIZE,
  type Course,
  type GradeAppeal,
  type Semester,
} from '@chaimir/api-client'
import {
  Button,
  Callout,
  cn,
  DataPanel,
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
  QueueDetailLayout,
  SegmentedControl,
  Select,
  Skeleton,
  StatusIndicator,
  Textarea,
  toast,
} from '@chaimir/ui'
import { api } from '../../../app/api'
import { ResourceState } from '../../../components/ResourceState'
import { useAsyncResource, usePagedResource } from '../../../hooks'
import { formatDateTime } from '../../../utils/formatters'
import { gradeAppealStatusLabel } from '../../../utils/labels/grade'
import { userFacingErrorMessage } from '../../../utils/userFacingError'
import { gradeAppealStatusTone } from '../statusPresentation'

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
        canRecompute
          ? { page: 1, size: PAGINATION_MAX_SIZE }
          : { role: 'teacher', page: 1, size: PAGINATION_MAX_SIZE },
      ),
    [canRecompute],
    () => false,
  )

  const courseNameById = useMemo(
    () => new Map((courses.data?.list ?? []).map((course: Course) => [course.id, course.name])),
    [courses.data],
  )

  /**
   * 审阅队列族(§6.5.3 第 ⑤ 族):左队列右详情。
   * 这一族的动作是「逐条处理完一批」而不是「找一条记录」,所以详情与处理表单常驻右侧、
   * 键盘上下切条、处理完自动进下一条 —— 原先的五列表格 + 弹窗确认要求
   * 「看行 → 开弹窗 → 关弹窗 → 再找下一行」,每条都要四次视线跳转。
   */
  // 队列数据:用 useMemo 稳住引用,否则每次渲染都是新数组,下方 useCallback 的依赖每帧都变
  const list = useMemo(() => appeals.data?.list ?? [], [appeals.data])
  const [selectedId, setSelectedId] = useState<string>()
  /** <lg 走两级页面(§6.4.1 规则 4):当前在队列层还是详情层 */
  const [mobileView, setMobileView] = useState<'queue' | 'detail'>('queue')
  // 选中项:未指定或已翻页失效时落到队列首条,避免右侧空白
  const selected = list.find((item) => item.id === selectedId) ?? list[0]
  const selectedIndex = selected ? list.findIndex((item) => item.id === selected.id) : -1

  /** 处理完后落到下一条待处理项;没有下一条就回到首条,让视线不必回到列表顶部重新找 */
  const advanceToNext = useCallback(() => {
    const nextPending = list.find(
      (item, index) => index > selectedIndex && item.status === GradeAppealStatus.PENDING,
    )
    setSelectedId(nextPending?.id ?? undefined)
    if (!nextPending) setMobileView('queue')
  }, [list, selectedIndex])

  /** 键盘上下切条:审阅动作靠键盘才快,鼠标逐条点会拖慢整批处理 */
  const onQueueKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return
    event.preventDefault()
    const step = event.key === 'ArrowDown' ? 1 : -1
    const next = list[Math.min(Math.max(selectedIndex + step, 0), list.length - 1)]
    if (next) setSelectedId(next.id)
  }

  return (
    <PageSection
      title="成绩申诉"
      description="受理会解锁该课程成绩,处理结论学生可见。"
    >
      {canRecompute ? (
        <Callout tone="info" className="mb-4">
          受理后由授课教师改分,改分事件会自动触发绩点重算。如果学生反馈绩点没更新,可以在已受理的申诉上手动重算一次。
        </Callout>
      ) : null}

      <QueueDetailLayout
        view={mobileView}
        detailHeader={
          selected ? (
            <div className="flex items-center gap-2">
              <Button variant="ghost" size="sm" leftIcon={ChevronLeft} onClick={() => setMobileView('queue')}>
                返回队列
              </Button>
              <span className="text-sm text-ink-sub tabular-nums">
                第 {selectedIndex + 1} 条 / 共 {appeals.total} 条
              </span>
            </div>
          ) : undefined
        }
        queue={
          <DataPanel
            label="申诉队列"
            className="min-h-0 flex-1"
            filter={
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
            }
            footer={
              <Pagination
                page={appeals.page}
                pageSize={appeals.pageSize}
                total={appeals.total}
                onPageChange={appeals.setPage}
              />
            }
          >
            <ResourceState
              resource={appeals}
              emptyIcon={Scale}
              emptyTitle={statusFilter ? '这个状态下没有申诉' : '没有待处理的申诉'}
              emptyDescription={
                statusFilter ? '换个状态看看。' : '学生对成绩有异议时提交的申诉会分配到这里。'
              }
              skeleton={<Skeleton variant="line" lines={5} />}
            >
              {(page) => (
                <div
                  role="listbox"
                  aria-label="申诉队列"
                  aria-activedescendant={selected ? `appeal-${selected.id}` : undefined}
                  tabIndex={0}
                  onKeyDown={onQueueKeyDown}
                  className="focus-visible:outline-2 focus-visible:outline-accent focus-visible:-outline-offset-2"
                >
                  {page.list.map((appeal) => {
                    const isActive = appeal.id === selected?.id
                    return (
                      <div
                        key={appeal.id}
                        id={`appeal-${appeal.id}`}
                        role="option"
                        aria-selected={isActive}
                        onClick={() => {
                          setSelectedId(appeal.id)
                          setMobileView('detail')
                        }}
                        className={cn(
                          'cursor-pointer border-t border-line px-4 py-3 first:border-t-0',
                          isActive ? 'bg-primary-soft' : 'hover:bg-surface-hover',
                        )}
                      >
                        <div className="flex items-center justify-between gap-2">
                          <span className="truncate text-sm font-medium text-ink">
                            {courseNameById.get(appeal.course_id) ?? '其他课程'}
                          </span>
                          <StatusIndicator
                            tone={gradeAppealStatusTone(appeal.status)}
                            label={gradeAppealStatusLabel(appeal.status)}
                          />
                        </div>
                        <p className="mt-1 line-clamp-2 text-xs text-ink-sub">{appeal.reason}</p>
                        <p className="mt-1 font-mono text-xs text-ink-faint">
                          {formatDateTime(appeal.created_at)}
                        </p>
                      </div>
                    )
                  })}
                  <p className="border-t border-line px-4 py-2 text-xs text-ink-faint">
                    ↑↓ 切换条目
                  </p>
                </div>
              )}
            </ResourceState>
          </DataPanel>
        }
        detail={
          selected ? (
            <AppealDetailPane
              appeal={selected}
              courseName={courseNameById.get(selected.course_id) ?? '该课程'}
              canRecompute={canRecompute}
              onHandled={() => {
                advanceToNext()
                appeals.reload()
              }}
              onRecompute={() => setRecomputeTarget(selected)}
            />
          ) : (
            <div className="flex min-h-48 items-center justify-center rounded-lg bg-surface p-6 text-sm text-ink-sub shadow-xs">
              左侧选一条申诉查看详情并处理。
            </div>
          )
        }
      />

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

interface AppealDetailPaneProps {
  appeal: GradeAppeal
  courseName: string
  /** 是否显示「重算绩点」(admin 组能力,由调用方声明) */
  canRecompute: boolean
  /** 处理成功:调用方据此进下一条并刷新队列 */
  onHandled: () => void
  onRecompute: () => void
}

/**
 * AppealDetailPane 是审阅队列族的右侧详情与处理表单(§6.5.3 第 ⑤ 族)。
 *
 * 处理表单**内联在详情里而不是弹窗**:这一族要的是「看完就处」的连续节奏,
 * 弹窗会把一条的处理拆成「开窗 → 填 → 关窗 → 回列表找下一条」四步;
 * 内联之后受理/驳回就在眼前,提交后由 onHandled 直接落到下一条待处理项。
 *
 * 受理与驳回共用一个说明输入:两者都要写给学生看的结论,分开两个框会让人以为要填两次。
 * key 绑 appeal.id 由调用方保证 —— 切换条目时输入框必须清空,否则上一条的说明会漂到下一条。
 */
function AppealDetailPane({
  appeal,
  courseName,
  canRecompute,
  onHandled,
  onRecompute,
}: AppealDetailPaneProps) {
  const [comment, setComment] = useState('')
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState<AppealDecision>()
  const isPending = appeal.status === GradeAppealStatus.PENDING

  const submit = useCallback(
    async (decision: AppealDecision) => {
      if (comment.trim() === '') {
        setFormError('请写下处理说明,学生会看到这段话')
        return
      }
      setFormError(undefined)
      setWorking(decision)
      try {
        const payload = { comment: comment.trim() }
        if (decision === 'accept') {
          await api.grade.acceptAppeal(appeal.id, payload)
          toast.success('申诉已受理,该课程成绩已解锁')
        } else {
          await api.grade.rejectAppeal(appeal.id, payload)
          toast.success('申诉已驳回')
        }
        setComment('')
        onHandled()
      } catch (error) {
        setFormError(userFacingErrorMessage(error, '处理没有成功,请稍后重试。'))
      } finally {
        setWorking(undefined)
      }
    },
    [appeal.id, comment, onHandled],
  )

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-4 rounded-lg bg-surface p-5 shadow-xs">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <h3 className="text-md font-semibold text-ink">{courseName}</h3>
          <p className="mt-0.5 font-mono text-xs text-ink-sub">
            提交于 {formatDateTime(appeal.created_at)}
            {appeal.handled_at ? ` · 处理于 ${formatDateTime(appeal.handled_at)}` : ''}
          </p>
        </div>
        <StatusIndicator
          tone={gradeAppealStatusTone(appeal.status)}
          label={gradeAppealStatusLabel(appeal.status)}
        />
      </div>

      <div className="well flex flex-col gap-1 p-4">
        <span className="text-xs text-ink-sub">申诉理由</span>
        <p className="text-sm text-ink">{appeal.reason}</p>
      </div>

      {appeal.result_comment ? (
        <div className="well flex flex-col gap-1 p-4">
          <span className="text-xs text-ink-sub">已给出的处理说明</span>
          <p className="text-sm text-ink">{appeal.result_comment}</p>
        </div>
      ) : null}

      {isPending ? (
        <>
          <Callout tone="warning" title="受理会解锁成绩">
            解锁后这门课的成绩回到可调整状态,调整完需要重新报送给学校审核。驳回则维持原成绩。
          </Callout>

          <FormField
            label="处理说明"
            htmlFor={`appeal-comment-${appeal.id}`}
            required
            helper="写清处理结论与依据,学生会看到这段话"
            error={formError}
          >
            <Textarea
              id={`appeal-comment-${appeal.id}`}
              value={comment}
              rows={4}
              invalid={Boolean(formError)}
              onChange={(event) => setComment(event.target.value)}
            />
          </FormField>

          {/* 动作条贴详情片底边;<lg 由 QueueDetailLayout 钉到屏幕底部 */}
          <div className="mt-auto flex flex-wrap items-center justify-end gap-2 border-t border-line pt-4">
            <Button
              variant="danger"
              leftIcon={CircleX}
              loading={working === 'reject'}
              onClick={() => void submit('reject')}
            >
              驳回并看下一条
            </Button>
            <Button
              variant="seal"
              leftIcon={CircleCheck}
              loading={working === 'accept'}
              onClick={() => void submit('accept')}
            >
              受理并看下一条
            </Button>
          </div>
        </>
      ) : (
        <div className="mt-auto flex flex-wrap items-center justify-between gap-2 border-t border-line pt-4">
          <span className="text-sm text-ink-sub">这条申诉已处理完。</span>
          {canRecompute && appeal.status === GradeAppealStatus.ACCEPTED ? (
            <Button variant="outline" leftIcon={RefreshCw} onClick={onRecompute}>
              重算绩点
            </Button>
          ) : null}
        </div>
      )}
    </div>
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
