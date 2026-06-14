// Tabs 组件：标签页

import React, { useState } from 'react'
import { clsx } from 'clsx'
import './Tabs.css'

export interface TabItem {
  key: string
  label: React.ReactNode
  disabled?: boolean
}

export interface TabsProps {
  /** 标签项 */
  items: TabItem[]
  /** 当前激活的 key */
  activeKey?: string
  /** 默认激活的 key */
  defaultActiveKey?: string
  /** 变化回调 */
  onChange?: (key: string) => void
  /** 自定义类名 */
  className?: string
}

export const Tabs: React.FC<TabsProps> = ({
  items,
  activeKey,
  defaultActiveKey,
  onChange,
  className,
}) => {
  const [active, setActive] = useState(activeKey || defaultActiveKey || items[0]?.key || '')

  React.useEffect(() => {
    if (activeKey !== undefined) {
      setActive(activeKey)
    }
  }, [activeKey])

  const handleClick = (key: string, disabled?: boolean) => {
    if (disabled) return
    setActive(key)
    onChange?.(key)
  }

  const classes = clsx('chaimir-tabs', className)

  return (
    <div className={classes}>
      <div className="chaimir-tabs__nav" role="tablist">
        {items.map((item) => {
          const isActive = item.key === active
          const tabClasses = clsx(
            'chaimir-tabs__tab',
            isActive && 'chaimir-tabs__tab--active',
            item.disabled && 'chaimir-tabs__tab--disabled'
          )

          return (
            <button
              key={item.key}
              type="button"
              role="tab"
              aria-selected={isActive}
              aria-disabled={item.disabled}
              className={tabClasses}
              onClick={() => handleClick(item.key, item.disabled)}
              disabled={item.disabled}
            >
              {item.label}
            </button>
          )
        })}
        <span className="chaimir-tabs__indicator" />
      </div>
    </div>
  )
}

Tabs.displayName = 'Tabs'
