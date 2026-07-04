// Tabs 组件：共享标签页导航，提供键盘切换、选中语义和内容面板关联。

import React, { useId, useRef, useState } from 'react'
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
  /** 标签组说明，供读屏用户识别当前标签组 */
  ariaLabel?: string
  /** 标签页内容 */
  children?: React.ReactNode
  /** 自定义类名 */
  className?: string
}

export const Tabs: React.FC<TabsProps> = ({
  items,
  activeKey,
  defaultActiveKey,
  onChange,
  ariaLabel = '内容分类',
  children,
  className,
}) => {
  const [active, setActive] = useState(activeKey || defaultActiveKey || items[firstEnabledIndex(items)]?.key || '')
  const generatedId = useId()
  const tabRefs = useRef<Array<HTMLButtonElement | null>>([])

  React.useEffect(() => {
    if (activeKey !== undefined) {
      setActive(activeKey)
    }
  }, [activeKey])

  /**
   * handleClick 切换当前标签，受控场景由外部 activeKey 回写状态。
   */
  const handleClick = (key: string, disabled?: boolean) => {
    if (disabled) return
    setActive(key)
    onChange?.(key)
  }

  /**
   * handleKeyDown 支持左右方向键与 Home/End，符合 WAI-ARIA Tabs 键盘模型。
   */
  const handleKeyDown = (event: React.KeyboardEvent<HTMLButtonElement>, currentIndex: number) => {
    const nextIndex = resolveNextEnabledIndex(items, currentIndex, event.key)
    if (nextIndex === currentIndex) {
      return
    }
    event.preventDefault()
    const nextItem = items[nextIndex]
    tabRefs.current[nextIndex]?.focus()
    handleClick(nextItem.key, nextItem.disabled)
  }

  const classes = clsx('chaimir-tabs', className)
  const activePanelId = `${generatedId}-panel-${active}`

  return (
    <div className={classes}>
      <div className="chaimir-tabs__nav" role="tablist" aria-label={ariaLabel}>
        {items.map((item, index) => {
          const isActive = item.key === active
          const tabId = `${generatedId}-tab-${item.key}`
          const panelId = `${generatedId}-panel-${item.key}`
          const tabClasses = clsx(
            'chaimir-tabs__tab',
            isActive && 'chaimir-tabs__tab--active',
            item.disabled && 'chaimir-tabs__tab--disabled'
          )

          return (
            <button
              key={item.key}
              ref={(element) => {
                tabRefs.current[index] = element
              }}
              id={tabId}
              type="button"
              role="tab"
              aria-selected={isActive}
              aria-disabled={item.disabled}
              aria-controls={panelId}
              tabIndex={isActive ? 0 : -1}
              className={tabClasses}
              onClick={() => handleClick(item.key, item.disabled)}
              onKeyDown={(event) => handleKeyDown(event, index)}
              disabled={item.disabled}
            >
              {item.label}
            </button>
          )
        })}
      </div>
      {children && (
        <div id={activePanelId} className="chaimir-tabs__panel" role="tabpanel" aria-labelledby={`${generatedId}-tab-${active}`}>
          {children}
        </div>
      )}
    </div>
  )
}

Tabs.displayName = 'Tabs'

/**
 * resolveNextEnabledIndex 根据键盘输入定位下一个可用标签，跳过禁用项。
 */
function resolveNextEnabledIndex(items: TabItem[], currentIndex: number, key: string): number {
  if (items.length === 0 || !['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(key)) {
    return currentIndex
  }
  if (key === 'Home') {
    return firstEnabledIndex(items)
  }
  if (key === 'End') {
    return lastEnabledIndex(items)
  }
  const direction = key === 'ArrowRight' ? 1 : -1
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
 * firstEnabledIndex 返回第一个可切换标签的位置。
 */
function firstEnabledIndex(items: TabItem[]): number {
  const index = items.findIndex((item) => !item.disabled)
  return index >= 0 ? index : 0
}

/**
 * lastEnabledIndex 返回最后一个可切换标签的位置。
 */
function lastEnabledIndex(items: TabItem[]): number {
  for (let index = items.length - 1; index >= 0; index -= 1) {
    if (!items[index]?.disabled) {
      return index
    }
  }
  return 0
}
