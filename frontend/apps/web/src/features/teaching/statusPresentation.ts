// teaching statusPresentation 文件维护 M6 状态到设计系统语义色的映射。

import type { BadgeTone, StatusTone } from '@chaimir/ui'
import { CourseStatus, ProgressStatus, SubmissionStatus } from '@chaimir/api-client'

const COURSE_STATUS_TONES: Record<CourseStatus, StatusTone> = {
  [CourseStatus.DRAFT]: 'neutral',
  [CourseStatus.PUBLISHED]: 'info',
  [CourseStatus.RUNNING]: 'primary',
  [CourseStatus.ENDED]: 'success',
  [CourseStatus.ARCHIVED]: 'neutral',
}

/** courseStatusTone 返回课程状态语义色。 */
export function courseStatusTone(status: CourseStatus): StatusTone {
  return COURSE_STATUS_TONES[status]
}

const PROGRESS_STATUS_TONES: Record<ProgressStatus, StatusTone> = {
  [ProgressStatus.NOT_STARTED]: 'neutral',
  [ProgressStatus.IN_PROGRESS]: 'info',
  [ProgressStatus.DONE]: 'success',
}

/** progressStatusTone 返回课时学习状态语义色。 */
export function progressStatusTone(status: ProgressStatus): StatusTone {
  return PROGRESS_STATUS_TONES[status]
}

const SUBMISSION_STATUS_TONES: Record<SubmissionStatus, StatusTone> = {
  [SubmissionStatus.SUBMITTED]: 'info',
  [SubmissionStatus.PENDING]: 'warning',
  [SubmissionStatus.GRADED]: 'success',
}

/** submissionStatusTone 返回提交状态语义色。 */
export function submissionStatusTone(status: SubmissionStatus): StatusTone {
  return SUBMISSION_STATUS_TONES[status]
}

/** ASSIGNMENT_DUE_TONE 是作业截止时间临近程度对应的徽标色。 */
export const ASSIGNMENT_DUE_TONE: Record<'overdue' | 'urgent' | 'normal', BadgeTone> = {
  overdue: 'danger',
  urgent: 'warning',
  normal: 'neutral',
}
