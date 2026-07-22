// judge labels 文件维护判题器类型、状态和任务状态文案。

import { JUDGE_TASK_STATUS, JudgerStatus, JudgerType } from '@chaimir/api-client'
import { labelFromMap } from './map'

/** judgerTypeLabel 返回判题器类型文案。 */
export function judgerTypeLabel(type: JudgerType): string {
  return labelFromMap(type, {
    [JudgerType.TESTCASE]: '测试用例', [JudgerType.ONCHAIN_ASSERT]: '链上断言', [JudgerType.FLAG]: 'Flag 校验',
    [JudgerType.STATIC_SCAN]: '静态扫描', [JudgerType.SIM_CHECKPOINT]: '仿真检查点', [JudgerType.MANUAL]: '人工评分',
  }, '未知')
}

/** judgerStatusLabel 返回判题器启用状态文案。 */
export function judgerStatusLabel(status: JudgerStatus): string {
  return labelFromMap(status, { [JudgerStatus.AVAILABLE]: '可用', [JudgerStatus.DISABLED]: '已停用' }, '未知')
}

/** judgeTaskStatusLabel 返回判题任务状态文案。 */
export function judgeTaskStatusLabel(status: string): string {
  return labelFromMap(status, {
    [JUDGE_TASK_STATUS.QUEUED]: '等待判题', [JUDGE_TASK_STATUS.JUDGING]: '判题中',
    [JUDGE_TASK_STATUS.DONE]: '已完成', [JUDGE_TASK_STATUS.TIMEOUT]: '运行超时',
    [JUDGE_TASK_STATUS.FAILED]: '未通过', [JUDGE_TASK_STATUS.ERROR]: '判题失败',
    [JUDGE_TASK_STATUS.CANCELLED]: '已取消',
  }, '未知状态')
}
