// ExperimentTableCells 统一教师实验列表里的实验状态展示。

import type { Experiment, StudentExperiment } from '@chaimir/api-client'
import { StatusIndicator } from '@chaimir/ui'
import { experimentStatusLabel } from '../../../utils/labels/experiment'
import { experimentStatusTone } from '../statusPresentation'

type ExperimentListItem = Experiment | StudentExperiment

export interface ExperimentTableCellProps {
  experiment: ExperimentListItem
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
