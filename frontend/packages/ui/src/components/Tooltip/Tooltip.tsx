// Tooltip 组件：为图标按钮、状态点和紧凑控件提供可访问提示。

import React from 'react'
import { clsx } from 'clsx'
import './Tooltip.css'

export interface TooltipProps {
  /** 被解释的控件。 */
  children: React.ReactElement
  /** 提示内容。 */
  content: React.ReactNode
  /** 提示出现位置。 */
  side?: 'top' | 'right' | 'bottom' | 'left'
  /** 自定义类名。 */
  className?: string
}

export function Tooltip({ children, content, side = 'top', className }: TooltipProps): React.ReactElement {
  const id = React.useId()
  const child = React.Children.only(children)

  return (
    <span className={clsx('chaimir-tooltip', `chaimir-tooltip--${side}`, className)}>
      {React.cloneElement(child, {
        'aria-describedby': [child.props['aria-describedby'], id].filter(Boolean).join(' ') || id,
      })}
      <span id={id} role="tooltip" className="chaimir-tooltip__content">
        {content}
      </span>
    </span>
  )
}
