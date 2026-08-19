// judge 领域维护判题任务状态对应的业务判断规则。

import { JUDGE_TASK_STATUS, type JudgeTaskStatus } from '@chaimir/api-client'

const ACTIVE_TASK_STATUSES: ReadonlySet<JudgeTaskStatus> = new Set([
  JUDGE_TASK_STATUS.QUEUED,
  JUDGE_TASK_STATUS.JUDGING,
])

/** isJudgeTaskActive 判断判题任务是否仍在进行。 */
export function isJudgeTaskActive(status: JudgeTaskStatus): boolean {
  return ACTIVE_TASK_STATUSES.has(status)
}

const ABNORMAL_TASK_STATUSES: ReadonlySet<JudgeTaskStatus> = new Set([JUDGE_TASK_STATUS.FAILED])

/** isJudgeTaskAbnormal 判断判题任务是否需要教师介入。 */
export function isJudgeTaskAbnormal(status: JudgeTaskStatus): boolean {
  return ABNORMAL_TASK_STATUSES.has(status)
}
