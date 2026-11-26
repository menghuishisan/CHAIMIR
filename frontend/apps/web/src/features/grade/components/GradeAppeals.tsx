// 教师与校管共用申诉队列和处理表单；权限由服务端核验。

import { useCallback, useState, type ReactNode } from 'react'
import { ChevronLeft, CircleCheck, CircleX, RefreshCw, Scale } from 'lucide-react'
import {
  GradeAppealStatus,
  PAGINATION_MAX_SIZE,
  type GradeAppeal,
  type Semester,
} from '@chaimir/api-client'
import {
  Button,
  Callout,
  cn,
  DataPanel,
  DescriptionList,
  FormField,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
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
import { ResourceError, ResourceState } from '../../../components/ResourceState'
import { useAsyncResource, usePagedResource, useReviewQueue, useUrlState } from '../../../hooks'
import { segmentCounts } from '../../../utils/facets'
import { collectAllPages } from '../../../utils/allPages'
import { AccountNameText } from '../../identity/components/AccountNameText'
import { useFormValidation } from '../../../hooks/useFormValidation'
import { formatDateTime, formatRelativeOccurrence, formatScore } from '../../../utils/formatters'
import { gradeAppealStatusLabel } from '../../../utils/labels/grade'
import { userFacingErrorMessage } from '../../../utils/userFacingError'
import { gradeAppealStatusTone } from '../statusPresentation'

/** 地址参数:status 申诉状态分段(默认待处理);页码与所在页的列表共用 page(切换标签或筛选时会被清掉) */
const APPEALS_QUERY = {
  status: { defaultValue: 'pending', allowed: ['pending', 'accepted', 'completed', 'rejected', 'all'] },
} as const

const STATUS_SEGMENTS = [
  { value: 'pending', label: '待处理', status: GradeAppealStatus.PENDING },
  { value: 'accepted', label: '已受理', status: GradeAppealStatus.ACCEPTED },
  { value: 'completed', label: '已完成', status: GradeAppealStatus.COMPLETED },
  { value: 'rejected', label: '未通过', status: GradeAppealStatus.REJECTED },
  { value: 'all', label: '全部' },
] as const

/** 处理动作:受理会解锁成绩,驳回维持原成绩。 */
type AppealDecision = 'accept' | 'reject'

export interface GradeAppealsProps {
  /**
   * 处理范围:teacher 是批改中心里的本人授课课程;school 是校管的全校范围,另有「重算绩点」。
   * 由调用方显式声明而不是在组件内判角色枚举 —— 权限边界由后端守卫,前端只按声明渲染。
   */
  scope: 'teacher' | 'school'
}

/**
 * GradeAppeals 列出分配给当前处理人的成绩申诉并承载受理与驳回。
 */
export function GradeAppeals({ scope }: GradeAppealsProps) {
  const canRecompute = scope === 'school'
  const [query, setQuery] = useUrlState(APPEALS_QUERY)
  const [recomputeTarget, setRecomputeTarget] = useState<GradeAppeal>()
  const segment = STATUS_SEGMENTS.find((item) => item.value === query.status)

  const appeals = usePagedResource<GradeAppeal>(
    (params) =>
      api.grade.listAppeals({
        status: segment && 'status' in segment ? segment.status : undefined,
        ...params,
      }),
    [query.status],
    { urlPageKey: 'page' },
  )
  // 分段计数取列表接口的 facets(范围同列表:教师按任教课程,校管全校);读取中或失败时不显示数字
  const facets = appeals.status === 'success' || appeals.status === 'empty' ? appeals.data?.facets : undefined
  const counts = segmentCounts(facets, 'status', STATUS_SEGMENTS, (item) => ('status' in item ? item.status : undefined))

  /** studentLabel 一位学生的姓名与学号(服务端解析;账号已移除时写明,不显示编号) */
  const studentLabel = (appeal: GradeAppeal) => (
    <AccountNameText name={appeal.student_name} no={appeal.student_no} compact />
  )

  const list = appeals.data?.list ?? []
  const [selectedId, setSelectedId] = useState<string>()
  /** <lg 走两级页面(§6.4.1 规则 4):当前在队列层还是详情层 */
  const [mobileView, setMobileView] = useState<'queue' | 'detail'>('queue')
  // 选中项:未指定或已翻页失效时落到队列首条,避免右侧空白
  const selected = list.find((item) => item.id === selectedId) ?? list[0]
  const selectedIndex = selected ? list.findIndex((item) => item.id === selected.id) : -1

  // 上下方向键切条、处理完进下一条待处理:四处审阅队列共用同一套规则(useReviewQueue)
  const queue = useReviewQueue({
    items: list,
    currentId: selected?.id,
    onSelect: setSelectedId,
    isPending: (item: (typeof list)[number]) => item.status === GradeAppealStatus.PENDING,
  })
  /** advanceToNext 处理完后到下一条待处理;没有就退回队列层 */
  const advanceToNext = () => {
    if (!queue.advance()) setMobileView('queue')
  }

  return (
    <>
      <SegmentedControl
        aria-label="按申诉状态筛选"
        options={STATUS_SEGMENTS.map((item) => ({ value: item.value, label: item.label, count: counts[item.value] }))}
        value={query.status}
        onValueChange={(value) => setQuery({ status: value })}
        className="mb-4 max-w-full overflow-x-auto"
      />

      <QueueDetailLayout
        view={mobileView}
        detailHeader={
          selected ? (
            <div className="flex items-center gap-2">
              <Button variant="ghost" size="sm" leftIcon={ChevronLeft} onClick={() => setMobileView('queue')}>
                返回队列
              </Button>
              <span className="text-sm text-ink-sub tabular-nums">
                第 {selectedIndex + 1} 条 / 共 {appeals.total ?? '—'} 条
              </span>
            </div>
          ) : undefined
        }
        queue={
          <DataPanel
            label="申诉队列"
            className="min-h-0 flex-1"
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
              emptyTitle={query.status === 'pending' ? '没有待处理的申诉' : query.status === 'all' ? '还没有成绩申诉' : '这个状态下没有申诉'}
              emptyDescription={
                query.status === 'all' ? '学生对成绩有异议时提交的申诉会分配到这里。' : '换个状态看看。'
              }
              skeleton={<Skeleton variant="line" lines={5} />}
            >
              {(page) => (
                <div
                  role="listbox"
                  aria-label="申诉队列"
                  aria-activedescendant={selected ? `appeal-${selected.id}` : undefined}
                  tabIndex={0}
                  onKeyDown={queue.onListKeyDown}
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
                          <span className="flex min-w-0 text-sm">{studentLabel(appeal)}</span>
                          <StatusIndicator
                            tone={gradeAppealStatusTone(appeal.status)}
                            label={gradeAppealStatusLabel(appeal.status)}
                          />
                        </div>
                        <p className="mt-1 truncate text-xs text-ink-sub" title={formatDateTime(appeal.created_at)}>
                          {appeal.course_name ?? '已删除的课程'} · {formatRelativeOccurrence(appeal.created_at)}
                        </p>
                        <p className="mt-1 line-clamp-1 text-xs text-ink-sub">{appeal.reason}</p>
                      </div>
                    )
                  })}
                  <p className="hidden border-t border-line px-4 py-2 text-xs text-ink-sub lg:block">↑↓ 切换条目</p>
                </div>
              )}
            </ResourceState>
          </DataPanel>
        }
        detail={
          selected ? (
            <AppealDetailPane
              key={selected.id}
              appeal={selected}
              student={studentLabel(selected)}
              courseName={selected.course_name ?? '该课程'}
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
          courseName={recomputeTarget.course_name ?? '该课程'}
          onClose={() => setRecomputeTarget(undefined)}
          onDone={() => {
            setRecomputeTarget(undefined)
            appeals.reload()
          }}
        />
      ) : null}
    </>
  )
}

