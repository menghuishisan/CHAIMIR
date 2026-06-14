// Card 组件：卡片容器
// 用于：内容分组、列表项、仪表盘面板

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

// CardHeader 子组件
export interface CardHeaderProps extends React.HTMLAttributes<HTMLDivElement> {
  children?: React.ReactNode
}

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

// CardBody 子组件
export interface CardBodyProps extends React.HTMLAttributes<HTMLDivElement> {
  children?: React.ReactNode
}

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

// CardFooter 子组件
export interface CardFooterProps extends React.HTMLAttributes<HTMLDivElement> {
  children?: React.ReactNode
}

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
