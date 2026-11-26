import { EXPERIMENT_VALIDATION_LEVEL, type ValidationIssue } from '@chaimir/api-client'
import { Badge } from '@chaimir/ui'

export function ExperimentValidationIssues({ issues }: { issues: ValidationIssue[] }) {
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
