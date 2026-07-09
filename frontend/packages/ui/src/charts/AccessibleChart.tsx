// AccessibleChart 提供轻量 SVG 图表、状态处理和数据表降级，满足 FE-2 与 §8 图表契约。

import React, { useMemo } from 'react'
import { AlertCircle, BarChart3, RefreshCw, Table2 } from 'lucide-react'
import { clsx } from 'clsx'
import './AccessibleChart.css'

export type AccessibleChartKind = 'bar' | 'line'
export type ChartPointTone = 'primary' | 'secondary' | 'success' | 'warning' | 'danger' | 'info'

export interface ChartPoint {
  label: string
  value: number
  description?: string
  tone?: ChartPointTone
}

export interface AccessibleChartProps extends React.HTMLAttributes<HTMLElement> {
  title: string
  summary: string
  data: ChartPoint[]
  kind?: AccessibleChartKind
  loading?: boolean
  errorMessage?: string
  onRetry?: () => void
  emptyTitle?: string
  emptyDescription?: string
  tableInitiallyOpen?: boolean
  valueFormatter?: (value: number) => string
}

const statusLabel: Record<ChartPointTone, string> = {
  primary: '主指标',
  secondary: '重点指标',
  success: '状态正常',
  warning: '需要关注',
  danger: '需要处理',
  info: '信息指标',
}

/**
 * AccessibleChart 让每张图都带摘要、图例、数据表、空/错/载状态和 reduced-motion 兜底。
 */
