// 最近成功备份的相对时间与48小时过期提示。

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
