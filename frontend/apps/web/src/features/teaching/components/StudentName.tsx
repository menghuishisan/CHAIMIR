// StudentName 按成员解析结果显示学生:姓名 + 学号;读取中显示占位,已退课、读取失败各有说法,不显示内部编号。

import { Skeleton } from '@chaimir/ui'
import type { MemberName } from '../useCourseMemberNames'

/** StudentName 一个学生的显示;compact 时学号与姓名在同一行 */
export function StudentName({ member, compact = false }: { member: MemberName; compact?: boolean }) {
  if (member.state === 'loading') return <Skeleton variant="line" width="half" />
  if (member.state === 'failed') return <span className="text-ink-sub">姓名暂时读不到</span>
  if (member.state === 'left') return <span className="text-ink-sub">已退课学生</span>
  return compact ? (
    <span className="min-w-0 truncate">
      <span className="font-medium text-ink">{member.name}</span>
      {member.no ? <span className="ml-2 font-mono text-xs text-ink-sub">{member.no}</span> : null}
    </span>
  ) : (
    <span className="min-w-0">
      <span className="block truncate font-medium text-ink">{member.name}</span>
      {member.no ? <span className="block font-mono text-xs text-ink-sub">{member.no}</span> : null}
    </span>
  )
}
