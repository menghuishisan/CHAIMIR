// 浏览器日期控件的自然日解析与本地时间格式转换。

const DAY_PATTERN = /^(\d{4})-(\d{2})-(\d{2})$/

/** 解析 YYYY-MM-DD；不存在的自然日返回 undefined。 */
export function parseDateInput(raw: string): { year: number; month: number; day: number } | undefined {
  const match = DAY_PATTERN.exec(raw.trim())
  if (!match) return undefined
  const [year, month, day] = [Number(match[1]), Number(match[2]), Number(match[3])]
  const date = new Date(year, month - 1, day)
  const exists =
    date.getFullYear() === year && date.getMonth() === month - 1 && date.getDate() === day
  return exists ? { year, month, day } : undefined
}

/** toDateTimeInputValue 把后端时间转成 datetime-local 控件需要的格式。 */
export function toDateTimeInputValue(value: string | undefined): string {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const pad = (number: number) => String(number).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(
    date.getHours(),
  )}:${pad(date.getMinutes())}`
}
