// backupFreshness.ts 「最近一次成功备份:N 小时前」的说法(设计 §7.6 备份记录:超过 48 小时未成功用警示色)。
// 备份只看「多久没有成功」,用小时比日历日更直接;超过 48 小时改用天数并要求警示。
// 纯函数,供 node:test 直接运行:不得使用 enum,也不得在运行时导入 @chaimir/*。

/** 超过这么久没有成功备份就要警示 */
const STALE_HOURS = 48

/** backupFreshness 最近一次成功备份离现在多久,以及是否需要警示;now 只为测试注入 */
export function backupFreshness(
  startedAt: string,
  now: Date = new Date()
): { text: string; stale: boolean } {
  const time = new Date(startedAt).getTime()
  if (Number.isNaN(time)) return { text: '时间未知', stale: true }
  const minutes = Math.max(0, Math.floor((now.getTime() - time) / 60_000))
  const hours = Math.floor(minutes / 60)
  if (hours >= STALE_HOURS) return { text: `${Math.floor(hours / 24)} 天前`, stale: true }
  if (hours >= 1) return { text: `${hours} 小时前`, stale: false }
  if (minutes >= 1) return { text: `${minutes} 分钟前`, stale: false }
  return { text: '刚刚', stale: false }
}
