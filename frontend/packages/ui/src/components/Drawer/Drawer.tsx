// Drawer 组件：侧栏抽屉和窄屏导航容器，提供 Esc 关闭和无障碍标题。

import React, { useEffect } from 'react'
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
  useEffect(() => {
    if (!open) return
    function handleKeyDown(event: KeyboardEvent): void {
      if (event.key === 'Escape') {
        onClose()
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
      <aside className={clsx('chaimir-drawer__panel', `is-${side}`, className)} role="dialog" aria-modal="true" aria-labelledby="chaimir-drawer-title" {...props}>
        <header className="chaimir-drawer__header">
          <h2 id="chaimir-drawer-title">{title}</h2>
          <Button variant="ghost" size="sm" icon={<X size={16} />} aria-label="关闭抽屉" onClick={onClose} />
        </header>
        <div className="chaimir-drawer__body">{children}</div>
      </aside>
    </div>
  )
}
