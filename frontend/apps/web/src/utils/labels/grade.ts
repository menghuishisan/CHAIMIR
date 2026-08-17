// grade labels 文件维护 M11 成绩中心枚举的用户向文案与语义色。

import type { StatusTone } from '@chaimir/ui'
import {
  GradeAppealStatus,
  GradeReviewStatus,
  GradeWarningStatus,
  GradeWarningType,
  TranscriptScope,
} from '@chaimir/api-client'

const REVIEW_STATUS_LABELS: Record<GradeReviewStatus, string> = {
  [GradeReviewStatus.PENDING]: '待审核',
  [GradeReviewStatus.APPROVED]: '已通过',
  [GradeReviewStatus.REJECTED]: '已驳回',
}

const REVIEW_STATUS_TONES: Record<GradeReviewStatus, StatusTone> = {
  [GradeReviewStatus.PENDING]: 'warning',
  [GradeReviewStatus.APPROVED]: 'success',
  [GradeReviewStatus.REJECTED]: 'danger',
}

/** gradeReviewStatusLabel 返回成绩审核状态文案。 */
export function gradeReviewStatusLabel(status: GradeReviewStatus): string {
  return REVIEW_STATUS_LABELS[status]
}

/** gradeReviewStatusTone 返回成绩审核状态语义色。 */
export function gradeReviewStatusTone(status: GradeReviewStatus): StatusTone {
  return REVIEW_STATUS_TONES[status]
}

const APPEAL_STATUS_LABELS: Record<GradeAppealStatus, string> = {
  [GradeAppealStatus.PENDING]: '待处理',
  [GradeAppealStatus.ACCEPTED]: '已受理',
  [GradeAppealStatus.COMPLETED]: '已完成',
  [GradeAppealStatus.REJECTED]: '未通过',
}

const APPEAL_STATUS_TONES: Record<GradeAppealStatus, StatusTone> = {
  [GradeAppealStatus.PENDING]: 'warning',
  [GradeAppealStatus.ACCEPTED]: 'info',
  [GradeAppealStatus.COMPLETED]: 'success',
  [GradeAppealStatus.REJECTED]: 'danger',
}

/** gradeAppealStatusLabel 返回成绩申诉状态文案(提交成功后的就近反馈用)。 */
export function gradeAppealStatusLabel(status: GradeAppealStatus): string {
  return APPEAL_STATUS_LABELS[status]
}

/** gradeAppealStatusTone 返回成绩申诉状态语义色。 */
export function gradeAppealStatusTone(status: GradeAppealStatus): StatusTone {
  return APPEAL_STATUS_TONES[status]
}

const WARNING_TYPE_LABELS: Record<GradeWarningType, string> = {
  [GradeWarningType.FAILED_COURSE]: '课程不及格',
  [GradeWarningType.LOW_GPA]: '平均学分绩点偏低',
}

/** gradeWarningTypeLabel 返回学业预警类型文案。 */
export function gradeWarningTypeLabel(type: GradeWarningType): string {
  return WARNING_TYPE_LABELS[type]
}

const WARNING_STATUS_LABELS: Record<GradeWarningStatus, string> = {
  [GradeWarningStatus.PENDING]: '待确认',
  [GradeWarningStatus.ACKNOWLEDGED]: '已确认',
  // 后端语义是「预警通知事件发布失败」,对使用者只有一件事要知道:学生没收到提醒,需要另行联系
  [GradeWarningStatus.NOTIFY_FAILED]: '提醒未送达',
}

const WARNING_STATUS_TONES: Record<GradeWarningStatus, StatusTone> = {
  [GradeWarningStatus.PENDING]: 'warning',
  [GradeWarningStatus.ACKNOWLEDGED]: 'success',
  [GradeWarningStatus.NOTIFY_FAILED]: 'danger',
}

/** gradeWarningStatusLabel 返回学业预警处理状态文案。 */
export function gradeWarningStatusLabel(status: GradeWarningStatus): string {
  return WARNING_STATUS_LABELS[status]
}

/** gradeWarningStatusTone 返回学业预警处理状态语义色。 */
export function gradeWarningStatusTone(status: GradeWarningStatus): StatusTone {
  return WARNING_STATUS_TONES[status]
}

const TRANSCRIPT_SCOPE_LABELS: Record<TranscriptScope, string> = {
  [TranscriptScope.SEMESTER]: '单学期成绩单',
  [TranscriptScope.FULL]: '全部学期成绩单',
}

/** transcriptScopeLabel 返回成绩单范围文案。 */
export function transcriptScopeLabel(scope: TranscriptScope): string {
  return TRANSCRIPT_SCOPE_LABELS[scope]
}

/**
 * 预警明细的 detail 是后端按预警类型写入的开放对象。
 * 页面只呈现已登记的键,未登记键不猜测语义、不把内部键名抛到界面上。
 */
const WARNING_DETAIL_TERMS: Record<string, string> = {
  gpa: '平均学分绩点',
  fail_count: '不及格课程数',
  min_gpa: '预警阈值',
  semester: '学期',
}

/** gradeWarningDetailTerm 返回预警明细字段的用户向名称,未登记键返回 undefined。 */
export function gradeWarningDetailTerm(key: string): string | undefined {
  return WARNING_DETAIL_TERMS[key]
}
