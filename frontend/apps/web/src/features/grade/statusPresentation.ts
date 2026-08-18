// grade 领域维护成绩状态对应的界面语义色。

import type { StatusTone } from '@chaimir/ui'
import { GradeAppealStatus, GradeReviewStatus, GradeWarningStatus } from '@chaimir/api-client'

const REVIEW_STATUS_TONES: Record<GradeReviewStatus, StatusTone> = {
  [GradeReviewStatus.PENDING]: 'warning',
  [GradeReviewStatus.APPROVED]: 'success',
  [GradeReviewStatus.REJECTED]: 'danger',
}

/** gradeReviewStatusTone 返回成绩审核状态语义色。 */
export function gradeReviewStatusTone(status: GradeReviewStatus): StatusTone {
  return REVIEW_STATUS_TONES[status]
}

const APPEAL_STATUS_TONES: Record<GradeAppealStatus, StatusTone> = {
  [GradeAppealStatus.PENDING]: 'warning',
  [GradeAppealStatus.ACCEPTED]: 'info',
  [GradeAppealStatus.COMPLETED]: 'success',
  [GradeAppealStatus.REJECTED]: 'danger',
}

/** gradeAppealStatusTone 返回成绩申诉状态语义色。 */
export function gradeAppealStatusTone(status: GradeAppealStatus): StatusTone {
  return APPEAL_STATUS_TONES[status]
}

const WARNING_STATUS_TONES: Record<GradeWarningStatus, StatusTone> = {
  [GradeWarningStatus.PENDING]: 'warning',
  [GradeWarningStatus.ACKNOWLEDGED]: 'success',
  [GradeWarningStatus.NOTIFY_FAILED]: 'danger',
}

/** gradeWarningStatusTone 返回学业预警处理状态语义色。 */
export function gradeWarningStatusTone(status: GradeWarningStatus): StatusTone {
  return WARNING_STATUS_TONES[status]
}
