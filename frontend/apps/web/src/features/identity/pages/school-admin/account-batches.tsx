// 导入记录(账号批量导入抽屉内,设计 §7.5)。
// 展示服务端持久化的导入批次:每次导入的文件、对象、时间、成功与失败数、状态;按时间从新到旧。
// 导入是账号管理的一部分,记录跟着导入抽屉走,不单独占页面。

import { ImportBatchStatus, ImportTarget } from '@chaimir/api-client'
import { Skeleton, StatusIndicator, type StatusTone } from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceError } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import { formatDateTime, formatRelativeOccurrence } from '../../../../utils/formatters'

/** 导入对象文案:只在导入记录里用到,不进全局 labels */
const TARGET_LABELS: Record<ImportTarget, string> = {
  [ImportTarget.TEACHER]: '教师账号',
  [ImportTarget.STUDENT]: '学生账号',
  [ImportTarget.ORG]: '组织架构',
}

const BATCH_STATUS_LABELS: Record<ImportBatchStatus, string> = {
  [ImportBatchStatus.PROCESSING]: '处理中',
  [ImportBatchStatus.COMPLETED]: '已完成',
  [ImportBatchStatus.FAILED]: '处理失败',
}

const BATCH_STATUS_TONES: Record<ImportBatchStatus, StatusTone> = {
  [ImportBatchStatus.PROCESSING]: 'info',
  [ImportBatchStatus.COMPLETED]: 'success',
  [ImportBatchStatus.FAILED]: 'danger',
}

/**
 * AccountImportBatches 列出历史导入批次;没有记录时一句说明,读不到时就地重试。
 */
export function AccountImportBatches() {
  const batches = useAsyncResource(() => api.identity.listAccountImportBatches(), [], () => false)

  return (
    <section aria-labelledby="import-batches-title" className="flex flex-col gap-2">
      <h3 id="import-batches-title" className="text-sm font-semibold text-ink">
        导入记录
      </h3>
      {batches.status === 'loading' ? (
        <Skeleton variant="line" lines={3} />
      ) : batches.status === 'error' ? (
        <ResourceError message={batches.error?.message ?? '导入记录暂时读取不到'} traceId={batches.error?.traceId} onRetry={batches.reload} />
      ) : (batches.data ?? []).length === 0 ? (
        <p className="text-sm text-ink-sub">还没有导入记录。每次导入的结果会记录在这里。</p>
      ) : (
        <ul className="divide-y divide-line">
          {(batches.data ?? []).map((batch) => (
            <li key={batch.id} className="flex flex-wrap items-center gap-x-4 gap-y-1 py-2.5">
              <span className="min-w-0 flex-1">
                <span className="block truncate text-sm font-medium text-ink">{batch.file_name}</span>
                <span className="block text-xs text-ink-sub">
                  {TARGET_LABELS[batch.target_type]} ·{' '}
                  <span title={formatDateTime(batch.created_at)}>{formatRelativeOccurrence(batch.created_at)}</span> · 共 {batch.total} 行,成功{' '}
                  {batch.success} 行{batch.failed > 0 ? `,失败 ${batch.failed} 行` : ''}
                </span>
              </span>
              <StatusIndicator
                tone={BATCH_STATUS_TONES[batch.status]}
                label={BATCH_STATUS_LABELS[batch.status]}
                loading={batch.status === ImportBatchStatus.PROCESSING}
              />
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
