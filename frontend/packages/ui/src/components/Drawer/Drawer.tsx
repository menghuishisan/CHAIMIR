// Drawer 组件：侧栏抽屉和窄屏导航容器，提供 Esc 关闭和无障碍标题。

import React, { useEffect, useRef } from 'react'
import { clsx } from 'clsx'
import { X } from 'lucide-react'
import { Button } from '../Button'
import './Drawer.css'

export interface DrawerProps extends React.HTMLAttributes<HTMLDivElement> {
  open: boolean
  title: string
  side?: 'left' | 'right'
  onClose: () => void
  children?: React.ReactNode
}

export function Drawer({ open, title, side = 'right', onClose, children, className, ...props }: DrawerProps): React.ReactElement | null {
  const panelRef = useRef<HTMLElement>(null)
  const previousActiveElement = useRef<HTMLElement | null>(null)

  useEffect(() => {
    if (!open) {
      previousActiveElement.current?.focus()
      document.body.style.overflow = ''
      return
    }

    previousActiveElement.current = document.activeElement as HTMLElement
    document.body.style.overflow = 'hidden'
    panelRef.current?.querySelector<HTMLElement>('button, a[href], input, select, textarea, [tabindex]:not([tabindex="-1"])')?.focus()

    return () => {
      document.body.style.overflow = ''
    }
  }, [open])

  useEffect(() => {
    if (!open) return
    function handleKeyDown(event: KeyboardEvent): void {
      if (event.key === 'Escape') {
        onClose()
        return
      }
      if (event.key === 'Tab') {
        trapFocus(event, panelRef.current)
      }
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [open, onClose])

  if (!open) {
    return null
  }

  return (
    <div className="chaimir-drawer" role="presentation">
      <button className="chaimir-drawer__scrim" type="button" aria-label="关闭抽屉" onClick={onClose} />
      <aside ref={panelRef} className={clsx('chaimir-drawer__panel', `is-${side}`, className)} role="dialog" aria-modal="true" aria-labelledby="chaimir-drawer-title" tabIndex={-1} {...props}>
        <header className="chaimir-drawer__header">
          <h2 id="chaimir-drawer-title">{title}</h2>
          <Button variant="ghost" size="sm" icon={<X size={16} />} aria-label="关闭抽屉" onClick={onClose} />
        </header>
        <div className="chaimir-drawer__body">{children}</div>
      </aside>
    </div>
  )
}

/**
 * trapFocus 让抽屉打开时键盘焦点保持在抽屉内部。
 */
function trapFocus(event: KeyboardEvent, container: HTMLElement | null): void {
  if (!container) {
    return
  }
  const focusable = getFocusableElements(container)
  if (focusable.length === 0) {
    event.preventDefault()
    container.focus()
    return
  }

  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault()
    last.focus()
    return
  }
  if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault()
    first.focus()
  }
}

/**
 * getFocusableElements 返回当前抽屉内可见可操作的元素。
 */
function getFocusableElements(container: HTMLElement): HTMLElement[] {
  const selector = [
    'a[href]',
    'button:not([disabled])',
    'input:not([disabled])',
    'select:not([disabled])',
    'textarea:not([disabled])',
    '[tabindex]:not([tabindex="-1"])',
  ].join(',')
  return Array.from(container.querySelectorAll<HTMLElement>(selector)).filter((element) => element.offsetParent !== null)
}