interface AppealDetailPaneProps {
  appeal: GradeAppeal
  /** 申诉学生(姓名与学号) */
  student: ReactNode
  courseName: string
  /** 是否显示「重算绩点」(admin 组能力,由调用方声明) */
  canRecompute: boolean
  /** 处理成功:调用方据此进下一条并刷新队列 */
  onHandled: () => void
  onRecompute: () => void
}

/** AppealDetailPane 按申诉编号挂载，切换条目时清空处理说明。 */
function AppealDetailPane({
  appeal,
  student,
  courseName,
  canRecompute,
  onHandled,
  onRecompute,
}: AppealDetailPaneProps) {
  const [comment, setComment] = useState('')
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState<AppealDecision>()
  const isPending = appeal.status === GradeAppealStatus.PENDING
  const { errors, validate } = useFormValidation(useCallback(() => ({
    comment: comment.trim() ? null : '请写下处理说明,学生会看到这段话',
  }), [comment]), appeal.id)
  // 当前课程成绩经 M6 读取，教师不读取学生的跨课程学业记录。
  const currentGrade = useAsyncResource(
    () =>
      collectAllPages((params) => api.teaching.listGrades(appeal.course_id, params), PAGINATION_MAX_SIZE).then(
        (list) => list.find((grade) => grade.student_id === appeal.student_id) ?? null,
      ),
    [appeal.course_id, appeal.student_id],
    () => false,
  )

  const submit = useCallback(
    async (decision: AppealDecision) => {
      if (working || !validate()) return
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
    [appeal.id, comment, onHandled, validate, working],
  )

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-4 rounded-lg bg-surface p-5 shadow-xs">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <h3 className="flex text-md font-semibold">{student}</h3>
          <p className="mt-0.5 text-sm text-ink-sub">{courseName}</p>
          <p className="mt-0.5 text-xs text-ink-sub">
            提交于 {formatDateTime(appeal.created_at)}
            {appeal.handled_at ? ` · 处理于 ${formatDateTime(appeal.handled_at)}` : ''}
          </p>
        </div>
        <StatusIndicator
          tone={gradeAppealStatusTone(appeal.status)}
          label={gradeAppealStatusLabel(appeal.status)}
        />
      </div>

      <div className="flex flex-col gap-1 rounded-md bg-surface-hover p-4">
        <span className="text-xs text-ink-sub">申诉理由</span>
        <p className="whitespace-pre-wrap text-sm text-ink">{appeal.reason}</p>
      </div>

      {currentGrade.status === 'error' ? (
        <ResourceError
          message={currentGrade.error?.message ?? '暂时读不到当前成绩,请重试。'}
          traceId={currentGrade.error?.traceId}
          onRetry={currentGrade.reload}
        />
      ) : <p className="text-sm text-ink-sub">
        该课当前成绩:{' '}
        {currentGrade.status === 'loading' ? (
          '正在读取…'
        ) : currentGrade.data ? (
          <span className="font-medium tabular-nums text-ink">
            {formatScore(currentGrade.data.final_total)} 分 · {currentGrade.data.credits} 学分
          </span>
        ) : (
          '还没有成绩'
        )}
      </p>}

      {appeal.result_comment ? (
        <div className="flex flex-col gap-1 rounded-md bg-surface-hover p-4">
          <span className="text-xs text-ink-sub">已给出的处理说明</span>
          <p className="text-sm text-ink">{appeal.result_comment}</p>
        </div>
      ) : null}

      {isPending ? (
        <>
          <ul className="flex flex-col gap-1 text-sm text-ink-sub">
            <li>
              <span className="font-medium text-ink">受理</span>:解锁这门课的成绩,由授课教师调分后由系统复核发布。
            </li>
            <li>
              <span className="font-medium text-ink">驳回</span>:维持原成绩,申诉结束。
            </li>
          </ul>

          <FormField
            label="处理说明"
            htmlFor={`appeal-comment-${appeal.id}`}
            required
            helper="写清处理结论与依据,学生会看到这段话"
            error={errors.comment}
          >
            <Textarea
              id={`appeal-comment-${appeal.id}`}
              value={comment}
              rows={4}
              invalid={Boolean(errors.comment)}
              onBlur={() => void validate(['comment'])}
              onChange={(event) => setComment(event.target.value)}
            />
          </FormField>
          {formError ? <Callout tone="danger">{formError}</Callout> : null}

          <div className="mt-auto flex flex-wrap items-center justify-end gap-2 border-t border-line pt-4">
            <Button
              variant="outline"
              leftIcon={CircleX}
              loading={working === 'reject'}
              onClick={() => void submit('reject')}
            >
              驳回
            </Button>
            <Button
              variant="primary"
              leftIcon={CircleCheck}
              loading={working === 'accept'}
              onClick={() => void submit('accept')}
            >
              受理
            </Button>
          </div>
        </>
      ) : (
        <div className="mt-auto flex flex-wrap items-center justify-between gap-2 border-t border-line pt-4">
          <span className="text-sm text-ink-sub">{appeal.status === GradeAppealStatus.ACCEPTED ? '这条申诉正在调分。' : '这条申诉已处理完。'}</span>
          {canRecompute && appeal.status === GradeAppealStatus.ACCEPTED ? (
            <Button variant="outline" leftIcon={RefreshCw} onClick={onRecompute}>
              重新计算绩点
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

  const semesters = useAsyncResource(() => api.grade.listSemesters(), [])
  const { errors, validate } = useFormValidation(useCallback(() => {
    if (!semesterId) return { semester: '请选择要重算的学期' }
    if (semesters.status === 'success' && !semesters.data?.some((semester) => semester.id === semesterId)) {
      return { semester: '请选择当前学校已登记的学期' }
    }
    return { semester: null }
  }, [semesterId, semesters.data, semesters.status]), appeal.id)

  const submit = useCallback(async () => {
    // 学期列表是提交前置;加载失败或旧选择已失效时不得沿用缓存发起重算。
    if (working || semesters.status !== 'success' || !validate()) return
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
  }, [appeal.student_id, onDone, semesterId, semesters.status, validate, working])

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

          <ResourceState
            resource={semesters}
            emptyIcon={Scale}
            emptyTitle="学校尚未登记学期"
            emptyDescription="请先在成绩配置中登记学期,再重算绩点。"
            skeleton={<Skeleton variant="line" lines={2} />}
          >
            {(items) => (
              <FormField
                label="重算学期"
                htmlFor="recompute-semester"
                required
                error={errors.semester}
                helper="选申诉课程所属的学期"
              >
                <Select
                  id="recompute-semester"
                  options={items.map((semester: Semester) => ({
                    value: semester.id,
                    label: semester.name,
                  }))}
                  value={semesterId}
                  disabled={working}
                  invalid={Boolean(errors.semester)}
                  placeholder="选择学期"
                  onValueChange={setSemesterId}
                />
              </FormField>
            )}
          </ResourceState>
          {formError ? <Callout tone="danger">{formError}</Callout> : null}

          <Callout tone="info">
            正常情况下教师改分会自动触发重算。这里是事件未到达时的补救,重复重算不会有副作用。
          </Callout>
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            取消
          </Button>
          <Button
            variant="primary"
            leftIcon={RefreshCw}
            loading={working}
            disabled={semesters.status !== 'success'}
            onClick={() => void submit()}
          >
            开始重算
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}
