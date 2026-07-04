// Badge 组件：状态标签/徽章
// 用于：状态指示、计数角标、标签分类

import React from 'react'
import { clsx } from 'clsx'
import './Badge.css'

export interface BadgeProps extends React.HTMLAttributes<HTMLSpanElement> {
  /** 语义变体 */
  variant?: 'default' | 'primary' | 'secondary' | 'success' | 'warning' | 'danger' | 'info' | 'purple'
  /** 尺寸 */
  size?: 'sm' | 'md'
  /** 是否为圆点（无文字） */
  dot?: boolean
  /** 子元素 */
  children?: React.ReactNode
}

export const Badge = React.forwardRef<HTMLSpanElement, BadgeProps>(
  (
    {
      variant = 'default',
      size = 'md',
      dot = false,
      children,
      className,
      ...props
    },
    ref
  ) => {
    const ariaLabel = props['aria-label']
    const classes = clsx(
      'chaimir-badge',
      `chaimir-badge--${variant}`,
      `chaimir-badge--${size}`,
      dot && 'chaimir-badge--dot',
      className
    )

    // FE-2: 状态点必配文字（颜色非唯一信息）
    // 纯圆点提供默认 aria-label，避免状态只靠颜色表达。
    return (
      <span ref={ref} className={classes} role="status" {...props} aria-label={dot ? ariaLabel ?? '状态' : ariaLabel}>
        {!dot && children}
      </span>
    )
  }
)

Badge.displayName = 'Badge'
