// AccountNameText 按账号解析结果显示一个人:姓名 + 学工号;读取中显示占位,解析不到、读取失败各有说法,不显示内部编号。

import { Skeleton } from '@chaimir/ui'
import type { AccountName } from '../useAccountNames'

/** AccountNameText 一个账号的显示;compact 时学工号与姓名在同一行 */
export function AccountNameText({ name, compact = false }: { name: AccountName; compact?: boolean }) {
  if (name.state === 'loading') return <Skeleton variant="line" width="half" />
  if (name.state === 'failed') return <span className="text-ink-sub">姓名暂时读不到</span>
  if (name.state === 'missing') return <span className="text-ink-sub">已移除的账号</span>
  const { account } = name
  return compact ? (
    <span className="min-w-0 truncate">
      <span className="font-medium text-ink">{account.name}</span>
      {account.no ? <span className="ml-2 font-mono text-xs text-ink-sub">{account.no}</span> : null}
    </span>
  ) : (
    <span className="min-w-0">
      <span className="block truncate font-medium text-ink">{account.name}</span>
      {account.no ? <span className="block font-mono text-xs text-ink-sub">{account.no}</span> : null}
    </span>
  )
}