export function AccessibleChart({
  title,
  summary,
  data,
  kind = 'bar',
  loading = false,
  errorMessage,
  onRetry,
  emptyTitle = '暂无图表数据',
  emptyDescription = '数据准备好后会在这里展示。',
  tableInitiallyOpen = false,
  valueFormatter = defaultValueFormatter,
  className,
  ...props
}: AccessibleChartProps): React.ReactElement {
  const chartId = useStableId('chaimir-chart')
  const normalizedData = useMemo(() => data.filter((point) => Number.isFinite(point.value)), [data])
  const max = Math.max(1, ...normalizedData.map((point) => Math.abs(point.value)))

  return (
    <section className={clsx('chaimir-chart', className)} aria-labelledby={`${chartId}-title`} aria-describedby={`${chartId}-summary`} {...props}>
      <header className="chaimir-chart__header">
        <div>
          <h3 id={`${chartId}-title`}>{title}</h3>
          <p id={`${chartId}-summary`}>{summary}</p>
        </div>
        <BarChart3 aria-hidden="true" size={22} />
      </header>

      {loading && <ChartState tone="loading" title="正在加载图表数据" description="请稍候，图表会在数据返回后显示。" />}
      {!loading && errorMessage && <ChartError message={errorMessage} onRetry={onRetry} />}
      {!loading && !errorMessage && normalizedData.length === 0 && <ChartState tone="empty" title={emptyTitle} description={emptyDescription} />}
      {!loading && !errorMessage && normalizedData.length > 0 && (
        <>
          <figure className="chaimir-chart__figure">
            <svg viewBox="0 0 320 180" role="img" aria-labelledby={`${chartId}-svg-title`} aria-describedby={`${chartId}-svg-desc`} preserveAspectRatio="none">
              <title id={`${chartId}-svg-title`}>{title}</title>
              <desc id={`${chartId}-svg-desc`}>{summary}</desc>
              <g className="chaimir-chart__grid" aria-hidden="true">
                {[0, 1, 2, 3].map((line) => <line key={line} x1="16" x2="304" y1={28 + line * 36} y2={28 + line * 36} />)}
              </g>
              {kind === 'line' ? renderLine(normalizedData, max, valueFormatter) : renderBars(normalizedData, max, valueFormatter)}
            </svg>
            <figcaption className="chaimir-chart__legend">
              {normalizedData.map((point) => (
                <span key={point.label} className={`is-${point.tone ?? 'primary'}`}>
                  <i aria-hidden="true" />
                  {point.label}
                  <small>{point.description ?? statusLabel[point.tone ?? 'primary']}</small>
                </span>
              ))}
            </figcaption>
          </figure>
          <details className="chaimir-chart__table" open={tableInitiallyOpen}>
            <summary><Table2 aria-hidden="true" size={16} />查看数据表</summary>
            <table>
              <caption>{title}数据表</caption>
              <thead>
                <tr>
                  <th scope="col">项目</th>
                  <th scope="col">数值</th>
                  <th scope="col">说明</th>
                </tr>
              </thead>
              <tbody>
                {normalizedData.map((point) => (
                  <tr key={point.label}>
                    <td>{point.label}</td>
                    <td>{valueFormatter(point.value)}</td>
                    <td>{point.description ?? statusLabel[point.tone ?? 'primary']}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </details>
        </>
      )}
    </section>
  )
}

/**
 * renderBars 用可聚焦 SVG 分组渲染柱状图，并给每个数据点提供可读标签。
 */
function renderBars(data: ChartPoint[], max: number, valueFormatter: (value: number) => string): React.ReactNode {
  const gap = 8
  const width = Math.max(12, (288 - gap * Math.max(0, data.length - 1)) / Math.max(1, data.length))
  return data.map((point, index) => {
    const height = Math.max(6, (Math.abs(point.value) / max) * 120)
    const x = 16 + index * (width + gap)
    const tone = point.tone ?? 'primary'
    return (
      <g key={point.label} className={`chaimir-chart__series is-${tone}`} tabIndex={0} role="listitem" aria-label={`${point.label}，${valueFormatter(point.value)}，${point.description ?? statusLabel[tone]}`}>
        <rect x={x} y={148 - height} width={width} height={height} rx="4" className="chaimir-chart__bar" />
        <text x={x + width / 2} y={164} className="chaimir-chart__label">{point.label}</text>
      </g>
    )
  })
}

/**
 * renderLine 用折线和点位渲染趋势图，并保留数据点键盘访问能力。
 */
function renderLine(data: ChartPoint[], max: number, valueFormatter: (value: number) => string): React.ReactNode {
  const points = data.map((point, index) => {
    const x = 16 + (index / Math.max(1, data.length - 1)) * 288
    const y = 148 - (Math.abs(point.value) / max) * 120
    return { point, x, y }
  })
  return (
    <>
      <polyline points={points.map(({ x, y }) => `${x},${y}`).join(' ')} className="chaimir-chart__line" fill="none" />
      {points.map(({ point, x, y }) => {
        const tone = point.tone ?? 'primary'
        return (
          <g key={point.label} className={`chaimir-chart__series is-${tone}`} tabIndex={0} role="listitem" aria-label={`${point.label}，${valueFormatter(point.value)}，${point.description ?? statusLabel[tone]}`}>
            <circle cx={x} cy={y} r="4" className="chaimir-chart__dot" />
            <text x={x} y={164} className="chaimir-chart__label">{point.label}</text>
          </g>
        )
      })}
    </>
  )
}

/**
 * ChartError 呈现图表加载失败状态，并在调用方提供时展示重试动作。
 */
function ChartError({ message, onRetry }: { message: string; onRetry?: () => void }): React.ReactElement {
  return (
    <div className="chaimir-chart__state is-error" role="alert">
      <AlertCircle aria-hidden="true" size={22} />
      <strong>图表暂时无法显示</strong>
      <p>{message}</p>
      {onRetry && (
        <button type="button" onClick={onRetry}>
          <RefreshCw aria-hidden="true" size={16} />
          重新加载
        </button>
      )}
    </div>
  )
}

/**
 * ChartState 呈现加载或空数据状态，避免图表区域出现空白。
 */
function ChartState({ tone, title, description }: { tone: 'loading' | 'empty'; title: string; description: string }): React.ReactElement {
  return (
    <div className={`chaimir-chart__state is-${tone}`} role="status" aria-live="polite">
      <BarChart3 aria-hidden="true" size={22} />
      <strong>{title}</strong>
      <p>{description}</p>
    </div>
  )
}

/**
 * defaultValueFormatter 使用中文数字格式化，避免图表默认输出原始数字。
 */
function defaultValueFormatter(value: number): string {
  return new Intl.NumberFormat('zh-CN').format(value)
}

/**
 * useStableId 去掉 React useId 中不适合拼接 SVG aria id 的分隔符。
 */
function useStableId(prefix: string): string {
  const id = React.useId()
  return `${prefix}-${id.replace(/:/g, '')}`
}
