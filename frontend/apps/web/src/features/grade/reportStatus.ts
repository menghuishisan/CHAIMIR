import { GradeReviewStatus } from '@chaimir/api-client'

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
    return !state || state.status === GradeReviewStatus.REJECTED
  })
  return open ?? courseIds[0]
}

/** reportBlockedReason 不能报送的原因(用户向);能报送时返回 undefined */
export function reportBlockedReason(state: ReportRecord | undefined): string | undefined {
  if (state?.status === GradeReviewStatus.PENDING) return '已报送,正在等待学校审核'
  if (state?.status === GradeReviewStatus.APPROVED) return '学校已审核通过,成绩已锁定;需要改分请先联系学校解锁'
  return undefined
}
