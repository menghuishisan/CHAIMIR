// judge labels 文件维护 M3 评测模块枚举的用户向文案。

import {
  JUDGE_TASK_STATUS,
  JudgerStatus,
  JudgerType,
  type JudgeTaskStatus,
} from '@chaimir/api-client'

const TASK_STATUS_LABELS: Record<JudgeTaskStatus, string> = {
  [JUDGE_TASK_STATUS.QUEUED]: '排队中',
  [JUDGE_TASK_STATUS.JUDGING]: '判题中',
  [JUDGE_TASK_STATUS.DONE]: '已完成',
  [JUDGE_TASK_STATUS.FAILED]: '判题失败',
  [JUDGE_TASK_STATUS.CANCELLED]: '已取消',
}

/** judgeTaskStatusLabel 返回判题任务状态文案。 */
export function judgeTaskStatusLabel(status: JudgeTaskStatus): string {
  return TASK_STATUS_LABELS[status]
}

const JUDGER_TYPE_LABELS: Record<JudgerType, string> = {
  [JudgerType.TESTCASE]: '测试用例',
  [JudgerType.ONCHAIN_ASSERT]: '链上断言',
  [JudgerType.FLAG]: '答案口令比对',
  [JudgerType.STATIC_SCAN]: '静态扫描',
  [JudgerType.SIM_CHECKPOINT]: '仿真检查点',
  [JudgerType.MANUAL]: '人工评分',
}

/** judgerTypeLabel 返回判题器类型文案。 */
export function judgerTypeLabel(type: JudgerType): string {
  return JUDGER_TYPE_LABELS[type]
}

const JUDGER_STATUS_LABELS: Record<JudgerStatus, string> = {
  [JudgerStatus.AVAILABLE]: '可用',
  [JudgerStatus.DISABLED]: '已停用',
}

/** judgerStatusLabel 返回判题器状态文案。 */
export function judgerStatusLabel(status: JudgerStatus): string {
  return JUDGER_STATUS_LABELS[status]
}
