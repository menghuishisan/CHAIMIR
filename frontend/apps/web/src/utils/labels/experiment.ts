// experiment labels 文件维护 M7 实验模块枚举的用户向文案与语义色。

import type { StatusTone } from '@chaimir/ui'
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

const EXPERIMENT_STATUS_TONES: Record<ExperimentStatus, StatusTone> = {
  [ExperimentStatus.DRAFT]: 'neutral',
  [ExperimentStatus.PUBLISHED]: 'primary',
  [ExperimentStatus.UNPUBLISHED]: 'neutral',
}

/** experimentStatusLabel 返回实验发布状态文案。 */
export function experimentStatusLabel(status: ExperimentStatus): string {
  return EXPERIMENT_STATUS_LABELS[status]
}

/** experimentStatusTone 返回实验发布状态语义色。 */
export function experimentStatusTone(status: ExperimentStatus): StatusTone {
  return EXPERIMENT_STATUS_TONES[status]
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

const INSTANCE_STATUS_TONES: Record<ExperimentInstanceStatus, StatusTone> = {
  [ExperimentInstanceStatus.CREATING]: 'info',
  [ExperimentInstanceStatus.RUNNING]: 'primary',
  [ExperimentInstanceStatus.PAUSED]: 'warning',
  [ExperimentInstanceStatus.FINISHED]: 'success',
  [ExperimentInstanceStatus.RECYCLED]: 'neutral',
  [ExperimentInstanceStatus.ERROR]: 'danger',
  [ExperimentInstanceStatus.RELEASED]: 'neutral',
}

/** experimentInstanceStatusLabel 返回实验实例状态文案。 */
export function experimentInstanceStatusLabel(status: ExperimentInstanceStatus): string {
  return INSTANCE_STATUS_LABELS[status]
}

/** experimentInstanceStatusTone 返回实验实例状态语义色。 */
export function experimentInstanceStatusTone(status: ExperimentInstanceStatus): StatusTone {
  return INSTANCE_STATUS_TONES[status]
}

/** 实例仍属于当前实验、可复用或等待恢复的状态,创建入口按此判定。 */
const ACTIVE_INSTANCE_STATUSES: ReadonlySet<ExperimentInstanceStatus> = new Set([
  ExperimentInstanceStatus.CREATING,
  ExperimentInstanceStatus.RUNNING,
  ExperimentInstanceStatus.PAUSED,
  ExperimentInstanceStatus.RELEASED,
])

/** isExperimentInstanceActive 判断实例是否仍属于当前实验的复用边界。 */
export function isExperimentInstanceActive(status: ExperimentInstanceStatus): boolean {
  return ACTIVE_INSTANCE_STATUSES.has(status)
}

const STAGE_STATUS_LABELS: Record<ExperimentStageStatus, string> = {
  [EXPERIMENT_STAGE_STATUS.LOCKED]: '未解锁',
  [EXPERIMENT_STAGE_STATUS.AVAILABLE]: '可进入',
  [EXPERIMENT_STAGE_STATUS.ACTIVE]: '进行中',
}

const STAGE_STATUS_TONES: Record<ExperimentStageStatus, StatusTone> = {
  [EXPERIMENT_STAGE_STATUS.LOCKED]: 'neutral',
  [EXPERIMENT_STAGE_STATUS.AVAILABLE]: 'info',
  [EXPERIMENT_STAGE_STATUS.ACTIVE]: 'primary',
}

/** experimentStageStatusLabel 返回实验阶段状态文案。 */
export function experimentStageStatusLabel(status: ExperimentStageStatus): string {
  return STAGE_STATUS_LABELS[status]
}

/** experimentStageStatusTone 返回实验阶段状态语义色。 */
export function experimentStageStatusTone(status: ExperimentStageStatus): StatusTone {
  return STAGE_STATUS_TONES[status]
}

const REPORT_STATUS_LABELS: Record<ExperimentReportStatus, string> = {
  [ExperimentReportStatus.SUBMITTED]: '待批改',
  [ExperimentReportStatus.GRADED]: '已批改',
}

const REPORT_STATUS_TONES: Record<ExperimentReportStatus, StatusTone> = {
  [ExperimentReportStatus.SUBMITTED]: 'warning',
  [ExperimentReportStatus.GRADED]: 'success',
}

/** experimentReportStatusLabel 返回实验报告状态文案。 */
export function experimentReportStatusLabel(status: ExperimentReportStatus): string {
  return REPORT_STATUS_LABELS[status]
}

/** experimentReportStatusTone 返回实验报告状态语义色。 */
export function experimentReportStatusTone(status: ExperimentReportStatus): StatusTone {
  return REPORT_STATUS_TONES[status]
}
