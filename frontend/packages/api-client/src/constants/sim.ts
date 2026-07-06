// 仿真契约常量：维护 M4 用户向字符串枚举，来源为后端 sim 转换层。

export const SIM_COMPUTE = {
  FRONTEND: 'frontend',
  BACKEND: 'backend',
} as const

export type SimCompute = (typeof SIM_COMPUTE)[keyof typeof SIM_COMPUTE]

export const SIM_PACKAGE_STATUS = {
  DRAFT: 'draft',
  REVIEWING: 'reviewing',
  PUBLISHED: 'published',
  ARCHIVED: 'archived',
  REJECTED: 'rejected',
} as const

export type SimPackageStatus = (typeof SIM_PACKAGE_STATUS)[keyof typeof SIM_PACKAGE_STATUS]

export const SIM_REVIEW_RESULT = {
  PENDING: 'pending',
  APPROVED: 'approved',
  REJECTED: 'rejected',
} as const

export type SimReviewResult = (typeof SIM_REVIEW_RESULT)[keyof typeof SIM_REVIEW_RESULT]

export const SIM_SHARE_STATUS = {
  ACTIVE: 'active',
  REVOKED: 'revoked',
  EXPIRED: 'expired',
} as const

export type SimShareStatus = (typeof SIM_SHARE_STATUS)[keyof typeof SIM_SHARE_STATUS]
