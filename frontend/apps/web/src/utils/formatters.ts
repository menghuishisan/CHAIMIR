// formatters.ts 提供 apps/web 内跨页面复用的日期、数字和容量展示工具。

import { adminMetricLabel } from './labels'

const DATE_TIME_FORMAT = new Intl.DateTimeFormat('zh-CN', {
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
})

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

const NUMBER_FORMAT = new Intl.NumberFormat('zh-CN')

/**
 * normalizeDateInput 把后端时间字符串或 Date 统一转换为可格式化日期。
 */
function normalizeDateInput(value?: string | number | Date | null): Date | null {
  if (value === undefined || value === null || value === '') {
    return null
  }
  const date = value instanceof Date ? value : new Date(value)
  return Number.isNaN(date.getTime()) ? null : date
}

/**
 * formatDateTime 把后端时间转换为完整本地日期时间。
 */
export function formatDateTime(value?: string | number | Date | null, fallback = '暂无时间'): string {
  const date = normalizeDateInput(value)
  return date ? DATE_TIME_FORMAT.format(date) : fallback
}

/**
 * formatShortDateTime 把后端时间转换为适合列表展示的短日期时间。
 */
export function formatShortDateTime(value?: string | number | Date | null, fallback = '暂无时间'): string {
  const date = normalizeDateInput(value)
  return date ? SHORT_DATE_TIME_FORMAT.format(date) : fallback
}

/**
 * formatDate 把后端时间转换为本地日期。
 */
export function formatDate(value?: string | number | Date | null, fallback = '暂无时间'): string {
  const date = normalizeDateInput(value)
  return date ? DATE_FORMAT.format(date) : fallback
}

/**
 * formatDateTimeLocalInput 把后端时间转换为 datetime-local 控件值。
 */
export function formatDateTimeLocalInput(value?: string | number | Date | null): string {
  const date = normalizeDateInput(value)
  return date ? date.toISOString().slice(0, 16) : ''
}

/**
 * parseDateTimeLocalInput 把 datetime-local 控件值转换为后端 ISO 时间。
 */
export function parseDateTimeLocalInput(value: string): string {
  return value ? new Date(value).toISOString() : ''
}

/**
 * recentDateRange 生成最近若干天的 ISO 日期查询区间。
 */
export function recentDateRange(days: number): { from: string; to: string } {
  const to = new Date()
  const from = new Date()
  from.setDate(to.getDate() - days)
  return {
    from: from.toISOString().slice(0, 10),
    to: to.toISOString().slice(0, 10),
  }
}

/**
 * formatNumber 统一统计数字展示。
 */
export function formatNumber(value?: number | null): string {
  return NUMBER_FORMAT.format(value || 0)
}

/**
 * formatBytes 把字节数转换为用户可读容量。
 */
export function formatBytes(value?: number | null): string {
  const bytes = value || 0
  if (bytes < 1024) {
    return `${bytes} B`
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} KB`
  }
  if (bytes < 1024 * 1024 * 1024) {
    return `${(bytes / 1024 / 1024).toFixed(1)} MB`
  }
  return `${(bytes / 1024 / 1024 / 1024).toFixed(1)} GB`
}

/**
 * formatSeconds 把秒数转换为分钟或秒的用户向时长。
 */
export function formatSeconds(value?: number | null): string {
  const seconds = value || 0
  if (seconds < 60) {
    return `${seconds} 秒`
  }
  return `${Math.round(seconds / 60)} 分钟`
}

/**
 * formatMetricsSummary 把后端指标快照转换成表格内的紧凑摘要。
 */
export function formatMetricsSummary(metrics: Record<string, unknown>): string {
  const entries = Object.entries(metrics)
  if (entries.length === 0) {
    return '暂无指标'
  }
  return entries
    .map(([key, value]) => `${adminMetricLabel(key)}: ${formatMetricValue(key, value)}`)
    .join('；')
}

/**
 * formatMetricValue 按已知指标单位转换后端快照值，未知复合值只提示可查看详情。
 */
function formatMetricValue(key: string, value: unknown): string {
  if (key === 'learning_duration_sec' && typeof value === 'number') {
    return formatSeconds(value)
  }
  if (key === 'max_memory_mb' && typeof value === 'number') {
    return `${formatNumber(value)} MB`
  }
  if (typeof value === 'number') {
    return formatNumber(value)
  }
  if (typeof value === 'string' || typeof value === 'boolean') {
    return String(value)
  }
  return '可查看详情'
}
