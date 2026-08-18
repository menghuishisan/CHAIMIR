// sim 领域维护仿真包状态对应的界面语义色。

import type { StatusTone } from '@chaimir/ui'
import {
  SIM_PACKAGE_STATUS,
  SIM_REVIEW_RESULT,
  type SimPackageStatus,
  type SimReviewResult,
} from '@chaimir/api-client'

const PACKAGE_STATUS_TONES: Record<SimPackageStatus, StatusTone> = {
  [SIM_PACKAGE_STATUS.DRAFT]: 'neutral',
  [SIM_PACKAGE_STATUS.REVIEWING]: 'warning',
  [SIM_PACKAGE_STATUS.PUBLISHED]: 'primary',
  [SIM_PACKAGE_STATUS.ARCHIVED]: 'neutral',
  [SIM_PACKAGE_STATUS.REJECTED]: 'danger',
}

/** simPackageStatusTone 返回仿真场景状态语义色。 */
export function simPackageStatusTone(status: SimPackageStatus): StatusTone {
  return PACKAGE_STATUS_TONES[status]
}

const REVIEW_RESULT_TONES: Record<SimReviewResult, StatusTone> = {
  [SIM_REVIEW_RESULT.PENDING]: 'warning',
  [SIM_REVIEW_RESULT.APPROVED]: 'success',
  [SIM_REVIEW_RESULT.REJECTED]: 'danger',
}

/** simReviewResultTone 返回仿真包审核结论语义色。 */
export function simReviewResultTone(result: SimReviewResult): StatusTone {
  return REVIEW_RESULT_TONES[result]
}
