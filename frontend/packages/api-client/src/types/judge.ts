// ===== M3 Judge 模块 =====

import type { JudgeTaskStatus, JudgerStatus, JudgerType } from '../constants/judge'
import type { SnowflakeID } from './common'
import type { WorkloadComponent } from './workload'

export interface JudgeTask {
  task_id: SnowflakeID
  tenant_id: SnowflakeID
  source_ref: string
  submitter_id: SnowflakeID
  status: JudgeTaskStatus
  existing?: boolean
  result?: JudgeTaskResult
}

export interface JudgeTaskResult {
  passed: boolean
  score: number
  max_score: number
  version?: number
  is_rejudge?: boolean
  details: JudgeResultDetail[]
  snapshot_ref: string
}

export interface JudgeResultDetail {
  case?: string
  source?: string
  target?: string
  passed: boolean
  expected_label?: string
  actual?: string
  hint?: string
}

export interface JudgeManualScoreRequest {
  score: number
  max_score: number
  passed: boolean
  comment: string
}

export interface JudgerRequest {
  code: string
  name: string
  type: JudgerType
  executor_ref: string
  runtime_required: boolean
  default_timeout_sec: number
  resource_spec: JudgerResourceSpec
  status: JudgerStatus
}

export interface JudgerResourceSpec {
  runtime_code?: string
  runtime_image_version?: string
  genesis_ref?: string
  tool_codes?: string[]
  init_script_ref?: string
  command?: string[]
  exec_target?: string
  execution_sidecars?: WorkloadComponent[]
  timeout_sec?: number
  max_retries?: number
  suite_archive_name?: string
  selftest?: Record<string, unknown>
}

export interface Judger extends JudgerRequest {
  id: SnowflakeID
  created_at?: string
  updated_at?: string
}
