// Card 组件：内容分组、列表项和仪表盘面板的共享容器。

import React from 'react'
import { clsx } from 'clsx'
import './Card.css'

export interface CardProps extends React.HTMLAttributes<HTMLDivElement> {
  /** 是否可悬浮（可点击卡片） */
  hoverable?: boolean
  /** 是否有内边距 */
  padding?: boolean
  /** 子元素 */
  children?: React.ReactNode
}

export const Card = React.forwardRef<HTMLDivElement, CardProps>(
  (
    {
      hoverable = false,
      padding = true,
      children,
      className,
      ...props
    },
    ref
  ) => {
    const classes = clsx(
      'chaimir-card',
      hoverable && 'chaimir-card--hoverable',
      padding && 'chaimir-card--padded',
      className
    )

    return (
      <div ref={ref} className={classes} {...props}>
        {children}
      </div>
    )
  }
)

Card.displayName = 'Card'

export interface CardHeaderProps extends React.HTMLAttributes<HTMLDivElement> {
  children?: React.ReactNode
}

/**
 * CardHeader 渲染卡片标题区域，保持卡片头部间距和分隔线一致。
 */
export const CardHeader = React.forwardRef<HTMLDivElement, CardHeaderProps>(
  ({ children, className, ...props }, ref) => {
    const classes = clsx('chaimir-card__header', className)
    return (
      <div ref={ref} className={classes} {...props}>
        {children}
      </div>
    )
  }
)

CardHeader.displayName = 'CardHeader'

export interface CardBodyProps extends React.HTMLAttributes<HTMLDivElement> {
  children?: React.ReactNode
}

/**
 * CardBody 渲染卡片主体内容，不额外限制内容布局。
 */
export const CardBody = React.forwardRef<HTMLDivElement, CardBodyProps>(
  ({ children, className, ...props }, ref) => {
    const classes = clsx('chaimir-card__body', className)
    return (
      <div ref={ref} className={classes} {...props}>
        {children}
      </div>
    )
  }
)

CardBody.displayName = 'CardBody'

export interface CardFooterProps extends React.HTMLAttributes<HTMLDivElement> {
  children?: React.ReactNode
}

/**
 * CardFooter 渲染卡片底部动作区，统一上分隔线与按钮间距。
 */
export const CardFooter = React.forwardRef<HTMLDivElement, CardFooterProps>(
  ({ children, className, ...props }, ref) => {
    const classes = clsx('chaimir-card__footer', className)
    return (
      <div ref={ref} className={classes} {...props}>
        {children}
      </div>
    )
  }
)

CardFooter.displayName = 'CardFooter'
