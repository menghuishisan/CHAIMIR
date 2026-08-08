// judge labels 文件维护 M3 评测模块枚举的用户向文案与语义色。

import type { StatusTone } from '@chaimir/ui'
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
  [JUDGE_TASK_STATUS.TIMEOUT]: '判题超时',
  [JUDGE_TASK_STATUS.FAILED]: '判题失败',
  [JUDGE_TASK_STATUS.ERROR]: '判题出错',
  [JUDGE_TASK_STATUS.CANCELLED]: '已取消',
}

const TASK_STATUS_TONES: Record<JudgeTaskStatus, StatusTone> = {
  [JUDGE_TASK_STATUS.QUEUED]: 'neutral',
  [JUDGE_TASK_STATUS.JUDGING]: 'info',
  [JUDGE_TASK_STATUS.DONE]: 'success',
  [JUDGE_TASK_STATUS.TIMEOUT]: 'warning',
  [JUDGE_TASK_STATUS.FAILED]: 'danger',
  [JUDGE_TASK_STATUS.ERROR]: 'danger',
  [JUDGE_TASK_STATUS.CANCELLED]: 'neutral',
}

/** judgeTaskStatusLabel 返回判题任务状态文案。 */
export function judgeTaskStatusLabel(status: JudgeTaskStatus): string {
  return TASK_STATUS_LABELS[status]
}

/** judgeTaskStatusTone 返回判题任务状态语义色。 */
export function judgeTaskStatusTone(status: JudgeTaskStatus): StatusTone {
  return TASK_STATUS_TONES[status]
}

/** 进行中的判题状态:决定是否显示旋转指示与是否可重判。 */
const ACTIVE_TASK_STATUSES: ReadonlySet<JudgeTaskStatus> = new Set([
  JUDGE_TASK_STATUS.QUEUED,
  JUDGE_TASK_STATUS.JUDGING,
])

/** isJudgeTaskActive 判断判题任务是否仍在进行。 */
export function isJudgeTaskActive(status: JudgeTaskStatus): boolean {
  return ACTIVE_TASK_STATUSES.has(status)
}

/** 需要人工介入的判题状态:失败、超时与出错都可以重判。 */
const ABNORMAL_TASK_STATUSES: ReadonlySet<JudgeTaskStatus> = new Set([
  JUDGE_TASK_STATUS.FAILED,
  JUDGE_TASK_STATUS.TIMEOUT,
  JUDGE_TASK_STATUS.ERROR,
])

/** isJudgeTaskAbnormal 判断判题任务是否需要教师介入。 */
export function isJudgeTaskAbnormal(status: JudgeTaskStatus): boolean {
  return ABNORMAL_TASK_STATUSES.has(status)
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

/** JUDGER_TYPES 供表单按登记顺序渲染判题器类型选项。 */
export const JUDGER_TYPES = [
  JudgerType.TESTCASE,
  JudgerType.ONCHAIN_ASSERT,
  JudgerType.FLAG,
  JudgerType.STATIC_SCAN,
  JudgerType.SIM_CHECKPOINT,
  JudgerType.MANUAL,
] as const

const JUDGER_STATUS_LABELS: Record<JudgerStatus, string> = {
  [JudgerStatus.AVAILABLE]: '可用',
  [JudgerStatus.DISABLED]: '已停用',
}

const JUDGER_STATUS_TONES: Record<JudgerStatus, StatusTone> = {
  [JudgerStatus.AVAILABLE]: 'success',
  [JudgerStatus.DISABLED]: 'neutral',
}

/** judgerStatusLabel 返回判题器状态文案。 */
export function judgerStatusLabel(status: JudgerStatus): string {
  return JUDGER_STATUS_LABELS[status]
}

/** judgerStatusTone 返回判题器状态语义色。 */
export function judgerStatusTone(status: JudgerStatus): StatusTone {
  return JUDGER_STATUS_TONES[status]
}

/** JUDGER_STATUSES 供表单渲染状态选项。 */
export const JUDGER_STATUSES = [JudgerStatus.AVAILABLE, JudgerStatus.DISABLED] as const
