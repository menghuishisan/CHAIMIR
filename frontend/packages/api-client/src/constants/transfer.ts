// 导入导出契约常量：维护基础层 transfer 任务状态和通道字符串。

export const TRANSFER_STATUS = {
  PENDING: 'pending',
  RUNNING: 'running',
  RETRYING: 'retrying',
  SUCCEEDED: 'succeeded',
  FAILED: 'failed',
} as const

export type TransferStatus = (typeof TRANSFER_STATUS)[keyof typeof TRANSFER_STATUS]

export const TRANSFER_CHANNEL = {
  IMPORT: 'import',
  EXPORT: 'export',
} as const

export type TransferChannel = (typeof TRANSFER_CHANNEL)[keyof typeof TRANSFER_CHANNEL]
