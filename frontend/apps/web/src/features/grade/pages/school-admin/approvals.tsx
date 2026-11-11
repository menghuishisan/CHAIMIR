// 成绩审核页(校管侧栏,/school-admin/approvals)。设计 §7.5:
// 顶部可点的计数「待审核 / 已通过 / 已驳回 / 全部」+ 锁定状态,都写进地址;队列 + 详情:队列行显示课程名、
// 报送教师、报送时间、锁定与审核状态;详情底部「驳回」「通过并锁定」,已通过的显示「解锁成绩」。
// 页头右侧次要入口「批量生成成绩单」(右侧抽屉)。通过或驳回后自动进入下一条待审核;上下方向键切换。
//
// 教师报送 → 学校审核 → 通过即锁定。锁定后教师不能改分,要改必须先解锁。
// 三个动作后果不同:通过会锁定并让成绩进入学生的成绩中心;驳回退回教师修改;
// 解锁把已通过的成绩重新打开(通常是申诉受理后的补救),故各自确认并说明。

import { useCallback, useMemo, useState, type ReactNode } from 'react'
import { ChevronLeft, CircleCheck, CircleX, FileText, LockOpen, Send } from 'lucide-react'
import {
  BaseIdentity,
  BOOL_FILTER,
  type BoolFilter,
  GradeReviewStatus,
  PAGINATION_MAX_SIZE,
  type Course,
  type GradeReview,
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
  PageHeader,
  PageScaffold,
  Pagination,
  QueueDetailLayout,
  SegmentedControl,
  Select,
  Skeleton,
  Textarea,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { type AsyncResourceState, useAsyncResource, usePagedResource, useResourceTotal, useReviewQueue, useUrlState } from '../../../../hooks'
import { AccountNameText } from '../../../identity/components/AccountNameText'
import { useAccountNames } from '../../../identity/useAccountNames'
import { useFormValidation } from '../../../../hooks/useFormValidation'
import { formatDateTime, formatRelativeOccurrence } from '../../../../utils/formatters'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { GradeReviewStatusCell } from '../../components/GradeReviewStatusCell'
import { TranscriptBatchDrawer } from './transcript-batch'

/** 地址参数:status 审核状态分段(默认待审核),locked 锁定状态;页码在 page */
const APPROVALS_QUERY = {
  status: { defaultValue: 'pending', allowed: ['pending', 'approved', 'rejected', 'all'] },
  locked: { defaultValue: String(BOOL_FILTER.ANY), allowed: [String(BOOL_FILTER.ANY), String(BOOL_FILTER.YES), String(BOOL_FILTER.NO)] },
} as const

const STATUS_SEGMENTS = [
  { value: 'pending', label: '待审核', status: GradeReviewStatus.PENDING },
  { value: 'approved', label: '已通过', status: GradeReviewStatus.APPROVED },
  { value: 'rejected', label: '已驳回', status: GradeReviewStatus.REJECTED },
  { value: 'all', label: '全部' },
] as const

/**
 * LOCKED_FILTERS 是「是否已锁定」筛选项。
 * 布尔列没法用「缺省即不过滤」表达三种意图,全平台统一用 BoolFilter(0 不限 / 1 是 / 2 否)。
 */
const LOCKED_FILTERS = [
  { value: BOOL_FILTER.ANY, label: '不限' },
  { value: BOOL_FILTER.YES, label: '已锁定' },
  { value: BOOL_FILTER.NO, label: '未锁定' },
] as const

/** 审核动作:三者后果不同,文案分别说明。 */
type ReviewAction = 'approve' | 'reject' | 'unlock'

const ACTION_COPY: Record<ReviewAction, { title: string; description: string; confirm: string; danger?: boolean }> = {
  approve: {
    title: '通过成绩审核',
    description:
      '通过后这门课的成绩会锁定并进入学生的成绩中心。锁定后教师不能再调分,需要改分要先解锁。',
    confirm: '确认通过',
  },
  reject: {
    title: '驳回成绩审核',
    description: '驳回后教师可以修改成绩并重新报送。请在意见里写清需要调整什么。',
    confirm: '确认驳回',
  },
  unlock: {
    title: '解锁成绩',
    description:
      '解锁把已通过的成绩重新打开,教师可以调分后重新报送。通常在受理学生申诉后使用。',
    confirm: '确认解锁',
    danger: true,
  },
}

/**
 * SchoolAdminApprovalsPage 审核教师报送的课程成绩。
 */
export default function SchoolAdminApprovalsPage() {
  const [query, setQuery] = useUrlState(APPROVALS_QUERY)
  const [target, setTarget] = useState<{ review: GradeReview; action: ReviewAction }>()
  const [transcriptOpen, setTranscriptOpen] = useState(false)
  const statusSegment = STATUS_SEGMENTS.find((segment) => segment.value === query.status)
  const lockedFilter = Number(query.locked) as BoolFilter

  const reviews = usePagedResource<GradeReview>(
    (params) =>
      api.grade.listReviews({
        status: statusSegment && 'status' in statusSegment ? statusSegment.status : undefined,
        is_locked: lockedFilter,
        ...params,
      }),
    [query.locked, query.status],
    { urlPageKey: 'page' },
  )
  // 报送记录只有报送人编号:用本校教师账号解析姓名(设计 §10,校管可以读账号列表)
  const teachers = useAccountNames(BaseIdentity.TEACHER)

  // 审核记录只回 course_id / semester_id,课程名与学期名在此解析,不把内部编号抛到界面上
  const courses = useAsyncResource(
    () => api.teaching.getCourses({ page: 1, size: PAGINATION_MAX_SIZE }),
    [],
    () => false,
  )

  const semesters = useAsyncResource(() => api.grade.listSemesters(), [])

  const courseNameById = useMemo(
    () => new Map((courses.data?.list ?? []).map((course: Course) => [course.id, course.name])),
    [courses.data],
  )

  const semesterNameById = useMemo(
    () => new Map((semesters.data ?? []).map((semester: Semester) => [semester.id, semester.name])),
    [semesters.data],
  )

  // 分段上的数字取服务端全量口径,不随锁定筛选变化(§6.5.4)
  const totalCount = useResourceTotal((params) => api.grade.listReviews(params), [], reviews.refreshVersion)
  const pendingCount = useResourceTotal(
    (params) => api.grade.listReviews({ status: GradeReviewStatus.PENDING, ...params }),
    [],
    reviews.refreshVersion,
  )
  const approvedCount = useResourceTotal(
    (params) => api.grade.listReviews({ status: GradeReviewStatus.APPROVED, ...params }),
    [],
    reviews.refreshVersion,
  )
  const rejectedCount = useResourceTotal(
    (params) => api.grade.listReviews({ status: GradeReviewStatus.REJECTED, ...params }),
    [],
    reviews.refreshVersion,
  )
  const segmentCounts: Record<string, number | undefined> = {
    pending: pendingCount,
    approved: approvedCount,
    rejected: rejectedCount,
    all: totalCount,
  }

  /**
   * 审阅队列族(§6.5.3 第 ⑤ 族):左队列右详情。
   * 成绩审核的动作是「逐条把报送处理完」,不是「找一条记录」——
   * 详情与处理动作常驻右侧、键盘上下切条、处理完自动进下一条待审核项。
   */
  const list = useMemo(() => reviews.data?.list ?? [], [reviews.data])
  const [selectedId, setSelectedId] = useState<string>()
  /** <lg 走两级页面(§6.4.1 规则 4) */
  const [mobileView, setMobileView] = useState<'queue' | 'detail'>('queue')
  const selected = list.find((item) => item.id === selectedId) ?? list[0]
  const selectedIndex = selected ? list.findIndex((item) => item.id === selected.id) : -1

  // 上下方向键切条、处理完进下一条待处理:四处审阅队列共用同一套规则(useReviewQueue)
  const queue = useReviewQueue({
    items: list,
    currentId: selected?.id,
    onSelect: setSelectedId,
    isPending: (item: (typeof list)[number]) => item.status === GradeReviewStatus.PENDING,
  })
  /** advanceToNext 处理完后到下一条待处理;没有就退回队列层 */
  const advanceToNext = () => {
    if (!queue.advance()) setMobileView('queue')
  }

  return (
    <PageScaffold>
      <PageHeader
        title="成绩审核"
        actions={
          <Button variant="outline" leftIcon={FileText} onClick={() => setTranscriptOpen(true)}>
            批量生成成绩单
          </Button>
        }
      />

      <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <SegmentedControl
          aria-label="按审核状态筛选"
          options={STATUS_SEGMENTS.map((segment) => ({ value: segment.value, label: segment.label, count: segmentCounts[segment.value] }))}
          value={query.status}
          onValueChange={(value) => setQuery({ status: value })}
          className="max-w-full overflow-x-auto"
        />
        <Select
          aria-label="按锁定状态筛选"
          value={query.locked}
          onValueChange={(value) => setQuery({ locked: value })}
          options={LOCKED_FILTERS.map((item) => ({ value: String(item.value), label: item.value === BOOL_FILTER.ANY ? '锁定与否' : item.label }))}
          className="w-full sm:w-36"
        />
      </div>

      <QueueDetailLayout
        view={mobileView}
        detailHeader={
          selected ? (
            <div className="flex items-center gap-2">
              <Button variant="ghost" size="sm" leftIcon={ChevronLeft} onClick={() => setMobileView('queue')}>
                返回队列
              </Button>
              <span className="text-sm text-ink-sub tabular-nums">
                第 {selectedIndex + 1} 条 / 共 {reviews.total ?? '—'} 条
              </span>
            </div>
          ) : undefined
        }
        queue={
          <DataPanel
            label="报送队列"
            className="min-h-0 flex-1"
            footer={
              <Pagination
                page={reviews.page}
                pageSize={reviews.pageSize}
                total={reviews.total}
                onPageChange={reviews.setPage}
              />
            }
          >
            <ResourceState
              resource={reviews}
              emptyIcon={Send}
              emptyTitle={query.status === 'pending' ? '没有待审核的报送' : query.status === 'all' ? '还没有成绩报送' : '这个状态下没有记录'}
              emptyDescription={
                query.status === 'all' ? '教师在教学端确认课程成绩后报送,记录会出现在这里。' : '全部处理完了,换个状态看看。'
              }
              skeleton={<Skeleton variant="line" lines={5} />}
            >
              {(page) => (
                <div
                  role="listbox"
                  aria-label="报送队列"
                  aria-activedescendant={selected ? `review-${selected.id}` : undefined}
                  tabIndex={0}
                  onKeyDown={queue.onListKeyDown}
                  className="focus-visible:outline-2 focus-visible:outline-accent focus-visible:-outline-offset-2"
                >
                  {page.list.map((review) => {
                    const isActive = review.id === selected?.id
                    return (
                      <div
                        key={review.id}
                        id={`review-${review.id}`}
                        role="option"
                        aria-selected={isActive}
                        onClick={() => {
                          setSelectedId(review.id)
                          setMobileView('detail')
                        }}
                        className={cn(
                          'cursor-pointer border-t border-line px-4 py-3 first:border-t-0',
                          isActive ? 'bg-primary-soft' : 'hover:bg-surface-hover',
                        )}
                      >
                        <div className="flex items-center justify-between gap-2">
                          <span className="truncate text-sm font-medium text-ink">
                            {courseNameById.get(review.course_id) ?? '已归档的课程'}
                          </span>
                          <GradeReviewStatusCell review={review} />
                        </div>
                        <p className="mt-1 flex min-w-0 items-center gap-1.5 text-xs text-ink-sub">
                          <AccountNameText name={teachers.lookup(review.submitter_id)} compact />
                        </p>
                        <p className="mt-1 text-xs text-ink-sub" title={formatDateTime(review.submitted_at)}>
                          {formatRelativeOccurrence(review.submitted_at)}报送 ·{' '}
                          {review.semester_id ? (semesterNameById.get(review.semester_id) ?? '未登记学期') : '按课程学期'}
                        </p>
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
            <ReviewDetailPane
              review={selected}
              submitter={<AccountNameText name={teachers.lookup(selected.submitter_id)} compact />}
              courseName={courseNameById.get(selected.course_id) ?? '已归档的课程'}
              semesterName={
                selected.semester_id
                  ? (semesterNameById.get(selected.semester_id) ?? '未登记学期')
                  : '按课程学期'
              }
              onRequestAction={(action) => setTarget({ review: selected, action })}
            />
          ) : (
            <div className="flex min-h-48 items-center justify-center rounded-lg bg-surface p-6 text-sm text-ink-sub shadow-xs">
              左侧选一条报送记录查看详情并审核。
            </div>
          )
        }
      />

      {/* 批量成绩单:出成绩单的前提是成绩已锁定,故归到本页(页头次要入口,右侧抽屉) */}
      {transcriptOpen ? <TranscriptBatchDrawer onClose={() => setTranscriptOpen(false)} /> : null}

      {target ? (
        <ReviewDecisionModal
          review={target.review}
          action={target.action}
          courseName={courseNameById.get(target.review.course_id) ?? '该课程'}
          semesters={semesters}
          onClose={() => setTarget(undefined)}
          onSaved={() => {
            setTarget(undefined)
            // 处理完直接落到下一条待审核项,不必回队列重新找(§6.5.3 第 ⑤ 族)
            advanceToNext()
            reviews.reload()
          }}
        />
      ) : null}
    </PageScaffold>
  )
}

interface ReviewDecisionModalProps {
  review: GradeReview
  action: ReviewAction
  courseName: string
  semesters: AsyncResourceState<Semester[]>
  onClose: () => void
  onSaved: () => void
}

/**
 * ReviewDecisionModal 提交通过、驳回或解锁决定。
 * 通过时可指定与课程一致的归档学期，否则沿用报送学期；驳回与解锁只需意见。
 */
function ReviewDecisionModal({
  review,
  action,
  courseName,
  semesters,
  onClose,
  onSaved,
}: ReviewDecisionModalProps) {
  const [semesterId, setSemesterId] = useState('')
  const [comment, setComment] = useState('')
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const copy = ACTION_COPY[action]
  const needsComment = action !== 'approve'
  const { errors, validate } = useFormValidation(() => ({
    semester: action === 'approve' && semesters.status !== 'success'
      ? '请先完成学期信息读取,再通过审核' : null,
    comment: needsComment && comment.trim() === ''
      ? action === 'reject' ? '请写下驳回理由,教师需要知道改什么' : '请写下解锁原因,记录会留档'
      : null,
  }))

  const submit = useCallback(async () => {
    if (working || !validate()) return
    setFormError(undefined)
    setWorking(true)
    try {
      const payload = {
        comment: comment.trim() || undefined,
        semester_id: action === 'approve' ? semesterId || undefined : undefined,
      }
      if (action === 'approve') await api.grade.approveReview(review.id, payload)
      if (action === 'reject') await api.grade.rejectReview(review.id, payload)
      if (action === 'unlock') await api.grade.unlockReview(review.id, payload)
      toast.success(
        action === 'approve' ? '已通过,成绩已锁定' : action === 'reject' ? '已驳回' : '已解锁',
      )
      onSaved()
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '操作没有完成,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [action, comment, onSaved, review.id, semesterId, validate, working])

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
              { term: '报送时间', description: formatDateTime(review.submitted_at), mono: true },
              { term: '教师说明', description: review.comment || '未填写' },
            ]}
          />

          {action === 'approve' ? (
            <FormField
              label="归档学期"
              htmlFor="approve-semester"
              error={errors.semester}
              helper="不选则沿用报送学期；未登记时按课程学期核对,不自动转换名称"
            >
              <ResourceState
                resource={semesters}
                emptyIcon={CircleCheck}
                emptyTitle="学校尚未登记学期"
                emptyDescription="请先在成绩配置中登记课程所属学期,再通过审核。"
              >
                {(items) => (
                  <Select
                    id="approve-semester"
                    options={[
                      { value: '', label: '按报送时的学期' },
                      ...items.map((semester) => ({ value: semester.id, label: semester.name })),
                    ]}
                    value={semesterId}
                    invalid={Boolean(errors.semester)}
                    onBlur={() => validate(['semester'])}
                    placeholder="按报送时的学期"
                    disabled={working}
                    onValueChange={setSemesterId}
                  />
                )}
              </ResourceState>
            </FormField>
          ) : null}

          <FormField
            label="审核意见"
            htmlFor="review-comment"
            required={needsComment}
            error={errors.comment}
            helper={
              action === 'reject'
                ? '写清需要教师调整什么,教师会看到这段话'
                : action === 'unlock'
                  ? '写清解锁原因,记录会计入审计'
                  : '可以留空'
            }
          >
            <Textarea
              id="review-comment"
              value={comment}
              rows={4}
              invalid={Boolean(errors.comment)}
              onBlur={() => validate(['comment'])}
              onChange={(event) => setComment(event.target.value)}
            />
          </FormField>

          {formError ? <Callout tone="danger">{formError}</Callout> : null}

          {action === 'unlock' ? (
            <Callout tone="warning">
              解锁后学生的成绩中心会暂时看不到这门课的最终成绩,教师重新报送并通过后恢复。
            </Callout>
          ) : null}
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            取消
          </Button>
          <Button
            variant={copy.danger ? 'danger' : 'seal'}
            loading={working}
            disabled={action === 'approve' && semesters.status !== 'success'}
            onClick={() => void submit()}
          >
            {copy.confirm}
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}

interface ReviewDetailPaneProps {
  review: GradeReview
  /** 报送教师(姓名与工号) */
  submitter: ReactNode
  courseName: string
  semesterName: string
  onRequestAction: (action: ReviewAction) => void
}

/**
 * ReviewDetailPane 是审阅队列族的右侧详情(§6.5.3 第 ⑤ 族)。
 *
 * 与申诉页不同,这里的三个动作**保留二次确认弹窗**:通过会锁定成绩并推送给全班学生、
 * 驳回会退回教师、解锁会把已生效的成绩重新打开 —— 三者都是不可轻易撤销的批量后果,
 * 属于 §7.2 B「就地二次确认」适用的情形。详情内联、确认仍走弹窗,两者不冲突:
 * 内联省掉的是「为了看清一条记录而开窗」,确认留下的是「为了不误触而停一下」。
 */
function ReviewDetailPane({
  review,
  submitter,
  courseName,
  semesterName,
  onRequestAction,
}: ReviewDetailPaneProps) {
  const isPending = review.status === GradeReviewStatus.PENDING

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-4 rounded-lg bg-surface p-5 shadow-xs">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <h3 className="text-md font-semibold text-ink">{courseName}</h3>
          <p className="mt-0.5 text-xs text-ink-sub">{semesterName}</p>
        </div>
        <GradeReviewStatusCell review={review} />
      </div>

      <DescriptionList
        dense
        items={[
          { term: '报送教师', description: submitter },
          { term: '报送时间', description: formatDateTime(review.submitted_at), mono: true },
          ...(review.reviewed_at
            ? [{ term: '审核时间', description: formatDateTime(review.reviewed_at), mono: true }]
            : []),
        ]}
      />

      <div className="rounded-md bg-surface-hover flex flex-col gap-1 p-4">
        <span className="text-xs text-ink-sub">教师报送说明</span>
        <p className="text-sm text-ink">{review.comment || '未填写'}</p>
      </div>

      {review.reviewed_at ? (
        <div className="rounded-md bg-surface-hover flex flex-col gap-1 p-4">
          <span className="text-xs text-ink-sub">审核结论</span>
          <p className="text-sm text-ink">
            {review.status === GradeReviewStatus.APPROVED
              ? '已通过。成绩已锁定并进入学生的成绩中心。'
              : '已驳回。教师可以改分后重新报送。'}
          </p>
        </div>
      ) : null}

      {/* 动作条贴详情片底边;<lg 由 QueueDetailLayout 钉到屏幕底部 */}
      <div className="mt-auto flex flex-wrap items-center justify-end gap-2 border-t border-line pt-4">
        {isPending ? (
          <>
            <Button variant="danger" leftIcon={CircleX} onClick={() => onRequestAction('reject')}>
              驳回
            </Button>
            <Button variant="seal" leftIcon={CircleCheck} onClick={() => onRequestAction('approve')}>
              通过并锁定
            </Button>
          </>
        ) : review.is_locked ? (
          <>
            <span className="mr-auto text-sm text-ink-sub">成绩已锁定,教师不能改分。</span>
            <Button variant="outline" leftIcon={LockOpen} onClick={() => onRequestAction('unlock')}>
              解锁成绩
            </Button>
          </>
        ) : (
          <span className="text-sm text-ink-sub">这条报送已处理完。</span>
        )}
      </div>
    </div>
  )
}
