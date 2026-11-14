// submissionWindow.ts 判断学生还能不能提交这份作业(设计 §7.2 作业作答 / 作业结果):
// 作答页据此决定可编辑还是只读,结果页据此决定显示「再交一次」还是一句原因。
// 规则与服务端提交校验同口径(teaching/service_assignment.go SubmitAssignment):
// 已用次数达到上限不接收;截止后只有「不接收」策略拒收,其余策略按迟交处理。
// 前端判断只用于呈现,最终以服务端为准。纯函数,供 node:test 直接运行。

/** 迟交策略「截止后不接收」,与 @chaimir/api-client 的 LatePolicy.REJECT 一致(本文件不在运行时导入它) */
const LATE_POLICY_REJECT = 1

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
  if (late && input.latePolicy === LATE_POLICY_REJECT) {
    return { open: false, late, remaining, reason: '已过截止时间,这份作业不再接收提交' }
  }
  return { open: true, late, remaining, reason: undefined }
}
