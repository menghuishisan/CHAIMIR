// 仿真治理(平台侧栏,/platform-admin/simulations)。设计 §7.6:
// 审核教师提交的仿真场景包,上架或退回;管理已上架的场景。
// 队列 + 详情:顶部可点的计数分段「待审核 / 已通过 / 已退回」(写进地址);队列行显示场景名、版本、分类、提交时间
// 与提交教师(审核列表随记录给出姓名);详情上方是四项自动校验(包元数据 / 静态扫描 / 结果可复现 /
// 渲染预览)的通过状态与失败原因,下方是渲染预览帧,底部「退回作者」「通过并上架」(四项没全过时通过不可用并写原因);
// 已上架的给「下架」,已下架的给「重新上架」。处理完自动进入下一条待审核;退回必须写原因(作者可见)。
//
// 审核记录接口自带校验报告与包信息;包列表接口属教师端,平台身份读不到,所以审核记录就是平台侧的包窗口。
// 后端对四个动作有互斥前提(只有待审记录能通过或退回,只有已上架能下架,只有已下架能重新上架),
// 所以按记录当前状态只给那一个可执行动作。

import { useMemo, useState } from 'react'
import { Archive, ArchiveRestore, ChevronLeft, CircleCheck, CircleX, Shield } from 'lucide-react'
import {
  SIM_PACKAGE_STATUS,
  SIM_REVIEW_RESULT,
  SIM_VALIDATION_STATUS,
  type SimPackageReview,
  type SimValidationReport,
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
  Skeleton,
  StatusIndicator,
  Textarea,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { usePagedResource, useReviewQueue, useUrlState } from '../../../../hooks'
import { segmentCounts } from '../../../../utils/facets'
import { useFormValidation } from '../../../../hooks/useFormValidation'
import { formatDateTime, formatRelativeOccurrence } from '../../../../utils/formatters'
import {
  simCategoryLabel,
  simComputeLabel,
  simPackageStatusLabel,
  simReviewResultLabel,
} from '../../../../utils/labels/sim'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { SimPreviewFrames } from '../../components/SimPreviewFrames'
import { simReviewResultTone } from '../../statusPresentation'

/** 地址参数:result 审核结论分段(默认待审核);页码在 page。三个结论穷尽了全部记录,接口没有「全部」 */
const SIM_REVIEW_QUERY = {
  result: { defaultValue: 'pending', allowed: ['pending', 'approved', 'rejected'] },
} as const

const RESULT_SEGMENTS = [
  { value: 'pending', label: '待审核', result: SIM_REVIEW_RESULT.PENDING },
  { value: 'approved', label: '已通过', result: SIM_REVIEW_RESULT.APPROVED },
  { value: 'rejected', label: '已退回', result: SIM_REVIEW_RESULT.REJECTED },
] as const

/** 四项自动校验:通过审核要求四项全过(后端 validateApprovalReport) */
const VALIDATION_STEPS = [
  { key: 'metadata_validation' as const, label: '包元数据' },
  { key: 'static_scan' as const, label: '静态扫描' },
  { key: 'determinism_check' as const, label: '结果可复现' },
  { key: 'worker_preview' as const, label: '渲染预览' },
]

/** 可执行的治理动作:同一条记录在同一时刻只会有其中一个可用 */
type GovernanceAction = 'approve' | 'reject' | 'archive' | 'republish'

const ACTION_COPY: Record<
  GovernanceAction,
  { title: string; description: string; confirm: string; danger?: boolean; needsComment?: boolean }
> = {
  approve: {
    title: '通过审核并上架',
    description: '通过后这个场景会上架给全平台的学校使用。',
    confirm: '确认上架',
  },
  reject: {
    title: '退回给作者',
    description: '退回后作者可以修改并重新提交。请写清需要改什么,作者会看到这段话。',
    confirm: '确认退回',
    danger: true,
    needsComment: true,
  },
  archive: {
    title: '下架这个场景',
    description: '下架后学校不能再新建这个场景的推演。已经在进行的推演不受影响,历史回放仍可查看。',
    confirm: '确认下架',
    danger: true,
  },
  republish: {
    title: '重新上架',
    description: '重新上架后学校可以再次使用这个场景。内容不变,不需要重新审核。',
    confirm: '确认上架',
  },
}

/** PlatformSimulationsPage 仿真场景包审核队列:左队列、右详情,处理完进下一条待审核 */
export default function PlatformSimulationsPage() {
  const [query, setQuery] = useUrlState(SIM_REVIEW_QUERY)
  const [target, setTarget] = useState<{ review: SimPackageReview; action: GovernanceAction }>()
  const [selectedId, setSelectedId] = useState<string>()
  /** <lg 走两级页面 */
  const [mobileView, setMobileView] = useState<'queue' | 'detail'>('queue')
  const segment = RESULT_SEGMENTS.find((item) => item.value === query.result) ?? RESULT_SEGMENTS[0]

  const reviews = usePagedResource<SimPackageReview>(
    (params) => api.sim.getReviews({ result: segment.result, ...params }),
    [segment.result],
    { urlPageKey: 'page' }
  )
  // 分段计数取列表接口的 facets(审核结论 result 一组)
  const facets = reviews.status === 'success' || reviews.status === 'empty' ? reviews.data?.facets : undefined
  const counts = segmentCounts(facets, 'result', RESULT_SEGMENTS, (item) => item.result)

  const list = useMemo(() => reviews.data?.list ?? [], [reviews.data])
  const selected = list.find((item) => item.id === selectedId) ?? list[0]
  const selectedIndex = selected ? list.findIndex((item) => item.id === selected.id) : -1
  const queue = useReviewQueue({
    items: list,
    currentId: selected?.id,
    onSelect: setSelectedId,
    isPending: (item: SimPackageReview) => item.result === SIM_REVIEW_RESULT.PENDING,
  })

  return (
    <PageScaffold>
      <PageHeader title="仿真治理" />

      <SegmentedControl
        aria-label="按审核结论筛选"
        options={RESULT_SEGMENTS.map((item) => ({
          value: item.value,
          label: item.label,
          count: counts[item.value],
        }))}
        value={query.result}
        onValueChange={(value) => {
          setSelectedId(undefined)
          setQuery({ result: value })
        }}
        className="mb-4 max-w-full overflow-x-auto"
      />

      <QueueDetailLayout
        view={mobileView}
        detailHeader={
          selected ? (
            <div className="flex items-center gap-2">
              <Button
                variant="ghost"
                size="sm"
                leftIcon={ChevronLeft}
                onClick={() => setMobileView('queue')}
              >
                返回队列
              </Button>
              <span className="text-sm tabular-nums text-ink-sub">
                第 {selectedIndex + 1} 条 / 共 {reviews.total ?? '—'} 条
              </span>
            </div>
          ) : undefined
        }
        queue={
          <DataPanel
            label="提交队列"
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
              emptyIcon={Shield}
              emptyTitle={query.result === 'pending' ? '没有待审核的场景' : '这个结论下没有记录'}
              emptyDescription={
                query.result === 'pending' ? '教师提交仿真场景后会出现在这里。' : '换个结论看看。'
              }
              skeleton={<Skeleton variant="line" lines={5} />}
            >
              {(page) => (
                <div
                  role="listbox"
                  aria-label="提交队列"
                  aria-activedescendant={selected ? `sim-review-${selected.id}` : undefined}
                  tabIndex={0}
                  onKeyDown={queue.onListKeyDown}
                  className="focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-accent"
                >
                  {page.list.map((review) => {
                    const active = review.id === selected?.id
                    const pkg = review.package
                    return (
                      <div
                        key={review.id}
                        id={`sim-review-${review.id}`}
                        role="option"
                        aria-selected={active}
                        onClick={() => {
                          setSelectedId(review.id)
                          setMobileView('detail')
                        }}
                        className={cn(
                          'cursor-pointer border-t border-line px-4 py-3 first:border-t-0',
                          active ? 'bg-primary-soft' : 'hover:bg-surface-hover'
                        )}
                      >
                        <div className="flex items-center justify-between gap-2">
                          <span className="truncate text-sm font-medium text-ink">
                            {pkg ? pkg.name : '已删除的场景'}
                          </span>
                          <StatusIndicator
                            appearance="text"
                            tone={simReviewResultTone(review.result)}
                            label={simReviewResultLabel(review.result)}
                          />
                        </div>
                        <p className="mt-1 truncate text-xs text-ink-sub">
                          {pkg
                            ? `${pkg.version} · ${simCategoryLabel(pkg.category)}`
                            : '场景已不存在'}{' '}
                          ·{' '}
                          <span title={formatDateTime(review.created_at)}>
                            {review.submitter_name ? `${review.submitter_name} · ` : ''}
                            {formatRelativeOccurrence(review.created_at)}提交
                          </span>
                        </p>
                      </div>
                    )
                  })}
                  <p className="hidden border-t border-line px-4 py-2 text-xs text-ink-sub lg:block">
                    ↑↓ 切换条目
                  </p>
                </div>
              )}
            </ResourceState>
          </DataPanel>
        }
        detail={
          selected ? (
            <ReviewDetail review={selected} />
          ) : (
            <div className="flex min-h-48 items-center justify-center rounded-lg bg-surface p-6 text-sm text-ink-sub shadow-xs">
              左侧选一条提交记录查看校验结果与预览。
            </div>
          )
        }
        detailActions={
          selected ? (
            <ReviewActions
              review={selected}
              onAct={(action) => setTarget({ review: selected, action })}
            />
          ) : undefined
        }
      />

      {target ? (
        <GovernanceModal
          review={target.review}
          action={target.action}
          onClose={() => setTarget(undefined)}
          onDone={() => {
            const wasPendingDecision = target.action === 'approve' || target.action === 'reject'
            setTarget(undefined)
            reviews.reload()
            // 通过或退回后进入下一条待审核;没有了就回到队列层
            if (wasPendingDecision && !queue.advance()) setMobileView('queue')
          }}
        />
      ) : null}
    </PageScaffold>
  )
}

/** ReviewDetail 一条提交:场景信息、四项校验(失败写原因)、处理说明、渲染预览帧 */
function ReviewDetail({ review }: { review: SimPackageReview }) {
  const pkg = review.package
  const validations = readValidations(review.preview_report)
  return (
    <div className="flex flex-col gap-5 rounded-lg bg-surface p-5 shadow-xs">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <h2 className="text-lg font-semibold text-ink">{pkg ? pkg.name : '已删除的场景'}</h2>
          <p className="mt-1 text-sm text-ink-sub">
            {pkg
              ? `${pkg.version} · ${simCategoryLabel(pkg.category)} · ${simComputeLabel(pkg.compute)}`
              : '这条记录对应的场景已不存在'}
          </p>
        </div>
        <span className="flex flex-wrap items-center gap-2">
          <StatusIndicator
            tone={simReviewResultTone(review.result)}
            label={simReviewResultLabel(review.result)}
          />
          {pkg ? (
            <StatusIndicator
              appearance="text"
              tone={pkg.status === SIM_PACKAGE_STATUS.PUBLISHED ? 'success' : 'neutral'}
              label={simPackageStatusLabel(pkg.status)}
            />
          ) : null}
        </span>
      </div>

      <section aria-label="自动校验" className="flex flex-col gap-2">
        <h3 className="text-sm font-semibold text-ink">自动校验</h3>
        <ul className="grid gap-2 sm:grid-cols-2">
          {validations.map((item) => (
            <li key={item.label} className="rounded-md bg-surface-hover px-3 py-2">
              <StatusIndicator
                appearance="text"
                tone={item.passed ? 'success' : 'danger'}
                label={`${item.label}${item.passed ? '通过' : '未通过'}`}
              />
              {!item.passed && item.detail ? (
                <p className="mt-1 text-xs text-ink-sub">{item.detail}</p>
              ) : null}
            </li>
          ))}
        </ul>
      </section>

      <DescriptionList
        dense
        items={[
          { term: '场景编码', description: pkg ? pkg.code : '—', mono: true },
          { term: '提交时间', description: formatDateTime(review.created_at), mono: true },
          ...(review.updated_at && review.result !== SIM_REVIEW_RESULT.PENDING
            ? [{ term: '处理时间', description: formatDateTime(review.updated_at), mono: true }]
            : []),
          ...(review.comment ? [{ term: '处理说明', description: review.comment }] : []),
        ]}
      />

      <section aria-label="渲染预览" className="flex flex-col gap-2">
        <h3 className="text-sm font-semibold text-ink">渲染预览</h3>
        <p className="text-xs text-ink-sub">
          自动校验只能说明能跑、结果可复现;算法对不对要看下面的教学帧。
        </p>
        <SimPreviewFrames frames={review.preview_report.preview_frames} />
      </section>
    </div>
  )
}

/** ReviewActions 当前记录唯一可执行的动作:待审核给退回与通过(四项没全过时通过不可用),已上架给下架,已下架给重新上架 */
function ReviewActions({
  review,
  onAct,
}: {
  review: SimPackageReview
  onAct: (action: GovernanceAction) => void
}) {
  const pkg = review.package
  if (review.result === SIM_REVIEW_RESULT.PENDING) {
    const failed = readValidations(review.preview_report).filter((item) => !item.passed)
    return (
      <div className="flex flex-wrap items-center justify-end gap-2">
        {failed.length > 0 ? (
          <span className="mr-auto text-sm text-ink-sub">
            {failed.map((item) => item.label).join('、')}没有通过,只能退回
          </span>
        ) : null}
        <Button variant="outline" leftIcon={CircleX} onClick={() => onAct('reject')}>
          退回作者
        </Button>
        <Button
          variant="seal"
          leftIcon={CircleCheck}
          disabled={failed.length > 0}
          onClick={() => onAct('approve')}
        >
          通过并上架
        </Button>
      </div>
    )
  }
  if (pkg?.status === SIM_PACKAGE_STATUS.PUBLISHED) {
    return (
      <div className="flex justify-end">
        <Button variant="outline" leftIcon={Archive} onClick={() => onAct('archive')}>
          下架
        </Button>
      </div>
    )
  }
  if (pkg?.status === SIM_PACKAGE_STATUS.ARCHIVED) {
    return (
      <div className="flex justify-end">
        <Button variant="outline" leftIcon={ArchiveRestore} onClick={() => onAct('republish')}>
          重新上架
        </Button>
      </div>
    )
  }
  return null
}

interface GovernanceModalProps {
  review: SimPackageReview
  action: GovernanceAction
  onClose: () => void
  onDone: () => void
}

/**
 * GovernanceModal 执行审核或上下架动作。
 * 退回必须写说明(后端要求 comment),其余三个动作只需确认。
 */
function GovernanceModal({ review, action, onClose, onDone }: GovernanceModalProps) {
  const [comment, setComment] = useState('')
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const copy = ACTION_COPY[action]
  const pkg = review.package
  const { errors, validate } = useFormValidation(() => ({
    comment:
      copy.needsComment && comment.trim() === '' ? '请写下退回原因,作者需要知道该改什么' : null,
  }))

  const submit = async () => {
    if (working || !validate()) return
    setFormError(undefined)
    setWorking(true)
    try {
      switch (action) {
        case 'approve':
          await api.sim.approveReview(review.id)
          toast.success('已通过审核,场景已上架')
          break
        case 'reject':
          await api.sim.rejectReview(review.id, comment.trim())
          toast.success('已退回给作者')
          break
        case 'archive':
          await api.sim.archivePackage(review.package_id)
          toast.success('场景已下架')
          break
        case 'republish':
          await api.sim.republishPackage(review.package_id)
          toast.success('场景已重新上架')
          break
      }
      onDone()
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '操作没有完成,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }

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
              { term: '场景', description: pkg ? `${pkg.name} · ${pkg.version}` : '—' },
              { term: '场景编码', description: pkg ? pkg.code : '—', mono: true },
              { term: '提交时间', description: formatDateTime(review.created_at), mono: true },
            ]}
          />

          {copy.needsComment ? (
            <FormField
              label="退回说明"
              htmlFor="sim-review-comment"
              required
              error={errors.comment}
              helper="写清需要修改什么,作者会看到这段话"
            >
              <Textarea
                id="sim-review-comment"
                value={comment}
                rows={4}
                invalid={Boolean(errors.comment)}
                onBlur={() => validate(['comment'])}
                onChange={(event) => setComment(event.target.value)}
              />
            </FormField>
          ) : null}

          {formError ? <Callout tone="danger">{formError}</Callout> : null}

          {action === 'archive' ? (
            <Callout tone="warning">
              下架不删除内容。已经在进行的推演不受影响,历史回放仍可查看。
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
            onClick={() => void submit()}
          >
            {copy.confirm}
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}

/** ValidationView 是一项自动校验的展示形态。 */
interface ValidationView {
  label: string
  passed: boolean
  detail: string
}

/**
 * readValidations 从校验报告里读四项结论。
 * 报告字段是可选的:缺失即视为未通过 —— 没有结论就不该放行。
 * 后端 validateApprovalReport 也是按「四项都必须是 passed」判定的。
 */
function readValidations(report: SimValidationReport): ValidationView[] {
  return VALIDATION_STEPS.map((step) => {
    const item = report[step.key]
    if (step.key === 'static_scan') {
      const scan = report.static_scan
      return {
        label: step.label,
        passed: scan?.status === SIM_VALIDATION_STATUS.PASSED,
        detail: (scan?.findings ?? []).join('、'),
      }
    }
    const status = item && 'status' in item ? item.status : undefined
    const message = item && 'message' in item ? item.message : undefined
    return {
      label: step.label,
      passed: status === SIM_VALIDATION_STATUS.PASSED,
      detail: message ?? '',
    }
  })
}
