// 实验契约常量：维护前端需要与后端 experiment 模块枚举编号对齐的值。

/**
 * 实验协作模式与后端 experiment 模块保持一致。
 */
export enum ExperimentCollabMode {
  SOLO = 1,
  GROUP = 2,
}

/**
 * 实验发布状态与后端 experiment 模块保持一致。
 */
export enum ExperimentStatus {
  DRAFT = 1,
  PUBLISHED = 2,
  UNPUBLISHED = 3,
}

/**
 * 实验实例运行状态与后端 experiment 模块保持一致。
 */
export enum ExperimentInstanceStatus {
  CREATING = 1,
  RUNNING = 2,
  PAUSED = 3,
  FINISHED = 4,
  RECYCLED = 5,
  ERROR = 6,
  RELEASED = 7,
}

/**
 * 实验报告状态与后端 experiment 模块保持一致。
 */
export enum ExperimentReportStatus {
  SUBMITTED = 1,
  GRADED = 2,
}

/**
 * 实验阶段状态与后端 experiment service_stage.go 输出保持一致。
 */
export const EXPERIMENT_STAGE_STATUS = {
  LOCKED: 'locked',
  AVAILABLE: 'available',
  ACTIVE: 'active',
} as const

export type ExperimentStageStatus = (typeof EXPERIMENT_STAGE_STATUS)[keyof typeof EXPERIMENT_STAGE_STATUS]
