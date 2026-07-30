// formatters.ts 提供 apps/web 内跨页面复用的时间展示工具。

const SHORT_DATE_TIME_FORMAT = new Intl.DateTimeFormat('zh-CN', {
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
})

/**
 * formatShortDateTime 把后端时间字符串转换为适合列表展示的短日期时间。
 * 时间来自 HTTP 响应,属外部输入需在边界处校验:无法解析时给用户向文案,
 * 不让 Intl 抛错把整个列表推到错误屏,也不把原始值抛到界面上。
 */
export function formatShortDateTime(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '时间未知' : SHORT_DATE_TIME_FORMAT.format(date)
}
