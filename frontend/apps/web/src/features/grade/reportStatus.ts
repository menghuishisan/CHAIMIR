// reportStatus.ts 成绩报送页的课程报送状态(设计 §7.4 成绩报送):
// 每门课以最近一次报送为准;默认选中第一门未报送(或被驳回待重报)的课;待审核、已通过时不能再报送。
// 规则与后端一致:待审核不接受重复报送,通过即锁定,需学校解锁后才能改分重报。
// 纯函数,供 node:test 直接运行:不使用 enum,不在运行时导入 @chaimir/*。

/** 报送状态取值,与 @chaimir/api-client 的 GradeReviewStatus 一致 */
const STATUS_PENDING = 1
const STATUS_APPROVED = 2
const STATUS_REJECTED = 3

/** 计算用到的报送记录字段 */
export interface ReportRecord {
  course_id: string
  status: number
  submitted_at: string
  comment?: string
}

/** courseReportStates 每门课最近一次报送(按报送时间);没有报送过的课不在结果里 */
export function courseReportStates<T extends ReportRecord>(reviews: readonly T[]): Map<string, T> {
  const latest = new Map<string, T>()
  for (const review of reviews) {
    const current = latest.get(review.course_id)
    if (!current || review.submitted_at > current.submitted_at) latest.set(review.course_id, review)
  }
  return latest
}

/** defaultReportCourse 默认选中的课:第一门未报送或被驳回的;都报过了就是第一门 */
export function defaultReportCourse(
  courseIds: readonly string[],
  states: ReadonlyMap<string, ReportRecord>,
): string | undefined {
  const open = courseIds.find((id) => {
    const state = states.get(id)
    return !state || state.status === STATUS_REJECTED
  })
  return open ?? courseIds[0]
}

/** reportBlockedReason 不能报送的原因(用户向);能报送时返回 undefined */
export function reportBlockedReason(state: ReportRecord | undefined): string | undefined {
  if (state?.status === STATUS_PENDING) return '已报送,正在等待学校审核'
  if (state?.status === STATUS_APPROVED) return '学校已审核通过,成绩已锁定;需要改分请先联系学校解锁'
  return undefined
}
