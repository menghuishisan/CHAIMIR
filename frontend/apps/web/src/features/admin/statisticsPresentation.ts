// 运营统计的自然日范围与指标分组。

import { parseDateInput } from '../../utils/dateInput.ts'

/** 自定义范围必须为有效闭区间；预设包含今天，无效地址回到近30天。 */
export function statisticsDateRange(
  query: { range: string; from: string; to: string },
  now: Date = new Date(),
): { from: string; to: string } {
  const from = query.from.trim()
  const to = query.to.trim()
  if (query.range === 'custom' && parseDateInput(from) && parseDateInput(to) && from <= to) {
    return { from, to }
  }
  const days = ['7', '30', '90'].includes(query.range) ? Number(query.range) : 30
  return { from: isoDate(1 - days, now), to: isoDate(0, now) }
}

/** 按本地自然日偏移，避免夏令时和 UTC 转换改变日期。 */
function isoDate(offsetDays: number, now: Date): string {
  const date = new Date(now)
  date.setDate(date.getDate() + offsetDays)
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

/** StatisticsMetricGroup 描述一张趋势图展示的一组指标。 */
export interface StatisticsMetricGroup {
  id: string
  title: string
  keys: string[]
}

/** STATISTICS_METRIC_GROUPS 按业务语义拆分趋势图,每组不超过图表系列上限。 */
export const STATISTICS_METRIC_GROUPS: StatisticsMetricGroup[] = [
  { id: 'account', title: '账号', keys: ['account_count', 'active_account_count'] },
  { id: 'tenant', title: '学校', keys: ['tenant_count'] },
  {
    id: 'teaching',
    title: '教学',
    keys: ['course_count', 'active_course_count', 'experiment_count', 'active_instance_count'],
  },
  {
    id: 'contest',
    title: '竞赛',
    keys: ['contest_count', 'active_contest_count'],
  },
  { id: 'resource', title: '实验环境', keys: ['active_sandbox_count'] },
  { id: 'learning', title: '学习时长', keys: ['learning_duration_sec'] },
]
