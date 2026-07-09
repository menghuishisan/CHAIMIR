// StatusIndicator 组件：统一状态点、图标和用户向状态文字，避免只靠颜色表达。

import React from 'react'
import { AlertTriangle, CheckCircle2, Clock3, Info, Loader2, XCircle } from 'lucide-react'
import { clsx } from 'clsx'
import './StatusIndicator.css'

export type StatusIndicatorTone = 'neutral' | 'primary' | 'success' | 'warning' | 'danger' | 'info'
export type StatusIndicatorEmphasis = 'soft' | 'solid' | 'text'

export interface StatusIndicatorProps extends React.HTMLAttributes<HTMLSpanElement> {
  /** 状态文字。 */
  label: React.ReactNode
  /** 状态语义色。 */
  tone?: StatusIndicatorTone
  /** 展示强度。 */
  emphasis?: StatusIndicatorEmphasis
  /** 是否展示图标。 */
  icon?: boolean
  /** 是否为进行中的状态。 */
  pulse?: boolean
}

/**
 * StatusIndicator 以图标和文字共同表达状态，避免只依赖颜色传达含义。
 */
export function StatusIndicator({
  label,
  tone = 'neutral',
  emphasis = 'soft',
  icon = true,
  pulse = false,
  className,
  ...props
}: StatusIndicatorProps): React.ReactElement {
  return (
    <span
      className={clsx(
        'chaimir-status-indicator',
        `chaimir-status-indicator--${tone}`,
        `chaimir-status-indicator--${emphasis}`,
        pulse && 'is-pulsing',
        className
      )}
      role="status"
      {...props}
    >
      {icon ? <span className="chaimir-status-indicator__icon" aria-hidden="true">{statusIcon(tone, pulse)}</span> : <span className="chaimir-status-indicator__dot" aria-hidden="true" />}
      <span className="chaimir-status-indicator__label">{label}</span>
    </span>
  )
}

/**
 * statusIcon 按语义返回 Lucide 图标；进行中状态使用 Loader2。
 */
function statusIcon(tone: StatusIndicatorTone, pulse: boolean): React.ReactElement {
  if (pulse) return <Loader2 size={14} className="chaimir-status-indicator__spinner" />
  if (tone === 'success') return <CheckCircle2 size={14} />
  if (tone === 'warning') return <AlertTriangle size={14} />
  if (tone === 'danger') return <XCircle size={14} />
  if (tone === 'info') return <Info size={14} />
  return <Clock3 size={14} />
}
