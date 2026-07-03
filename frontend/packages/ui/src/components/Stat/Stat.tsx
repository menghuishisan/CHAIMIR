// Stat 组件：用于看板统计、成绩指标和运行状态数字展示。

import React from 'react'
import { clsx } from 'clsx'
import { ArrowDownRight, ArrowUpRight, Minus } from 'lucide-react'
import './Stat.css'

export interface StatProps extends React.HTMLAttributes<HTMLDivElement> {
  label: string
  value: React.ReactNode
  description?: string
  delta?: string
  trend?: 'up' | 'down' | 'flat'
  icon?: React.ReactNode
}

export function Stat({ label, value, description, delta, trend = 'flat', icon, className, ...props }: StatProps): React.ReactElement {
  return (
    <section className={clsx('chaimir-stat', className)} {...props}>
      <header>
        <span>{label}</span>
        {icon && <span className="chaimir-stat__icon">{icon}</span>}
      </header>
      <strong>{value}</strong>
      {(delta || description) && (
        <footer>
          {delta && (
            <span className={`chaimir-stat__delta is-${trend}`}>
              {trend === 'up' ? <ArrowUpRight size={14} /> : trend === 'down' ? <ArrowDownRight size={14} /> : <Minus size={14} />}
              {delta}
            </span>
          )}
          {description && <small>{description}</small>}
        </footer>
      )}
    </section>
  )
}
