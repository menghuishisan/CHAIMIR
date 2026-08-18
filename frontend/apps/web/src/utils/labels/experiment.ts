// experiment labels 文件维护 M7 实验模块枚举的用户向文案。

import {
  EXPERIMENT_STAGE_STATUS,
  ExperimentCollabMode,
  ExperimentInstanceStatus,
  ExperimentReportStatus,
  ExperimentStatus,
  type ExperimentStageStatus,
} from '@chaimir/api-client'

const EXPERIMENT_STATUS_LABELS: Record<ExperimentStatus, string> = {
  [ExperimentStatus.DRAFT]: '未发布',
  [ExperimentStatus.PUBLISHED]: '可开始',
  [ExperimentStatus.UNPUBLISHED]: '已下架',
}

/** experimentStatusLabel 返回实验发布状态文案。 */
export function experimentStatusLabel(status: ExperimentStatus): string {
  return EXPERIMENT_STATUS_LABELS[status]
}

const COLLAB_MODE_LABELS: Record<ExperimentCollabMode, string> = {
  [ExperimentCollabMode.SOLO]: '独立完成',
  [ExperimentCollabMode.GROUP]: '小组协作',
}

/** experimentCollabModeLabel 返回实验协作方式文案。 */
export function experimentCollabModeLabel(mode: ExperimentCollabMode): string {
  return COLLAB_MODE_LABELS[mode]
}

const INSTANCE_STATUS_LABELS: Record<ExperimentInstanceStatus, string> = {
  [ExperimentInstanceStatus.CREATING]: '环境准备中',
  [ExperimentInstanceStatus.RUNNING]: '进行中',
  [ExperimentInstanceStatus.PAUSED]: '已暂停',
  [ExperimentInstanceStatus.FINISHED]: '已完成',
  [ExperimentInstanceStatus.RECYCLED]: '环境已回收',
  [ExperimentInstanceStatus.ERROR]: '环境准备失败',
  [ExperimentInstanceStatus.RELEASED]: '环境已释放',
}

/** experimentInstanceStatusLabel 返回实验实例状态文案。 */
export function experimentInstanceStatusLabel(status: ExperimentInstanceStatus): string {
  return INSTANCE_STATUS_LABELS[status]
}

const STAGE_STATUS_LABELS: Record<ExperimentStageStatus, string> = {
  [EXPERIMENT_STAGE_STATUS.LOCKED]: '未解锁',
  [EXPERIMENT_STAGE_STATUS.AVAILABLE]: '可进入',
  [EXPERIMENT_STAGE_STATUS.ACTIVE]: '进行中',
}

/** experimentStageStatusLabel 返回实验阶段状态文案。 */
export function experimentStageStatusLabel(status: ExperimentStageStatus): string {
  return STAGE_STATUS_LABELS[status]
}

const REPORT_STATUS_LABELS: Record<ExperimentReportStatus, string> = {
  [ExperimentReportStatus.SUBMITTED]: '待批改',
  [ExperimentReportStatus.GRADED]: '已批改',
}

/** experimentReportStatusLabel 返回实验报告状态文案。 */
export function experimentReportStatusLabel(status: ExperimentReportStatus): string {
  return REPORT_STATUS_LABELS[status]
}
