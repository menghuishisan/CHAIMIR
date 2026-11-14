// AccountNameText 显示列表记录里服务端解析好的一个人:姓名 + 学工号;解析不到(账号已移除)时写明,不显示内部编号。

/** AccountNameText 一个人的显示;compact 时学工号与姓名在同一行 */
export function AccountNameText({
  name,
  no,
  compact = false,
}: {
  /** 姓名;服务端解析不到时为空 */
  name?: string
  /** 学工号 */
  no?: string
  compact?: boolean
}) {
  if (!name) return <span className="text-ink-sub">已移除的账号</span>
  return compact ? (
    <span className="min-w-0 truncate">
      <span className="font-medium text-ink">{name}</span>
      {no ? <span className="ml-2 font-mono text-xs text-ink-sub">{no}</span> : null}
    </span>
  ) : (
    <span className="min-w-0">
      <span className="block truncate font-medium text-ink">{name}</span>
      {no ? <span className="block font-mono text-xs text-ink-sub">{no}</span> : null}
    </span>
  )
}
