// Tooltip 组件：为图标按钮、状态点和紧凑控件提供可访问提示。

import React, { useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { useFloating, offset, flip, shift, autoUpdate } from '@floating-ui/react'
import { clsx } from 'clsx'
import { useHoverPointer, useReducedMotion, useTransformOrigin } from '../../hooks'
import './Tooltip.css'

const POINTER_DELAY_MS = 300
let instantWindowEndsAt = 0

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
  const [motion, setMotion] = useState<'pointer' | 'instant' | 'keyboard'>('keyboard')
  const openTimer = useRef<number>()
  const canHover = useHoverPointer()
  const reducedMotion = useReducedMotion()
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
  const transformOrigin = useTransformOrigin(placement)

  useEffect(() => () => window.clearTimeout(openTimer.current), [])

  /** openFromPointer 对首个指针提示稍作延迟，相邻提示在宽限期内即时显示。 */
  const openFromPointer = (): void => {
    const instant = Date.now() <= instantWindowEndsAt
    setMotion(instant ? 'instant' : 'pointer')
    window.clearTimeout(openTimer.current)
    if (instant) {
      setIsOpen(true)
      return
    }
    openTimer.current = window.setTimeout(() => setIsOpen(true), POINTER_DELAY_MS)
  }

  /** closePointerTooltip 关闭当前提示并开启相邻提示即时窗口。 */
  const closePointerTooltip = (): void => {
    window.clearTimeout(openTimer.current)
    instantWindowEndsAt = Date.now() + POINTER_DELAY_MS
    setIsOpen(false)
  }

  return (
    <>
      {React.cloneElement(child, {
        ref: refs.setReference,
        'aria-describedby': [child.props['aria-describedby'], isOpen ? id : undefined].filter(Boolean).join(' ') || undefined,
        onMouseEnter: (e: React.MouseEvent) => {
          if (!canHover) return
          openFromPointer()
          child.props.onMouseEnter?.(e)
        },
        onMouseLeave: (e: React.MouseEvent) => {
          if (!canHover) return
          closePointerTooltip()
          child.props.onMouseLeave?.(e)
        },
        onFocus: (e: React.FocusEvent) => {
          window.clearTimeout(openTimer.current)
          setMotion('keyboard')
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
            data-motion={reducedMotion ? 'reduced' : motion}
            data-instant={motion === 'instant' ? '' : undefined}
            style={{
              ...floatingStyles,
              '--transform-origin': transformOrigin,
              visibility: isPositioned ? 'visible' : 'hidden',
              pointerEvents: 'none',
            } as React.CSSProperties}
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
