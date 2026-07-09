// Menu 组件：下拉菜单基础结构，供顶栏头像、更多操作和表格行操作复用。

import React from 'react'
import { clsx } from 'clsx'
import { triggerHaptic } from '../../utils/haptics'
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

/**
 * Menu 提供键盘可达的垂直操作列表，支持上下键移动和 Enter/Space 触发。
 */
export function Menu({ items, label = '操作菜单', className, ...props }: MenuProps): React.ReactElement {
  const itemRefs = React.useRef<Array<HTMLButtonElement | null>>([])
  const firstEnabled = firstEnabledIndex(items)

  /**
   * handleKeyDown 支持菜单方向键导航，避免菜单项只能通过 Tab 逐个进入。
   */
  const handleKeyDown = (event: React.KeyboardEvent<HTMLDivElement>) => {
    const currentIndex = itemRefs.current.findIndex((item) => item === document.activeElement)
    const nextIndex = resolveMenuIndex(items, currentIndex >= 0 ? currentIndex : firstEnabled, event.key)
    if (nextIndex === currentIndex || nextIndex < 0) {
      return
    }
    event.preventDefault()
    itemRefs.current[nextIndex]?.focus()
  }

  return (
    <div className={clsx('chaimir-menu', className)} role="menu" aria-label={label} onKeyDown={handleKeyDown} {...props}>
      {items.map((item, index) => (
        <button
          key={item.id}
          ref={(element) => {
            itemRefs.current[index] = element
          }}
          type="button"
          role="menuitem"
          disabled={item.disabled}
          aria-disabled={item.disabled || undefined}
          tabIndex={index === firstEnabled ? 0 : -1}
          className={clsx('chaimir-menu__item', item.danger && 'is-danger')}
          onClick={() => {
            triggerHaptic(10)
            item.onSelect?.()
          }}
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

/**
 * resolveMenuIndex 根据键盘输入计算下一个可用菜单项。
 */
function resolveMenuIndex(items: MenuItem[], currentIndex: number, key: string): number {
  if (key === 'Home') {
    return firstEnabledIndex(items)
  }
  if (key === 'End') {
    return lastEnabledIndex(items)
  }
  if (key !== 'ArrowDown' && key !== 'ArrowUp') {
    return currentIndex
  }
  const direction = key === 'ArrowDown' ? 1 : -1
  let nextIndex = currentIndex
  for (let count = 0; count < items.length; count += 1) {
    nextIndex = (nextIndex + direction + items.length) % items.length
    if (!items[nextIndex]?.disabled) {
      return nextIndex
    }
  }
  return currentIndex
}

/**
 * firstEnabledIndex 返回第一个可操作菜单项。
 */
function firstEnabledIndex(items: MenuItem[]): number {
  const index = items.findIndex((item) => !item.disabled)
  return index >= 0 ? index : -1
}

/**
 * lastEnabledIndex 返回最后一个可操作菜单项。
 */
function lastEnabledIndex(items: MenuItem[]): number {
  for (let index = items.length - 1; index >= 0; index -= 1) {
    if (!items[index]?.disabled) {
      return index
    }
  }
  return -1
}
