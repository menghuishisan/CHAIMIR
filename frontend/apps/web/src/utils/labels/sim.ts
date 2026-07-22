// sim labels 文件维护仿真包生命周期、计算方式和审核结果文案。

import { SIM_COMPUTE, SIM_PACKAGE_STATUS, SIM_REVIEW_RESULT, type SimCompute, type SimPackageStatus, type SimReviewResult } from '@chaimir/api-client'
import { labelFromMap } from './map'

/** simPackageStatusLabel 返回仿真包生命周期状态文案。 */
export function simPackageStatusLabel(status: SimPackageStatus): string {
  return labelFromMap(status, {
    [SIM_PACKAGE_STATUS.DRAFT]: '草稿', [SIM_PACKAGE_STATUS.REVIEWING]: '审核中',
    [SIM_PACKAGE_STATUS.PUBLISHED]: '已上架', [SIM_PACKAGE_STATUS.ARCHIVED]: '已下架',
    [SIM_PACKAGE_STATUS.REJECTED]: '已退回',
  }, '未知状态')
}

/** simComputeLabel 返回仿真包运行方式文案。 */
export function simComputeLabel(compute: SimCompute): string {
  return labelFromMap(compute, { [SIM_COMPUTE.FRONTEND]: '前端仿真', [SIM_COMPUTE.BACKEND]: '云端仿真' }, '未识别的运行方式')
}

/** simReviewResultLabel 返回仿真包审核结果文案。 */
export function simReviewResultLabel(result: SimReviewResult): string {
  return labelFromMap(result, {
    [SIM_REVIEW_RESULT.PENDING]: '待审核', [SIM_REVIEW_RESULT.APPROVED]: '已通过',
    [SIM_REVIEW_RESULT.REJECTED]: '已退回',
  }, '未识别的审核结果')
}
