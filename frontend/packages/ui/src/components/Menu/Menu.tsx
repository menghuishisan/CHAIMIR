// Menu 组件：下拉菜单基础结构，供顶栏头像、更多操作和表格行操作复用。

import React from 'react'
import { clsx } from 'clsx'
import './Menu.css'

export interface MenuItem {
  id: string
  label: string
  description?: string
  icon?: React.ReactNode
  danger?: boolean
  disabled?: boolean
  onSelect?: () => void
}

export interface MenuProps extends React.HTMLAttributes<HTMLDivElement> {
  items: MenuItem[]
  label?: string
}

export function Menu({ items, label = '操作菜单', className, ...props }: MenuProps): React.ReactElement {
  return (
    <div className={clsx('chaimir-menu', className)} role="menu" aria-label={label} {...props}>
      {items.map((item) => (
        <button
          key={item.id}
          type="button"
          role="menuitem"
          disabled={item.disabled}
          className={clsx('chaimir-menu__item', item.danger && 'is-danger')}
          onClick={item.onSelect}
        >
          {item.icon && <span aria-hidden="true">{item.icon}</span>}
          <span>
            <strong>{item.label}</strong>
            {item.description && <small>{item.description}</small>}
          </span>
        </button>
      ))}
    </div>
  )
}
