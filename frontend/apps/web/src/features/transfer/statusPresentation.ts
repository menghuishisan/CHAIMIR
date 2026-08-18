// transfer 领域维护导入导出任务状态对应的界面语义色。

import type { StatusTone } from '@chaimir/ui'
import { TRANSFER_STATUS, type TransferStatus } from '@chaimir/api-client'

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
