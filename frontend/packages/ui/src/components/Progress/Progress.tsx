// Progress 组件：进度条

import React from 'react'
import { clsx } from 'clsx'
import './Progress.css'

export interface ProgressProps {
  /** 进度值 (0-100) */
  percent: number
  /** 颜色变体 */
  variant?: 'primary' | 'success' | 'warning' | 'danger'
  /** 尺寸 */
  size?: 'sm' | 'md' | 'lg'
  /** 是否显示进度文字 */
  showPercent?: boolean
  /** 是否显示条纹动画 */
  striped?: boolean
  /** 自定义类名 */
  className?: string
}

export const Progress = React.forwardRef<HTMLDivElement, ProgressProps>(
  (
    {
      percent,
      variant = 'primary',
      size = 'md',
      showPercent = true,
      striped = false,
      className,
    },
    ref
  ) => {
    const safePercent = Math.min(100, Math.max(0, percent))

    const classes = clsx(
      'chaimir-progress',
      `chaimir-progress--${size}`,
      className
    )

    const barClasses = clsx(
      'chaimir-progress__bar',
      `chaimir-progress__bar--${variant}`,
      striped && 'chaimir-progress__bar--striped'
    )

    return (
      <div ref={ref} className={classes} role="progressbar" aria-valuenow={safePercent} aria-valuemin={0} aria-valuemax={100}>
        <div className="chaimir-progress__track">
          <div
            className={barClasses}
            style={{ width: `${safePercent}%` }}
          />
        </div>
        {showPercent && (
          <span className="chaimir-progress__text">{safePercent}%</span>
        )}
      </div>
    )
  }
)

Progress.displayName = 'Progress'
