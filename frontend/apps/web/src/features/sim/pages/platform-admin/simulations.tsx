// 仿真治理页(平台侧栏,/platform-admin/simulations)。
//
// 教师提交仿真场景包 → 平台审核 → 通过即上架给全平台使用。上架后可以下架,下架后可以重新上架。
// 后端对这四个动作有互斥前提(只有待审记录能通过/退回,只有已上架能下架,只有已下架能重新上架),
// 故本页按记录当前状态只给出那一个可执行动作 —— 同时摊出互斥按钮等于让人去点必然失败的操作。
//
// 四项校验报告(元数据、静态扫描、确定性、渲染预览)直接读审核记录自带的 preview_report:
// GET /sim/reviews 已经带上了它,而 GET /sim/packages/{key}/preview 在 teacher 组且断言作者本人,
// 平台身份调用必然被拒(对齐清单 §3.4)。
//
// 报告里还带着隔离容器在上架前渲出的样例教学帧,本页必须把它摊开:自动校验只回答「能不能跑、
// 是否确定性」,回答不了「这个算法实现对不对」—— 只看四个徽章的审核等于没审
// (见 docs/04-仿真可视化引擎/06-业务流程与状态机.md §4)。
//
// 仿真包列表接口也在 teacher 组,平台端看不到;审核记录本身就是平台侧的包窗口 ——
// 每次提交都会产生一条审核记录,记录里带包的编码、版本与当前状态。

import { useCallback, useMemo, useState } from 'react'
import { Archive, ArchiveRestore, CircleCheck, CircleX, Shield } from 'lucide-react'
import {
  SIM_PACKAGE_STATUS,
  SIM_REVIEW_RESULT,
  SIM_VALIDATION_STATUS,
  type SimPackageReview,
  type SimReviewResult,
  type SimValidationReport,
} from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  MetricStrip,
  Card,
  CardBody,
  CardHeader,
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
  PageHeader,
  PageScaffold,
  PageSection,
  Pagination,
  SegmentedControl,
  Skeleton,
  StatusIndicator,
  Textarea,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { usePagedResource, useResourceTotal } from '../../../../hooks'
import { formatDateTime } from '../../../../utils/formatters'
import {
  simCategoryLabel,
  simComputeLabel,
  simPackageStatusLabel,
  simReviewResultLabel,
} from '../../../../utils/labels/sim'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { SimPreviewFrames } from '../../components/SimPreviewFrames'
import { simReviewResultTone } from '../../statusPresentation'

/**
 * 审核状态筛选项:三个结论已经穷尽所有记录,故不给「全部」——
 * GET /sim/reviews 不带 result 时后端按待审核处理(docs/04-仿真可视化引擎/05-接口设计.md §4),
 * 摆一个叫「全部」却只回待审核的选项等于让页面说假话。
 */
const RESULT_FILTERS = [
  { value: SIM_REVIEW_RESULT.PENDING, label: '待审核' },
  { value: SIM_REVIEW_RESULT.APPROVED, label: '已通过' },
  { value: SIM_REVIEW_RESULT.REJECTED, label: '已退回' },
] as const

/** 四项自动校验:通过审核要求四项全过(后端 validateApprovalReport)。 */
const VALIDATION_STEPS = [
  { key: 'metadata_validation' as const, label: '包元数据' },
  { key: 'static_scan' as const, label: '静态扫描' },
  { key: 'determinism_check' as const, label: '结果可复现' },
  { key: 'worker_preview' as const, label: '渲染预览' },
]

/** 可执行的治理动作:同一条记录在同一时刻只会有其中一个可用。 */
type GovernanceAction = 'approve' | 'reject' | 'archive' | 'republish'

const ACTION_COPY: Record<
  GovernanceAction,
  { title: string; description: string; confirm: string; danger?: boolean; needsComment?: boolean }
