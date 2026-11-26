// 校验审计地址筛选，按本地自然日生成完整起止时间；无效参数不进入请求。

import { parseDateInput } from '../../utils/dateInput.ts'

/** 动作标识:点分的小写短词,如 auth.login;与 audit_log.action 的列宽一致,最长 64 */
const ACTION_PATTERN = /^[a-z_]+(?:\.[a-z_]+)+$/
const ACTION_MAX_LENGTH = 64
/** 账号编号:正的 64 位整数(雪花编号),十进制且不带前导零 */
const ACCOUNT_ID_PATTERN = /^[1-9]\d{0,18}$/
const ACCOUNT_ID_MAX = 9223372036854775807n

/** 一段审计时间范围:界面回显用的日期,与发给接口的时间参数(只含有效的一端) */
export interface AuditDayRange {
  from: string
  to: string
  params: { from?: string; to?: string }
}

/** auditDayRange 把地址里的起止日期换成查询时间;无效的一端忽略,开始晚于结束时对调 */
export function auditDayRange(rawFrom: string, rawTo: string): AuditDayRange {
  let start = parseDateInput(rawFrom) ? rawFrom.trim() : ''
  let end = parseDateInput(rawTo) ? rawTo.trim() : ''
  // 同为 YYYY-MM-DD 时字符串顺序就是日期顺序
  if (start && end && start > end) [start, end] = [end, start]
  const params: AuditDayRange['params'] = {}
  const first = start ? parseDateInput(start) : undefined
  const last = end ? parseDateInput(end) : undefined
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
