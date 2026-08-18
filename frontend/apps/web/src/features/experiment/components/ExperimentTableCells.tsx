// ExperimentTableCells 统一学生与教师实验列表里的实验身份和状态展示。

import type { Experiment, StudentExperiment } from '@chaimir/api-client'
import { StatusIndicator } from '@chaimir/ui'
import { experimentStatusLabel } from '../../../utils/labels/experiment'
import { experimentStatusTone } from '../statusPresentation'

type ExperimentListItem = Experiment | StudentExperiment

export interface ExperimentTableCellProps {
  experiment: ExperimentListItem
}

/** ExperimentIdentityCell 展示实验名称与说明。 */
export function ExperimentIdentityCell({ experiment }: ExperimentTableCellProps) {
  return (
    <div className="min-w-0">
      <div className="truncate font-medium text-ink">{experiment.name}</div>
      <div className="line-clamp-1 text-xs text-ink-sub">{experiment.description}</div>
    </div>
  )
}

/** ExperimentStatusCell 按统一口径展示实验状态。 */
export function ExperimentStatusCell({ experiment }: ExperimentTableCellProps) {
  return (
    <StatusIndicator
      tone={experimentStatusTone(experiment.status)}
      label={experimentStatusLabel(experiment.status)}
    />
  )
}