> = {
  approve: {
    title: '通过审核并上架',
    description:
      '通过后这个场景包会上架给全平台的学校使用。四项自动校验必须全部通过,否则后端会拒绝。',
    confirm: '确认上架',
  },
  reject: {
    title: '退回给作者',
    description: '退回后作者可以修改并重新提交。请写清需要改什么,作者会看到这段话。',
    confirm: '确认退回',
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

/**
 * PlatformSimulationsPage 承载仿真场景包的审核与上下架治理。
 */
export default function PlatformSimulationsPage() {
  const [resultFilter, setResultFilter] = useState<SimReviewResult>(SIM_REVIEW_RESULT.PENDING)
  const [target, setTarget] = useState<{ review: SimPackageReview; action: GovernanceAction }>()

  const reviews = usePagedResource<SimPackageReview>(
    (params) => api.sim.getReviews({ result: resultFilter, ...params }),
    [resultFilter]
  )

  // 指标带取服务端全量口径,不随下方筛选变化;三张卡同为「审核记录条数」,可以并列比较。
  // 不放「已上架/已下架场景」:那是包的状态,而 GET /sim/packages 在 teacher 组,
  // 平台身份调用必然被拒(见文件头);拿不到的数不摆卡,更不用别的口径顶替。
  const pendingCount = useResourceTotal(
    (params) => api.sim.getReviews({ result: SIM_REVIEW_RESULT.PENDING, ...params }),
    []
  )
  const approvedCount = useResourceTotal(
    (params) => api.sim.getReviews({ result: SIM_REVIEW_RESULT.APPROVED, ...params }),
    []
  )
  const rejectedCount = useResourceTotal(
    (params) => api.sim.getReviews({ result: SIM_REVIEW_RESULT.REJECTED, ...params }),
    []
  )

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '底层资源' }]} />}
        title="仿真治理"
        description="教师提交的仿真场景包在这里审核。通过后上架给全平台使用,之后也可以下架或重新上架。"
        icon={Shield}
      />

      {/* 指标降为内联摘要:本页主体是提交记录,不是这三个数字(§6.5.3 第 ① 族) */}
      <MetricStrip
        label="审核积压摘要"
        className="mb-5"
        items={[
          {
            label: '待审核',
            value: pendingCount ?? '—',
            hint: pendingCount === 0 ? '暂时没有积压' : '需要你处理',
          },
          { label: '已通过', value: approvedCount ?? '—', hint: '通过即已上架' },
          { label: '已退回', value: rejectedCount ?? '—', hint: '教师可修改后重提' },
        ]}
      />

      <PageSection
        title="提交记录"
        description="每一条对应一次场景包提交。四项自动校验全过才能通过审核。"
      >
        <div className="flex flex-col gap-4">
          {/* 数据区是一排 ReviewCard(已是抬起片),筛选走 bare 无底形态,避免片里套片(§6.5.2) */}
          <FilterBar label="提交记录筛选" bare>
            <FilterField label="审核状态" group>
              <SegmentedControl
                aria-label="按审核状态筛选"
                size="sm"
                options={RESULT_FILTERS.map((item) => ({ value: item.value, label: item.label }))}
                value={resultFilter}
                onValueChange={(value) => setResultFilter(value as SimReviewResult)}
              />
            </FilterField>
          </FilterBar>

          <ResourceState
            resource={reviews}
            emptyIcon={Shield}
            emptyTitle={
              resultFilter === SIM_REVIEW_RESULT.PENDING ? '没有待审核的记录' : '这个状态下没有记录'
            }
            emptyDescription={
              resultFilter === SIM_REVIEW_RESULT.PENDING
                ? '教师在教学端提交仿真场景包后,记录会出现在这里。'
                : '换个状态看看。'
            }
            skeleton={<Skeleton variant="line" lines={4} />}
          >
            {(page) => (
              <div className="flex flex-col gap-4">
                <div className="grid gap-4 lg:grid-cols-2">
                  {page.list.map((review) => (
                    <ReviewCard
                      key={review.id}
                      review={review}
                      onAct={(action) => setTarget({ review, action })}
                    />
                  ))}
                </div>
                <Pagination
                  page={reviews.page}
                  pageSize={reviews.pageSize}
                  total={reviews.total}
                  onPageChange={reviews.setPage}
                />
              </div>
            )}
          </ResourceState>

          <Callout tone="info">
            四项校验由平台在提交时自动完成:包元数据、静态扫描、结果可复现、渲染预览。有任意一项没过就只能退回。
          </Callout>
        </div>
      </PageSection>

      {target ? (
        <GovernanceModal
          review={target.review}
          action={target.action}
          onClose={() => setTarget(undefined)}
          onDone={() => {
            setTarget(undefined)
            reviews.reload()
          }}
        />
      ) : null}
    </PageScaffold>
  )
}

interface ReviewCardProps {
  review: SimPackageReview
  onAct: (action: GovernanceAction) => void
}

