// ExperimentValidationIssues 统一实验发布前校验的阻断与提醒列表。

import { EXPERIMENT_VALIDATION_LEVEL, type ValidationIssue } from '@chaimir/api-client'
import { Badge } from '@chaimir/ui'

export interface ExperimentValidationIssuesProps {
  issues: ValidationIssue[]
}

/** ExperimentValidationIssues 按校验级别展示用户下一步需要处理的事项。 */
export function ExperimentValidationIssues({ issues }: ExperimentValidationIssuesProps) {
  if (issues.length === 0) return null
  return (
    <ul className="flex flex-col gap-2">
      {issues.map((issue, index) => (
        <li key={`${issue.level}-${issue.message}-${index}`} className="flex items-start gap-2 text-sm">
          <Badge tone={issue.level === EXPERIMENT_VALIDATION_LEVEL.ERROR ? 'danger' : 'warning'}>
            {issue.level === EXPERIMENT_VALIDATION_LEVEL.ERROR ? '必须修正' : '建议检查'}
          </Badge>
          <span className="min-w-0 text-ink">{issue.message}</span>
        </li>
      ))}
    </ul>
  )
}
