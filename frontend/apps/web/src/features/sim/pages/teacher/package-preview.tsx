// 仿真场景审核报告(仿真场景页内弹层)。
//
// 只读展示后端 GET /sim/packages/{id}/preview 的审核报告。四个校验子项与后端字段一一对应:
// 元数据校验与静态扫描在上传时由后端生成,确定性校验与运行预览由平台的隔离预览任务在容器里跑完后回写。
// 平台要求四项全部通过才能上架,因此这里逐项呈现状态与说明,并摊开容器渲出的样例画面 ——
// 「跑不起来」与「跑起来但效果不对」是两件事,后者只能看画面。
//
// 教师只看自己提交的包(后端 PackagePreview 校验作者归属),本页不含任何审核决策动作。

import { CircleCheck, FileSearch, ShieldAlert } from 'lucide-react'
import {
  SIM_VALIDATION_STATUS,
  type SimPackageMeta,
  type SimPackageReview,
  type SimValidationStatus,
  type SimValidationStatusValue,
} from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  Skeleton,
  StatusIndicator,
  type StatusTone,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import { formatDateTime } from '../../../../utils/formatters'
import { simReviewResultLabel, simReviewResultTone } from '../../../../utils/labels/sim'
import { SimPreviewFrames } from '../../SimPreviewFrames'

/** 校验子项的用户向名称:后端只给字段名,界面文案在前端维护。 */
const CHECK_LABELS = {
  metadata: '包信息与表单一致',
  staticScan: '安全扫描',
  determinism: '结果可复现校验',
  workerPreview: '运行预览',
} as const

/** 后端校验状态取值只有 passed / failed 两种(enum.go),其余视为尚未执行。 */
function checkTone(status: SimValidationStatusValue | undefined): StatusTone {
  if (status === SIM_VALIDATION_STATUS.PASSED) return 'success'
  if (status === SIM_VALIDATION_STATUS.FAILED) return 'danger'
  return 'neutral'
}

/** checkLabel 把校验状态翻成用户向文案。 */
function checkLabel(status: SimValidationStatusValue | undefined): string {
  if (status === SIM_VALIDATION_STATUS.PASSED) return '通过'
  if (status === SIM_VALIDATION_STATUS.FAILED) return '未通过'
  return '尚未执行'
}

export interface SimPackagePreviewModalProps {
  item: SimPackageMeta
  onClose: () => void
}

/**
 * SimPackagePreviewModal 展示单个场景包的最新审核报告。
 */
export function SimPackagePreviewModal({ item, onClose }: SimPackagePreviewModalProps) {
  const preview = useAsyncResource(
    () => api.sim.previewPackage(item.id),
    [item.id],
    () => false
  )

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="lg">
        <ModalHeader>
          <ModalTitle>审核报告</ModalTitle>
          <ModalDescription>
            {item.name} · 版本 {item.version}。四项校验全部通过后平台才能让场景上架。
          </ModalDescription>
        </ModalHeader>
        <ResourceState
          resource={preview}
          emptyIcon={FileSearch}
          emptyTitle="还没有审核报告"
          emptyDescription="提交场景后平台会生成校验报告。"
          skeleton={
            <ModalBody>
              <Skeleton variant="line" lines={5} />
            </ModalBody>
          }
        >
          {(data) => <PreviewBody review={data.review} />}
        </ResourceState>
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            关闭
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}

/**
 * PreviewBody 渲染审核结论、四项校验与隔离预览渲出的样例画面。
 */
function PreviewBody({ review }: { review: SimPackageReview }) {
  const report = review.preview_report
  const scanFindings = report.static_scan?.findings ?? []

  return (
    <ModalBody className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-2">
        <StatusIndicator
          tone={simReviewResultTone(review.result)}
          label={simReviewResultLabel(review.result)}
        />
        <span className="font-mono text-xs tabular-nums text-ink-sub">
          提交于 {formatDateTime(review.created_at)}
        </span>
      </div>

      {review.comment ? (
        <Callout tone={review.result === 'rejected' ? 'warning' : 'info'} title="平台审核意见">
          {review.comment}
        </Callout>
      ) : null}

      <div className="flex flex-col gap-2">
        <CheckRow label={CHECK_LABELS.metadata} status={report.metadata_validation} />
        <CheckRow
          label={CHECK_LABELS.staticScan}
          status={{ status: report.static_scan?.status }}
          extra={
            scanFindings.length > 0 ? (
              <div className="flex flex-wrap gap-1">
                {scanFindings.map((finding) => (
                  <Badge key={finding} tone="danger">
                    {finding}
                  </Badge>
                ))}
              </div>
            ) : undefined
          }
        />
        <CheckRow label={CHECK_LABELS.determinism} status={report.determinism_check} />
        <CheckRow label={CHECK_LABELS.workerPreview} status={report.worker_preview} />
      </div>

      {scanFindings.length > 0 ? (
        <Callout tone="danger" title="包里出现了不允许的调用">
          <span className="flex items-start gap-2">
            <ShieldAlert aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
            <span>
              仿真场景在隔离环境里运行,不能访问网络、页面文档或本机存储。请去掉上面列出的调用后重新提交。
            </span>
          </span>
        </Callout>
      ) : null}

      <SimPreviewFrames frames={report.preview_frames} />

      {report.bundle_hash ? (
        <p className="font-mono text-xs text-ink-faint">场景包校验值 {report.bundle_hash}</p>
      ) : null}
    </ModalBody>
  )
}

interface CheckRowProps {
  label: string
  status?: SimValidationStatus
  extra?: React.ReactNode
}

/**
 * CheckRow 渲染单项校验结果:状态 + 后端给的说明。
 */
function CheckRow({ label, status, extra }: CheckRowProps) {
  return (
    <div className="flex flex-col gap-1.5 well p-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="text-sm text-ink">{label}</span>
        <StatusIndicator
          tone={checkTone(status?.status)}
          label={checkLabel(status?.status)}
          icon={status?.status === SIM_VALIDATION_STATUS.PASSED ? CircleCheck : undefined}
        />
      </div>
      {status?.message ? <p className="text-xs text-ink-sub">{status.message}</p> : null}
      {extra}
    </div>
  )
}