/**
 * ReviewCard 展示一条提交记录与它当下唯一可执行的动作。
 * 动作按后端的互斥前提推导:待审可通过/退回,已上架可下架,已下架可重新上架,
 * 其余状态不给动作 —— 不摊出必然失败的按钮。
 */
function ReviewCard({ review, onAct }: ReviewCardProps) {
  const pkg = review.package
  const pending = review.result === SIM_REVIEW_RESULT.PENDING
  const validations = useMemo(() => readValidations(review.preview_report), [review.preview_report])
  const allPassed = validations.every((item) => item.passed)

  return (
    <Card>
      <CardHeader
        title={pkg ? pkg.name : '已删除的场景包'}
        description={
          pkg
            ? `${simCategoryLabel(pkg.category)} · ${simComputeLabel(pkg.compute)} · ${pkg.version}`
            : '这条记录对应的场景包已不存在'
        }
        actions={
          <div className="flex flex-wrap items-center gap-2">
            <StatusIndicator
              tone={simReviewResultTone(review.result)}
              label={simReviewResultLabel(review.result)}
            />
            {pkg ? (
              <Badge tone={pkg.status === SIM_PACKAGE_STATUS.PUBLISHED ? 'jade' : 'neutral'}>
                {simPackageStatusLabel(pkg.status)}
              </Badge>
            ) : null}
          </div>
        }
      />
      <CardBody className="flex flex-col gap-4">
        <DescriptionList
          dense
          items={[
            { term: '场景编码', description: pkg ? pkg.code : '—', mono: true },
            { term: '提交时间', description: formatDateTime(review.created_at), mono: true },
            ...(review.updated_at
              ? [{ term: '处理时间', description: formatDateTime(review.updated_at), mono: true }]
              : []),
            ...(review.comment ? [{ term: '处理说明', description: review.comment }] : []),
          ]}
        />

        <div className="flex flex-col gap-2">
          <span className="text-sm text-ink-sub">自动校验</span>
          <div className="flex flex-wrap gap-1.5">
            {validations.map((item) => (
              <Badge key={item.label} tone={item.passed ? 'success' : 'danger'}>
                {item.label}
                {item.passed ? '' : ' 未通过'}
              </Badge>
            ))}
          </div>
          {!allPassed && pending ? (
            <span className="text-sm text-ink-sub">有校验项没通过,这个包只能退回给作者修改。</span>
          ) : null}
        </div>

        <SimPreviewFrames frames={review.preview_report.preview_frames} />

        <div className="flex flex-wrap items-center gap-2">
          {pending ? (
            <>
              <Button
                variant="seal"
                size="sm"
                leftIcon={CircleCheck}
                disabled={!allPassed}
                onClick={() => onAct('approve')}
              >
                通过并上架
              </Button>
              <Button
                variant="outline"
                size="sm"
                leftIcon={CircleX}
                onClick={() => onAct('reject')}
              >
                退回作者
              </Button>
            </>
          ) : pkg?.status === SIM_PACKAGE_STATUS.PUBLISHED ? (
            <Button variant="outline" size="sm" leftIcon={Archive} onClick={() => onAct('archive')}>
              下架
            </Button>
          ) : pkg?.status === SIM_PACKAGE_STATUS.ARCHIVED ? (
            <Button
              variant="outline"
              size="sm"
              leftIcon={ArchiveRestore}
              onClick={() => onAct('republish')}
            >
              重新上架
            </Button>
          ) : (
            <span className="text-sm text-ink-faint">这条记录当前没有可执行的动作</span>
          )}
        </div>

        {validations.some((item) => !item.passed && item.detail !== '') ? (
          <Callout tone="warning" title="校验失败原因">
            {validations
              .filter((item) => !item.passed && item.detail !== '')
              .map((item) => `${item.label}:${item.detail}`)
              .join(';')}
          </Callout>
        ) : null}
      </CardBody>
    </Card>
  )
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

  const submit = useCallback(async () => {
    if (copy.needsComment && comment.trim() === '') {
      setFormError('请写下退回原因,作者需要知道该改什么')
      return
    }
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
  }, [action, comment, copy.needsComment, onDone, review.id, review.package_id])

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
              error={formError}
              helper="写清需要修改什么,作者会看到这段话"
            >
              <Textarea
                id="sim-review-comment"
                value={comment}
                rows={4}
                invalid={Boolean(formError)}
                onChange={(event) => setComment(event.target.value)}
              />
            </FormField>
          ) : formError ? (
            <Callout tone="danger">{formError}</Callout>
          ) : null}

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
