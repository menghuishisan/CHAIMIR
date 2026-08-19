// transfer 工具文件提供跨布局与业务页面共用的任务状态判定。

import { TRANSFER_STATUS, type TransferStatus } from '@chaimir/api-client'
import type { StatusTone } from '@chaimir/ui'

/** TRANSFER_ACTIVE_STATUSES 是任务中心与任务列表共同使用的进行中状态集合。 */
export const TRANSFER_ACTIVE_STATUSES: ReadonlySet<TransferStatus> = new Set([
  TRANSFER_STATUS.PENDING,
  TRANSFER_STATUS.RUNNING,
  TRANSFER_STATUS.RETRYING,
])

const TRANSFER_TASK_STATUS_TONES: Record<TransferStatus, StatusTone> = {
  [TRANSFER_STATUS.PENDING]: 'neutral',
  [TRANSFER_STATUS.RUNNING]: 'info',
  [TRANSFER_STATUS.RETRYING]: 'warning',
  [TRANSFER_STATUS.SUCCEEDED]: 'success',
  [TRANSFER_STATUS.FAILED]: 'danger',
}

/** transferTaskStatusTone 返回导入导出任务状态语义色。 */
export function transferTaskStatusTone(status: TransferStatus): StatusTone {
  return TRANSFER_TASK_STATUS_TONES[status]
}
