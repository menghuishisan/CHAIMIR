// Drawer 组件：侧栏抽屉和窄屏导航容器，提供 Esc 关闭和无障碍标题。

import React, { useEffect, useId, useRef } from 'react'
import { createPortal } from 'react-dom'
import { clsx } from 'clsx'
import { X } from 'lucide-react'
import { Button } from '../Button'
import { useDelayedUnmount, useEscapeKey, useFocusTrap, useMediaQuery } from '../../hooks'
import { breakpoints, motionDurationMs } from '../../tokens'
import './Drawer.css'

export interface DrawerProps extends React.HTMLAttributes<HTMLDivElement> {
  open: boolean
  title: string
  side?: 'left' | 'right'
  onClose: () => void
  children?: React.ReactNode
}

/**
 * Drawer 通过 portal 渲染侧向浮层，统一遮罩关闭、Esc 关闭、焦点陷阱和触屏滑动关闭。
 */
export function Drawer({ open, title, side = 'right', onClose, children, className, ...props }: DrawerProps): React.ReactElement | null {
  const panelRef = useRef<HTMLElement>(null)
  const titleId = useId()
  const touchStart = useRef({ x: 0, y: 0 })
  const currentOffset = useRef(0)
  const [swipeStyle, setSwipeStyle] = React.useState<React.CSSProperties>({})
  const compact = useMediaQuery(`(max-width: ${breakpoints.md - 1}px)`)
  const presence = useDelayedUnmount(open, motionDurationMs.drawerExit)

  // 快捷键与焦点陷阱
  useFocusTrap(panelRef as React.RefObject<HTMLElement>, open && presence.mounted)
  useEscapeKey(() => {
    if (open) onClose()
  }, open)

  useEffect(() => {
    if (!presence.mounted) {
      document.body.style.overflow = ''
      return
    }

    document.body.style.overflow = 'hidden'

    return () => {
      document.body.style.overflow = ''
    }
  }, [presence.mounted])

  useEffect(() => {
    const panel = panelRef.current
    if (!panel) return
    if (open) panel.removeAttribute('inert')
    else panel.setAttribute('inert', '')
  }, [open, presence.mounted])

  if (!presence.mounted) {
    return null
  }

  const handleTouchStart = (e: React.TouchEvent) => {
    touchStart.current = { x: e.touches[0].clientX, y: e.touches[0].clientY }
  }

  const handleTouchMove = (e: React.TouchEvent) => {
    const deltaY = e.touches[0].clientY - touchStart.current.y
    if (compact && deltaY > 0) {
      currentOffset.current = deltaY
      setSwipeStyle({ transform: `translateY(${deltaY}px)`, transition: 'none' })
      return
    }
    const deltaX = e.touches[0].clientX - touchStart.current.x
    if ((side === 'right' && deltaX > 0) || (side === 'left' && deltaX < 0)) {
      currentOffset.current = deltaX
      setSwipeStyle({ transform: `translateX(${deltaX}px)`, transition: 'none' })
    }
  }

  const handleTouchEnd = () => {
    if (Math.abs(currentOffset.current) > 80) {
      setSwipeStyle({})
      onClose()
    } else {
      setSwipeStyle({
        transform: compact ? 'translateY(0)' : 'translateX(0)',
        transition: 'transform var(--t-slow) var(--ease-drawer)',
      })
    }
    currentOffset.current = 0
  }

  const content = (
    <div
      className="chaimir-drawer"
      role="presentation"
      data-state={presence.state}
      aria-hidden={!open || undefined}
    >
      <button className="chaimir-drawer__scrim" type="button" aria-label="关闭抽屉" onClick={onClose} />
      <aside
        ref={panelRef}
        className={clsx('chaimir-drawer__panel', `is-${side}`, className)}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        tabIndex={-1}
        style={swipeStyle}
        onTouchStart={handleTouchStart}
        onTouchMove={handleTouchMove}
        onTouchEnd={handleTouchEnd}
        {...props}
      >
        <header className="chaimir-drawer__header">
          <h2 id={titleId}>{title}</h2>
          <Button variant="ghost" size="sm" icon={<X size={16} />} aria-label="关闭抽屉" onClick={onClose} />
        </header>
        <div className="chaimir-drawer__body">{children}</div>
      </aside>
    </div>
  )

  return typeof document !== 'undefined' ? createPortal(content, document.body) : null
}
