// auditFilters.ts 把审计页地址里的筛选参数换成查询条件(设计 §7.7:对象类型、动作、操作人、时间范围都写进地址)。
// 地址是用户可以手改的外部输入:格式不对的值一律当作没有筛选,而不是拿着它去请求后端。
// 起止日期按本地时区取整天:从「开始那天 00:00」到「结束那天 23:59:59」;开始晚于结束时对调。
// 纯函数,供 node:test 直接运行:不得使用 enum,也不得在运行时导入 @chaimir/*。

const DAY_PATTERN = /^(\d{4})-(\d{2})-(\d{2})$/
/** 动作标识:点分的小写短词,如 auth.login;与 audit_log.action 的列宽一致,最长 64 */
const ACTION_PATTERN = /^[a-z_]+(?:\.[a-z_]+)+$/
const ACTION_MAX_LENGTH = 64
/** 账号编号:正的 64 位整数(雪花编号),十进制且不带前导零 */
const ACCOUNT_ID_PATTERN = /^[1-9]\d{0,18}$/
const ACCOUNT_ID_MAX = 9223372036854775807n

/** parseDay 解析 YYYY-MM-DD;不存在的日期(如 2 月 30 日)返回 undefined */
function parseDay(raw: string): { year: number; month: number; day: number } | undefined {
  const match = DAY_PATTERN.exec(raw.trim())
  if (!match) return undefined
  const [year, month, day] = [Number(match[1]), Number(match[2]), Number(match[3])]
  const date = new Date(year, month - 1, day)
  const exists =
    date.getFullYear() === year && date.getMonth() === month - 1 && date.getDate() === day
  return exists ? { year, month, day } : undefined
}

/** 一段审计时间范围:界面回显用的日期,与发给接口的时间参数(只含有效的一端) */
export interface AuditDayRange {
  from: string
  to: string
  params: { from?: string; to?: string }
}

/** auditDayRange 把地址里的起止日期换成查询时间;无效的一端忽略,开始晚于结束时对调 */
export function auditDayRange(rawFrom: string, rawTo: string): AuditDayRange {
  let start = parseDay(rawFrom) ? rawFrom.trim() : ''
  let end = parseDay(rawTo) ? rawTo.trim() : ''
  // 同为 YYYY-MM-DD 时字符串顺序就是日期顺序
  if (start && end && start > end) [start, end] = [end, start]
  const params: AuditDayRange['params'] = {}
  const first = start ? parseDay(start) : undefined
  const last = end ? parseDay(end) : undefined
  if (first)
    params.from = new Date(first.year, first.month - 1, first.day, 0, 0, 0, 0).toISOString()
  if (last) params.to = new Date(last.year, last.month - 1, last.day, 23, 59, 59, 999).toISOString()
  return { from: start, to: end, params }
}

/** auditActionFilter 校验地址里的动作参数;不是动作标识时返回 undefined(不筛选) */
export function auditActionFilter(raw: string): string | undefined {
  const value = raw.trim()
  return value.length <= ACTION_MAX_LENGTH && ACTION_PATTERN.test(value) ? value : undefined
}

/** auditActorFilter 校验地址里的操作人参数;不是有效账号编号时返回 undefined(不筛选) */
export function auditActorFilter(raw: string): string | undefined {
  const value = raw.trim()
  return ACCOUNT_ID_PATTERN.test(value) && BigInt(value) <= ACCOUNT_ID_MAX ? value : undefined
}
