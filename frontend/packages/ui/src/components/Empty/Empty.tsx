// Empty 组件：空状态
// 符合前端设计规范 §5.1：图标 + 标题 + 引导 + 行动按钮

import React from 'react'
import { clsx } from 'clsx'
import './Empty.css'

export interface EmptyProps {
  /** 图标（使用 Lucide） */
  icon?: React.ReactNode
  /** 标题 */
  title?: string
  /** 描述 */
  description?: string
  /** 行动按钮 */
  action?: React.ReactNode
  /** 自定义类名 */
  className?: string
}

export const Empty: React.FC<EmptyProps> = ({
  icon,
  title = '暂无数据',
  description,
  action,
  className,
}) => {
  const classes = clsx('chaimir-empty', className)

  return (
    <div className={classes}>
      {icon && (
        <div className="chaimir-empty__icon" aria-hidden="true">
          {icon}
        </div>
      )}
      <div className="chaimir-empty__title">{title}</div>
      {description && (
        <div className="chaimir-empty__description">{description}</div>
      )}
      {action && (
        <div className="chaimir-empty__action">{action}</div>
      )}
    </div>
  )
}

Empty.displayName = 'Empty'
