import { LatePolicy } from '@chaimir/api-client'

// 前端只决定作答入口与提示；提交资格仍由服务端校验。

/** 判断用到的作业字段与已提交次数 */
export interface SubmissionWindowInput {
  dueAt: string
  latePolicy: number
  maxAttempts: number
  attemptsUsed: number
}

/** 判断结果 */
export interface SubmissionWindow {
  /** 还能不能提交 */
  open: boolean
  /** 现在提交是否算迟交 */
  late: boolean
  /** 剩余可提交次数 */
  remaining: number
  /** 不能提交的原因(用户向);能提交时为 undefined */
  reason: string | undefined
}

/** submissionWindow 给出当前时刻能否提交、是否迟交与剩余次数;now 只为测试注入 */
export function submissionWindow(
  input: SubmissionWindowInput,
  now: Date = new Date()
): SubmissionWindow {
  const remaining = Math.max(0, input.maxAttempts - input.attemptsUsed)
  const due = new Date(input.dueAt).getTime()
  // 截止时间读不出来时不在前端判迟交,交给服务端判断
  const late = Number.isFinite(due) && now.getTime() > due
  if (remaining === 0) {
    return { open: false, late, remaining, reason: `提交次数已用完(共 ${input.maxAttempts} 次)` }
  }
  if (late && input.latePolicy === LatePolicy.REJECT) {
    return { open: false, late, remaining, reason: '已过截止时间,这份作业不再接收提交' }
  }
  return { open: true, late, remaining, reason: undefined }
}
