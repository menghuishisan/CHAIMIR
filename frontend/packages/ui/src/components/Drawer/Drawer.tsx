// Drawer 组件：侧栏抽屉和窄屏导航容器，提供 Esc 关闭和无障碍标题。

import React, { useEffect, useId, useRef } from 'react'
import { createPortal } from 'react-dom'
import { clsx } from 'clsx'
import { X } from 'lucide-react'
import { Button } from '../Button'
import { useEscapeKey, useFocusTrap } from '../../hooks'
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
  const touchStartX = useRef(0)
  const currentTranslateX = useRef(0)
  const [swipeStyle, setSwipeStyle] = React.useState<React.CSSProperties>({})

  // 快捷键与焦点陷阱
  useFocusTrap(panelRef as React.RefObject<HTMLElement>, open)
  useEscapeKey(() => {
    if (open) onClose()
  }, open)

  useEffect(() => {
    if (!open) {
      document.body.style.overflow = ''
      return
    }

    document.body.style.overflow = 'hidden'

    return () => {
      document.body.style.overflow = ''
      setSwipeStyle({})
    }
  }, [open])

  if (!open) {
    return null
  }

  const handleTouchStart = (e: React.TouchEvent) => {
    touchStartX.current = e.touches[0].clientX
  }

  const handleTouchMove = (e: React.TouchEvent) => {
    const deltaX = e.touches[0].clientX - touchStartX.current
    if ((side === 'right' && deltaX > 0) || (side === 'left' && deltaX < 0)) {
      currentTranslateX.current = deltaX
      setSwipeStyle({ transform: `translateX(${deltaX}px)`, transition: 'none' })
    }
  }

  const handleTouchEnd = () => {
    if (Math.abs(currentTranslateX.current) > 80) {
      onClose()
    } else {
      setSwipeStyle({ transform: `translateX(0)`, transition: 'transform 0.4s var(--ease-spring)' })
    }
    currentTranslateX.current = 0
  }

  const content = (
    <div className="chaimir-drawer" role="presentation">
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
