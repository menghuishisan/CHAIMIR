// sim labels 文件维护 M4 仿真模块枚举的用户向文案与语义色。

import type { StatusTone } from '@chaimir/ui'
import {
  SIM_COMPUTE,
  SIM_PACKAGE_STATUS,
  SIM_REVIEW_RESULT,
  SIM_SHARE_STATUS,
  type SimCompute,
  type SimPackageStatus,
  type SimReviewResult,
  type SimShareStatus,
} from '@chaimir/api-client'

const COMPUTE_LABELS: Record<SimCompute, string> = {
  [SIM_COMPUTE.FRONTEND]: '本机推演',
  [SIM_COMPUTE.BACKEND]: '服务端计算',
}

/** simComputeLabel 返回仿真运行方式文案。 */
export function simComputeLabel(compute: SimCompute): string {
  return COMPUTE_LABELS[compute]
}

const PACKAGE_STATUS_LABELS: Record<SimPackageStatus, string> = {
  [SIM_PACKAGE_STATUS.DRAFT]: '草稿',
  [SIM_PACKAGE_STATUS.REVIEWING]: '审核中',
  [SIM_PACKAGE_STATUS.PUBLISHED]: '可使用',
  [SIM_PACKAGE_STATUS.ARCHIVED]: '已下架',
  [SIM_PACKAGE_STATUS.REJECTED]: '未通过审核',
}

const PACKAGE_STATUS_TONES: Record<SimPackageStatus, StatusTone> = {
  [SIM_PACKAGE_STATUS.DRAFT]: 'neutral',
  [SIM_PACKAGE_STATUS.REVIEWING]: 'warning',
  [SIM_PACKAGE_STATUS.PUBLISHED]: 'primary',
  [SIM_PACKAGE_STATUS.ARCHIVED]: 'neutral',
  [SIM_PACKAGE_STATUS.REJECTED]: 'danger',
}

/** simPackageStatusLabel 返回仿真场景状态文案。 */
export function simPackageStatusLabel(status: SimPackageStatus): string {
  return PACKAGE_STATUS_LABELS[status]
}

/** simPackageStatusTone 返回仿真场景状态语义色。 */
export function simPackageStatusTone(status: SimPackageStatus): StatusTone {
  return PACKAGE_STATUS_TONES[status]
}

const REVIEW_RESULT_LABELS: Record<SimReviewResult, string> = {
  [SIM_REVIEW_RESULT.PENDING]: '待审核',
  [SIM_REVIEW_RESULT.APPROVED]: '已通过',
  [SIM_REVIEW_RESULT.REJECTED]: '已退回',
}

const REVIEW_RESULT_TONES: Record<SimReviewResult, StatusTone> = {
  [SIM_REVIEW_RESULT.PENDING]: 'warning',
  [SIM_REVIEW_RESULT.APPROVED]: 'success',
  [SIM_REVIEW_RESULT.REJECTED]: 'danger',
}

/** simReviewResultLabel 返回仿真包审核结论文案。 */
export function simReviewResultLabel(result: SimReviewResult): string {
  return REVIEW_RESULT_LABELS[result]
}

/** simReviewResultTone 返回仿真包审核结论语义色。 */
export function simReviewResultTone(result: SimReviewResult): StatusTone {
  return REVIEW_RESULT_TONES[result]
}

const SHARE_STATUS_LABELS: Record<SimShareStatus, string> = {
  [SIM_SHARE_STATUS.ACTIVE]: '分享有效',
  [SIM_SHARE_STATUS.REVOKED]: '分享已撤销',
  [SIM_SHARE_STATUS.EXPIRED]: '分享已过期',
}

/** simShareStatusLabel 返回分享状态文案。 */
export function simShareStatusLabel(status: SimShareStatus): string {
  return SHARE_STATUS_LABELS[status]
}

/**
 * 仿真场景分类的用户向名称。category 是后端开放字符串(仿真包元数据自带),
 * 未登记的分类直接显示原值 —— 它是包作者写的业务分类名,不是内部标识。
 */
const CATEGORY_LABELS: Record<string, string> = {
  consensus: '共识机制',
  crypto: '密码学',
  network: '网络与传播',
  contract: '智能合约',
  ledger: '账本结构',
}

/** simCategoryLabel 返回仿真分类文案。 */
export function simCategoryLabel(category: string): string {
  return CATEGORY_LABELS[category] ?? category
}
