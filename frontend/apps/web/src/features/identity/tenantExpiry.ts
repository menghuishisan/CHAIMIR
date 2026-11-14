// tenantExpiry.ts 学校服务到期的说法(设计 §7.6 学校管理:30 天内到期用警示色并显示剩余天数)。
// 后端以到期时刻判定能否登录(EnsureTenantCanLogin:当前时间晚于 expire_at 即拒绝),故「已过期」按时刻判断;
// 剩余天数按本地日历天数算,与界面上显示的到期日期一致。
// 纯函数,供 node:test 直接运行:不得使用 enum,也不得在运行时导入 @chaimir/*。

/** 30 天内到期需要提醒续期 */
const EXPIRY_WARNING_DAYS = 30

/** 学校服务到期的几种情形;days 为距今的日历天数(今天到期为 0) */
export type TenantExpiry =
  | { kind: 'none' }
  | { kind: 'unknown' }
  | { kind: 'expired' }
  | { kind: 'soon'; days: number }
  | { kind: 'normal'; days: number }

/** calendarDays 从 from 到 to 跨过的本地日历天数 */
function calendarDays(from: Date, to: Date): number {
  const start = new Date(from.getFullYear(), from.getMonth(), from.getDate()).getTime()
  const end = new Date(to.getFullYear(), to.getMonth(), to.getDate()).getTime()
  return Math.round((end - start) / 86_400_000)
}

/** tenantExpiry 判断学校服务到期的情形;now 只为测试注入 */
export function tenantExpiry(expireAt: string | undefined, now: Date = new Date()): TenantExpiry {
  if (!expireAt) return { kind: 'none' }
  const date = new Date(expireAt)
  if (Number.isNaN(date.getTime())) return { kind: 'unknown' }
  if (date.getTime() <= now.getTime()) return { kind: 'expired' }
  const days = calendarDays(now, date)
  return days <= EXPIRY_WARNING_DAYS ? { kind: 'soon', days } : { kind: 'normal', days }
}
