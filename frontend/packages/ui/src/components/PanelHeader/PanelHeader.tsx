// PanelHeader 组件：统一卡片、抽屉、工作台面板和图表容器的标题区域。

import React from 'react'
import { clsx } from 'clsx'
import './PanelHeader.css'

export interface PanelHeaderProps extends Omit<React.HTMLAttributes<HTMLElement>, 'title'> {
  eyebrow?: React.ReactNode
  title: React.ReactNode
  description?: React.ReactNode
  meta?: React.ReactNode
  icon?: React.ReactNode
  actions?: React.ReactNode
  compact?: boolean
}

/**
 * PanelHeader 统一面板标题、说明、图标和操作区布局，避免业务面板重复搭结构。
 */
export function PanelHeader({
  eyebrow,
  title,
  description,
  meta,
  icon,
  actions,
  compact = false,
  className,
  ...props
}: PanelHeaderProps): React.ReactElement {
  return (
    <header className={clsx('chaimir-panel-header', compact && 'is-compact', className)} {...props}>
      <div className="chaimir-panel-header__main">
        {icon && <span className="chaimir-panel-header__icon" aria-hidden="true">{icon}</span>}
        <div className="chaimir-panel-header__copy">
          {eyebrow && <span className="chaimir-panel-header__eyebrow">{eyebrow}</span>}
          <div className="chaimir-panel-header__title-row">
            <h2>{title}</h2>
            {meta && <span className="chaimir-panel-header__meta">{meta}</span>}
          </div>
          {description && <p>{description}</p>}
        </div>
      </div>
      {actions && <div className="chaimir-panel-header__actions">{actions}</div>}
    </header>
  )
}
