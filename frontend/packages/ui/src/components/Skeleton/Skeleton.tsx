// Skeleton 组件：骨架屏
// 用于加载态空间预留，防止 CLS（累积布局偏移）

import React from 'react'
import { clsx } from 'clsx'
import './Skeleton.css'

export interface SkeletonProps extends React.HTMLAttributes<HTMLDivElement> {
  /** 变体 */
  variant?: 'text' | 'title' | 'block' | 'circle'
  /** 宽度 */
  width?: string | number
  /** 高度 */
  height?: string | number
  /** 是否显示动画 */
  animate?: boolean
}

export const Skeleton = React.forwardRef<HTMLDivElement, SkeletonProps>(
  (
    {
      variant = 'text',
      width,
      height,
      animate = true,
      className,
      style,
      ...props
    },
    ref
  ) => {
    const classes = clsx(
      'chaimir-skeleton',
      `chaimir-skeleton--${variant}`,
      animate && 'chaimir-skeleton--animate',
      className
    )

    const inlineStyle: React.CSSProperties = {
      ...style,
      width: typeof width === 'number' ? `${width}px` : width,
      height: typeof height === 'number' ? `${height}px` : height,
    }

    return (
      <div
        ref={ref}
        className={classes}
        style={inlineStyle}
        aria-busy="true"
        aria-live="polite"
        {...props}
      />
    )
  }
)

Skeleton.displayName = 'Skeleton'
