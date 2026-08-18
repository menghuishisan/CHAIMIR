// ContestTableCells 统一学生与教师赛事列表里的赛事身份和赛程展示。

import type { Contest } from '@chaimir/api-client'
import { formatDateTime } from '../../../utils/formatters'
import { contestModeLabel, teamModeLabel } from '../../../utils/labels/contest'

export interface ContestTableCellProps {
  contest: Contest
}

/** ContestIdentityCell 展示赛事名称、赛制与组队方式。 */
export function ContestIdentityCell({ contest }: ContestTableCellProps) {
  return (
    <div className="min-w-0">
      <div className="truncate font-medium text-ink">{contest.name}</div>
      <div className="truncate text-xs text-ink-sub">
        {contestModeLabel(contest.mode)} · {teamModeLabel(contest.team_mode)}
      </div>
    </div>
  )
}

/** ContestScheduleCell 统一展示赛事开始与结束时间。 */
export function ContestScheduleCell({ contest }: ContestTableCellProps) {
  return (
    <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
      {formatDateTime(contest.start_at)} — {formatDateTime(contest.end_at)}
    </span>
  )
}
