import type { Experiment } from '@chaimir/api-client'
import { StatusIndicator } from '@chaimir/ui'
import { experimentStatusLabel } from '../../../utils/labels/experiment'
import { experimentStatusTone } from '../statusPresentation'

export function ExperimentStatusCell({ experiment }: { experiment: Pick<Experiment, 'status'> }) {
  return (
    <StatusIndicator
      tone={experimentStatusTone(experiment.status)}
      label={experimentStatusLabel(experiment.status)}
    />
  )
}
