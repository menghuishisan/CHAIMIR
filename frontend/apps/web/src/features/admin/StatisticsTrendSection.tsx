// 运营统计趋势区(平台看板与学校看板共用)。
//
// 两端的统计接口形状完全一致(逐日 { date, metrics } 快照),差别只有调用哪个接口,
// 故这里只有一份实现 —— 此前两个看板各自复制了一遍同样的摊平表格与日期表单。
//
// 呈现形态按规范 §8.3:历史序列先出折线图,数据表是同一份数据的等价切换(ChartContainer 提供),
// 而不是只给一张 40 行的「日期 × 指标 × 数值」长表 —— 那等于让读者自己在数字里找趋势。
// metrics 是后端写入的开放对象:只呈现已登记的键,未登记键跳过,不猜语义、不把内部键名抛到界面上。

import { useMemo, useState } from 'react'
import { CalendarRange } from 'lucide-react'
import type { Statistics } from '@chaimir/api-client'
import {
  Badge,
  ChartContainer,
  FilterBar,
  FilterField,
  Input,
  PageSection,
  TrendLineChart,
  type ChartDataTable,
  type TrendSeries,
} from '@chaimir/ui'
import { useAsyncResource } from '../../hooks'
import { formatDate } from '../../utils/formatters'
import {
  STATISTICS_METRIC_GROUPS,
  statisticsMetricLabel,
  type StatisticsMetricGroup,
} from '../../utils/labels/admin'
import { userFacingErrorMessage } from '../../utils/userFacingError'

/** 趋势默认区间:近 30 天。 */
const DEFAULT_RANGE_DAYS = 30

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
 * StatisticsTrendSection 渲染「区间筛选 + 每组一张趋势图」。
 * 区间是需要确认才生效的字段,故交给 FilterBar 的 onSubmit 收口(回车即查询)。
 */
export function StatisticsTrendSection({ load, idPrefix, description }: StatisticsTrendSectionProps) {
  const [from, setFrom] = useState(() => isoDate(-DEFAULT_RANGE_DAYS))
  const [to, setTo] = useState(() => isoDate(0))
  const [range, setRange] = useState({ from: isoDate(-DEFAULT_RANGE_DAYS), to: isoDate(0) })

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
    <PageSection title="运营统计" description={description}>
      <div className="flex flex-col gap-4">
        <FilterBar
          label="统计区间筛选"
          onSubmit={() => setRange({ from, to })}
          submitLabel="查询"
          submitIcon={CalendarRange}
          submitting={loading}
        >
          <FilterField label="开始日期" htmlFor={`${idPrefix}-stats-from`}>
            <Input
              id={`${idPrefix}-stats-from`}
              type="date"
              value={from}
              onChange={(event) => setFrom(event.target.value)}
            />
          </FilterField>
          <FilterField label="结束日期" htmlFor={`${idPrefix}-stats-to`}>
            <Input
              id={`${idPrefix}-stats-to`}
              type="date"
              value={to}
              onChange={(event) => setTo(event.target.value)}
            />
          </FilterField>
        </FilterBar>

        {latest ? (
          <div className="flex flex-wrap items-center gap-2">
            <Badge tone="neutral">最新快照 {formatDate(latest.date)}</Badge>
            <Badge tone="jade">共 {snapshots.length} 天数据</Badge>
          </div>
        ) : null}

        {/* 没有任何可呈现的分组时也要出一张容器:它自己有空/载/错三态与引导文案,
            比页面另外拼一个空块更一致(§8.2) */}
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
            <TrendLineChart data={item.data} xKey="date" series={item.series} />
          </ChartContainer>
        ))}
      </div>
    </PageSection>
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
        const row: Record<string, string | number> = { date: formatDate(snapshot.date) }
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

/** isoDate 按天偏移生成 date 控件需要的 YYYY-MM-DD。 */
function isoDate(offsetDays: number): string {
  const date = new Date()
  date.setDate(date.getDate() + offsetDays)
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}
