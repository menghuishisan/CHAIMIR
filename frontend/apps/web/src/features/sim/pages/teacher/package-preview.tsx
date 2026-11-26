// 教师只读查看本人场景的运行校验、扫描提示与样例帧，作者归属由后端校验。

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
  Drawer,
  DrawerBody,
  DrawerContent,
  DrawerDescription,
  DrawerFooter,
  DrawerHeader,
  DrawerTitle,
  Skeleton,
  StatusIndicator,
  type StatusTone,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import { formatDateTime } from '../../../../utils/formatters'
import { simReviewResultLabel } from '../../../../utils/labels/sim'
import { SimPreviewFrames } from '../../components/SimPreviewFrames'
import { simReviewResultTone } from '../../statusPresentation'

/** 校验子项的用户向名称:后端只给字段名,界面文案在前端维护。 */
const CHECK_LABELS = {
  metadata: '包信息与表单一致',
  staticScan: '安全扫描',
  determinism: '结果可复现校验',
  workerPreview: '运行预览',
} as const

/** 扫描提示与运行失败分开呈现，缺失结论表示尚未执行。 */
function checkTone(status: SimValidationStatusValue | undefined): StatusTone {
  if (status === SIM_VALIDATION_STATUS.PASSED) return 'success'
  if (status === SIM_VALIDATION_STATUS.FAILED) return 'danger'
  if (status === SIM_VALIDATION_STATUS.WARNING) return 'warning'
  return 'neutral'
}

/** checkLabel 把校验状态翻成用户向文案。 */
function checkLabel(status: SimValidationStatusValue | undefined): string {
  if (status === SIM_VALIDATION_STATUS.PASSED) return '通过'
  if (status === SIM_VALIDATION_STATUS.FAILED) return '未通过'
  if (status === SIM_VALIDATION_STATUS.WARNING) return '需人工确认'
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
    <Drawer open onOpenChange={(open) => !open && onClose()}>
      <DrawerContent>
        <DrawerHeader>
          <DrawerTitle>审核报告</DrawerTitle>
          <DrawerDescription>
            {item.name} · 版本 {item.version}。平台将结合运行校验、风险提示与预览画面审核。
          </DrawerDescription>
        </DrawerHeader>
        <ResourceState
          resource={preview}
          emptyIcon={FileSearch}
          emptyTitle="还没有审核报告"
          emptyDescription="提交场景后平台会生成校验报告。"
          skeleton={
            <DrawerBody>
              <Skeleton variant="line" lines={5} />
            </DrawerBody>
          }
        >
          {(data) => <PreviewBody review={data.review} />}
        </ResourceState>
        <DrawerFooter>
          <Button variant="outline" onClick={onClose}>
            关闭
          </Button>
        </DrawerFooter>
      </DrawerContent>
    </Drawer>
  )
}

/**
 * PreviewBody 渲染审核结论、四项校验与隔离预览渲出的样例画面。
 */
function PreviewBody({ review }: { review: SimPackageReview }) {
  const report = review.preview_report
  const scanFindings = report.static_scan?.findings ?? []

  return (
    <DrawerBody className="flex flex-col gap-4">
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
                  <Badge key={finding} tone="warning">
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
        <Callout tone="warning" title="发现需要人工确认的调用">
          <span className="flex items-start gap-2">
            <ShieldAlert aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
            <span>
              平台会结合这些提示和运行预览判断是否上架。场景始终在隔离环境里运行，提示不会改变运行权限。
            </span>
          </span>
        </Callout>
      ) : null}

      <SimPreviewFrames frames={report.preview_frames} />

      {report.bundle_hash ? (
        <p className="font-mono text-xs text-ink-sub">场景包校验值 {report.bundle_hash}</p>
      ) : null}
    </DrawerBody>
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
    <div className="flex flex-col gap-1.5 rounded-md bg-surface-hover p-3">
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
