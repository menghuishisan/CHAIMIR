/** 执行位置由后端按包的作者类型确定。内置包在浏览器执行，扩展包在隔离环境执行。 */
export const SIM_COMPUTE = {
  BROWSER: 'browser',
  ISOLATED: 'isolated',
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

/** 仿真包自动校验子项的封闭结果值。 */
export const SIM_VALIDATION_STATUS = {
  PASSED: 'passed',
  FAILED: 'failed',
  /** 仅静态扫描可产生，保留给平台人工判断。 */
  WARNING: 'warning',
} as const

export type SimValidationStatusValue =
  (typeof SIM_VALIDATION_STATUS)[keyof typeof SIM_VALIDATION_STATUS]

export const SIM_STREAM_COMMAND = {
  STEP: 'step',
  EVENT: 'event',
  BACK: 'back',
  RESTART: 'restart',
} as const

export type SimStreamCommand = (typeof SIM_STREAM_COMMAND)[keyof typeof SIM_STREAM_COMMAND]

/** 首帧带包自描述信息，之后只发送教学快照。 */
export const SIM_STREAM_FRAME = {
  READY: 'ready',
  SNAPSHOT: 'snapshot',
} as const

export type SimStreamFrame = (typeof SIM_STREAM_FRAME)[keyof typeof SIM_STREAM_FRAME]
