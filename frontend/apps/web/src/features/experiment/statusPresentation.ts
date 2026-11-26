import type { StatusTone } from '@chaimir/ui'
import {
  ExperimentInstanceStatus,
  ExperimentReportStatus,
  ExperimentStatus,
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
