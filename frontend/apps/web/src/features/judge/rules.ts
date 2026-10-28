// judge 领域维护判题任务状态规则与判题器已冻结执行事实的统一判定。

import {
  JUDGE_TASK_STATUS,
  type JudgeTaskStatus,
  type JudgerResourceSpec,
  type SandboxCompositionSnapshot,
} from '@chaimir/api-client'

/** getFrozenJudgerSnapshot 区分待编译声明与已有摘要的冻结事实,不以自检状态隐藏已冻结结果。 */
export function getFrozenJudgerSnapshot(
  spec: JudgerResourceSpec | undefined,
): SandboxCompositionSnapshot | undefined {
  const snapshot = spec?.composition_snapshot
  return snapshot?.composition_digest ? snapshot : undefined
}

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
