export enum ExperimentCollabMode {
  SOLO = 1,
  GROUP = 2,
}

export enum ExperimentStatus {
  DRAFT = 1,
  PUBLISHED = 2,
  UNPUBLISHED = 3,
}

export enum ExperimentInstanceStatus {
  CREATING = 1,
  RUNNING = 2,
  PAUSED = 3,
  FINISHED = 4,
  RECYCLED = 5,
  ERROR = 6,
  // 底层沙箱已回收，继续实验时需重建。
  RELEASED = 7,
}

export enum ExperimentReportStatus {
  SUBMITTED = 1,
  GRADED = 2,
}

/** 实验学生报告读取边界的错误码。 */
export const EXPERIMENT_REPORT_ERROR_CODES = {
  NOT_FOUND: '74004',
} as const

/** 实验实例访问边界的错误码。 */
export const EXPERIMENT_INSTANCE_ERROR_CODES = {
  ACCESS_DENIED: '72004',
} as const

export const EXPERIMENT_STAGE_STATUS = {
  LOCKED: 'locked',
  AVAILABLE: 'available',
  ACTIVE: 'active',
} as const

export type ExperimentStageStatus =
  (typeof EXPERIMENT_STAGE_STATUS)[keyof typeof EXPERIMENT_STAGE_STATUS]

export const EXPERIMENT_VALIDATION_LEVEL = {
  ERROR: 'error',
  WARNING: 'warning',
} as const

export type ExperimentValidationLevel =
  (typeof EXPERIMENT_VALIDATION_LEVEL)[keyof typeof EXPERIMENT_VALIDATION_LEVEL]
