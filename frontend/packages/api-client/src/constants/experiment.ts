// 实验契约常量：维护前端需要与后端 experiment 模块枚举编号对齐的值。

/**
 * 实验发布状态与后端 experiment 模块保持一致。
 */
export enum ExperimentStatus {
  DRAFT = 1,
  PUBLISHED = 2,
  UNPUBLISHED = 3,
}
