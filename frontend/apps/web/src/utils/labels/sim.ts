// sim labels 文件维护 M4 仿真模块枚举的用户向文案。

import {
  SIM_COMPUTE,
  SIM_PACKAGE_STATUS,
  SIM_REVIEW_RESULT,
  type SimCompute,
  type SimPackageStatus,
  type SimReviewResult,
} from '@chaimir/api-client'

const COMPUTE_LABELS: Record<SimCompute, string> = {
  [SIM_COMPUTE.BROWSER]: '本机推演',
  [SIM_COMPUTE.ISOLATED]: '服务端推演',
}

/** simComputeLabel 返回仿真运行位置文案。 */
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

/** simPackageStatusLabel 返回仿真场景状态文案。 */
export function simPackageStatusLabel(status: SimPackageStatus): string {
  return PACKAGE_STATUS_LABELS[status]
}

const REVIEW_RESULT_LABELS: Record<SimReviewResult, string> = {
  [SIM_REVIEW_RESULT.PENDING]: '待审核',
  [SIM_REVIEW_RESULT.APPROVED]: '已通过',
  [SIM_REVIEW_RESULT.REJECTED]: '已退回',
}

/** simReviewResultLabel 返回仿真包审核结论文案。 */
export function simReviewResultLabel(result: SimReviewResult): string {
  return REVIEW_RESULT_LABELS[result]
}

/**
 * 仿真场景分类的用户向名称。键必须与 `packages/sim-sdk/src/types.ts` 的 `SimCategory`
 * 七个取值逐字对齐 —— 之前这里写的是 crypto/contract/ledger 三个不存在的键,导致 41 个内置包里
 * 有 29 个把 `contract-security`、`transaction-runtime` 这类内部取值直接显示给学生(违反 FE-4)。
 * 扩展包(教师/第三方)可自带分类字符串,未登记时按原值显示:那是包作者写的业务分类名,不是内部标识。
 */
const CATEGORY_LABELS: Record<string, string> = {
  consensus: '共识机制',
  cryptography: '密码学',
  network: '网络与传播',
  'data-structure': '账本与数据结构',
  'contract-security': '合约安全',
  'transaction-runtime': '交易与执行',
  'cross-chain-system': '跨链与扩容',
}

/** simCategoryLabel 返回仿真分类文案。 */
export function simCategoryLabel(category: string): string {
  return CATEGORY_LABELS[category] ?? category
}
