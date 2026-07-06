// ===== M3 Judge 模块 =====

import type { JudgeTaskStatus, JudgerStatus, JudgerType } from '../constants/judge'

export interface JudgeTask {
  task_id: string
  tenant_id: string
  source_ref: string
  submitter_id: string
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
  resource_spec: Record<string, unknown>
  status: JudgerStatus
}

export interface Judger extends JudgerRequest {
  id: string
  created_at?: string
  updated_at?: string
}
