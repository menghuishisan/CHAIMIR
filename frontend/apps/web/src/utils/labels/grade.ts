// grade labels 文件维护成绩审核、申诉和预警领域文案。

import { GradeAppealStatus, GradeReviewStatus, GradeWarningStatus, GradeWarningType } from '@chaimir/api-client'
import { labelFromMap } from './map'

/** gradeReviewStatusLabel 返回成绩审核状态文案。 */
export function gradeReviewStatusLabel(status: GradeReviewStatus): string {
  return labelFromMap(status, {
    [GradeReviewStatus.PENDING]: '待审核', [GradeReviewStatus.APPROVED]: '已通过',
    [GradeReviewStatus.REJECTED]: '已驳回',
  }, '未知')
}

/** gradeAppealStatusLabel 返回成绩申诉状态文案。 */
export function gradeAppealStatusLabel(status: GradeAppealStatus): string {
  return labelFromMap(status, {
    [GradeAppealStatus.PENDING]: '待处理', [GradeAppealStatus.ACCEPTED]: '已受理',
    [GradeAppealStatus.COMPLETED]: '已完成', [GradeAppealStatus.REJECTED]: '已驳回',
  }, '未知')
}

/** gradeWarningTypeLabel 返回学业预警类型文案。 */
export function gradeWarningTypeLabel(type: GradeWarningType): string {
  return labelFromMap(type, { [GradeWarningType.FAILED_COURSE]: '课程未通过', [GradeWarningType.LOW_GPA]: '绩点偏低' }, '学业预警')
}

/** gradeWarningStatusLabel 返回学业预警确认状态文案。 */
export function gradeWarningStatusLabel(status: GradeWarningStatus): string {
  return labelFromMap(status, { [GradeWarningStatus.PENDING]: '待确认', [GradeWarningStatus.ACKNOWLEDGED]: '已确认' }, '未知')
}

/** gradeWarningDetailLabel 将预警详情对象转换为用户向说明。 */
export function gradeWarningDetailLabel(detail?: Record<string, unknown>): string {
  if (!detail || Object.keys(detail).length === 0) return '已记录预警触发条件'
  const labels: Record<string, string> = {
    fail_count: '未通过课程数', min_gpa: '最低绩点要求', gpa: '当前绩点', course_id: '课程编号',
  }
  return Object.entries(detail).map(([key, value]) => `${labels[key] || '触发参数'}：${String(value)}`).join('，')
}
