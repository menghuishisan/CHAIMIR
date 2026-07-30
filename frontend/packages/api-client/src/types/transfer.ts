// ===== Platform Transfer 模块 =====

import type { TransferChannel, TransferStatus } from '../constants/transfer'
import type { SnowflakeID } from './common'

export interface TransferTask {
  task_id: SnowflakeID
  channel: TransferChannel
  subject: string
  status: TransferStatus
  content_type?: string
  // 任务入口强制非空(internal/platform/transfer NewTask),所以快照里必定带原始文件名。
  file_name: string
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
  // 授权只对 succeeded 且已登记产物的任务签发(BuildDownloadGrant),产物字段在授权响应里必定存在。
  task: TransferTask & { artifact_file_name: string; artifact_size: number }
  expires_at: string
}
