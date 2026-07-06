// ===== Platform Transfer 模块 =====

import type { TransferChannel, TransferStatus } from '../constants/transfer'

export interface TransferTask {
  task_id: string
  channel: TransferChannel
  subject: string
  status: TransferStatus
  content_type?: string
  file_name?: string
  attempt_count: number
  max_attempts: number
  artifact_size?: number
  artifact_content_type?: string
  artifact_file_name?: string
  created_at: string
  updated_at: string
  completed_at?: string
  next_attempt_after?: string
}

export interface TransferTaskListResponse {
  items: TransferTask[]
  page: number
  size: number
}

export interface TransferDownloadGrant {
  token: string
  task: TransferTask
  expires_at: string
}
