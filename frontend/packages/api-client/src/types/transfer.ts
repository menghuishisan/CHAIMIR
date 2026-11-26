import type { TransferChannel, TransferStatus } from '../constants/transfer'
import type { SnowflakeID } from './common'

export interface TransferTask {
  task_id: SnowflakeID
  channel: TransferChannel
  subject: string
  status: TransferStatus
  content_type?: string
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

export interface TransferDownloadGrant {
  token: string
  /** 只有已生成产物的成功任务才能签发授权。 */
  task: TransferTask & { artifact_file_name: string; artifact_size: number }
  expires_at: string
}
