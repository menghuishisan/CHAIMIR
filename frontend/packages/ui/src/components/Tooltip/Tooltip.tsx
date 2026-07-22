// Tooltip 组件：为图标按钮、状态点和紧凑控件提供可访问提示。

import React, { useState } from 'react'
import { createPortal } from 'react-dom'
import { useFloating, offset, flip, shift, autoUpdate } from '@floating-ui/react'
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

/**
 * Tooltip 在悬停和键盘聚焦时通过 portal 展示短提示，不占用原布局空间。
 */
export function Tooltip({ children, content, side = 'top', className }: TooltipProps): React.ReactElement {
  const [isOpen, setIsOpen] = useState(false)
  const id = React.useId()
  const child = React.Children.only(children)

  const { refs, floatingStyles, placement, isPositioned } = useFloating({
    open: isOpen,
    onOpenChange: setIsOpen,
    placement: side,
    whileElementsMounted: autoUpdate,
    middleware: [
      offset(8),
      flip({ fallbackAxisSideDirection: 'start' }),
      shift({ padding: 8 }),
    ],
  })

  return (
    <>
      {React.cloneElement(child, {
        ref: refs.setReference,
        'aria-describedby': [child.props['aria-describedby'], isOpen ? id : undefined].filter(Boolean).join(' ') || undefined,
        onMouseEnter: (e: React.MouseEvent) => {
          setIsOpen(true)
          child.props.onMouseEnter?.(e)
        },
        onMouseLeave: (e: React.MouseEvent) => {
          setIsOpen(false)
          child.props.onMouseLeave?.(e)
        },
        onFocus: (e: React.FocusEvent) => {
          setIsOpen(true)
          child.props.onFocus?.(e)
        },
        onBlur: (e: React.FocusEvent) => {
          setIsOpen(false)
          child.props.onBlur?.(e)
        },
      })}
      {isOpen &&
        typeof document !== 'undefined' &&
        createPortal(
          <span
            ref={refs.setFloating}
            className="chaimir-tooltip__positioner"
            data-placement={placement}
            style={{
              ...floatingStyles,
              visibility: isPositioned ? 'visible' : 'hidden',
              pointerEvents: 'none',
            }}
          >
            <span id={id} role="tooltip" className={clsx('chaimir-tooltip__content', className)}>
              {content}
            </span>
          </span>,
          document.body
        )}
    </>
  )
}
