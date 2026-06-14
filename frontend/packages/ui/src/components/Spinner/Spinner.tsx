// Spinner 组件：加载指示器

import React from 'react'
import { Loader2 } from 'lucide-react'
import { clsx } from 'clsx'
import './Spinner.css'

export interface SpinnerProps extends React.HTMLAttributes<HTMLDivElement> {
  /** 尺寸 */
  size?: 'sm' | 'md' | 'lg'
  /** 颜色变体 */
  variant?: 'primary' | 'secondary' | 'white'
  /** 辅助文字 */
  label?: string
}

export const Spinner = React.forwardRef<HTMLDivElement, SpinnerProps>(
  (
    {
      size = 'md',
      variant = 'primary',
      label,
      className,
      ...props
    },
    ref
  ) => {
    const iconSize = {
      sm: 16,
      md: 24,
      lg: 32,
    }[size]

    const classes = clsx(
      'chaimir-spinner',
      `chaimir-spinner--${variant}`,
      className
    )

    return (
      <div
        ref={ref}
        className={classes}
        role="status"
        aria-live="polite"
        aria-label={label || '加载中'}
        {...props}
      >
        <Loader2 size={iconSize} className="chaimir-spinner__icon" aria-hidden="true" />
        {label && <span className="chaimir-spinner__label">{label}</span>}
      </div>
    )
  }
)

Spinner.displayName = 'Spinner'
