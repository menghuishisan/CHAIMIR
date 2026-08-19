// judge 领域维护判题任务与判题器状态对应的界面语义色。

import type { StatusTone } from '@chaimir/ui'
import { JUDGE_TASK_STATUS, JudgerStatus, type JudgeTaskStatus } from '@chaimir/api-client'

const TASK_STATUS_TONES: Record<JudgeTaskStatus, StatusTone> = {
  [JUDGE_TASK_STATUS.QUEUED]: 'neutral',
  [JUDGE_TASK_STATUS.JUDGING]: 'info',
  [JUDGE_TASK_STATUS.DONE]: 'success',
  [JUDGE_TASK_STATUS.FAILED]: 'danger',
  [JUDGE_TASK_STATUS.CANCELLED]: 'neutral',
}

/** judgeTaskStatusTone 返回判题任务状态语义色。 */
export function judgeTaskStatusTone(status: JudgeTaskStatus): StatusTone {
  return TASK_STATUS_TONES[status]
}

const JUDGER_STATUS_TONES: Record<JudgerStatus, StatusTone> = {
  [JudgerStatus.AVAILABLE]: 'success',
  [JudgerStatus.DISABLED]: 'neutral',
}

/** judgerStatusTone 返回判题器状态语义色。 */
export function judgerStatusTone(status: JudgerStatus): StatusTone {
  return JUDGER_STATUS_TONES[status]
}
