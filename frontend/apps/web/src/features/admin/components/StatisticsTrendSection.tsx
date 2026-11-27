// 平台与学校看板共用的日期筛选、趋势图和等价数据表。

import { useMemo, useState } from 'react'
import { CalendarRange } from 'lucide-react'
import type { Statistics } from '@chaimir/api-client'
import {
  ChartContainer,
  DescriptionList,
  FilterBar,
  FilterField,
  Input,
  SegmentedControl,
  TrendLineChart,
  type ChartDataTable,
  type TrendSeries,
} from '@chaimir/ui'
import { useAsyncResource, useUrlState } from '../../../hooks'
import { formatCalendarDate } from '../../../utils/formatters'
import { statisticsMetricLabel } from '../../../utils/labels/admin'
import { userFacingErrorMessage } from '../../../utils/userFacingError'
import {
  STATISTICS_METRIC_GROUPS,
  statisticsDateRange,
  type StatisticsMetricGroup,
} from '../statisticsPresentation'

/** 地址参数:range 是预设区间(天数)或 custom;from / to 只在 custom 时生效 */
const TREND_QUERY = {
  range: { defaultValue: '30', allowed: ['7', '30', '90', 'custom'] },
  from: { defaultValue: '' },
  to: { defaultValue: '' },
} as const

const RANGE_OPTIONS = [
  { value: '7', label: '近 7 天' },
  { value: '30', label: '近 30 天' },
  { value: '90', label: '近 90 天' },
  { value: 'custom', label: '自定义' },
]

/** 一组指标在当前区间内的图表数据。 */
interface TrendGroup {
  group: StatisticsMetricGroup
  /** 该组真正出现过的指标键 */
  keys: string[]
  series: TrendSeries[]
  /** 每行一个日期,列为各指标键 */
  data: Array<Record<string, string | number>>
}

export interface StatisticsTrendSectionProps {
  /** 取数:按区间拿逐日快照。平台端与校管端各传自己的接口 */
  load: (range: { from: string; to: string }) => Promise<Statistics[]>
  /** 控件 id 前缀:保证同页多条筛选的 id 唯一 */
  idPrefix: string
  /** 分组说明(用户向) */
  description: string
}

/**
 * StatisticsTrendSection 渲染「标题行(右上角区间切换)+ 每组一张趋势图」。
 * 自定义区间是需要确认才生效的字段,交给 FilterBar 的 onSubmit 收口(回车即查询)。
 */
export function StatisticsTrendSection({ load, idPrefix, description }: StatisticsTrendSectionProps) {
  const [query, setQuery] = useUrlState(TREND_QUERY)
  const range = statisticsDateRange(query)

  const statistics = useAsyncResource(
    () => load({ from: range.from, to: range.to }),
    [range.from, range.to],
    (value) => value.length === 0,
  )

  const snapshots = useMemo(() => statistics.data ?? [], [statistics.data])
  const groups = useMemo(() => trendGroups(snapshots), [snapshots])
  const latest = snapshots.length > 0 ? snapshots[snapshots.length - 1] : undefined
  const loading = statistics.status === 'loading'
  const error = statistics.error
    ? userFacingErrorMessage(statistics.error, '统计数据没有读到,请重试。')
    : undefined

  return (
    <section aria-labelledby={`${idPrefix}-trend-title`} className="mt-2">
      <div className="mb-3 flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <h2 id={`${idPrefix}-trend-title`} className="text-md font-semibold text-ink">
            运营趋势
          </h2>
          <p className="mt-0.5 text-sm text-ink-sub">
            {description}
            {latest ? ` 最新快照 ${formatCalendarDate(latest.date)},共 ${snapshots.length} 天数据。` : ''}
          </p>
        </div>
        <SegmentedControl
          aria-label="统计区间"
          size="sm"
          options={RANGE_OPTIONS}
          value={query.range}
          onValueChange={(value) =>
            setQuery(value === 'custom' ? { range: value, ...range } : { range: value, from: '', to: '' })
          }
        />
      </div>
      <div className="flex flex-col gap-4">
        {query.range === 'custom' ? (
          <StatisticsRangeForm
            key={`${range.from}:${range.to}`}
            range={range}
            idPrefix={idPrefix}
            loading={loading}
            onSubmit={(value) => setQuery({ range: 'custom', ...value })}
          />
        ) : null}

        {/* 没有已登记指标时也保留加载、失败和空态。 */}
        <div className={groups.length > 1 ? 'grid gap-4 lg:grid-cols-2' : 'grid gap-4'}>
          {(groups.length > 0 ? groups : [emptyGroup()]).map((item) => (
            <ChartContainer
              key={item.group.id}
              title={item.group.title}
              description={groupDescription(item)}
              loading={loading}
              error={error}
              onRetry={statistics.reload}
              isEmpty={item.data.length === 0}
              emptyHint="这个区间内还没有统计数据。统计每日生成,新开通的租户需要等一天。"
              ariaSummary={trendSummary(item)}
              dataTable={groupDataTable(item)}
            >
              {item.data.length < 4 ? (
                <DescriptionList items={item.series.map((series) => {
                  const value = item.data[item.data.length - 1]?.[series.key]
                  return {
                    term: series.name,
                    description: typeof value === 'number' ? value.toLocaleString('zh-CN') : '没有数据',
                  }
                })} />
              ) : (
                <TrendLineChart data={item.data} xKey="date" series={item.series} />
              )}
            </ChartContainer>
          ))}
        </div>
      </div>
    </section>
  )
}

