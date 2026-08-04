// formatters.ts 提供 apps/web 内跨页面复用的时间与数值展示工具。

const SHORT_DATE_TIME_FORMAT = new Intl.DateTimeFormat('zh-CN', {
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
})

const DATE_FORMAT = new Intl.DateTimeFormat('zh-CN', {
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
})

const DATE_TIME_FORMAT = new Intl.DateTimeFormat('zh-CN', {
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
})

/** 时间解析失败时的用户向占位,不把原始值抛到界面上。 */
const UNKNOWN_TIME = '时间未知'

/**
 * formatShortDateTime 把后端时间字符串转换为适合列表展示的短日期时间。
 * 时间来自 HTTP 响应,属外部输入需在边界处校验:无法解析时给用户向文案,
 * 不让 Intl 抛错把整个列表推到错误屏,也不把原始值抛到界面上。
 */
export function formatShortDateTime(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? UNKNOWN_TIME : SHORT_DATE_TIME_FORMAT.format(date)
}

/** formatDate 展示仅到日的日期(课程起止、学期区间等)。 */
export function formatDate(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? UNKNOWN_TIME : DATE_FORMAT.format(date)
}

/** formatDateTime 展示含年份的完整日期时间(截止时间、提交时刻等)。 */
export function formatDateTime(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? UNKNOWN_TIME : DATE_TIME_FORMAT.format(date)
}

/**
 * formatDuration 把秒数换成用户向时长文案(学习时长、用时等)。
 * 不足一分钟按「不足 1 分钟」表达,避免出现「0 分钟」这种看起来像没记录的说法。
 */
export function formatDuration(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds <= 0) return '暂无记录'
  const totalMinutes = Math.floor(seconds / 60)
  if (totalMinutes < 1) return '不足 1 分钟'
  const hours = Math.floor(totalMinutes / 60)
  const minutes = totalMinutes % 60
  if (hours === 0) return `${minutes} 分钟`
  return minutes === 0 ? `${hours} 小时` : `${hours} 小时 ${minutes} 分钟`
}

/**
 * formatRelativeDeadline 表达截止时间与当前的关系,并给出紧迫程度。
 * 紧迫程度供调用方选择徽标语义色,避免各页面各自定义"多久算紧急"。
 */
export function formatRelativeDeadline(value: string): {
  text: string
  urgency: 'overdue' | 'urgent' | 'normal'
} {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return { text: UNKNOWN_TIME, urgency: 'normal' }
  const diffMs = date.getTime() - Date.now()
  if (diffMs <= 0) return { text: '已截止', urgency: 'overdue' }
  const hours = Math.floor(diffMs / 3_600_000)
  if (hours < 1) return { text: '不足 1 小时', urgency: 'urgent' }
  if (hours < 24) return { text: `剩余 ${hours} 小时`, urgency: 'urgent' }
  const days = Math.floor(hours / 24)
  return { text: `剩余 ${days} 天`, urgency: days <= 3 ? 'urgent' : 'normal' }
}

/** formatScore 展示分数:整数不带小数,小数保留一位,缺失给「—」。 */
export function formatScore(value: number | undefined): string {
  if (value === undefined || !Number.isFinite(value)) return '—'
  return Number.isInteger(value) ? String(value) : value.toFixed(1)
}

/** formatGpa 展示绩点,固定两位小数便于纵向对齐。 */
export function formatGpa(value: number): string {
  return Number.isFinite(value) ? value.toFixed(2) : '—'
}

/** formatPercent 展示完成率等比例,四舍五入到整数百分比。 */
export function formatPercent(done: number, total: number): string {
  if (total <= 0) return '—'
  return `${Math.round((done / total) * 100)}%`
}
