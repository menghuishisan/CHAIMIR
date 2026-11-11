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

/** 时间流族的事件轴按天分组,左列只需时刻(规范 §6.5.3 第 ⑥ 族)。 */
const TIME_FORMAT = new Intl.DateTimeFormat('zh-CN', {
  hour: '2-digit',
  minute: '2-digit',
  hour12: false,
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

/** dotDate 把日期写成 2026.09.01;读不出时给用户向占位。 */
function dotDate(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return UNKNOWN_TIME
  return `${date.getFullYear()}.${pad2(date.getMonth() + 1)}.${pad2(date.getDate())}`
}

/** formatDateRange 把日期区间统一写成「2026.09.01 – 2027.01.16」(规范 §6.5.0 通则 7)。 */
export function formatDateRange(start: string, end: string): string {
  return `${dotDate(start)} – ${dotDate(end)}`
}

/** formatTime 只展示时刻(HH:mm),给按天分组的事件轴当左列用(§6.5.3 第 ⑥ 族)。 */
export function formatTime(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '—' : TIME_FORMAT.format(date)
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

/** 发生类时间里,服务器时钟略快于本机时仍算「刚刚」的容差(毫秒)。 */
const CLOCK_SKEW_TOLERANCE_MS = 5 * 60_000

/** pad2 把月、日、时、分补成两位。 */
function pad2(value: number): string {
  return String(value).padStart(2, '0')
}

/** clockText 给出本地时刻 HH:mm;自己拼而不用 Intl,避免部分环境把零点写成 24:00。 */
function clockText(date: Date): string {
  return `${pad2(date.getHours())}:${pad2(date.getMinutes())}`
}

/** calendarDayDiff 返回从 from 到 to 跨过的日历天数(按本地日期算,不按 24 小时算)。 */
function calendarDayDiff(from: Date, to: Date): number {
  const fromDay = new Date(from.getFullYear(), from.getMonth(), from.getDate()).getTime()
  const toDay = new Date(to.getFullYear(), to.getMonth(), to.getDate()).getTime()
  return Math.round((toDay - fromDay) / 86_400_000)
}

/**
 * formatRelativeDeadline 表达截止时间与当前的关系,并给出紧迫程度(规范 §6.5.0 通则 7)。
 * 当天写「今天 23:59 截止」、次日写「明天 09:00 截止」,其余写「还剩 N 天」;三天内为 urgent。
 * 紧迫程度供调用方选择徽标语义色,避免各页面各自定义"多久算紧急"。now 只为测试注入。
 */
export function formatRelativeDeadline(
  value: string,
  now: Date = new Date(),
): {
  text: string
  urgency: 'overdue' | 'urgent' | 'normal'
} {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return { text: UNKNOWN_TIME, urgency: 'normal' }
  if (date.getTime() <= now.getTime()) return { text: '已截止', urgency: 'overdue' }
  const days = calendarDayDiff(now, date)
  if (days === 0) return { text: `今天 ${clockText(date)} 截止`, urgency: 'urgent' }
  if (days === 1) return { text: `明天 ${clockText(date)} 截止`, urgency: 'urgent' }
  return { text: `还剩 ${days} 天`, urgency: days <= 3 ? 'urgent' : 'normal' }
}

/**
 * formatRelativeOccurrence 表达一件已经发生的事离现在多久(规范 §6.5.0 通则 7):
 * 刚刚 / N 分钟前 / 今天 09:05 / 昨天 14:20 / 11月9日;相隔一年以上写完整日期 2025.11.01。
 * 完整时间由调用方放在 title 或读屏文字里。now 只为测试注入。
 */
export function formatRelativeOccurrence(value: string, now: Date = new Date()): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return UNKNOWN_TIME
  const diffMs = now.getTime() - date.getTime()
  if (diffMs < 60_000 && diffMs > -CLOCK_SKEW_TOLERANCE_MS) return '刚刚'
  if (diffMs >= 60_000 && diffMs < 3_600_000) return `${Math.floor(diffMs / 60_000)} 分钟前`
  const days = calendarDayDiff(date, now)
  if (days === 0) return `今天 ${clockText(date)}`
  if (days === 1) return `昨天 ${clockText(date)}`
  if (Math.abs(days) < 365) return `${date.getMonth() + 1}月${date.getDate()}日`
  return `${date.getFullYear()}.${pad2(date.getMonth() + 1)}.${pad2(date.getDate())}`
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

/**
 * formatFileSize 把字节数换成用户向文件大小文案。
 * 课时材料、题库附件、备份记录与导出产物共用它 —— 这是跨模块的通用格式化,
 * 不属于任何单个模块的文案映射,故放在此处而不是 labels/<module>.ts。
 */
export function formatFileSize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '大小未知'
  if (bytes < 1024) return `${bytes} 字节`
  const kb = bytes / 1024
  if (kb < 1024) return `${kb.toFixed(1)} KB`
  const mb = kb / 1024
  if (mb < 1024) return `${mb.toFixed(1)} MB`
  return `${(mb / 1024).toFixed(2)} GB`
}

/** 常见文件扩展名对应的用户向种类名 */
const FILE_KIND_BY_EXTENSION: Record<string, string> = {
  pdf: 'PDF 文档',
  doc: 'Word 文档',
  docx: 'Word 文档',
  ppt: '演示文稿',
  pptx: '演示文稿',
  xls: '表格',
  xlsx: '表格',
  csv: '表格',
  txt: '文本',
  md: '文本',
  zip: '压缩包',
  rar: '压缩包',
  '7z': '压缩包',
  png: '图片',
  jpg: '图片',
  jpeg: '图片',
  gif: '图片',
  webp: '图片',
  mp4: '视频',
  mp3: '音频',
}

/**
 * fileKindLabel 把文件名的扩展名写成用户看得懂的种类(「PDF 文档」「演示文稿」),
 * 代替界面上的 MIME 类型这类开发术语;不认识的扩展名写成「SOL 文件」,没有扩展名时返回 undefined。
 */
export function fileKindLabel(fileName: string): string | undefined {
  const dot = fileName.lastIndexOf('.')
  if (dot <= 0 || dot === fileName.length - 1) return undefined
  const extension = fileName.slice(dot + 1).toLowerCase()
  return FILE_KIND_BY_EXTENSION[extension] ?? `${extension.toUpperCase()} 文件`
}
