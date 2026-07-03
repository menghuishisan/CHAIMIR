// AccessibleChart：轻量 SVG 图表封装，默认提供表格化数据摘要作为无障碍降级。

import React from 'react'
import { clsx } from 'clsx'
import './AccessibleChart.css'

export type AccessibleChartKind = 'bar' | 'line'

export interface ChartPoint {
  label: string
  value: number
}

export interface AccessibleChartProps extends React.HTMLAttributes<HTMLElement> {
  title: string
  summary: string
  data: ChartPoint[]
  kind?: AccessibleChartKind
}

export function AccessibleChart({ title, summary, data, kind = 'bar', className, ...props }: AccessibleChartProps): React.ReactElement {
  const max = Math.max(1, ...data.map((point) => point.value))
  return (
    <section className={clsx('chaimir-chart', className)} aria-label={title} {...props}>
      <header>
        <h3>{title}</h3>
        <p>{summary}</p>
      </header>
      <svg viewBox="0 0 320 160" role="img" aria-label={summary} preserveAspectRatio="none">
        {kind === 'line' ? renderLine(data, max) : renderBars(data, max)}
      </svg>
      <details className="chaimir-chart__table">
        <summary>查看数据表</summary>
        <table>
          <thead>
            <tr>
              <th scope="col">项目</th>
              <th scope="col">数值</th>
            </tr>
          </thead>
          <tbody>
            {data.map((point) => (
              <tr key={point.label}>
                <td>{point.label}</td>
                <td>{point.value}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </details>
    </section>
  )
}

function renderBars(data: ChartPoint[], max: number): React.ReactNode {
  const gap = 8
  const width = Math.max(12, (300 - gap * Math.max(0, data.length - 1)) / Math.max(1, data.length))
  return data.map((point, index) => {
    const height = Math.max(4, (point.value / max) * 120)
    const x = 10 + index * (width + gap)
    return <rect key={point.label} x={x} y={138 - height} width={width} height={height} rx="4" className="chaimir-chart__bar" />
  })
}

function renderLine(data: ChartPoint[], max: number): React.ReactNode {
  const points = data.map((point, index) => {
    const x = 10 + (index / Math.max(1, data.length - 1)) * 300
    const y = 138 - (point.value / max) * 120
    return `${x},${y}`
  })
  return (
    <>
      <polyline points={points.join(' ')} className="chaimir-chart__line" fill="none" />
      {points.map((point, index) => {
        const [x, y] = point.split(',').map(Number)
        return <circle key={data[index].label} cx={x} cy={y} r="4" className="chaimir-chart__dot" />
      })}
    </>
  )
}