/** emptyGroup 在没有任何数据时给出一个占位分组,让容器有三态可渲染。 */
function emptyGroup(): TrendGroup {
  return { group: STATISTICS_METRIC_GROUPS[0], keys: [], series: [], data: [] }
}

/**
 * trendGroups 把逐日快照按指标分组转成折线图数据。
 * 只保留区间内真正出现过的已登记指标键:某端没有的指标不该留一张空图。
 */
function trendGroups(snapshots: Statistics[]): TrendGroup[] {
  const present = new Set<string>()
  for (const snapshot of snapshots) {
    for (const [key, value] of Object.entries(snapshot.metrics)) {
      if (typeof value === 'number' && statisticsMetricLabel(key)) present.add(key)
    }
  }

  const out: TrendGroup[] = []
  for (const group of STATISTICS_METRIC_GROUPS) {
    const keys = group.keys.filter((key) => present.has(key))
    if (keys.length === 0) continue
    out.push({
      group,
      keys,
      series: keys.map((key) => ({ key, name: statisticsMetricLabel(key) ?? key })),
      data: snapshots.map((snapshot) => {
        const row: Record<string, string | number> = { date: formatCalendarDate(snapshot.date) }
        for (const key of keys) {
          const value = snapshot.metrics[key]
          if (typeof value === 'number') row[key] = value
        }
        return row
      }),
    })
  }
  return out
}

/** groupDescription 说明这张图回答什么,并点明粒度。 */
function groupDescription(item: TrendGroup): string {
  const names = item.series.map((series) => series.name).join('、')
  return names === '' ? '按日聚合的历史快照。' : `按日聚合:${names}。`
}

/** groupDataTable 生成与图同源的数据表替代(§8.2 强制)。 */
function groupDataTable(item: TrendGroup): ChartDataTable {
  return {
    columns: ['日期', ...item.series.map((series) => series.name)],
    rows: item.data.map((row) => [
      String(row.date),
      ...item.keys.map((key) => (typeof row[key] === 'number' ? (row[key] as number) : '—')),
    ]),
  }
}

/**
 * trendSummary 写出读屏用的关键洞察:区间、每条系列的首尾值与峰值。
 * 不写「见下图」这类无信息量的句子 —— 读屏用户拿不到图形。
 */
function trendSummary(item: TrendGroup): string {
  if (item.data.length === 0) return `${item.group.title}:当前区间内没有数据。`
  const span = `${String(item.data[0].date)} 至 ${String(item.data[item.data.length - 1].date)}`
  const parts = item.series.map((series) => {
    const values = item.data
      .map((row) => row[series.key])
      .filter((value): value is number => typeof value === 'number')
    if (values.length === 0) return `${series.name} 无数据`
    const first = values[0]
    const last = values[values.length - 1]
    const peak = Math.max(...values)
    const trend = last > first ? '上升' : last < first ? '下降' : '持平'
    return `${series.name} 从 ${first} 到 ${last}(${trend}),峰值 ${peak}`
  })
  return `${item.group.title},${span}:${parts.join(';')}。`
}

/** 未提交日期留在表单；地址范围变化时通过 key 重建，不另存已应用范围。 */
function StatisticsRangeForm({ range, idPrefix, loading, onSubmit }: {
  range: { from: string; to: string }
  idPrefix: string
  loading: boolean
  onSubmit: (range: { from: string; to: string }) => void
}) {
  const [from, setFrom] = useState(range.from)
  const [to, setTo] = useState(range.to)
  return (
    <FilterBar
      label="自定义统计区间"
      onSubmit={() => onSubmit({ from, to })}
      submitLabel="查询"
      submitIcon={CalendarRange}
      submitting={loading}
    >
      <FilterField label="开始日期" htmlFor={`${idPrefix}-stats-from`}>
        <Input
          id={`${idPrefix}-stats-from`}
          type="date"
          required
          max={to || undefined}
          value={from}
          onChange={(event) => setFrom(event.target.value)}
        />
      </FilterField>
      <FilterField label="结束日期" htmlFor={`${idPrefix}-stats-to`}>
        <Input
          id={`${idPrefix}-stats-to`}
          type="date"
          required
          min={from || undefined}
          value={to}
          onChange={(event) => setTo(event.target.value)}
        />
      </FilterField>
    </FilterBar>
  )
}
