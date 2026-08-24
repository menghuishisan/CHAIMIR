// experiment 领域维护实验状态对应的界面语义色。

import type { StatusTone } from '@chaimir/ui'
import {
  EXPERIMENT_STAGE_STATUS,
  ExperimentInstanceStatus,
  ExperimentReportStatus,
  ExperimentStatus,
  type ExperimentStageStatus,
} from '@chaimir/api-client'

const EXPERIMENT_STATUS_TONES: Record<ExperimentStatus, StatusTone> = {
  [ExperimentStatus.DRAFT]: 'neutral',
  [ExperimentStatus.PUBLISHED]: 'primary',
  [ExperimentStatus.UNPUBLISHED]: 'neutral',
}

/** experimentStatusTone 返回实验发布状态语义色。 */
export function experimentStatusTone(status: ExperimentStatus): StatusTone {
  return EXPERIMENT_STATUS_TONES[status]
}

const STAGE_STATUS_TONES: Record<ExperimentStageStatus, StatusTone> = {
  [EXPERIMENT_STAGE_STATUS.LOCKED]: 'neutral',
  [EXPERIMENT_STAGE_STATUS.AVAILABLE]: 'info',
  [EXPERIMENT_STAGE_STATUS.ACTIVE]: 'primary',
}

/** experimentStageStatusTone 返回实验阶段状态语义色。 */
export function experimentStageStatusTone(status: ExperimentStageStatus): StatusTone {
  return STAGE_STATUS_TONES[status]
}

const REPORT_STATUS_TONES: Record<ExperimentReportStatus, StatusTone> = {
  [ExperimentReportStatus.SUBMITTED]: 'warning',
  [ExperimentReportStatus.GRADED]: 'success',
}

/** experimentReportStatusTone 返回实验报告状态语义色。 */
export function experimentReportStatusTone(status: ExperimentReportStatus): StatusTone {
  return REPORT_STATUS_TONES[status]
}

/**
 * 实例状态语义色:出错要红、暂停要黄、运行中走主色,已结束与已回收退成中性。
 * 「已释放」是资源已归还但记录仍在,同样退成中性 —— 它不需要注意力。
 */
const EXPERIMENT_INSTANCE_STATUS_TONES: Record<ExperimentInstanceStatus, StatusTone> = {
  [ExperimentInstanceStatus.CREATING]: 'info',
  [ExperimentInstanceStatus.RUNNING]: 'primary',
  [ExperimentInstanceStatus.PAUSED]: 'warning',
  [ExperimentInstanceStatus.FINISHED]: 'success',
  [ExperimentInstanceStatus.RECYCLED]: 'neutral',
  [ExperimentInstanceStatus.ERROR]: 'danger',
  [ExperimentInstanceStatus.RELEASED]: 'neutral',
}

/** experimentInstanceStatusTone 返回实验实例状态语义色。 */
export function experimentInstanceStatusTone(status: ExperimentInstanceStatus): StatusTone {
  return EXPERIMENT_INSTANCE_STATUS_TONES[status]
}
