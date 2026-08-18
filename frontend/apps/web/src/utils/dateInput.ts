// dateInput 提供浏览器日期时间控件需要的本地时间格式转换。
// 后端时间进入 datetime-local 控件时必须统一按本地年月日时分输出,无法解析则回空串。

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
